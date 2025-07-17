package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (d *Database) InsertEvent(ctx context.Context, event Event) error {
	query := `
		INSERT INTO events (id, name, details, address, location, creator_id, cover_photo_url, event_time, event_end_time, discord_interested, created_by_community_ambassador, campfire_live_event_id, campfire_live_event_name, club_id, raw_json)
		VALUES (:id, :name, :details, :address, :location, :creator_id, :cover_photo_url, :event_time, :event_end_time, :discord_interested, :created_by_community_ambassador, :campfire_live_event_id, :campfire_live_event_name, :club_id, :raw_json)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			details = EXCLUDED.details,
			address = EXCLUDED.address,
			location = EXCLUDED.location,
			creator_id = EXCLUDED.creator_id,
			cover_photo_url = EXCLUDED.cover_photo_url,
			event_time = EXCLUDED.event_time,
			event_end_time = EXCLUDED.event_end_time,
			discord_interested = EXCLUDED.discord_interested,
			created_by_community_ambassador = EXCLUDED.created_by_community_ambassador,
			campfire_live_event_id = EXCLUDED.campfire_live_event_id,
			campfire_live_event_name = EXCLUDED.campfire_live_event_name,
			club_id = EXCLUDED.club_id,
			imported_at = NOW(),
			raw_json = EXCLUDED.raw_json
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

func (d *Database) GetBiggestCheckInEvent(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool) (*TopEvent, error) {
	query := `
		SELECT e.*, 
			COUNT(er.member_id) FILTER (WHERE er.status = 'ACCEPTED' OR er.status = 'CHECKED_IN') AS accepted,
			COUNT(er.member_id) FILTER (WHERE er.status = 'CHECKED_IN') AS check_ins
		FROM events e
		LEFT JOIN event_rsvps er ON e.id = er.event_id
		WHERE e.club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		AND (NOT $4 OR e.created_by_community_ambassador = TRUE)
		GROUP BY e.id, e.event_time, e.name
		ORDER BY check_ins DESC, e.event_time DESC, e.name DESC, e.id
		LIMIT 1
	`

	var event TopEvent
	if err := d.db.GetContext(ctx, &event, query, clubID, from, to, caOnly); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no events found in range: %w", err)
		}
		return nil, fmt.Errorf("failed to get biggest check-in event: %w", err)
	}

	return &event, nil
}

func (d *Database) GetTopEventsByClub(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool, limit int) ([]TopEvent, error) {
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
        AND (NOT $4 OR e.created_by_community_ambassador = TRUE)
        GROUP BY e.id, e.event_time, e.name
        ORDER BY check_ins DESC, accepted DESC, e.event_time DESC, e.name DESC, e.id
        LIMIT $5
	`

	var events []TopEvent
	if err := d.db.SelectContext(ctx, &events, query, clubID, from, to, caOnly, limit); err != nil {
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
		ORDER BY e.event_time DESC, e.name, e.id
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
		ORDER BY e.event_time DESC, e.name, e.id
	`

	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get accepted club events by user: %w", err)
	}

	return events, nil
}
