package tracker

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerClubEventVars struct {
	models.Event

	Club             models.Club
	CheckedInMembers []models.Member
	AcceptedMembers  []models.Member
}

func (h *handler) TrackerClubEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	event, err := h.DB.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	club, err := h.DB.GetClub(ctx, event.ClubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", event.ClubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	checkedInMembers, err := h.DB.GetCheckedInMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch checked-in members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	checkedInTrackerMembers := make([]models.Member, len(checkedInMembers))
	for i, member := range checkedInMembers {
		checkedInTrackerMembers[i] = models.NewMember(member, event.ClubID, 32)
	}

	acceptedMembers, err := h.DB.GetAcceptedMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch accepted members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	acceptedTrackerMembers := make([]models.Member, len(acceptedMembers))
	for i, member := range acceptedMembers {
		acceptedTrackerMembers[i] = models.NewMember(member, event.ClubID, 32)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_event.gohtml", TrackerClubEventVars{
		Event:            models.NewEventWithCreator(*event),
		Club:             models.NewClub(*club),
		CheckedInMembers: checkedInTrackerMembers,
		AcceptedMembers:  acceptedTrackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club event template", slog.String("event_id", eventID), slog.Any("err", err))
	}
}
