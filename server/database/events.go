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

func (d *Database) CreateEvent(ctx context.Context, event Event) error {
	query := `
		INSERT INTO events (id, name, details, address, location, creator_id, cover_photo_url, event_time, event_end_time, discord_interested, created_by_community_ambassador, campfire_live_event_id, campfire_live_event_name, club_id, raw_json)
		VALUES (:id, :name, :details, :address, :location, :creator_id, :cover_photo_url, :event_time, :event_end_time, :discord_interested, :created_by_community_ambassador, :campfire_live_event_id, :campfire_live_event_name, :club_id, :raw_json)
		`

	if _, err := d.db.NamedExecContext(ctx, query, event); err != nil {
		var sqlErr *pgconn.PgError
		if errors.As(err, &sqlErr) && sqlErr.Code == "23505" { // Unique violation
			return ErrDuplicate
		}

		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

func (d *Database) UpdateEvent(ctx context.Context, event Event) error {
	query := `
		UPDATE events
		SET name = :name, details = :details, address = :address, location = :location,
		    creator_id = :creator_id, cover_photo_url = :cover_photo_url, event_time = :event_time, 
		    event_end_time = :event_end_time, discord_interested = :discord_interested, 
		    created_by_community_ambassador = :created_by_community_ambassador, campfire_live_event_id = :campfire_live_event_id, 
		    campfire_live_event_name = :campfire_live_event_name, club_id = :club_id, raw_json = :raw_json
		WHERE id = :id
	`

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
	query := `
		SELECT * FROM events
		WHERE club_id = $1
		ORDER BY event_time DESC, name DESC
	`

	var events []Event
	if err := d.db.SelectContext(ctx, &events, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	return events, nil
}

func (d *Database) GetAllEvents(ctx context.Context) ([]Event, error) {
	query := `
		SELECT * FROM events
		ORDER BY event_time DESC, name DESC
	`

	var events []Event
	if err := d.db.SelectContext(ctx, &events, query); err != nil {
		return nil, fmt.Errorf("failed to get all events: %w", err)
	}

	return events, nil
}

func (d *Database) GetTopEventsByClub(ctx context.Context, clubID string, from time.Time, to time.Time, limit int) ([]TopEvent, error) {
	query := `
        SELECT
            e.*, 
            COUNT(er.member_id) FILTER (WHERE er.status = 'ACCEPTED' OR er.status = 'CHECKED_IN') AS accepted,
            COUNT(er.member_id) FILTER (WHERE er.status = 'CHECKED_IN') AS check_ins
        FROM events e
        LEFT JOIN event_rsvps er ON e.id = er.event_id
        WHERE e.club_id = $1
        AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
        GROUP BY e.id, e.event_time, e.name
        ORDER BY check_ins DESC, accepted DESC, e.event_time DESC, e.name DESC 
        LIMIT $4
	`

	var events []TopEvent
	if err := d.db.SelectContext(ctx, &events, query, clubID, from, to, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club events in range: %w", err)
	}

	return events, nil
}

func (d *Database) GetCheckedInClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]Event, error) {
	var events []Event
	query := `
		SELECT e.*
		FROM events e
		JOIN event_rsvps re ON e.id = re.event_id
		WHERE e.club_id = $1 AND re.member_id = $2 AND re.status = 'CHECKED_IN'
		ORDER BY e.event_time DESC, e.name
    `

	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get checked-in club events by user: %w", err)
	}

	return events, nil
}

func (d *Database) GetAcceptedClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]Event, error) {
	var events []Event
	query := `
		SELECT e.*
		FROM events e
		JOIN event_rsvps re ON e.id = re.event_id
		WHERE e.club_id = $1 AND re.member_id = $2 AND re.status = 'ACCEPTED'
		ORDER BY e.event_time DESC, e.name
	`

	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get accepted club events by user: %w", err)
	}

	return events, nil
}
