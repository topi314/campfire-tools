package database

import (
	"context"
	"fmt"
)

func (d *Database) InsertEventRSVPs(ctx context.Context, rsvps []EventRSVP) error {
	query := `
		INSERT INTO event_rsvps (event_rsvp_event_id, event_rsvp_member_id, event_rsvp_status)
		VALUES (:event_rsvp_event_id, :event_rsvp_member_id, :event_rsvp_status)
		ON CONFLICT (event_rsvp_event_id, event_rsvp_member_id) DO UPDATE SET
			event_rsvp_status = EXCLUDED.event_rsvp_status,
			event_rsvp_imported_at = NOW()
	`

	_, err := d.db.NamedExecContext(ctx, query, rsvps)
	if err != nil {
		return fmt.Errorf("failed to insert or update event RSVPs: %w", err)
	}

	return nil
}
