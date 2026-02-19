package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/shaun/flux/server/internal/github"
	"github.com/shaun/flux/server/internal/sync"
)

// Syncer syncs files to GitHub. Implemented by *github.Client; inject a fake in tests.
type Syncer interface {
	Sync(ctx context.Context, token, owner, repo string, files []*sync.File, deleted []string) error
}

type Handler struct {
	store *sync.Store
	gh    Syncer
}

func NewHandler(store *sync.Store) *Handler {
	return &Handler{store: store, gh: github.NewClient()}
}

// NewHandlerWithSyncer builds a handler with a custom Syncer (e.g. for tests).
func NewHandlerWithSyncer(store *sync.Store, gh Syncer) *Handler {
	return &Handler{store: store, gh: gh}
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

const maxPushBodyBytes = 10 << 20 // 10 MiB

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPushBodyBytes)
	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	for _, f := range req.Files {
		if safePath(f.Path) {
			h.store.UpsertFile(f.Path, f.Content, f.Hash)
		}
	}
	for _, path := range req.Deleted {
		if safePath(path) {
			h.store.DeleteFile(path)
		}
	}
	if err := h.syncToGitHub(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// safePath rejects path traversal and invalid paths. Paths must be relative, no "..", length capped.
func safePath(p string) bool {
	if p == "" || len(p) > 2048 {
		return false
	}
	// Normalize: no leading/trailing slash for consistency; reject ".." and absolute
	if p[0] == '/' || p[0] == '\\' {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] == '.' && i+1 < len(p) && p[i+1] == '.' {
			return false
		}
		if p[i] == 0 || p[i] == '\\' {
			return false
		}
	}
	return true
}

func (h *Handler) Pull(w http.ResponseWriter, r *http.Request) {
	files, deleted := h.store.GetFiles()
	res := PullResponse{Files: make([]PullFile, len(files)), Deleted: deleted}
	for i, f := range files {
		res.Files[i] = PullFile{Path: f.Path, Content: f.Content, Hash: f.Hash}
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *Handler) syncToGitHub(ctx context.Context) error {
	owner := os.Getenv("FLUX_GIT_OWNER")
	repo := os.Getenv("FLUX_GIT_REPO")
	token := os.Getenv("FLUX_GIT_TOKEN")
	files, deleted := h.store.GetFiles()
	log.Printf("[Flux] Syncing %d files, %d deletes to %s/%s", len(files), len(deleted), owner, repo)
	if err := h.gh.Sync(ctx, token, owner, repo, files, deleted); err != nil {
		log.Printf("[Flux] GitHub sync failed: %v", err)
		return err
	}
	log.Print("[Flux] GitHub sync done")
	return nil
}
