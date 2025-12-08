package tools

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerClubVars struct {
	models.Club
	Events []models.Event
	Pinned bool
}

func (h *handler) TrackerClub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID, time.Time{}, time.Time{}, false, "")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]models.Event, len(events))
	for i, event := range events {
		trackerEvents[i] = models.NewEventWithCheckIns(event, 32)
	}

	session := auth.GetSession(r)
	pinnedClubs, err := h.DB.GetDiscordUserPinnedClubs(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch pinned clubs for user", slog.String("user_id", session.UserID), slog.Any("err", err))
		http.Error(w, "Failed to fetch pinned clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pinned := slices.Contains(pinnedClubs, clubID)

	if err = h.Templates().ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		Club:   models.NewClub(*club),
		Events: trackerEvents,
		Pinned: pinned,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}
