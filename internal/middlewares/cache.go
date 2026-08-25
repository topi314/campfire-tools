package middlewares

import (
	"fmt"
	"net/http"
)

func CacheWithMaxAge(maxAgeSeconds int) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", fmt.Sprintf("stale-while-revalidate, max-age=%d", maxAgeSeconds))
			handler.ServeHTTP(w, r)
		})
	}
}

func Cache(handler http.Handler) http.Handler {
	return CacheWithMaxAge(3600)(handler)
}
