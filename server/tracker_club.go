package server

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

func (s *Server) TrackerClub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := s.db.GetEvents(ctx, clubID)
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
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		Club: Club{
			ClubID:        club.ID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL),
		},
		Events: trackerEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

func (s *Server) TrackerClubEventsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	events, err := s.db.GetEvents(ctx, clubID)
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
