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

	fs := cache(http.FileServer(h.StaticFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)

	mux.HandleFunc("GET /login", h.Login)
	mux.HandleFunc("GET /login/callback", h.LoginCallback)

	mux.HandleFunc("GET  /raffle", h.Raffle)
	mux.HandleFunc("POST /raffle", h.RunRaffle)
	mux.HandleFunc("POST /raffle/{raffle_id}", h.RerunRaffle)
	mux.HandleFunc("GET  /raffle/{raffle_id}", h.GetRaffle)
	mux.HandleFunc("POST /raffle/{raffle_id}/confirm/{member_id}", h.ConfirmRaffleWinner)

	mux.HandleFunc("GET  /export", h.Export)
	mux.HandleFunc("POST /export", h.DoExport)

	mux.HandleFunc("GET  /tracker", h.Tracker)
	mux.HandleFunc("POST /tracker", h.TrackerAdd)

	mux.HandleFunc("GET /tracker/refresh", h.TrackerRefresh)

	mux.HandleFunc("GET /tracker/club/{club_id}", h.TrackerClub)
	mux.HandleFunc("GET /tracker/club/{club_id}/stats", h.TrackerClubStats)
	mux.HandleFunc("GET /tracker/club/{club_id}/member/{member_id}", h.TrackerClubMember)

	mux.HandleFunc("POST /tracker/club/{club_id}/pin", h.TrackerClubPin)
	mux.HandleFunc("POST /tracker/club/{club_id}/unpin", h.TrackerClubPin)

	mux.HandleFunc("GET  /tracker/club/{club_id}/export", h.TrackerClubExport)
	mux.HandleFunc("POST /tracker/club/{club_id}/export", h.DoExport)

	mux.HandleFunc("GET  /tracker/club/{club_id}/raffle", h.TrackerClubRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle", h.RunRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle/{raffle_id}", h.RerunRaffle)
	mux.HandleFunc("GET  /tracker/club/{club_id}/raffle/{raffle_id}", h.GetRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle/{raffle_id}/confirm/{member_id}", h.ConfirmRaffleWinner)

	mux.HandleFunc("GET  /tracker/club/{club_id}/events/export", h.TrackerClubEventsExport)
	mux.HandleFunc("POST /tracker/club/{club_id}/events/export", h.DoExport)

	mux.HandleFunc("GET /tracker/event/{event_id}", h.TrackerClubEvent)
	mux.HandleFunc("GET /tracker/event/{event_id}/export", h.TrackerClubEventExport)

	mux.HandleFunc("GET  /api/docs", h.APIDocs)
	mux.HandleFunc("GET  /api/events", h.APIExportEvents)
	mux.HandleFunc("POST /api/events", h.APIImportEvents)
	mux.HandleFunc("GET  /api/clubs/{club_id}/events", h.APIClubEvents)

	mux.HandleFunc("GET /images/{image_id}", h.Image)

	mux.Handle("GET  /static/", fs)
	mux.Handle("HEAD /static/", fs)

	mux.HandleFunc("/", h.NotFound)

	return cleanPath(h.auth(mux))
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

func cache(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "stale-while-revalidate, max-age=3600") // Cache for 1 hour, revalidate after stale
		handler.ServeHTTP(w, r)
	})
}

func cleanPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the request URL path
		// r.URL.Path = path.Clean(r.URL.Path)
		// r.URL.RawPath = path.Clean(r.URL.RawPath)
		next.ServeHTTP(w, r)
	})
}
