package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
)

type TrackerCheckIns struct {
	Event string
	Error string
}

type TrackerEventCheckIns struct {
	Event

	Club             Club
	CheckedInMembers []Member
	AcceptedMembers  []Member
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

		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event", event), slog.String("error", err.Error()))
		h.renderCheckIns(w, r, "", "Failed to fetch event details")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/event/%s", campfireEvent.ID), http.StatusSeeOther)
}

func (h *handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	slog.InfoContext(ctx, "Fetching event check-ins", slog.String("event_id", eventID))

	event, err := h.fetchEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, campfire.ErrEventNotFound) {
			h.renderCheckIns(w, r, "", "Event not found")
			return
		}

		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.String("error", err.Error()))
		h.renderCheckIns(w, r, "", "Failed to fetch event details")
		return
	}

	var clubImportedAt time.Time
	if club, err := h.DB.GetClub(ctx, event.ClubID); err == nil {
		clubImportedAt = club.Club.ImportedAt
	}

	if err = h.Templates().ExecuteTemplate(w, "event_details.gohtml", TrackerEventCheckIns{
		Event: Event{
			ID:                           event.ID,
			Name:                         event.Name,
			URL:                          fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL:                imageURL(event.CoverPhotoURL, 48),
			Details:                      event.Details,
			Time:                         event.EventTime,
			EndTime:                      event.EventEndTime,
			CampfireLiveEventID:          event.CampfireLiveEventID,
			CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
			Creator:                      newMemberFromCampfire(event.Creator, event.ClubID, 32),
			CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			ImportedAt:                   time.Time{},
		},
		Club: Club{
			ID:                           event.ClubID,
			Name:                         event.Club.Name,
			AvatarURL:                    imageURL(event.Club.AvatarURL, 32),
			Creator:                      newMemberFromCampfire(event.Club.Creator, event.Club.ID, 32),
			CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
			ImportedAt:                   clubImportedAt,
			URL:                          fmt.Sprintf("/tracker/club/%s", event.Club.ID),
		},
		CheckedInMembers: getEventMembers(*event, "CHECKED_IN"),
		AcceptedMembers:  getEventMembers(*event, "ACCEPTED"),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render event details template", slog.String("error", err.Error()))
	}
}

func (h *handler) renderCheckIns(w http.ResponseWriter, r *http.Request, event string, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "event.gohtml", TrackerCheckIns{
		Event: event,
		Error: errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render check-ins template", slog.String("error", err.Error()))
	}
}

func getEventMembers(event campfire.Event, status string) []Member {
	var members []Member
	for _, rsvpStatus := range event.RSVPStatuses {
		if rsvpStatus.RSVPStatus != status {
			continue
		}
		member, ok := campfire.FindMember(rsvpStatus.UserID, event)
		if !ok {
			continue
		}
		members = append(members, newMemberFromCampfire(member, event.ClubID, 32))
	}
	slices.SortFunc(members, func(a, b Member) int {
		if a.DisplayName != b.DisplayName {
			return strings.Compare(a.DisplayName, b.DisplayName)
		}
		return strings.Compare(a.Username, b.Username)
	})
	return members
}
