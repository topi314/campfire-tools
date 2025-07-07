package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func (s *Server) TrackerRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	slog.InfoContext(ctx, "Received refresh request", slog.String("url", r.URL.String()))
	query := r.URL.Query()
	if query.Get("password") != s.cfg.Auth.RefreshPassword {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	events, err := s.db.GetAllEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get all events", slog.Any("err", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "Successfully retrieved all events", slog.Int("count", len(events)))
	var failed int
	for i, event := range events {
		// Skip events that already have a valid json raw representation
		if bytes.HasPrefix(event.RawJSON, []byte("{")) && bytes.HasSuffix(event.RawJSON, []byte("}")) {
			continue
		}
		if err = s.refreshEvent(ctx, event); err != nil {
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

func (s *Server) refreshEvent(ctx context.Context, oldEvent database.Event) error {
	fullEvent, err := s.campfire.FetchFullEvent(ctx, oldEvent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch full event: %w", err)
	}

	return s.processEvent(ctx, fullEvent)
}

func (s *Server) processEvent(ctx context.Context, fullEvent *campfire.FullEvent) error {
	if err := s.db.InsertMembers(ctx, []database.Member{
		{
			ID:          fullEvent.Event.Creator.ID,
			Username:    fullEvent.Event.Creator.Username,
			DisplayName: fullEvent.Event.Creator.DisplayName,
			AvatarURL:   fullEvent.Event.Creator.AvatarURL,
		},
	}); err != nil {
		return fmt.Errorf("failed to insert creator member: %w", err)
	}

	if err := s.db.InsertClub(ctx, database.Club{
		ID:                           fullEvent.Event.Club.ID,
		Name:                         fullEvent.Event.Club.Name,
		AvatarURL:                    fullEvent.Event.Club.AvatarURL,
		CreatorID:                    fullEvent.Event.Club.Creator.ID,
		CreatedByCommunityAmbassador: fullEvent.Event.Club.CreatedByCommunityAmbassador,
	}); err != nil {
		return fmt.Errorf("failed to insert club: %w", err)
	}

	rawJSON, _ := json.Marshal(fullEvent)

	if err := s.db.CreateEvent(ctx, database.Event{
		ID:                           fullEvent.Event.ID,
		Name:                         fullEvent.Event.Name,
		Details:                      fullEvent.Event.Details,
		Address:                      fullEvent.Event.Address,
		Location:                     fullEvent.Event.Location,
		CreatorID:                    fullEvent.Event.Creator.ID,
		CoverPhotoURL:                fullEvent.Event.CoverPhotoURL,
		EventTime:                    fullEvent.Event.EventTime,
		EventEndTime:                 fullEvent.Event.EventEndTime,
		DiscordInterested:            fullEvent.Event.DiscordInterested,
		CreatedByCommunityAmbassador: fullEvent.Event.CreatedByCommunityAmbassador,
		CampfireLiveEventID:          fullEvent.Event.CampfireLiveEventID,
		CampfireLiveEventName:        fullEvent.Event.CampfireLiveEvent.EventName,
		ClubID:                       fullEvent.Event.ClubID,
		RawJSON:                      rawJSON,
	}); err != nil {
		if errors.Is(err, database.ErrDuplicate) {
			return nil
		}
		return fmt.Errorf("failed to create event: %w", err)
	}

	slog.InfoContext(ctx, "Event added", slog.String("name", fullEvent.Event.Name), slog.String("id", fullEvent.Event.ID))

	var members []database.Member
	for _, member := range fullEvent.Event.Members.Edges {
		members = append(members, database.Member{
			ID:          member.Node.ID,
			Username:    member.Node.Username,
			DisplayName: member.Node.DisplayName,
			AvatarURL:   member.Node.AvatarURL,
		})
	}
	var rsvps []database.EventRSVP
	for _, rsvpStatus := range fullEvent.Event.RSVPStatuses {
		if i := slices.IndexFunc(members, func(member database.Member) bool {
			return member.ID == rsvpStatus.UserID
		}); i == -1 {
			members = append(members, database.Member{
				ID:          rsvpStatus.UserID,
				Username:    "",
				DisplayName: "",
				AvatarURL:   "",
			})
		}
		rsvps = append(rsvps, database.EventRSVP{
			EventID:  fullEvent.Event.ID,
			MemberID: rsvpStatus.UserID,
			Status:   rsvpStatus.RSVPStatus,
		})
	}

	if err := s.db.InsertMembers(ctx, members); err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}

	if err := s.db.InsertEventRSVPs(ctx, rsvps); err != nil {
		return fmt.Errorf("failed to add event RSVPs: %w", err)
	}

	slog.InfoContext(ctx, "Members added for event", slog.String("name", fullEvent.Event.Name), slog.String("id", fullEvent.Event.ID), slog.Int("members", len(members)), slog.Int("rsvps", len(rsvps)))
	return nil
}
