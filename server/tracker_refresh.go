package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	event, err := s.campfire.FetchFullEvent(ctx, oldEvent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch full event: %w", err)
	}

	rawJson, _ := json.Marshal(event)
	if err = s.db.UpdateEvent(ctx, database.Event{
		ID:                    event.Event.ID,
		Name:                  event.Event.Name,
		Details:               event.Event.Details,
		CoverPhotoURL:         event.Event.CoverPhotoURL,
		EventTime:             event.Event.EventTime,
		EventEndTime:          event.Event.EventEndTime,
		CampfireLiveEventID:   event.Event.CampfireLiveEventID,
		CampfireLiveEventName: event.Event.CampfireLiveEvent.EventName,
		ClubID:                event.Event.ClubID,
		ClubName:              event.Event.Club.Name,
		ClubAvatarURL:         event.Event.Club.AvatarURL,
		RawJSON:               rawJson,
	}); err != nil {
		return fmt.Errorf("failed to update event in database: %w", err)
	}

	var members []database.Member
	for _, rsvpStatus := range event.Event.RSVPStatuses {
		member, _ := campfire.FindMember(rsvpStatus.UserID, *event)

		members = append(members, database.Member{
			ClubMember: database.ClubMember{
				ID:          rsvpStatus.UserID,
				Username:    member.Username,
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
			},
			Status:  rsvpStatus.RSVPStatus,
			EventID: oldEvent.ID,
		})
	}
	if err = s.db.AddMembers(ctx, members); err != nil {
		return fmt.Errorf("failed to add members to database: %w", err)
	}

	return nil
}
