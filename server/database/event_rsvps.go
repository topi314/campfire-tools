package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
)

const batchSize = 10_000

func (d *Database) InsertEventRSVPs(ctx context.Context, rsvps []EventRSVP) error {
	if len(rsvps) == 0 {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.ErrorContext(ctx, "failed to rollback transaction", "error", err)
		}
	}()

	for chunk := range slices.Chunk(rsvps, batchSize) {
		query := `
			INSERT INTO event_rsvps (event_rsvp_event_id, event_rsvp_member_id, event_rsvp_status, event_rsvp_imported_at)
			VALUES (:event_rsvp_event_id, :event_rsvp_member_id, :event_rsvp_status, now())
			ON CONFLICT (event_rsvp_event_id, event_rsvp_member_id) DO UPDATE SET
				event_rsvp_status = EXCLUDED.event_rsvp_status,
				event_rsvp_imported_at = now()
			`

		_, err = d.db.NamedExecContext(ctx, query, chunk)
		if err != nil {
			return fmt.Errorf("failed to insert event RSVPs: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
