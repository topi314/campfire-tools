package tracker

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerCheckIns struct {
	Event string
	Error string
}

type TrackerEventCheckIns struct {
	models.Event

	Club             models.Club
	CheckedInMembers []models.Member
	AcceptedMembers  []models.Member
	Sort             string
	TotalAccepted    int
	TotalCheckIns    int
	TotalCheckInRate float64
}

func (h *handler) Event(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	event := query.Get("event")

	h.renderCheckIns(w, r, event, "")
}

func (h *handler) ShowEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	event := r.FormValue("event")

	slog.InfoContext(ctx, "Showing check-ins for event", slog.String("event", event))

	campfireEvent, err := h.fetchEvent(ctx, event)
	if err != nil {
		if errors.Is(err, campfire.ErrEventNotFound) {
			h.renderCheckIns(w, r, "", "Event not found")
			return
		}

		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event", event), slog.String("err", err.Error()))
		h.renderCheckIns(w, r, "", "Failed to fetch event details")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/event/%s", campfireEvent.ID), http.StatusSeeOther)
}

func (h *handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	eventID := r.PathValue("event_id")
	sortBy := query.Get("sort")
	switch sortBy {
	case "name", "name-desc":
	default:
		sortBy = "name"
	}

	slog.InfoContext(ctx, "Fetching event check-ins", slog.String("event_id", eventID))

	event, err := h.fetchEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, campfire.ErrEventNotFound) {
			h.renderCheckIns(w, r, "", "Event not found")
			return
		}

		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.String("err", err.Error()))
		h.renderCheckIns(w, r, "", "Failed to fetch event details")
		return
	}

	var clubImportedAt time.Time
	if club, err := h.DB.GetClub(ctx, event.ClubID); err == nil {
		clubImportedAt = club.Club.ImportedAt
	}

	eventClubAvatarURL := models.ImageURL(event.Club.AvatarURL, 48)
	clubAvatarURL := models.ImageURL(event.Club.AvatarURL, 32)

	checkedInMembers := getEventMembers(*event, "CHECKED_IN", sortBy)
	acceptedMembers := getEventMembers(*event, "ACCEPTED", sortBy)
	totalCheckIns := len(checkedInMembers)
	totalAccepted := totalCheckIns + len(acceptedMembers)

	if err = h.Templates().ExecuteTemplate(w, "event_details.gohtml", TrackerEventCheckIns{
		Event: models.Event{
			ID:                           event.ID,
			Name:                         event.Name,
			URL:                          fmt.Sprintf("/event/%s", event.ID),
			CoverPhotoURL:                models.ImageURL(event.CoverPhotoURL, 48),
			ClubAvatarURL:                eventClubAvatarURL,
			Details:                      event.Details,
			Address:                      event.Address,
			Location:                     event.Location,
			MapURL:                       models.EventMapURL(event.Location, event.Address),
			Time:                         event.EventTime,
			EndTime:                      event.EventEndTime,
			Finished:                     !event.EventEndTime.IsZero() && event.EventEndTime.Before(time.Now()),
			DiscordInterested:            event.DiscordInterested,
			CampfireLiveEventID:          event.CampfireLiveEventID,
			CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
			Creator:                      models.NewMemberFromCampfire(event.Creator, event.ClubID, 32),
			CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			ImportedAt:                   time.Time{},
		},
		Club: models.Club{
			ID:                           event.ClubID,
			Name:                         event.Club.Name,
			AvatarURL:                    clubAvatarURL,
			Creator:                      models.NewMemberFromCampfire(event.Club.Creator, event.Club.ID, 32),
			CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
			ImportedAt:                   clubImportedAt,
			URL:                          fmt.Sprintf("/tracker/club/%s", event.Club.ID),
		},
		CheckedInMembers: checkedInMembers,
		AcceptedMembers:  acceptedMembers,
		Sort:             sortBy,
		TotalAccepted:    totalAccepted,
		TotalCheckIns:    totalCheckIns,
		TotalCheckInRate: models.CalcCheckInRate(totalAccepted, totalCheckIns),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render event details template", slog.String("err", err.Error()))
	}
}

func (h *handler) renderCheckIns(w http.ResponseWriter, r *http.Request, event string, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "event.gohtml", TrackerCheckIns{
		Event: event,
		Error: errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render check-ins template", slog.String("err", err.Error()))
	}
}

func getEventMembers(event campfire.Event, status string, sortBy string) []models.Member {
	var members []models.Member
	for _, rsvpStatus := range event.RSVPStatuses {
		if rsvpStatus.RSVPStatus != status {
			continue
		}
		member, ok := campfire.FindMember(rsvpStatus.UserID, event)
		if !ok {
			continue
		}
		members = append(members, models.NewMemberFromCampfire(member, event.ClubID, 32))
	}
	sortEventMembers(members, sortBy)
	return members
}
