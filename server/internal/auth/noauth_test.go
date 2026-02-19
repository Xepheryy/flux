package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractUser(t *testing.T) {
	defaultUser := "default"
	mw := ExtractUser(defaultUser)
	var gotUser string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromRequest(r)
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	t.Run("health bypass", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if gotUser != "" {
			t.Errorf("health should not set user, got %q", gotUser)
		}
	})

	t.Run("basic auth sets user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pull", nil)
		req.SetBasicAuth("alice", "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if gotUser != "alice" {
			t.Errorf("got user %q", gotUser)
		}
	})

	t.Run("no auth uses default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pull", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if gotUser != defaultUser {
			t.Errorf("got user %q", gotUser)
		}
	})
}

func TestUserFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if UserFromRequest(req) != "" {
		t.Fatal("expected empty without header")
	}
	req.Header.Set("X-Flux-User", "bob")
	if UserFromRequest(req) != "bob" {
		t.Fatal("expected bob")
	}
}
