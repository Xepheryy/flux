package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/shaun/flux/server/internal/auth"
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

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.GitHubToken == "" || req.RepoOwner == "" || req.RepoName == "" {
		http.Error(w, "githubToken, repoOwner, repoName required", http.StatusBadRequest)
		return
	}
	user := auth.UserFromRequest(r)
	h.store.SetUserMeta(user, &sync.UserMeta{GitHubToken: req.GitHubToken, RepoOwner: req.RepoOwner, RepoName: req.RepoName})
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user := auth.UserFromRequest(r)
	for _, f := range req.Files {
		if f.Path != "" {
			h.store.UpsertFile(user, f.Path, f.Content, f.Hash)
		}
	}
	for _, path := range req.Deleted {
		if path != "" {
			h.store.DeleteFile(user, path)
		}
	}
	if err := h.syncToGitHub(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Pull(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	files, deleted := h.store.GetFiles(user)
	res := PullResponse{Files: make([]PullFile, len(files)), Deleted: deleted}
	for i, f := range files {
		res.Files[i] = PullFile{Path: f.Path, Content: f.Content, Hash: f.Hash}
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *Handler) syncToGitHub(ctx context.Context, user string) error {
	meta := h.store.GetUserMeta(user)
	if meta == nil {
		if owner, repo, token := os.Getenv("FLUX_GITHUB_OWNER"), os.Getenv("FLUX_GITHUB_REPO"), os.Getenv("FLUX_GITHUB_TOKEN"); owner != "" && repo != "" && token != "" {
			meta = &sync.UserMeta{GitHubToken: token, RepoOwner: owner, RepoName: repo}
		}
	}
	if meta == nil {
		log.Print("[Flux] GitHub sync skipped: FLUX_GITHUB_TOKEN not set")
		return nil
	}
	files, deleted := h.store.GetFiles(user)
	log.Printf("[Flux] Syncing %d files, %d deletes to %s/%s", len(files), len(deleted), meta.RepoOwner, meta.RepoName)
	if err := h.gh.Sync(ctx, meta.GitHubToken, meta.RepoOwner, meta.RepoName, files, deleted); err != nil {
		log.Printf("[Flux] GitHub sync failed: %v", err)
		return err
	}
	log.Print("[Flux] GitHub sync done")
	return nil
}
