package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/shaun/flux/server/internal/api"
	"github.com/shaun/flux/server/internal/github"
	"github.com/shaun/flux/server/internal/sync"
)

func main() {
	_ = godotenv.Load(".env")
	owner := os.Getenv("FLUX_GIT_OWNER")
	if owner == "" {
		log.Fatal("[Flux] FLUX_GIT_OWNER not set")
	}
	repo := os.Getenv("FLUX_GIT_REPO")
	if repo == "" {
		log.Fatal("[Flux] FLUX_GIT_REPO not set")
	}
	token := os.Getenv("FLUX_GIT_TOKEN")
	if token == "" {
		log.Fatal("[Flux] FLUX_GIT_TOKEN not set")
	}

	store := sync.NewStore()

	// Seed store from GitHub on startup
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	gh := github.NewClient()
	fetched, err := gh.FetchFromRepo(ctx, token, owner, repo)
	if err != nil {
		log.Printf("[Flux] Fetch from GitHub failed (continuing with empty store): %v", err)
	} else {
		for _, f := range fetched {
			store.UpsertFile(f.Path, f.Content, f.Hash)
		}
		log.Printf("[Flux] Loaded %d files from GitHub", len(fetched))
	}

	handler := api.NewHandler(store)
	router := api.NewRouter(handler)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("Flux server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
