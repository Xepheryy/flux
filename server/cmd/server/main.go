package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/shaun/flux/server/internal/api"
	"github.com/shaun/flux/server/internal/sync"
)

func main() {
	_ = godotenv.Load(".env")
	if v := os.Getenv("FLUX_GIT_OWNER"); v == "" {
		log.Fatal("[Flux] FLUX_GIT_OWNER not set")
	}
	if v := os.Getenv("FLUX_GIT_REPO"); v == "" {
		log.Fatal("[Flux] FLUX_GIT_REPO not set")
	}
	if v := os.Getenv("FLUX_GIT_TOKEN"); v == "" {
		log.Fatal("[Flux] FLUX_GIT_TOKEN not set")
	}
	store := sync.NewStore()
	handler := api.NewHandler(store)
	router := api.NewRouter(handler)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("Flux server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
