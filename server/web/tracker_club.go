package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubVars struct {
	Club
	Events []Event
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

	events, err := h.DB.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]Event, len(events))
	for i, event := range events {
		trackerEvents[i] = Event{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL, 32),
		}
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		Club: Club{
			ClubID:        club.ID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL, 48),
		},
		Events: trackerEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

func (h *handler) TrackerClubEventsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	events, err := h.DB.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var exportEvents []json.RawMessage
	for _, event := range events {
		exportEvents = append(exportEvents, event.RawJSON)
	}

	if err = json.NewEncoder(w).Encode(exportEvents); err != nil {
		slog.ErrorContext(ctx, "Failed to write events export", slog.String("club_id", clubID), slog.Any("err", err))
		return
	}
}
