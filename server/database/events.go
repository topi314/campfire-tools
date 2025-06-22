package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicate = errors.New("duplicate entry")

func (d *Database) AddEvent(ctx context.Context, event Event) error {
	query := `
		INSERT INTO events (id, name, details, cover_photo_url, event_time, event_end_time, campfire_live_event_id, campfire_live_event_name, club_id, club_name, club_avatar_url) 
		VALUES 
    	(:id, :name, :details, :cover_photo_url, :event_time, :event_end_time, :campfire_live_event_id, :campfire_live_event_name, :club_id, :club_name, :club_avatar_url)`
	if _, err := d.db.NamedExecContext(ctx, query, event); err != nil {
		var sqlErr *pgconn.PgError
		if errors.As(err, &sqlErr) && sqlErr.Code == "23505" { // Unique violation
			return ErrDuplicate
		}

		return fmt.Errorf("failed to add event: %w", err)
	}
	return nil
}

func (d *Database) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	var event Event
	if err := d.db.GetContext(ctx, &event, "SELECT * FROM events WHERE id = $1", eventID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("event not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	return &event, nil
}

func (d *Database) GetEvents(ctx context.Context, clubID string) ([]Event, error) {
	var events []Event
	if err := d.db.SelectContext(ctx, &events, "SELECT * FROM events WHERE club_id = $1 ORDER BY event_time DESC", clubID); err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	return events, nil
}
