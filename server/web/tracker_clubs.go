package web

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerClubsVars struct {
	Sort   string
	Clubs  []ClubWithEvents
	Errors []string
}

func (h *handler) TrackerClubs(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubs(w, r)
}

func (h *handler) renderTrackerClubs(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()
	query := r.URL.Query()

	sort := query.Get("sort")

	clubs, err := h.DB.GetClubs(ctx, sort)
	if err != nil {
		http.Error(w, "Failed to fetch clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)

	pinnedClubs, err := h.DB.GetDiscordUserPinnedClubs(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch pinned clubs for user", slog.String("user_id", session.UserID), slog.Any("err", err))
		http.Error(w, "Failed to fetch pinned clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerClubs := make([]ClubWithEvents, 0, len(clubs))
	for _, club := range pinnedClubs {
		i := slices.IndexFunc(clubs, func(c database.ClubWithEvents) bool {
			return c.ID == club
		})
		if i == -1 {
			continue
		}

		trackerClubs = append(trackerClubs, newPinnedClubWithEvents(clubs[i]))
		clubs = append(clubs[:i], clubs[i+1:]...)
	}

	for _, club := range clubs {
		trackerClubs = append(trackerClubs, newClubWithEvents(club))
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_clubs.gohtml", TrackerClubsVars{
		Sort:   sort,
		Clubs:  trackerClubs,
		Errors: errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker template", slog.Any("err", err))
	}
}
