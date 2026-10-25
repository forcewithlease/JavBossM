package xunlei

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"javboss/internal/download/downloader"
)

const testBasePath = "/webman/3rdparty/pan-xunlei-com/index.cgi"

type fakeXunlei struct {
	mu          sync.Mutex
	added       map[string]any
	removed     string
	statusCalls int
}

func (f *fakeXunlei) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == testBasePath+"/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<script>function getToken(){return "eyJhbGciOiJIUzI1NiJ9.test"}</script>`))
		case r.Header.Get("pan-auth") == "" && r.URL.Query().Get("pan_auth") == "":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == testBasePath+"/device/info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user":      map[string]any{"name": "tester", "vip": map[string]any{"is_vip": true}},
				"downloads": []map[string]any{{"path": "/xunlei/AV/"}},
			})
		case r.URL.Path == testBasePath+"/drive/v1/tasks" && r.Method == http.MethodGet:
			if r.URL.Query().Get("type") == "user#runner" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"tasks": []map[string]any{{
						"id": "runner", "type": "user#runner",
						"params": map[string]any{"target": "device#abc"},
					}},
				})
				return
			}
			if r.URL.Query().Get("filters") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"tasks": []map[string]any{
						{
							"id": "task-1", "name": "TANF-045", "type": "user#download-url",
							"phase": "PHASE_TYPE_RUNNING", "file_size": "1000",
							"message": "已添加", "created_time": "2026-10-25T16:08:31+08:00",
							"params": map[string]any{
								"checked_size": "400", "speed": "50",
								"real_path": "/xunlei/AV/TANF-045",
							},
						},
						{"id": "runner", "name": "runner", "type": "user#runner"},
						{"id": "collection", "name": "collection", "type": "collection"},
					},
				})
				return
			}
			f.mu.Lock()
			f.statusCalls++
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tasks": []map[string]any{{
					"id": "task-1", "name": "TANF-045", "phase": "PHASE_TYPE_RUNNING",
					"file_size": "1000", "message": "",
					"params": map[string]any{
						"checked_size": "400", "speed": "50",
						"real_path": "/xunlei/AV/TANF-045",
					},
				}},
			})
		case r.URL.Path == testBasePath+"/drive/v1/task" && r.Method == http.MethodPost:
			f.mu.Lock()
			_ = json.NewDecoder(r.Body).Decode(&f.added)
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"task": map[string]any{"id": "task-1"}})
		case r.URL.Path == testBasePath+"/drive/v1/tasks" && r.Method == http.MethodDelete:
			f.mu.Lock()
			f.removed = r.URL.Query().Get("task_ids")
			f.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestClientLifecycle(t *testing.T) {
	fake := &fakeXunlei{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()

	client, err := New(server.URL+testBasePath, "admin", "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	result, err := client.Test(ctx)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if result.UserName != "tester" || result.Details["space"] != "device#abc" {
		t.Fatalf("unexpected test result: %+v", result)
	}

	taskID, err := client.Add(ctx, "magnet:?xt=urn:btih:abc&dn=TANF-045", "/xunlei/AV")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if taskID != "task-1" {
		t.Fatalf("task id = %q", taskID)
	}
	fake.mu.Lock()
	added := fake.added
	fake.mu.Unlock()
	if added["space"] != "device#abc" || added["name"] != "TANF-045" {
		t.Fatalf("unexpected add payload: %+v", added)
	}
	params, _ := added["params"].(map[string]any)
	if params["target"] != "device#abc" || !strings.HasPrefix(params["url"].(string), "magnet:") {
		t.Fatalf("unexpected add params: %+v", params)
	}

	status, err := client.Status(ctx, taskID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != downloader.OfflineRunning || status.DoneBytes != 400 ||
		status.TotalBytes != 1000 || status.SpeedBytes != 50 || status.Path != "/xunlei/AV/TANF-045" {
		t.Fatalf("unexpected status: %+v", status)
	}

	tasks, err := client.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-1" || tasks[0].State != "running" ||
		tasks[0].DoneBytes != 400 || tasks[0].TotalBytes != 1000 ||
		tasks[0].SpeedBytes != 50 || tasks[0].Path != "/xunlei/AV/TANF-045" ||
		tasks[0].CreatedAt == "" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}

	if err := client.Remove(ctx, taskID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	fake.mu.Lock()
	removed := fake.removed
	fake.mu.Unlock()
	if removed != taskID {
		t.Fatalf("removed = %q", removed)
	}
}

func TestMagnetName(t *testing.T) {
	tests := []struct {
		name   string
		magnet string
		want   string
	}{
		{name: "dn", magnet: "magnet:?xt=urn:btih:abc&dn=TANF-045", want: "TANF-045"},
		{name: "encoded", magnet: "magnet:?xt=urn:btih:abc&dn=My+Video", want: "My Video"},
		{name: "missing", magnet: "magnet:?xt=urn:btih:abc", want: "Xunlei task"},
		{name: "invalid", magnet: "not a magnet", want: "Xunlei task"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := magnetName(test.magnet); got != test.want {
				t.Fatalf("magnetName(%q) = %q, want %q", test.magnet, got, test.want)
			}
		})
	}
}
