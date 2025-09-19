package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (d *Database) InsertEvents(ctx context.Context, events []Event) error {
	query := `
		INSERT INTO events (event_id, event_name, event_details, event_address, event_location, event_creator_id, event_cover_photo_url, event_time, event_end_time, event_finished, event_discord_interested, event_created_by_community_ambassador, event_campfire_live_event_id, event_campfire_live_event_name, event_club_id, event_raw_json)
		VALUES (:event_id, :event_name, :event_details, :event_address, :event_location, :event_creator_id, :event_cover_photo_url, :event_time, :event_end_time, :event_finished, :event_discord_interested, :event_created_by_community_ambassador, :event_campfire_live_event_id, :event_campfire_live_event_name, :event_club_id, :event_raw_json)
		ON CONFLICT (event_id) DO UPDATE SET
			event_name = EXCLUDED.event_name,
			event_details = EXCLUDED.event_details,
			event_address = EXCLUDED.event_address,
			event_location = EXCLUDED.event_location,
			event_creator_id = EXCLUDED.event_creator_id,
			event_cover_photo_url = EXCLUDED.event_cover_photo_url,
			event_time = EXCLUDED.event_time,
			event_end_time = EXCLUDED.event_end_time,
			event_finished = EXCLUDED.event_finished,
			event_discord_interested = EXCLUDED.event_discord_interested,
			event_created_by_community_ambassador = EXCLUDED.event_created_by_community_ambassador,
			event_campfire_live_event_id = EXCLUDED.event_campfire_live_event_id,
			event_campfire_live_event_name = EXCLUDED.event_campfire_live_event_name,
			event_club_id = EXCLUDED.event_club_id,
			event_imported_at = NOW(),
			event_raw_json = EXCLUDED.event_raw_json
	`

	if _, err := d.db.NamedExecContext(ctx, query, events); err != nil {
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

func (d *Database) GetEvents(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool) ([]EventWithCheckIns, error) {
	query := `
		SELECT events.*, 
			COUNT(event_rsvp_member_id) FILTER (WHERE event_rsvp_status = 'ACCEPTED' OR event_rsvp_status = 'CHECKED_IN') AS accepted,
			COUNT(event_rsvp_member_id) FILTER (WHERE event_rsvp_status = 'CHECKED_IN') AS check_ins
		FROM events
		LEFT JOIN event_rsvps ON event_id = event_rsvp_event_id WHERE event_club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR event_time <= $3)
		AND (NOT $4 OR event_created_by_community_ambassador = TRUE)
		GROUP BY event_id, event_time, event_name
		ORDER BY event_time DESC, event_name DESC
	`

	var events []EventWithCheckIns
	if err := d.db.SelectContext(ctx, &events, query, clubID, from, to, caOnly); err != nil {
		return nil, fmt.Errorf("failed to get events in range: %w", err)
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

func (d *Database) GetBiggestCheckInEvent(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool) (*EventWithCheckIns, error) {
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

	var event EventWithCheckIns
	if err := d.db.GetContext(ctx, &event, query, clubID, from, to, caOnly); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no events found in range: %w", err)
		}
		return nil, fmt.Errorf("failed to get biggest check-in event: %w", err)
	}

	return &event, nil
}

func (d *Database) GetTopEventsByClub(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool, limit int) ([]EventWithCheckIns, error) {
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
        LIMIT CASE WHEN $5 < 0 THEN NULL ELSE $5 END
	`

	var events []EventWithCheckIns
	if err := d.db.SelectContext(ctx, &events, query, clubID, from, to, caOnly, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club events in range: %w", err)
	}

	return events, nil
}

func (d *Database) GetCheckedInClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]Event, error) {
	query := `
		SELECT e.*
		FROM events e
		JOIN event_rsvps re ON e.event_id = re.event_rsvp_event_id
		WHERE e.event_club_id = $1 AND re.event_rsvp_member_id = $2 AND re.event_rsvp_status = 'CHECKED_IN'
		ORDER BY e.event_time DESC, e.event_name, e.event_id
    `

	var events []Event
	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get checked-in club events by user: %w", err)
	}

	return events, nil
}

func (d *Database) GetAcceptedClubEventsByMember(ctx context.Context, clubID string, memberID string) ([]Event, error) {
	query := `
		SELECT e.*
		FROM events e
		JOIN event_rsvps re ON e.event_id = re.event_rsvp_event_id
		WHERE e.event_club_id = $1 AND re.event_rsvp_member_id = $2 AND re.event_rsvp_status = 'ACCEPTED'
		ORDER BY e.event_time DESC, e.event_name, e.event_id
	`

	var events []Event
	if err := d.db.SelectContext(ctx, &events, query, clubID, memberID); err != nil {
		return nil, fmt.Errorf("failed to get accepted club events by user: %w", err)
	}

	return events, nil
}

func (d *Database) GetNextUpdateEvent(ctx context.Context) (*Event, error) {
	// get the first event which has not finished yet and the club is set to auto import events
	query := `
			SELECT events.*
			FROM events
			JOIN clubs ON event_club_id = club_id
			WHERE event_finished = FALSE AND club_auto_event_import = TRUE AND event_imported_at < NOW() - INTERVAL '1 hour'
			ORDER BY event_imported_at, event_end_time
			LIMIT 1
		`

	var event Event
	if err := d.db.GetContext(ctx, &event, query); err != nil {
		return nil, fmt.Errorf("failed to get next event to update: %w", err)
	}

	return &event, nil
}
