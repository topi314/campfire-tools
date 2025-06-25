package server

import (
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubMemberVars struct {
	ClubName      string
	ClubAvatarURL string
	ClubID        string
	ID            string
	Name          string
	Events        []TrackerEvent
	RSVPEvents    []TrackerEvent
}

func (s *Server) TrackerClubMember(w http.ResponseWriter, r *http.Request) {
	clubID := r.PathValue("club_id")
	memberID := r.PathValue("member_id")

	member, err := s.database.GetClubMember(r.Context(), clubID, memberID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch club member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	club, err := s.database.GetClub(r.Context(), clubID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := s.database.GetCheckedInClubEventsByMember(r.Context(), clubID, memberID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch club events by member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club events by member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerEvents := make([]TrackerEvent, len(events))
	for i, event := range events {
		trackerEvents[i] = TrackerEvent{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	rsvpEvents, err := s.database.GetRSVPClubEventsByMember(r.Context(), clubID, memberID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch RSVP club events by member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch RSVP club events by member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rsvpTrackerEvents := make([]TrackerEvent, len(rsvpEvents))
	for i, event := range rsvpEvents {
		rsvpTrackerEvents[i] = TrackerEvent{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_member.gohtml", TrackerClubMemberVars{
		ClubName:      club.ClubName,
		ClubAvatarURL: imageURL(club.ClubAvatarURL),
		ClubID:        club.ClubID,
		ID:            member.ID,
		Name:          member.DisplayName,
		Events:        trackerEvents,
		RSVPEvents:    rsvpTrackerEvents,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render tracker club export template", slog.Any("err", err))
	}
}
