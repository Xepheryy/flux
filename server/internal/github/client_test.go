package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shaun/flux/server/internal/sync"
)

func TestClient_FetchFromRepo_emptyToken(t *testing.T) {
	c := NewClient()
	files, err := c.FetchFromRepo(context.Background(), "", "o", "r")
	if err != nil {
		t.Fatalf("FetchFromRepo empty token: %v", err)
	}
	if files != nil {
		t.Fatalf("expected nil files for empty token, got %d", len(files))
	}
}

func TestClient_FetchFromRepo(t *testing.T) {
	// Mock GitHub Contents API: root dir -> Flux/; Flux/ -> note.md; Flux/note.md -> file content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("ref")
		_ = path
		p := strings.TrimPrefix(r.URL.Path, "/repos/o/r/contents/")
		p = strings.TrimSuffix(p, "/")

		if p == "" {
			// Root: return dir "Flux"
			json.NewEncoder(w).Encode([]map[string]any{
				{"name": "Flux", "path": "Flux", "type": "dir"},
			})
			return
		}
		if p == "Flux" {
			// Flux dir: return file note.md
			json.NewEncoder(w).Encode([]map[string]any{
				{"name": "note.md", "path": "Flux/note.md", "type": "file"},
			})
			return
		}
		if p == "Flux/note.md" {
			// File content (base64 of "# hello\n")
			json.NewEncoder(w).Encode(map[string]any{
				"type": "file", "path": "Flux/note.md",
				"encoding": "base64",
				"content": "IyBoZWxsbwo=",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &http.Client{Transport: &rewriteTransport{baseURL: server.URL}}
	c := NewClientWithHTTPClient(client)
	files, err := c.FetchFromRepo(context.Background(), "token", "o", "r")
	if err != nil {
		t.Fatalf("FetchFromRepo: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "Flux/note.md" || files[0].Content != "# hello\n" {
		t.Fatalf("got %+v", files[0])
	}
	if files[0].Hash == "" {
		t.Fatalf("expected non-empty hash")
	}
}

func TestClient_Sync_emptyToken(t *testing.T) {
	c := NewClient()
	err := c.Sync(context.Background(), "", "o", "r", nil, nil)
	if err != nil {
		t.Fatalf("Sync empty token: %v", err)
	}
}

func TestClient_Sync_createAndUpdate(t *testing.T) {
	// Fake GitHub API: GET 404 -> create; GET 200 with body -> update
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":{"sha":"abc"}}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := &http.Client{Transport: &rewriteTransport{baseURL: server.URL}}
	c := NewClientWithHTTPClient(client)
	files := []*sync.File{{Path: "a.md", Content: "hi", Hash: "h1"}}
	err := c.Sync(context.Background(), "token", "o", "r", files, nil)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func TestClient_Sync_updateAndDelete(t *testing.T) {
	var gotUpdate, gotDelete bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return existing content so UpdateFile/DeleteFile paths are taken.
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"sha":"abc"}`))
		case http.MethodPut:
			gotUpdate = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":{"sha":"new"}}`))
		case http.MethodDelete:
			if strings.Contains(r.URL.Path, "gone.md") {
				gotDelete = true
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := &http.Client{Transport: &rewriteTransport{baseURL: server.URL}}
	c := NewClientWithHTTPClient(client)
	files := []*sync.File{{Path: "existing.md", Content: "hi", Hash: "h1"}}
	deleted := []string{"gone.md"}

	if err := c.Sync(context.Background(), "token", "o", "r", files, deleted); err != nil {
		t.Fatalf("Sync update+delete: %v", err)
	}
	if !gotUpdate {
		t.Fatalf("expected update call")
	}
	if !gotDelete {
		t.Fatalf("expected delete call for gone.md")
	}
}

func TestClient_Sync_retryOn409(t *testing.T) {
	// First UpdateFile returns 409, retry with fresh SHA succeeds
	updateCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"sha":"abc","content":"aW5pdA=="}`))
			return
		}
		if r.Method == http.MethodPut {
			updateCalls++
			if updateCalls == 1 {
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(`{"message":"update does not match sha"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":{"sha":"new"}}`))
		}
	}))
	defer server.Close()

	client := &http.Client{Transport: &rewriteTransport{baseURL: server.URL}}
	c := NewClientWithHTTPClient(client)
	files := []*sync.File{{Path: "x.md", Content: "init", Hash: "h1"}}
	if err := c.Sync(context.Background(), "token", "o", "r", files, nil); err != nil {
		t.Fatalf("Sync 409 retry: %v", err)
	}
	if updateCalls != 2 {
		t.Fatalf("expected 2 update calls (retry), got %d", updateCalls)
	}
}

func TestClient_Sync_deleteSkip404(t *testing.T) {
	// Deleted path returns 404 -> skip (no error)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{Transport: &rewriteTransport{baseURL: server.URL}}
	c := NewClientWithHTTPClient(client)
	err := c.Sync(context.Background(), "token", "o", "r", nil, []string{"gone.md"})
	if err != nil {
		t.Fatalf("Sync delete 404: %v", err)
	}
}

// rewriteTransport sends requests to baseURL instead of the original host (for fake GitHub API).
type rewriteTransport struct {
	baseURL string
	base    http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	u, err := url.Parse(t.baseURL)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return t.base.RoundTrip(req)
}
