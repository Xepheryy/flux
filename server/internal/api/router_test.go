package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaun/flux/server/internal/sync"
)

func TestRouter_healthAndPull(t *testing.T) {
	store := sync.NewStore()
	h := NewHandler(store)
	router := NewRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("GET /health: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/pull", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /pull: %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header missing")
	}
}

func TestRouter_options(t *testing.T) {
	store := sync.NewStore()
	h := NewHandler(store)
	router := NewRouter(h)
	req := httptest.NewRequest(http.MethodOptions, "/pull", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: %d", rec.Code)
	}
}
