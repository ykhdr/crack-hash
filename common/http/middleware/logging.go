package middleware

import (
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"net/http"
)

func LoggingMiddleware(l zerolog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.Debug().Str("method", r.Method).Str("path", r.URL.Path).Msg("Incoming request")
			next.ServeHTTP(w, r)
		})
	}
}
