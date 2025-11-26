package rewards

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/internal/middlewares"
	"github.com/topi314/campfire-tools/server"
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

	mux.HandleFunc("GET  /campfire", h.CampfireLogin)
	mux.HandleFunc("GET  /signup", h.SignUp)
	mux.HandleFunc("POST /signup", h.PostSignUp)
	mux.HandleFunc("POST /signup/callback", h.SignUpCallback)

	mux.HandleFunc("POST /login", h.PostSignUp)

	mux.HandleFunc("GET /code", h.Code)
	mux.HandleFunc("GET /code/{code}", h.GetCode)
	mux.Handle("GET /code/{code}/qr", middlewares.Cache(http.HandlerFunc(h.QRCode)))

	mux.Handle("GET  /static/", fs)
	mux.Handle("HEAD /static/", fs)

	mux.Handle(server.ReloadRoute, srv.Reloader.Handler())

	mux.HandleFunc("/", h.NotFound)

	return mux
}

func (h *handler) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "not_found.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render not found template", slog.String("error", err.Error()))
		return
	}
}
