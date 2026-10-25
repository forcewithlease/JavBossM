package aria2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"javboss/internal/download/downloader"
)

const requestTimeout = 15 * time.Second

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// ErrUnauthorized reports a missing or invalid RPC secret.
var ErrUnauthorized = errors.New("aria2 unauthorized")

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("aria2 error %d: %s", e.Code, e.Message)
}

// New builds an aria2 JSON-RPC client. The address may omit the scheme.
func New(address, token string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = "http://127.0.0.1:6800"
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	address = strings.TrimRight(address, "/")
	return &Client{
		endpoint: address + "/jsonrpc",
		token:    strings.TrimSpace(token),
		http:     &http.Client{Timeout: requestTimeout},
	}, nil
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func (c *Client) call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	if c.token != "" {
		params = append([]any{"token:" + c.token}, params...)
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "javboss",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var decoded rpcResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("aria2 returned HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("decode aria2 response: %w", err)
	}
	if decoded.Error != nil {
		if strings.Contains(strings.ToLower(decoded.Error.Message), "unauthorized") {
			return nil, ErrUnauthorized
		}
		return nil, decoded.Error
	}
	return decoded.Result, nil
}

func (c *Client) Test(ctx context.Context) (*downloader.TestResult, error) {
	raw, err := c.call(ctx, "aria2.getVersion", nil)
	if err != nil {
		return nil, err
	}
	var version struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(raw, &version)
	details := map[string]any{}
	if version.Version != "" {
		details["version"] = version.Version
	}
	if statRaw, statErr := c.call(ctx, "aria2.getGlobalStat", nil); statErr == nil {
		var stat struct {
			DownloadSpeed string `json:"downloadSpeed"`
			NumActive     string `json:"numActive"`
		}
		if json.Unmarshal(statRaw, &stat) == nil {
			if speed, convErr := strconv.ParseInt(stat.DownloadSpeed, 10, 64); convErr == nil {
				details["download_speed"] = speed
			}
			details["active"] = stat.NumActive
		}
	}
	return &downloader.TestResult{Provider: "aria2", Details: details}, nil
}

func (c *Client) Add(ctx context.Context, magnet, directory string) (string, error) {
	options := map[string]string{"seed-time": "0"}
	if strings.TrimSpace(directory) != "" {
		options["dir"] = directory
	}
	raw, err := c.call(ctx, "aria2.addUri", []any{[]string{magnet}, options})
	if err != nil {
		return "", err
	}
	var gid string
	if err := json.Unmarshal(raw, &gid); err != nil {
		return "", fmt.Errorf("decode aria2 task id: %w", err)
	}
	if strings.TrimSpace(gid) == "" {
		return "", errors.New("aria2 did not return a task id")
	}
	return gid, nil
}

func (c *Client) Status(ctx context.Context, taskID string) (*downloader.LocalStatus, error) {
	raw, err := c.call(ctx, "aria2.tellStatus", []any{taskID, []string{
		"status", "totalLength", "completedLength", "errorMessage",
	}})
	if err != nil {
		var rpcErr *rpcError
		if errors.As(err, &rpcErr) && strings.Contains(strings.ToLower(rpcErr.Message), "not found") {
			return &downloader.LocalStatus{State: downloader.OfflineNotFound}, nil
		}
		if errors.Is(err, ErrUnauthorized) {
			return nil, err
		}
		return nil, err
	}
	var status struct {
		Status          string `json:"status"`
		TotalLength     string `json:"totalLength"`
		CompletedLength string `json:"completedLength"`
		ErrorMessage    string `json:"errorMessage"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decode aria2 status: %w", err)
	}
	result := &downloader.LocalStatus{Message: strings.TrimSpace(status.ErrorMessage)}
	result.TotalBytes, _ = strconv.ParseInt(status.TotalLength, 10, 64)
	result.DoneBytes, _ = strconv.ParseInt(status.CompletedLength, 10, 64)
	switch status.Status {
	case "complete":
		result.State = downloader.OfflineComplete
	case "error", "removed":
		result.State = downloader.OfflineFailed
	case "active", "waiting", "paused":
		result.State = downloader.OfflineRunning
	default:
		result.State = downloader.OfflineRunning
	}
	return result, nil
}

func (c *Client) Remove(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	// The task may already be finished; both calls are best-effort.
	_, _ = c.call(ctx, "aria2.forceRemove", []any{taskID})
	_, _ = c.call(ctx, "aria2.removeDownloadResult", []any{taskID})
	return nil
}

func (c *Client) Close() error { return nil }
