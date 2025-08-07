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

func (h *handler) CheckIns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	event := query.Get("event")

	slog.InfoContext(ctx, "Check-ins request received", slog.String("url", r.URL.String()), slog.String("event", event))

	h.renderCheckIns(w, r, event, "")
}

func (h *handler) ShowCheckIns(w http.ResponseWriter, r *http.Request) {
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

	http.Redirect(w, r, fmt.Sprintf("/check-ins/%s", campfireEvent.ID), http.StatusSeeOther)
}

func (h *handler) GetCheckIns(w http.ResponseWriter, r *http.Request) {
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

	if err = h.Templates().ExecuteTemplate(w, "event_check_ins.gohtml", TrackerEventCheckIns{
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
			Creator:                      newMemberFromCampfire(event.Creator, event.ClubID),
			CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			ImportedAt:                   time.Now(),
		},
		Club: Club{
			ID:                           event.ClubID,
			Name:                         event.Club.Name,
			AvatarURL:                    imageURL(event.Club.AvatarURL, 32),
			Creator:                      newMemberFromCampfire(event.Club.Creator, event.Club.ID),
			CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
			ImportedAt:                   time.Now(),
			URL:                          fmt.Sprintf("/tracker/club/%s", event.Club.ID),
		},
		CheckedInMembers: getEventMembers(*event, "CHECKED_IN"),
		AcceptedMembers:  getEventMembers(*event, "ACCEPTED"),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render event check-ins template", slog.String("error", err.Error()))
	}
}

func (h *handler) renderCheckIns(w http.ResponseWriter, r *http.Request, event string, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "check_ins.gohtml", TrackerCheckIns{
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
		members = append(members, newMemberFromCampfire(member, event.ClubID))
	}
	slices.SortFunc(members, func(a, b Member) int {
		if a.DisplayName != b.DisplayName {
			return strings.Compare(a.DisplayName, b.DisplayName)
		}
		return strings.Compare(a.Username, b.Username)
	})
	return members
}
