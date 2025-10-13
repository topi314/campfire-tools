package web

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server"
	"github.com/topi314/campfire-tools/server/web/middlewares"
	"github.com/topi314/campfire-tools/server/web/rewards"
	"github.com/topi314/campfire-tools/server/web/tracker"
)

type handler struct {
	*server.Server
}

func Routes(srv *server.Server) http.Handler {
	h := &handler{
		Server: srv,
	}

	fileServer := h.Reloader.CacheMiddleware(http.FileServer(h.StaticFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)

	mux.HandleFunc("GET /admin", h.Admin)
	mux.HandleFunc("POST /admin/tokens", h.AdminTokens)

	mux.Handle("/rewards/*", rewards.Routes(srv))
	mux.Handle("/tracker/*", tracker.Routes(srv))

	mux.Handle("/static/", fileServer)

	if srv.Cfg.Dev {
		mux.Handle(server.ReloadRoute, h.Reloader.Handler())
	}

	mux.HandleFunc("/", h.NotFound)

	return cleanPath(mux)
}

func (h *handler) api() http.Handler {
	mux := http.NewServeMux()

	return http.StripPrefix("/api", mux)
}

func (h *handler) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "not_found.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render not found template", slog.String("error", err.Error()))
		return
	}
}

func cleanPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the request URL path
		// r.URL.Path = path.Clean(r.URL.Path)
		// r.URL.RawPath = path.Clean(r.URL.RawPath)
		next.ServeHTTP(w, r)
	})
}
