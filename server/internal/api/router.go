package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NewRouter(h *Handler, authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(cors)
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Get("/health", h.Health)
	r.Group(func(r chi.Router) {
		r.Post("/setup", h.Setup)
		r.Post("/push", h.Push)
		r.Get("/pull", h.Pull)
	})
	return r
}
