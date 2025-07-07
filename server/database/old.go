package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type OldEvent struct {
	ID            string    `db:"id"`
	Name          string    `db:"name"`
	Details       string    `db:"details"`
	CoverPhotoURL string    `db:"cover_photo_url"`
	EventTime     time.Time `db:"event_time"`
	EventEndTime  time.Time `db:"event_end_time"`

	CampfireLiveEventID   string `db:"campfire_live_event_id"`
	CampfireLiveEventName string `db:"campfire_live_event_name"`

	ClubID        string `db:"club_id"`
	ClubName      string `db:"club_name"`
	ClubAvatarURL string `db:"club_avatar_url"`

	RawJSON json.RawMessage `db:"raw_json"`
}

func (d *Database) GetOldEvents(ctx context.Context) ([]OldEvent, error) {
	var events []OldEvent
	if err := d.db.SelectContext(ctx, &events, `SELECT * FROM events_old`); err != nil {
		return nil, fmt.Errorf("failed to get old events: %w", err)
	}

	return events, nil
}
