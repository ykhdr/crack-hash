package middleware

import (
	"github.com/gorilla/mux"
	"net/http"
)

type logger func(msg string, args ...any)

func LoggingMiddleware(log logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log("Incoming request", "method", r.Method, "url", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
