package auth

import (
	"net/http"
)

// ExtractUser sets X-Flux-User from the Basic Auth username. Auth validation is
// delegated to the reverse proxy (e.g. Caddy basicauth); we only extract the user.
func ExtractUser(defaultUser string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			user, _, ok := r.BasicAuth()
			if ok && user != "" {
				r.Header.Set("X-Flux-User", user)
			} else {
				r.Header.Set("X-Flux-User", defaultUser)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UserFromRequest(r *http.Request) string {
	return r.Header.Get("X-Flux-User")
}
