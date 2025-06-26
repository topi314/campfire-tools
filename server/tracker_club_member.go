package server

import (
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubMemberVars struct {
	ClubName       string
	ClubAvatarURL  string
	ClubID         string
	ID             string
	Username       string
	DisplayName    string
	AvatarURL      string
	Events         []Event
	AcceptedEvents []Event
}

func (s *Server) TrackerClubMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")
	memberID := r.PathValue("member_id")

	member, err := s.db.GetClubMember(ctx, clubID, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := s.db.GetCheckedInClubEventsByMember(ctx, clubID, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club events by member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club events by member: "+err.Error(), http.StatusInternalServerError)
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

	acceptedEvents, err := s.db.GetAcceptedClubEventsByMember(ctx, clubID, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch RSVP club events by member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch RSVP club events by member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	acceptedTrackerEvents := make([]Event, len(acceptedEvents))
	for i, event := range acceptedEvents {
		acceptedTrackerEvents[i] = Event{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_member.gohtml", TrackerClubMemberVars{
		ClubName:       club.ClubName,
		ClubAvatarURL:  imageURL(club.ClubAvatarURL),
		ClubID:         club.ClubID,
		ID:             member.ID,
		Username:       member.Username,
		DisplayName:    member.DisplayName,
		AvatarURL:      imageURL(member.AvatarURL),
		Events:         trackerEvents,
		AcceptedEvents: acceptedTrackerEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club export template", slog.Any("err", err))
	}
}
