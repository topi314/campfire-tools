package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (s *Server) updateEvents() {
	for {
		s.doUpdateEvents()
		time.Sleep(10 * time.Second)
	}
}

func (s *Server) doUpdateEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.doUpdateNextEvent(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to update next event", slog.Any("err", err))
	}
}

func (s *Server) doUpdateNextEvent(ctx context.Context) error {
	event, err := s.DB.GetNextUpdateEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	slog.InfoContext(ctx, "Updating event", slog.String("club_id", event.ClubID), slog.String("event_id", event.ID), slog.String("event_name", event.Name))

	importErr := s.importEvent(ctx, event.ID)

	if err = s.DB.UpdateEventLastAutoImported(ctx, event.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to update event last auto import", slog.String("event_id", event.ID), slog.Any("err", err))
	}

	return importErr
}

func (s *Server) importEvent(ctx context.Context, eventID string) error {
	event, err := s.Campfire.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, campfire.ErrEventNotFound) {
			if err = s.DB.DeleteEvent(ctx, eventID); err != nil {
				slog.ErrorContext(ctx, "Failed to delete not found event", slog.String("event_id", eventID), slog.Any("err", err))
			}
			return nil
		}
		return err
	}

	if err = s.ProcessFullEventImport(ctx, *event, true); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Updated event",
		slog.String("club_id", event.Club.ID),
		slog.String("event_id", event.ID),
		slog.String("event_name", event.Name),
		slog.Int("rsvps", len(event.RSVPStatuses)),
		slog.Int("members", len(event.Members.Edges)),
	)

	return nil
}
