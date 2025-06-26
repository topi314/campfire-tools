package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type TrackerClubEventVars struct {
	ClubName              string
	ClubAvatarURL         string
	ClubID                string
	Name                  string
	CoverPhotoURL         string
	Details               string
	StartTime             time.Time
	EndTime               time.Time
	CampfireLiveEventID   string
	CampfireLiveEventName string
	Members               []TrackerMember
	RSVPMembers           []TrackerMember
}

func (s *Server) TrackerClubEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("event_id")

	event, err := s.db.GetEvent(r.Context(), eventID)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			http.NotFound(w, r)
			return
		}
		slog.ErrorContext(r.Context(), "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	members, err := s.db.GetCheckedInMembersByEvent(r.Context(), eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch checked-in members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerMembers := make([]TrackerMember, len(members))
	for i, member := range members {
		displayName := member.DisplayName
		if displayName == "" {
			displayName = "<unknown>"
		}
		trackerMembers[i] = TrackerMember{
			ID:   member.ID,
			Name: displayName,
			URL:  fmt.Sprintf("/tracker/club/%s/member/%s", event.ClubID, member.ID),
		}
	}

	rsvpMembers, err := s.db.GetRSVPMembersByEvent(r.Context(), eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch RSVP members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch RSVP members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rsvpTrackerMembers := make([]TrackerMember, len(rsvpMembers))
	for i, member := range rsvpMembers {
		displayName := member.DisplayName
		if displayName == "" {
			displayName = "<unknown>"
		}
		rsvpTrackerMembers[i] = TrackerMember{
			ID:   member.ID,
			Name: displayName,
			URL:  fmt.Sprintf("/tracker/club/%s/member/%s", event.ClubID, member.ID),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_event.gohtml", TrackerClubEventVars{
		ClubName:              event.ClubName,
		ClubAvatarURL:         imageURL(event.ClubAvatarURL),
		ClubID:                event.ClubID,
		Name:                  event.Name,
		CoverPhotoURL:         imageURL(event.CoverPhotoURL),
		Details:               event.Details,
		StartTime:             event.EventTime,
		EndTime:               event.EventEndTime,
		CampfireLiveEventID:   event.CampfireLiveEventID,
		CampfireLiveEventName: event.CampfireLiveEventName,
		Members:               trackerMembers,
		RSVPMembers:           rsvpTrackerMembers,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render tracker club event template", slog.String("event_id", eventID), slog.Any("err", err))
	}
}
