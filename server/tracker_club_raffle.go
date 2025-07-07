package server

import (
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubRaffleVars struct {
	Club
	Events          []Event
	SelectedEventID string
	Error           string
}

func (s *Server) TrackerClubRaffle(w http.ResponseWriter, r *http.Request) {
	s.renderTrackerClubRaffle(w, r, "")
}

func (s *Server) renderTrackerClubRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	eventID := query.Get("event")

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
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

	if err = s.templates().ExecuteTemplate(w, "tracker_club_raffle.gohtml", TrackerClubRaffleVars{
		Club: Club{
			ClubID:        club.ID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL),
		},
		Events:          trackerEvents,
		SelectedEventID: eventID,
		Error:           errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club raffle template", slog.Any("err", err))
	}
}
