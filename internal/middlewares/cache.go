package middlewares

import (
	"net/http"
)

func Cache(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "stale-while-revalidate, max-age=3600") // Cache for 1 hour, revalidate after stale
		handler.ServeHTTP(w, r)
	})
}
