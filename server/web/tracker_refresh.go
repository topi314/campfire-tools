package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) TrackerRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	slog.InfoContext(ctx, "Received refresh request", slog.String("url", r.URL.String()))
	query := r.URL.Query()
	if query.Get("password") != h.Cfg.Auth.RefreshPassword {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	events, err := h.DB.GetAllEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get all events", slog.Any("err", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "Successfully retrieved all events", slog.Int("count", len(events)))
	var failed int
	for i, event := range events {
		if err = h.refreshEvent(ctx, event); err != nil {
			slog.ErrorContext(ctx, "Failed to refresh event", slog.String("event_id", event.ID), slog.Int("index", i+1), slog.Int("total", len(events)), slog.Any("err", err))
			failed++
			continue
		}
		slog.InfoContext(ctx, "Successfully refreshed event", slog.String("event_id", event.ID), slog.Int("index", i+1), slog.Int("total", len(events)))
		<-time.After(1 * time.Second)
	}
	if failed > 0 {
		slog.WarnContext(ctx, "Some events failed to refresh", slog.Int("failed_count", failed))
	}

	if _, err = fmt.Fprintf(w, "Refreshed %d events successfully, %d failed", len(events)-failed, failed); err != nil {
		slog.ErrorContext(ctx, "Failed to write refresh response", slog.Any("err", err))
		return
	}
}

func (h *handler) refreshEvent(ctx context.Context, oldEvent database.Event) error {
	event, err := h.Campfire.GetEvent(ctx, oldEvent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch full event: %w", err)
	}

	return h.processEvent(ctx, *event)
}

func (h *handler) processEvent(ctx context.Context, event campfire.Event) error {
	members := []database.Member{
		{
			ID:          event.Creator.ID,
			Username:    event.Creator.Username,
			DisplayName: event.Creator.DisplayName,
			AvatarURL:   event.Creator.AvatarURL,
			RawJSON:     event.Creator.Raw,
		},
	}
	if !slices.ContainsFunc(members, func(item database.Member) bool {
		return item.ID == event.Club.Creator.ID
	}) {
		members = append(members, database.Member{
			ID:          event.Club.Creator.ID,
			Username:    event.Club.Creator.Username,
			DisplayName: event.Club.Creator.DisplayName,
			AvatarURL:   event.Club.Creator.AvatarURL,
			RawJSON:     event.Club.Creator.Raw,
		})
	}

	if err := h.DB.InsertMembers(ctx, members); err != nil {
		return fmt.Errorf("failed to insert creator member: %w", err)
	}

	if err := h.DB.InsertClub(ctx, database.Club{
		ID:                           event.Club.ID,
		Name:                         event.Club.Name,
		AvatarURL:                    event.Club.AvatarURL,
		CreatorID:                    event.Club.Creator.ID,
		CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
		RawJSON:                      event.Club.Raw,
	}); err != nil {
		return fmt.Errorf("failed to insert club: %w", err)
	}

	if err := h.DB.InsertEvent(ctx, database.Event{
		ID:                           event.ID,
		Name:                         event.Name,
		Details:                      event.Details,
		Address:                      event.Address,
		Location:                     event.Location,
		CreatorID:                    event.Creator.ID,
		CoverPhotoURL:                event.CoverPhotoURL,
		Time:                         event.EventTime,
		EndTime:                      event.EventEndTime,
		DiscordInterested:            event.DiscordInterested,
		CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
		CampfireLiveEventID:          event.CampfireLiveEventID,
		CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
		ClubID:                       event.ClubID,
		RawJSON:                      event.Raw,
	}); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	slog.InfoContext(ctx, "Event added", slog.String("name", event.Name), slog.String("id", event.ID))

	var eventMembers []database.Member
	for _, member := range event.Members.Edges {
		eventMembers = append(eventMembers, database.Member{
			ID:          member.Node.ID,
			Username:    member.Node.Username,
			DisplayName: member.Node.DisplayName,
			AvatarURL:   member.Node.AvatarURL,
			RawJSON:     member.Node.Raw,
		})
	}
	var rsvps []database.EventRSVP
	for _, rsvpStatus := range event.RSVPStatuses {
		if i := slices.IndexFunc(eventMembers, func(member database.Member) bool {
			return member.ID == rsvpStatus.UserID
		}); i == -1 {
			eventMembers = append(eventMembers, database.Member{
				ID:          rsvpStatus.UserID,
				Username:    "",
				DisplayName: "",
				AvatarURL:   "",
				RawJSON:     []byte("{}"),
			})
		}
		rsvps = append(rsvps, database.EventRSVP{
			EventID:  event.ID,
			MemberID: rsvpStatus.UserID,
			Status:   rsvpStatus.RSVPStatus,
		})
	}

	if err := h.DB.InsertMembers(ctx, eventMembers); err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}

	if err := h.DB.InsertEventRSVPs(ctx, rsvps); err != nil {
		return fmt.Errorf("failed to add event RSVPs: %w", err)
	}

	slog.InfoContext(ctx, "Members added for event", slog.String("name", event.Name), slog.String("id", event.ID), slog.Int("members", len(members)), slog.Int("rsvps", len(rsvps)))
	return nil
}
