package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			user, pass, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Flux"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			r.Header.Set("X-Flux-User", strings.TrimSpace(user))
			next.ServeHTTP(w, r)
		})
	}
}

func UserFromRequest(r *http.Request) string {
	return r.Header.Get("X-Flux-User")
}
