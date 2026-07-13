package database

import (
	"context"
	"fmt"

	"github.com/lib/pq"
)

// GetLiveEventRSVPs returns one row per (club, live event, member) for the given
// clubs and set of Campfire live event IDs. StatusRank is the member's best
// status within that live event at that club (2 = checked in, 1 = accepted
// only). Members who attended multiple meetups within the same live event at a
// club are deduplicated.
func (d *Database) GetLiveEventRSVPs(ctx context.Context, clubIDs []string, liveEventIDs []string) ([]LiveEventMemberRSVP, error) {
	query := `
		SELECT e.event_club_id                AS club_id,
		       e.event_campfire_live_event_id AS live_event_id,
		       m.*,
		       MAX(CASE WHEN er.event_rsvp_status = 'CHECKED_IN' THEN 2
		                WHEN er.event_rsvp_status = 'ACCEPTED'   THEN 1 ELSE 0 END) AS status_rank
		FROM events e
		JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
		JOIN members m ON er.event_rsvp_member_id = m.member_id
		WHERE e.event_club_id = ANY($1) AND e.event_campfire_live_event_id = ANY($2)
		GROUP BY e.event_club_id, e.event_campfire_live_event_id, m.member_id
	`

	var rsvps []LiveEventMemberRSVP
	if err := d.db.SelectContext(ctx, &rsvps, query, pq.Array(clubIDs), pq.Array(liveEventIDs)); err != nil {
		return nil, fmt.Errorf("failed to get live event rsvps: %w", err)
	}

	return rsvps, nil
}
