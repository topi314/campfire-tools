package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicate = errors.New("duplicate entry")

func (d *Database) AddEvent(ctx context.Context, event Event) error {
	query := `
		INSERT INTO events (id, name, details, cover_photo_url, event_time, event_end_time, campfire_live_event_id, campfire_live_event_name, club_id, club_name, club_avatar_url, raw_json) 
		VALUES
		(:id, :name, :details, :cover_photo_url, :event_time, :event_end_time, :campfire_live_event_id, :campfire_live_event_name, :club_id, :club_name, :club_avatar_url, :raw_json)
		`
	if _, err := d.db.NamedExecContext(ctx, query, event); err != nil {
		var sqlErr *pgconn.PgError
		if errors.As(err, &sqlErr) && sqlErr.Code == "23505" { // Unique violation
			return ErrDuplicate
		}

		return fmt.Errorf("failed to add event: %w", err)
	}

	return nil
}

func (d *Database) UpdateEvent(ctx context.Context, event Event) error {
	query := `
		UPDATE events 
		SET name = :name, details = :details, cover_photo_url = :cover_photo_url, event_time = :event_time, event_end_time = :event_end_time, 
		    campfire_live_event_id = :campfire_live_event_id, campfire_live_event_name = :campfire_live_event_name, club_id = :club_id, 
		    club_name = :club_name, club_avatar_url = :club_avatar_url, raw_json = :raw_json
		WHERE id = :id`
	if _, err := d.db.NamedExecContext(ctx, query, event); err != nil {
		return fmt.Errorf("failed to update event: %w", err)
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

func (d *Database) GetAllEvents(ctx context.Context) ([]Event, error) {
	var events []Event
	if err := d.db.SelectContext(ctx, &events, "SELECT * FROM events ORDER BY event_time DESC"); err != nil {
		return nil, fmt.Errorf("failed to get all events: %w", err)
	}

	return events, nil
}

func (d *Database) GetTopClubEvents(ctx context.Context, clubID string, from time.Time, to time.Time, limit int) ([]TopEvent, error) {
	query := `
		SELECT e.*, COUNT(CASE WHEN m.status != 'DECLINED' THEN 1 END) AS accepted, COUNT(CASE WHEN m.status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM events e
		LEFT JOIN members m ON e.id = m.event_id
		WHERE e.club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		GROUP BY e.id
		ORDER BY check_ins DESC, accepted DESC
		LIMIT $4`
	var events []TopEvent
	if err := d.db.SelectContext(ctx, &events, query, clubID, from, to, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club events in range: %w", err)
	}

	return events, nil
}

func (d *Database) GetCheckedInClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]MemberEvent, error) {
	var events []MemberEvent
	query := `
		SELECT e.*, m.status
		FROM events e
		JOIN members m ON e.id = m.event_id
		WHERE e.club_id = $1 AND m.id = $2 AND m.status = 'CHECKED_IN'
		ORDER BY e.event_time DESC`
	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get club events by member: %w", err)
	}

	return events, nil
}

func (d *Database) GetAcceptedClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]MemberEvent, error) {
	var events []MemberEvent
	query := `
		SELECT e.*, m.status
		FROM events e
		JOIN members m ON e.id = m.event_id
		WHERE e.club_id = $1 AND m.id = $2 AND m.status = 'ACCEPTED'
		ORDER BY e.event_time DESC`
	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get accepted club events by member: %w", err)
	}

	return events, nil
}
