package qbittorrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"javboss/internal/download/downloader"
)

const requestTimeout = 15 * time.Second

type Client struct {
	base     string
	username string
	password string
	http     *http.Client
	loggedIn bool
}

// New builds a qBittorrent Web API client. The address may omit the scheme.
func New(address, username, password string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = "http://127.0.0.1:8080"
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		base:     strings.TrimRight(address, "/"),
		username: strings.TrimSpace(username),
		password: password,
		http:     &http.Client{Timeout: requestTimeout, Jar: jar},
	}, nil
}

func (c *Client) do(ctx context.Context, method, endpoint string, form url.Values) (*http.Response, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base+endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Referer", c.base)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.http.Do(request)
}

func (c *Client) login(ctx context.Context) error {
	if c.loggedIn {
		return nil
	}
	if c.username == "" {
		return errors.New("qBittorrent requires a username")
	}
	response, err := c.do(ctx, http.MethodPost, "/api/v2/auth/login", url.Values{
		"username": {c.username},
		"password": {c.password},
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "Ok." {
		return errors.New("qBittorrent login failed")
	}
	c.loggedIn = true
	return nil
}

// get performs an authenticated GET, retrying once after a fresh login.
func (c *Client) get(ctx context.Context, endpoint string) ([]byte, int, error) {
	for attempt := 0; attempt < 2; attempt++ {
		response, err := c.do(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, 0, readErr
		}
		if response.StatusCode == http.StatusForbidden && attempt == 0 {
			c.loggedIn = false
			if loginErr := c.login(ctx); loginErr != nil {
				return nil, response.StatusCode, loginErr
			}
			continue
		}
		return body, response.StatusCode, nil
	}
	return nil, http.StatusForbidden, errors.New("qBittorrent authentication failed")
}

func (c *Client) Test(ctx context.Context) (*downloader.TestResult, error) {
	body, status, err := c.get(ctx, "/api/v2/app/version")
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, errors.New("qBittorrent requires login")
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("qBittorrent returned HTTP %d", status)
	}
	details := map[string]any{}
	if version := strings.TrimSpace(string(body)); version != "" {
		details["version"] = version
	}
	return &downloader.TestResult{Provider: "qbittorrent", Details: details}, nil
}

func (c *Client) Add(ctx context.Context, magnet, directory string) (string, error) {
	hash, err := hashFromMagnet(magnet)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"urls":          {magnet},
		"paused":        {"false"},
		"autoTMM":       {"false"},
		"contentLayout": {"Original"},
	}
	if strings.TrimSpace(directory) != "" {
		form.Set("savepath", directory)
	}
	response, err := c.do(ctx, http.MethodPost, "/api/v2/torrents/add", form)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode == http.StatusForbidden {
		if err := c.login(ctx); err != nil {
			return "", err
		}
		return c.Add(ctx, magnet, directory)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(string(body)), "Ok") {
		return "", fmt.Errorf("qBittorrent rejected the torrent: %s", strings.TrimSpace(string(body)))
	}
	return hash, nil
}

type torrentInfo struct {
	Hash      string  `json:"hash"`
	State     string  `json:"state"`
	Progress  float64 `json:"progress"`
	Size      int64   `json:"size"`
	Completed int64   `json:"completed"`
	DlSpeed   int64   `json:"dlspeed"`
}

func (c *Client) Status(ctx context.Context, taskID string) (*downloader.LocalStatus, error) {
	endpoint := "/api/v2/torrents/info?hashes=" + url.QueryEscape(strings.ToLower(strings.TrimSpace(taskID)))
	body, status, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("qBittorrent returned HTTP %d", status)
	}
	var items []torrentInfo
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("decode qBittorrent response: %w", err)
	}
	if len(items) == 0 {
		return &downloader.LocalStatus{State: downloader.OfflineNotFound}, nil
	}
	item := items[0]
	result := &downloader.LocalStatus{
		TotalBytes: item.Size, DoneBytes: item.Completed, SpeedBytes: item.DlSpeed,
	}
	switch item.State {
	case "error", "missingFiles":
		result.State = downloader.OfflineFailed
		result.Message = item.State
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "checkingUP", "pausedUP", "stoppedUP":
		result.State = downloader.OfflineComplete
	default:
		result.State = downloader.OfflineRunning
	}
	if item.Progress >= 1 {
		result.State = downloader.OfflineComplete
	}
	return result, nil
}

func (c *Client) Remove(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	response, err := c.do(ctx, http.MethodPost, "/api/v2/torrents/delete", url.Values{
		"hashes":      {strings.ToLower(strings.TrimSpace(taskID))},
		"deleteFiles": {"true"},
	})
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (c *Client) Close() error { return nil }

// hashFromMagnet extracts the btih hash so jobs can be polled before the API
// returns a task identifier.
func hashFromMagnet(magnet string) (string, error) {
	for _, part := range strings.Split(strings.TrimPrefix(magnet, "magnet:?"), "&") {
		if !strings.HasPrefix(part, "xt=urn:btih:") {
			continue
		}
		hash := strings.TrimPrefix(part, "xt=urn:btih:")
		if hash == "" {
			break
		}
		return strings.ToLower(hash), nil
	}
	return "", errors.New("magnet link has no btih hash")
}
