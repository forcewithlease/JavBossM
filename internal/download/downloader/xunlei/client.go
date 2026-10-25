package xunlei

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/download/downloader"
)

const (
	defaultBase     = "http://127.0.0.1:2345/webman/3rdparty/pan-xunlei-com/index.cgi"
	requestTimeout  = 20 * time.Second
	maxResponseSize = 8 << 20
)

var tokenPattern = regexp.MustCompile(`return\s+"(eyJ[^"]+)"`)

// Client talks to the Xunlei NAS kernel's local web API.
type Client struct {
	base     string
	username string
	password string
	http     *http.Client

	mu    sync.Mutex
	token string
	space string
}

func New(base, username, password string) (*Client, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultBase
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")
	if !strings.Contains(base, "/webman/") {
		base += "/webman/3rdparty/pan-xunlei-com/index.cgi"
	}
	return &Client{
		base:     base,
		username: strings.TrimSpace(username),
		password: password,
		http:     &http.Client{Timeout: requestTimeout},
	}, nil
}

func (c *Client) Test(ctx context.Context) (*downloader.TestResult, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	body, status, err := c.request(ctx, http.MethodGet, "device/info", nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("xunlei device info returned HTTP %d", status)
	}
	var info struct {
		User struct {
			Name string `json:"name"`
			VIP  struct {
				IsVIP bool `json:"is_vip"`
			} `json:"vip"`
		} `json:"user"`
		Downloads []struct {
			Path string `json:"path"`
		} `json:"downloads"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode xunlei device info: %w", err)
	}
	details := map[string]any{"space": c.space, "vip": info.User.VIP.IsVIP}
	paths := make([]string, 0, len(info.Downloads))
	for _, download := range info.Downloads {
		paths = append(paths, download.Path)
	}
	details["download_paths"] = paths
	return &downloader.TestResult{Provider: "xunlei", UserName: info.User.Name, Details: details}, nil
}

func (c *Client) Add(ctx context.Context, magnet, directory string) (string, error) {
	if err := c.ensureSession(ctx); err != nil {
		return "", err
	}
	name := magnetName(magnet)
	payload := map[string]any{
		"type":      "user#download-url",
		"name":      name,
		"file_name": name,
		"file_size": "0",
		"space":     c.space,
		"params": map[string]any{
			"target":           c.space,
			"url":              magnet,
			"total_file_count": "0",
			"parent_folder_id": "",
			"mime_type":        "",
			"file_id":          "",
		},
	}
	body, status, err := c.request(ctx, http.MethodPost, "drive/v1/task", nil, payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("xunlei add task returned HTTP %d: %s", status, strings.TrimSpace(string(body)))
	}
	var response struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode xunlei add task: %w", err)
	}
	if response.Task.ID == "" {
		if response.Error != "" {
			return "", fmt.Errorf("xunlei add task: %s", response.Error)
		}
		return "", errors.New("xunlei did not return a task id")
	}
	return response.Task.ID, nil
}

type task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Phase       string `json:"phase"`
	FileSize    string `json:"file_size"`
	Message     string `json:"message"`
	CreatedTime string `json:"created_time"`
	Params      struct {
		CheckedSize string `json:"checked_size"`
		Speed       string `json:"speed"`
		RealPath    string `json:"real_path"`
		Target      string `json:"target"`
	} `json:"params"`
}

func (c *Client) Status(ctx context.Context, taskID string) (*downloader.LocalStatus, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	filters, _ := json.Marshal(map[string]any{"id": map[string]string{"in": taskID}})
	query := url.Values{
		"space":   {c.space},
		"limit":   {"10"},
		"filters": {string(filters)},
	}
	body, status, err := c.request(ctx, http.MethodGet, "drive/v1/tasks", query, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("xunlei tasks returned HTTP %d", status)
	}
	var response struct {
		Tasks []task `json:"tasks"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode xunlei tasks: %w", err)
	}
	if len(response.Tasks) == 0 {
		return &downloader.LocalStatus{State: downloader.OfflineNotFound}, nil
	}
	found := response.Tasks[0]
	result := &downloader.LocalStatus{
		Path:    strings.TrimSpace(found.Params.RealPath),
		Message: strings.TrimSpace(found.Message),
	}
	result.TotalBytes, _ = strconv.ParseInt(found.FileSize, 10, 64)
	result.DoneBytes, _ = strconv.ParseInt(found.Params.CheckedSize, 10, 64)
	result.SpeedBytes, _ = strconv.ParseInt(found.Params.Speed, 10, 64)
	switch found.Phase {
	case "PHASE_TYPE_COMPLETE":
		result.State = downloader.OfflineComplete
	case "PHASE_TYPE_ERROR":
		result.State = downloader.OfflineFailed
	case "PHASE_TYPE_PAUSED":
		result.State = downloader.OfflineRunning
		if result.Message == "" {
			result.Message = "paused"
		}
	default:
		result.State = downloader.OfflineRunning
	}
	return result, nil
}

