package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/shaun/flux/server/internal/api"
	"github.com/shaun/flux/server/internal/auth"
	"github.com/shaun/flux/server/internal/sync"
)

func main() {
	_ = godotenv.Load(".env")
	store := sync.NewStore()
	handler := api.NewHandler(store)

	// Auth is delegated to reverse proxy (e.g. Caddy basicauth).
	// We extract the username from the Authorization header for per-user storage.
	authMw := auth.ExtractUser("default")
	router := api.NewRouter(handler, authMw)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("Flux server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
