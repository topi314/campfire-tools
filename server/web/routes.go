package web

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server"
)

type handler struct {
	*server.Server
}

func Routes(srv *server.Server) http.Handler {
	h := &handler{
		Server: srv,
	}

	fs := http.FileServer(h.StaticFS)

	mux := http.NewServeMux()
	mux.HandleFunc("/", h.NotFound)
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("GET /login", h.Login)
	mux.HandleFunc("GET /login/callback", h.LoginCallback)

	mux.HandleFunc("GET  /raffle", h.Raffle)
	mux.HandleFunc("POST /raffle", h.DoRaffle)

	mux.HandleFunc("GET  /export", h.Export)
	mux.HandleFunc("POST /export", h.DoExport)

	mux.HandleFunc("GET  /tracker", h.Tracker)
	mux.HandleFunc("POST /tracker", h.TrackerAdd)

	mux.HandleFunc("GET /tracker/club/{club_id}", h.TrackerClub)
	mux.HandleFunc("GET /tracker/club/{club_id}/events/export", h.TrackerClubEventsExport)
	mux.HandleFunc("GET /tracker/club/{club_id}/stats", h.TrackerClubStats)

	mux.HandleFunc("GET  /tracker/club/{club_id}/export", h.TrackerClubExport)

	mux.HandleFunc("GET /tracker/club/{club_id}/raffle", h.TrackerClubRaffle)

	mux.HandleFunc("GET /tracker/club/{club_id}/member/{member_id}", h.TrackerClubMember)
	mux.HandleFunc("GET /tracker/event/{event_id}", h.TrackerClubEvent)
	mux.HandleFunc("GET /tracker/event/{event_id}/export", h.TrackerClubEventExport)
	mux.HandleFunc("GET /tracker/refresh", h.TrackerRefresh)
	mux.HandleFunc("GET /tracker/migrate", h.TrackerMigrate)

	mux.HandleFunc("GET /images/{image_id}", h.Image)
	mux.Handle("GET /static/", fs)
	mux.Handle("HEAD /static/", fs)

	return h.AuthMiddleware(mux)
}

func (h *handler) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "not_found.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render not found template", slog.String("error", err.Error()))
		return
	}
}
