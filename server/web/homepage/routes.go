package homepage

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/server"
	"github.com/topi314/campfire-tools/server/database"
)

type handler struct {
	*server.Server
}

func Routes(srv *server.Server) http.Handler {
	h := &handler{
		Server: srv,
	}

	fs := srv.Reloader.CacheMiddleware(http.FileServer(h.StaticFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("GET /events", h.Events)
	mux.HandleFunc("GET /leaderboard", h.Leaderboard)

	mux.Handle("GET  /static/", fs)
	mux.Handle("HEAD /static/", fs)

	mux.Handle(server.ReloadRoute, srv.Reloader.Handler())

	mux.HandleFunc("/", h.NotFound)

	return mux
}

func (h *handler) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "homepage_not_found.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render not found template", slog.String("err", err.Error()))
		return
	}
}

func (h *handler) GetHomepage(r *http.Request) (*database.Homepage, error) {
	host := routingHost(r)

	return h.DB.GetHomepageByHost(host)
}

func routingHost(r *http.Request) string {
	host := r.URL.Host
	if host == "" {
		host = r.Host
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")

	return host
}
