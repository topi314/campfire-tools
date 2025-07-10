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
	Club
	ID                    string
	Name                  string
	CoverPhotoURL         string
	Details               string
	StartTime             time.Time
	EndTime               time.Time
	CampfireLiveEventID   string
	CampfireLiveEventName string
	CheckedInMembers      []Member
	AcceptedMembers       []Member
}

func (s *Server) TrackerClubEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	event, err := s.db.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	club, err := s.db.GetClub(ctx, event.ClubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", event.ClubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	checkedInMembers, err := s.db.GetCheckedInMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch checked-in members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	checkedInTrackerMembers := make([]Member, len(checkedInMembers))
	for i, member := range checkedInMembers {
		checkedInTrackerMembers[i] = Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.GetDisplayName(),
			AvatarURL:   imageURL(member.AvatarURL),
			URL:         fmt.Sprintf("/tracker/club/%s/member/%s", event.ClubID, member.ID),
		}
	}

	acceptedMembers, err := s.db.GetAcceptedMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch accepted members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch accpeted members: "+err.Error(), http.StatusInternalServerError)
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
		Club: Club{
			ClubID:        event.ClubID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL),
		},
		ID:                    event.ID,
		Name:                  event.Name,
		CoverPhotoURL:         imageURL(event.CoverPhotoURL),
		Details:               event.Details,
		StartTime:             event.EventTime,
		EndTime:               event.EventEndTime,
		CampfireLiveEventID:   event.CampfireLiveEventID,
		CampfireLiveEventName: event.CampfireLiveEventName,
		CheckedInMembers:      checkedInTrackerMembers,
		AcceptedMembers:       acceptedTrackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club event template", slog.String("event_id", eventID), slog.Any("err", err))
	}
}
