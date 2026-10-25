package aria2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientLifecycle(t *testing.T) {
	t.Parallel()
	var sawToken bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if len(request.Params) == 0 || request.Params[0] != "token:secret" {
			t.Errorf("missing token param: %#v", request.Params)
		} else {
			sawToken = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "aria2.getVersion":
			_, _ = w.Write([]byte(`{"result":{"version":"1.37.0"}}`))
		case "aria2.getGlobalStat":
			_, _ = w.Write([]byte(`{"result":{"downloadSpeed":"1024","numActive":"1"}}`))
		case "aria2.addUri":
			_, _ = w.Write([]byte(`{"result":"gid-1"}`))
		case "aria2.tellStatus":
			_, _ = w.Write([]byte(`{"result":{"status":"complete","totalLength":"100","completedLength":"100"}}`))
		case "aria2.forceRemove", "aria2.removeDownloadResult":
			_, _ = w.Write([]byte(`{"result":"OK"}`))
		default:
			t.Errorf("unexpected method %q", request.Method)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "secret")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.Test(context.Background())
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if !sawToken {
		t.Fatal("token was not sent")
	}
	if result.Details["version"] != "1.37.0" {
		t.Fatalf("unexpected version: %#v", result.Details)
	}
	taskID, err := client.Add(context.Background(), "magnet:?xt=urn:btih:abc", "/downloads")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if taskID != "gid-1" {
		t.Fatalf("unexpected task id: %q", taskID)
	}
	status, err := client.Status(context.Background(), taskID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != "complete" || status.DoneBytes != 100 || status.TotalBytes != 100 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := client.Remove(context.Background(), taskID); err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func TestClientUnauthorized(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"code":1,"message":"Unauthorized"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Test(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestStatusNotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"code":1,"message":"GID gid-1 is not found"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	status, err := client.Status(context.Background(), "gid-1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != "not_found" {
		t.Fatalf("unexpected state: %q", status.State)
	}
}

func TestNewNormalizesAddress(t *testing.T) {
	t.Parallel()
	client, err := New("127.0.0.1:6800/", "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if !strings.HasSuffix(client.endpoint, "/jsonrpc") || !strings.HasPrefix(client.endpoint, "http://") {
		t.Fatalf("unexpected endpoint: %q", client.endpoint)
	}
}
