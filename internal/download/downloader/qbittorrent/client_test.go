package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientLifecycle(t *testing.T) {
	t.Parallel()
	loggedIn := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Fails."))
				return
			}
			loggedIn = true
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/app/version":
			if !loggedIn {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte("v4.6.0"))
		case "/api/v2/torrents/add":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			if r.Form.Get("savepath") != "/downloads" || r.Form.Get("autoTMM") != "false" {
				t.Errorf("unexpected add form: %#v", r.Form)
			}
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/info":
			_, _ = w.Write([]byte(`[{"hash":"abc","state":"uploading","progress":1,"size":100,"completed":100}]`))
		case "/api/v2/torrents/delete":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "admin", "secret")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.Test(context.Background())
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if result.Details["version"] != "v4.6.0" {
		t.Fatalf("unexpected version: %#v", result.Details)
	}
	hash, err := client.Add(context.Background(), "magnet:?xt=urn:btih:ABC", "/downloads")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if hash != "abc" {
		t.Fatalf("unexpected hash: %q", hash)
	}
	status, err := client.Status(context.Background(), hash)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != "complete" || status.DoneBytes != 100 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := client.Remove(context.Background(), hash); err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func TestLoginFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/app/version":
			w.WriteHeader(http.StatusForbidden)
		default:
			_, _ = w.Write([]byte("Fails."))
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "admin", "wrong")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.Test(context.Background()); err == nil {
		t.Fatal("expected login failure")
	}
}

func TestHashFromMagnet(t *testing.T) {
	t.Parallel()
	hash, err := hashFromMagnet("magnet:?xt=urn:btih:ABCDEF&dn=name")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash != "abcdef" {
		t.Fatalf("unexpected hash: %q", hash)
	}
	if _, err := hashFromMagnet("magnet:?dn=name"); err == nil {
		t.Fatal("expected missing hash error")
	}
}
