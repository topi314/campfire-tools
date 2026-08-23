package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubStatsTotals(ctx context.Context, clubID string) (*ClubStatsTotals, error) {
	query := `
		SELECT
			(SELECT COUNT(*) FROM events WHERE event_club_id = $1) AS events,
			(SELECT COUNT(DISTINCT er.event_rsvp_member_id)
			 FROM event_rsvps er
			 JOIN events e ON er.event_rsvp_event_id = e.event_id
			 WHERE e.event_club_id = $1) AS unique_participants,
			(SELECT COUNT(*)
			 FROM event_rsvps er
			 JOIN events e ON er.event_rsvp_event_id = e.event_id
			 WHERE e.event_club_id = $1) AS total_rsvps,
			(SELECT COUNT(*)
			 FROM event_rsvps er
			 JOIN events e ON er.event_rsvp_event_id = e.event_id
			 WHERE e.event_club_id = $1 AND er.event_rsvp_status = 'CHECKED_IN') AS total_check_ins,
			(SELECT COUNT(*)
			 FROM event_rsvps er
			 JOIN events e ON er.event_rsvp_event_id = e.event_id
			 WHERE e.event_club_id = $1 AND er.event_rsvp_status = 'ACCEPTED') AS total_accepted,
			(SELECT COUNT(*)
			 FROM event_rsvps er
			 JOIN events e ON er.event_rsvp_event_id = e.event_id
			 WHERE e.event_club_id = $1 AND er.event_rsvp_status = 'DECLINED') AS total_declined,
			COALESCE((SELECT MIN(event_time) FROM events WHERE event_club_id = $1), '0001-01-01'::timestamp) AS first_event_date,
			COALESCE((SELECT MAX(event_time) FROM events WHERE event_club_id = $1), '0001-01-01'::timestamp) AS last_event_date
	`

	var totals ClubStatsTotals
	if err := d.db.GetContext(ctx, &totals, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club stats totals: %w", err)
	}

	return &totals, nil
}

func (d *Database) GetClubStatsEvents(ctx context.Context, clubID string) ([]ClubStatsEventRow, error) {
	query := `
		SELECT
			e.*,
			COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'ACCEPTED') AS accepted,
			COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') AS check_ins,
			COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'DECLINED') AS declined,
			COUNT(er.event_rsvp_member_id) AS rsvps
		FROM events e
		LEFT JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
		WHERE e.event_club_id = $1
		GROUP BY e.event_id
		ORDER BY e.event_time ASC, e.event_name ASC, e.event_id ASC
	`

	var events []ClubStatsEventRow
	if err := d.db.SelectContext(ctx, &events, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club stats events: %w", err)
	}

	return events, nil
}

func (d *Database) GetClubStatsMonthly(ctx context.Context, clubID string) ([]ClubStatsMonthRow, error) {
	query := `
		WITH monthly AS (
			SELECT
				to_char(date_trunc('month', e.event_time), 'YYYY-MM') AS month,
				COUNT(DISTINCT e.event_id) AS events,
				COUNT(er.event_rsvp_member_id) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') AS check_ins,
				COUNT(er.event_rsvp_member_id) AS rsvps
			FROM events e
			LEFT JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
			WHERE e.event_club_id = $1
			GROUP BY date_trunc('month', e.event_time)
		),
		new_participants AS (
			SELECT
				to_char(date_trunc('month', first_seen), 'YYYY-MM') AS month,
				COUNT(*) AS new_participants
			FROM (
				SELECT er.event_rsvp_member_id, MIN(e.event_time) AS first_seen
				FROM event_rsvps er
				JOIN events e ON er.event_rsvp_event_id = e.event_id
				WHERE e.event_club_id = $1
				GROUP BY er.event_rsvp_member_id
			) member_first
			GROUP BY date_trunc('month', first_seen)
		)
		SELECT
			m.month,
			m.events,
			m.check_ins,
			m.rsvps,
			COALESCE(np.new_participants, 0) AS new_participants
		FROM monthly m
		LEFT JOIN new_participants np ON m.month = np.month
		ORDER BY m.month ASC
	`

	var months []ClubStatsMonthRow
	if err := d.db.SelectContext(ctx, &months, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club stats monthly: %w", err)
	}

	return months, nil
}

func (d *Database) GetClubMemberCheckInCounts(ctx context.Context, clubID string) ([]ClubMemberCheckInCount, error) {
	query := `
		SELECT
			er.event_rsvp_member_id AS member_id,
			COUNT(*) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') AS check_ins
		FROM event_rsvps er
		JOIN events e ON er.event_rsvp_event_id = e.event_id
		WHERE e.event_club_id = $1
		GROUP BY er.event_rsvp_member_id
		HAVING COUNT(*) FILTER (WHERE er.event_rsvp_status = 'CHECKED_IN') > 0
	`

	var counts []ClubMemberCheckInCount
	if err := d.db.SelectContext(ctx, &counts, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club member check-in counts: %w", err)
	}

	return counts, nil
}
