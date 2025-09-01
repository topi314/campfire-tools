package web

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
)

type TrackerVars struct {
	Sort       string
	PinnedClub *ClubWithEvents
	Clubs      []ClubWithEvents
	Errors     []string
}

func (h *handler) Tracker(w http.ResponseWriter, r *http.Request) {
	h.renderTracker(w, r)
}

func (h *handler) renderTracker(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()
	query := r.URL.Query()

	sort := query.Get("sort")

	clubs, err := h.DB.GetClubs(ctx, sort)
	if err != nil {
		http.Error(w, "Failed to fetch clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)

	var pinnedClub *ClubWithEvents
	trackerClubs := make([]ClubWithEvents, 0, len(clubs))
	for _, club := range clubs {
		if session.PinnedClubID != nil && *session.PinnedClubID == club.Club.ID {
			c := newClubWithEvents(club)
			pinnedClub = &c
			continue
		}
		trackerClubs = append(trackerClubs, newClubWithEvents(club))
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		Sort:       sort,
		PinnedClub: pinnedClub,
		Clubs:      trackerClubs,
		Errors:     errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker template", slog.Any("err", err))
	}
}
