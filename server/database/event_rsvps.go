package database

import (
	"context"
	"fmt"
)

func (d *Database) InsertEventRSVPs(ctx context.Context, rsvps []EventRSVP) error {
	query := `
		INSERT INTO event_rsvps (rsvp_event_id, rsvp_member_id, rsvp_status)
		VALUES (:rsvp_event_id, :rsvp_member_id, :rsvp_status)
		ON CONFLICT (rsvp_event_id, rsvp_member_id) DO UPDATE SET
			rsvp_status = EXCLUDED.rsvp_status,
			rsvp_imported_at = NOW()
	`

	_, err := d.db.NamedExecContext(ctx, query, rsvps)
	if err != nil {
		return fmt.Errorf("failed to insert or update event RSVPs: %w", err)
	}

	return nil
}
