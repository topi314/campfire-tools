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
	Members               []Member
	AcceptedMembers       []Member
}

func (s *Server) TrackerClubEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	event, err := s.db.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			s.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	members, err := s.db.GetCheckedInMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch checked-in members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerMembers := make([]Member, len(members))
	for i, member := range members {
		trackerMembers[i] = Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.GetDisplayName(),
			AvatarURL:   imageURL(member.AvatarURL),
			URL:         fmt.Sprintf("/tracker/club/%s/member/%s", event.ClubID, member.ID),
		}
	}

	acceptedMembers, err := s.db.GetAcceptedMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch RSVP members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch RSVP members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	acceptedTrackerMembers := make([]Member, len(acceptedMembers))
	for i, member := range acceptedMembers {
		acceptedTrackerMembers[i] = Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.GetDisplayName(),
			AvatarURL:   imageURL(member.AvatarURL),
			URL:         fmt.Sprintf("/tracker/club/%s/member/%s", event.ClubID, member.ID),
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
		AcceptedMembers:       acceptedTrackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club event template", slog.String("event_id", eventID), slog.Any("err", err))
	}
}