func (c *Client) ListTasks(ctx context.Context) ([]downloader.RemoteTask, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	query := url.Values{
		"space": {c.space},
		"limit": {"100"},
	}
	body, status, err := c.request(ctx, http.MethodGet, "drive/v1/tasks", query, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("xunlei tasks returned HTTP %d", status)
	}
	var response struct {
		Tasks []task `json:"tasks"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode xunlei tasks: %w", err)
	}
	result := make([]downloader.RemoteTask, 0, len(response.Tasks))
	for _, item := range response.Tasks {
		if !strings.HasPrefix(item.Type, "user#download") {
			continue
		}
		remote := downloader.RemoteTask{
			ID:        item.ID,
			Name:      item.Name,
			Type:      item.Type,
			State:     remoteState(item.Phase),
			Phase:     item.Phase,
			Path:      strings.TrimSpace(item.Params.RealPath),
			Message:   strings.TrimSpace(item.Message),
			CreatedAt: item.CreatedTime,
		}
		remote.TotalBytes, _ = strconv.ParseInt(item.FileSize, 10, 64)
		remote.DoneBytes, _ = strconv.ParseInt(item.Params.CheckedSize, 10, 64)
		remote.SpeedBytes, _ = strconv.ParseInt(item.Params.Speed, 10, 64)
		result = append(result, remote)
	}
	return result, nil
}

func remoteState(phase string) string {
	switch phase {
	case "PHASE_TYPE_COMPLETE":
		return "complete"
	case "PHASE_TYPE_ERROR":
		return "failed"
	case "PHASE_TYPE_PAUSED":
		return "paused"
	case "PHASE_TYPE_PENDING":
		return "pending"
	default:
		return "running"
	}
}

func (c *Client) Remove(ctx context.Context, taskID string) error {
	if err := c.ensureSession(ctx); err != nil {
		return err
	}
	query := url.Values{
		"space":    {c.space},
		"task_ids": {taskID},
	}
	_, status, err := c.request(ctx, http.MethodDelete, "drive/v1/tasks", query, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("xunlei delete task returned HTTP %d", status)
	}
	return nil
}

func (c *Client) Close() error { return nil }

func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	needToken := c.token == ""
	needSpace := c.space == ""
	c.mu.Unlock()
	if needToken {
		if err := c.refreshToken(ctx); err != nil {
			return err
		}
	}
	if needSpace {
		if err := c.refreshSpace(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/", nil)
	if err != nil {
		return err
	}
	if c.username != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return errors.New("xunlei dashboard authentication failed")
	}
	match := tokenPattern.FindSubmatch(body)
	if len(match) < 2 {
		return errors.New("xunlei dashboard token was not found")
	}
	c.mu.Lock()
	c.token = string(match[1])
	c.mu.Unlock()
	return nil
}

func (c *Client) refreshSpace(ctx context.Context) error {
	query := url.Values{"type": {"user#runner"}, "space": {""}, "limit": {"10"}}
	body, status, err := c.request(ctx, http.MethodGet, "drive/v1/tasks", query, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("xunlei runner lookup returned HTTP %d", status)
	}
	var response struct {
		Tasks []task `json:"tasks"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}
	for _, item := range response.Tasks {
		if item.Type == "user#runner" && strings.TrimSpace(item.Params.Target) != "" {
			c.mu.Lock()
			c.space = strings.TrimSpace(item.Params.Target)
			c.mu.Unlock()
			return nil
		}
	}
	return errors.New("xunlei device is not connected or logged in")
}

func (c *Client) request(ctx context.Context, method, endpoint string, query url.Values, payload any) ([]byte, int, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("pan_auth", c.token)
	requestURL := c.base + "/" + endpoint + "?" + query.Encode()
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, 0, err
	}
	if c.username != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	request.Header.Set("pan-auth", c.token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		// The dashboard JWT expires after a few days; refresh once and retry.
		if refreshErr := c.refreshToken(ctx); refreshErr != nil {
			return body, response.StatusCode, refreshErr
		}
		return c.request(ctx, method, endpoint, query, payload)
	}
	return body, response.StatusCode, nil
}

func magnetName(magnet string) string {
	parsed, err := url.Parse(strings.TrimSpace(magnet))
	if err != nil {
		return "Xunlei task"
	}
	name := strings.TrimSpace(parsed.Query().Get("dn"))
	if name == "" {
		name = "Xunlei task"
	}
	return name
}
