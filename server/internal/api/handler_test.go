package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/shaun/flux/server/internal/sync"
)

func init() {
	log.SetOutput(io.Discard)
}

func TestHandler_Health(t *testing.T) {
	store := sync.NewStore()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Health: code %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("Health: body %q", body)
	}
}

func TestHandler_Push(t *testing.T) {
	store := sync.NewStore()
	fake := &fakeSyncer{}
	h := NewHandlerWithSyncer(store, fake)
	os.Setenv("FLUX_GIT_OWNER", "o")
	os.Setenv("FLUX_GIT_REPO", "r")
	os.Setenv("FLUX_GIT_TOKEN", "tk")
	defer func() {
		os.Unsetenv("FLUX_GIT_OWNER")
		os.Unsetenv("FLUX_GIT_REPO")
		os.Unsetenv("FLUX_GIT_TOKEN")
	}()
	reqBody := PushRequest{
		Files:   []PushFile{{Path: "a.md", Content: "hi", Hash: "h1"}},
		Deleted: []string{},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Push: code %d body %s", rec.Code, rec.Body.String())
	}
	files, deleted := store.GetFiles()
	if len(files) != 1 || files[0].Path != "a.md" || files[0].Content != "hi" {
		t.Fatalf("Push: store state %+v %+v", files, deleted)
	}
}


func TestHandler_Push_withDelete(t *testing.T) {
	store := sync.NewStore()
	h := NewHandlerWithSyncer(store, &fakeSyncer{})
	os.Setenv("FLUX_GIT_OWNER", "o")
	os.Setenv("FLUX_GIT_REPO", "r")
	os.Setenv("FLUX_GIT_TOKEN", "tk")
	defer func() {
		os.Unsetenv("FLUX_GIT_OWNER")
		os.Unsetenv("FLUX_GIT_REPO")
		os.Unsetenv("FLUX_GIT_TOKEN")
	}()
	store.UpsertFile("old.md", "x", "h")
	reqBody := PushRequest{Files: []PushFile{}, Deleted: []string{"old.md"}}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Push delete: code %d", rec.Code)
	}
	files, deleted := store.GetFiles()
	if len(files) != 0 || len(deleted) != 1 || deleted[0] != "old.md" {
		t.Fatalf("Push delete: files=%v deleted=%v", files, deleted)
	}
}

func TestHandler_Push_rejectGet(t *testing.T) {
	store := sync.NewStore()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/push", nil)
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Push GET: code %d", rec.Code)
	}
}

func TestHandler_Push_badJSON(t *testing.T) {
	store := sync.NewStore()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Push bad JSON: code %d", rec.Code)
	}
}

func TestHandler_Push_withSyncer(t *testing.T) {
	store := sync.NewStore()
	fake := &fakeSyncer{}
	h := NewHandlerWithSyncer(store, fake)
	os.Setenv("FLUX_GIT_OWNER", "o")
	os.Setenv("FLUX_GIT_REPO", "r")
	os.Setenv("FLUX_GIT_TOKEN", "tk")
	defer func() {
		os.Unsetenv("FLUX_GIT_OWNER")
		os.Unsetenv("FLUX_GIT_REPO")
		os.Unsetenv("FLUX_GIT_TOKEN")
	}()
	reqBody := PushRequest{Files: []PushFile{{Path: "x.md", Content: "c", Hash: "h"}}, Deleted: nil}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Push with syncer: code %d %s", rec.Code, rec.Body.String())
	}
	if !fake.called {
		t.Error("Syncer.Sync was not called")
	}
}

func TestHandler_Push_syncerError(t *testing.T) {
	store := sync.NewStore()
	h := NewHandlerWithSyncer(store, &fakeSyncer{err: http.ErrAbortHandler})
	os.Setenv("FLUX_GIT_OWNER", "o")
	os.Setenv("FLUX_GIT_REPO", "r")
	os.Setenv("FLUX_GIT_TOKEN", "tk")
	defer func() {
		os.Unsetenv("FLUX_GIT_OWNER")
		os.Unsetenv("FLUX_GIT_REPO")
		os.Unsetenv("FLUX_GIT_TOKEN")
	}()
	reqBody := PushRequest{Files: []PushFile{{Path: "x.md", Content: "c", Hash: "h"}}, Deleted: nil}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Push syncer error: code %d", rec.Code)
	}
}

func TestHandler_Push_rejectsUnsafePaths(t *testing.T) {
	store := sync.NewStore()
	h := NewHandlerWithSyncer(store, &fakeSyncer{})
	os.Setenv("FLUX_GIT_OWNER", "o")
	os.Setenv("FLUX_GIT_REPO", "r")
	os.Setenv("FLUX_GIT_TOKEN", "tk")
	defer func() {
		os.Unsetenv("FLUX_GIT_OWNER")
		os.Unsetenv("FLUX_GIT_REPO")
		os.Unsetenv("FLUX_GIT_TOKEN")
	}()
	reqBody := PushRequest{
		Files: []PushFile{
			{Path: "a.md", Content: "ok", Hash: "h"},
			{Path: "../etc/passwd", Content: "x", Hash: "h"},
			{Path: "/absolute", Content: "x", Hash: "h"},
			{Path: "", Content: "x", Hash: "h"},
		},
		Deleted: []string{"good.md", "../other", ""},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Push: code %d", rec.Code)
	}
	files, deleted := store.GetFiles()
	if len(files) != 1 || files[0].Path != "a.md" {
		t.Fatalf("only safe path stored: %+v", files)
	}
	if len(deleted) != 1 || deleted[0] != "good.md" {
		t.Fatalf("only safe deleted: %+v", deleted)
	}
}

type fakeSyncer struct {
	called bool
	err    error
}

func (f *fakeSyncer) Sync(_ context.Context, _, _, _ string, _ []*sync.File, _ []string) error {
	f.called = true
	return f.err
}

func TestHandler_Pull(t *testing.T) {
	store := sync.NewStore()
	store.UpsertFile("f.md", "content", "hash")
	store.DeleteFile("gone.md")
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/pull", nil)
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Pull: code %d", rec.Code)
	}
	var res PullResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("Pull: decode %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].Path != "f.md" || res.Files[0].Content != "content" {
		t.Fatalf("Pull: files %+v", res.Files)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "gone.md" {
		t.Fatalf("Pull: deleted %+v", res.Deleted)
	}
}
