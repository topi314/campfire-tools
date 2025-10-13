package tracker

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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.TrackerIndex)

	mux.HandleFunc("GET  /tracker/rewards", h.TrackerRewards)
	mux.HandleFunc("GET  /tracker/rewards/new", h.TrackerRewardsNew)
	mux.HandleFunc("POST /tracker/rewards/new", h.PostTrackerRewardsNew)
	mux.HandleFunc("GET /tracker/reward-pool/{pool_id}", h.TrackerRewardPool)

	mux.HandleFunc("GET  /tracker/code/{code}", h.TrackerCode)
	mux.HandleFunc("POST /tracker/code/{code}", h.PostTrackerCode)

	mux.HandleFunc("GET /tracker/login", h.Login)
	mux.HandleFunc("GET /tracker/login/callback", h.LoginCallback)

	mux.HandleFunc("GET  /tracker/raffle", h.Raffle)
	mux.HandleFunc("POST /tracker/raffle", h.RunRaffle)
	mux.HandleFunc("POST /tracker/raffle/{raffle_id}", h.RerunRaffle)
	mux.HandleFunc("GET  /tracker/raffle/{raffle_id}", h.GetRaffle)
	mux.HandleFunc("POST /tracker/raffle/{raffle_id}/confirm/{member_id}", h.ConfirmRaffleWinner)

	mux.HandleFunc("GET  /tracker/export", h.Export)
	mux.HandleFunc("POST /tracker/export", h.DoExport)

	mux.HandleFunc("GET  /tracker/clubs", h.TrackerClubs)

	mux.HandleFunc("GET /tracker/refresh", h.TrackerRefresh)

	mux.HandleFunc("GET  /tracker/club/import", h.TrackerClubImport)
	mux.HandleFunc("POST /tracker/club/import", h.TrackerClubDoImport)

	mux.HandleFunc("GET  /tracker/club/{club_id}", h.TrackerClub)
	mux.HandleFunc("POST /tracker/club/{club_id}", h.TrackerClubUpdate)
	mux.HandleFunc("GET  /tracker/club/{club_id}/stats", h.TrackerClubStats)
	mux.HandleFunc("GET  /tracker/club/{club_id}/events", h.TrackerClubEvents)
	mux.HandleFunc("GET  /tracker/club/{club_id}/members", h.TrackerClubMembers)
	mux.HandleFunc("GET  /tracker/club/{club_id}/member/{member_id}", h.TrackerClubMember)

	mux.HandleFunc("GET /tracker/quarter-filters", h.GetQuarterFilters)

	mux.HandleFunc("GET  /tracker/club/{club_id}/export", h.TrackerClubExport)
	mux.HandleFunc("POST /tracker/club/{club_id}/export", h.DoExport)

	mux.HandleFunc("GET  /tracker/club/{club_id}/raffle", h.TrackerClubRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle", h.RunRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle/{raffle_id}", h.RerunRaffle)
	mux.HandleFunc("GET  /tracker/club/{club_id}/raffle/{raffle_id}", h.GetRaffle)
	mux.HandleFunc("POST /tracker/club/{club_id}/raffle/{raffle_id}/confirm/{member_id}", h.ConfirmRaffleWinner)

	mux.HandleFunc("GET  /tracker/event", h.Event)
	mux.HandleFunc("POST /tracker/event", h.ShowEvent)
	// TODO: merge into TrackerClubEvent
	// mux.HandleFunc("GET  /tracker/event/{event_id}", h.GetEvent)

	mux.HandleFunc("GET  /tracker/event/import", h.TrackerEventImport)
	mux.HandleFunc("POST /tracker/event/import", h.TrackerEventDoImport)

	mux.HandleFunc("GET /tracker/event/{event_id}", h.TrackerClubEvent)
	mux.HandleFunc("GET /tracker/event/{event_id}/refresh", h.TrackerClubEventRefresh)

	mux.HandleFunc("GET  /tracker/api/docs", h.APIDocs)
	mux.HandleFunc("GET  /tracker/api/events", h.APIExportEvents)
	mux.HandleFunc("POST /tracker/api/events", h.APIImportEvents)
	mux.HandleFunc("GET  /tracker/api/clubs/{club_id}/events", h.APIClubEvents)

	mux.HandleFunc("GET /tracker/images/{image_id}", h.Image)

	return h.auth(mux)
}

func (h *handler) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "not_found.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render not found template", slog.String("error", err.Error()))
		return
	}
}
