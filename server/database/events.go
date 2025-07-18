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
		INSERT INTO events (event_id, event_name, event_details, event_address, event_location, event_creator_id, event_cover_photo_url, event_time, event_end_time, event_discord_interested, event_created_by_community_ambassador, event_campfire_live_event_id, event_campfire_live_event_name, event_club_id, event_raw_json)
		VALUES (:event_id, :event_name, :event_details, :event_address, :event_location, :event_creator_id, :event_cover_photo_url, :event_time, :event_end_time, :event_discord_interested, :event_created_by_community_ambassador, :event_campfire_live_event_id, :event_campfire_live_event_name, :event_club_id, :event_raw_json)
		ON CONFLICT (event_id) DO UPDATE SET
			event_name = EXCLUDED.event_name,
			event_details = EXCLUDED.event_details,
			event_address = EXCLUDED.event_address,
			event_location = EXCLUDED.event_location,
			event_creator_id = EXCLUDED.event_creator_id,
			event_cover_photo_url = EXCLUDED.event_cover_photo_url,
			event_time = EXCLUDED.event_time,
			event_end_time = EXCLUDED.event_end_time,
			event_discord_interested = EXCLUDED.event_discord_interested,
			event_created_by_community_ambassador = EXCLUDED.event_created_by_community_ambassador,
			event_campfire_live_event_id = EXCLUDED.event_campfire_live_event_id,
			event_campfire_live_event_name = EXCLUDED.event_campfire_live_event_name,
			event_club_id = EXCLUDED.event_club_id,
			event_imported_at = NOW(),
			event_raw_json = EXCLUDED.event_raw_json
	`

	if _, err := d.db.NamedExecContext(ctx, query, event); err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

func (d *Database) GetEvent(ctx context.Context, eventID string) (*EventWithCreator, error) {
	query := `
		SELECT events.*, members.*
		FROM events
		JOIN members ON events.event_creator_id = members.member_id
		WHERE events.event_id = $1
	`

	var event EventWithCreator
	if err := d.db.GetContext(ctx, &event, query, eventID); err != nil {
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
		WHERE event_club_id = $1
		ORDER BY event_time DESC, event_name DESC
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
		ORDER BY event_time DESC, event_name DESC
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
			COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'ACCEPTED' OR er.event_rsvp_status = 'CHECKED_IN') AS accepted,
			COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') AS check_ins
		FROM events e
		LEFT JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
		WHERE e.event_club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		AND (NOT $4 OR e.event_created_by_community_ambassador = TRUE)
		GROUP BY e.event_id, e.event_time, e.event_name
		ORDER BY check_ins DESC, e.event_time DESC, e.event_name DESC, e.event_id
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
            COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'ACCEPTED' OR er.event_rsvp_status = 'CHECKED_IN') AS accepted,
            COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') AS check_ins
        FROM events e
        LEFT JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
        WHERE e.event_club_id = $1
        AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
        AND (NOT $4 OR e.event_created_by_community_ambassador = TRUE)
        GROUP BY e.event_id, e.event_time, e.event_name
        ORDER BY check_ins DESC, accepted DESC, e.event_time DESC, e.event_name DESC, e.event_id
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
		JOIN event_rsvps re ON e.event_id = re.event_rsvp_event_id
		WHERE e.event_club_id = $1 AND re.event_rsvp_member_id = $2 AND re.event_rsvp_status = 'CHECKED_IN'
		ORDER BY e.event_time DESC, e.event_name, e.event_id
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
		JOIN event_rsvps re ON e.event_id = re.event_rsvp_event_id
		WHERE e.event_club_id = $1 AND re.event_rsvp_member_id = $2 AND re.event_rsvp_status = 'ACCEPTED'
		ORDER BY e.event_time DESC, e.event_name, e.event_id
	`

	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get accepted club events by user: %w", err)
	}

	return events, nil
}
