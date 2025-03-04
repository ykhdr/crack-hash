package middleware

import (
	log "log/slog"
	"net/http"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Incoming request", "method", r.Method, "url", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
