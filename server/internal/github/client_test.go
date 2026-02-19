package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shaun/flux/server/internal/sync"
)

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
