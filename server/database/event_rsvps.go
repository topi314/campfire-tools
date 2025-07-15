package database

import (
	"context"
	"fmt"
)

func (d *Database) InsertEventRSVPs(ctx context.Context, rsvps []EventRSVP) error {
	query := `
		INSERT INTO event_rsvps (event_id, member_id, status)
		VALUES (:event_id, :member_id, :status)
		ON CONFLICT (event_id, member_id) DO UPDATE SET
			status = EXCLUDED.status,
			imported_at = NOW()
	`

	_, err := d.db.NamedExecContext(ctx, query, rsvps)
	if err != nil {
		return fmt.Errorf("failed to insert or update event RSVPs: %w", err)
	}

	return nil
}
