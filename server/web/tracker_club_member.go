package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubMemberVars struct {
	Club
	ID             string
	Username       string
	DisplayName    string
	AvatarURL      string
	Events         []Event
	AcceptedEvents []Event
}

func (h *handler) TrackerClubMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")
	memberID := r.PathValue("member_id")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "Club not found", slog.String("club_id", clubID))
			http.Error(w, "Club not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	member, err := h.DB.GetMember(ctx, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club member", slog.String("club_id", clubID), slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetCheckedInClubEventsByMember(ctx, clubID, memberID)
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
			CoverPhotoURL: imageURL(event.CoverPhotoURL, 32),
		}
	}

	acceptedEvents, err := h.DB.GetAcceptedClubEventsByMember(ctx, clubID, memberID)
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
			CoverPhotoURL: imageURL(event.CoverPhotoURL, 32),
		}
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_member.gohtml", TrackerClubMemberVars{
		Club: Club{
			ClubID:        club.ID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL, 48),
		},
		ID:             member.ID,
		Username:       member.Username,
		DisplayName:    member.DisplayName,
		AvatarURL:      imageURL(member.AvatarURL, 48),
		Events:         trackerEvents,
		AcceptedEvents: acceptedTrackerEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club export template", slog.Any("err", err))
	}
}
