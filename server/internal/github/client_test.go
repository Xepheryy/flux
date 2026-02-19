package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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
