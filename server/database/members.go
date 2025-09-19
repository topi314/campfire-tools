package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
)

func (d *Database) GetMember(ctx context.Context, memberID string) (*Member, error) {
	query := `
		SELECT *
		FROM members
		WHERE member_id = $1
	`

	var member Member
	if err := d.db.GetContext(ctx, &member, query, memberID); err != nil {
		return nil, fmt.Errorf("failed to get member by ID: %w", err)
	}

	return &member, nil
}

// InsertMembers inserts or updates multiple members in the database.
// Fields which are an empty string will not be updated.
func (d *Database) InsertMembers(ctx context.Context, members []Member) error {
	if len(members) == 0 {
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

	for chunk := range slices.Chunk(members, batchSize) {
		query := `
			INSERT INTO members (member_id, member_username, member_display_name, member_avatar_url, member_raw_json)
			VALUES (:member_id, :member_username, :member_display_name, :member_avatar_url, :member_raw_json)
			ON CONFLICT (member_id) DO UPDATE SET
				member_username = COALESCE(NULLIF(EXCLUDED.member_username, ''), members.member_username),
				member_display_name = COALESCE(NULLIF(EXCLUDED.member_display_name, ''), members.member_display_name),
				member_avatar_url = COALESCE(NULLIF(EXCLUDED.member_avatar_url, ''), members.member_avatar_url),
				member_imported_at = NOW(),
				member_raw_json = COALESCE(NULLIF(EXCLUDED.member_raw_json, '{}'), members.member_raw_json)
			`

		_, err = d.db.NamedExecContext(ctx, query, chunk)
		if err != nil {
			return fmt.Errorf("failed to create or update members: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (d *Database) GetEventMembers(ctx context.Context, eventID string) ([]EventMember, error) {
	query := `
		SELECT e.*, er.*, m.*
		FROM events e
		JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
		JOIN members m ON er.event_rsvp_member_id = m.member_id
		WHERE e.event_id = $1
		ORDER BY m.member_display_name, m.member_username, m.member_id
	`

	var members []EventMember
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get event members: %w", err)
	}

	return members, nil
}

func (d *Database) GetCheckedInMembersByEvent(ctx context.Context, eventID string) ([]Member, error) {
	query := `
		SELECT m.*
		FROM members m
		JOIN event_rsvps er ON m.member_id = er.event_rsvp_member_id
		WHERE er.event_rsvp_event_id = $1 AND er.event_rsvp_status = 'CHECKED_IN'
		ORDER BY m.member_display_name, m.member_username, m.member_id
	`

	var members []Member
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get checked-in members by event: %w", err)
	}

	return members, nil
}

func (d *Database) GetAcceptedMembersByEvent(ctx context.Context, eventID string) ([]Member, error) {
	query := `
		SELECT m.*
		FROM members m
		JOIN event_rsvps er ON m.member_id = er.event_rsvp_member_id
		WHERE er.event_rsvp_event_id = $1 AND er.event_rsvp_status = 'ACCEPTED'
		ORDER BY m.member_display_name, m.member_username, m.member_id
	`

	var members []Member
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get accepted members by event: %w", err)
	}

	return members, nil
}

func (d *Database) GetTopMembersByClub(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool, limit int) ([]TopMember, error) {
	query := `
		SELECT m.*,
			COUNT(CASE WHEN er.event_rsvp_status = 'ACCEPTED' or er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS accepted,
			COUNT(CASE WHEN er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM event_rsvps er
		JOIN events e ON er.event_rsvp_event_id = e.event_id
		JOIN members m ON er.event_rsvp_member_id = m.member_id
		WHERE e.event_club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		AND (NOT $4 OR e.event_created_by_community_ambassador = TRUE)
		GROUP BY m.member_id, m.member_username, m.member_display_name, m.member_avatar_url
		ORDER BY check_ins DESC, accepted DESC, m.member_display_name, m.member_username, m.member_id
		LIMIT CASE WHEN $5 < 0 THEN NULL ELSE $5 END
	`

	var members []TopMember
	if err := d.db.SelectContext(ctx, &members, query, clubID, from, to, caOnly, limit); err != nil {
		return nil, fmt.Errorf("failed to get top members by club: %w", err)
	}

	return members, nil
}

func (d *Database) GetClubTotalCheckInsAccepted(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool) (int, int, error) {
	query := `
		SELECT
			COUNT(CASE WHEN er.event_rsvp_status = 'ACCEPTED' OR er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS accepted,
			COUNT(CASE WHEN er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM event_rsvps er
		JOIN events e ON er.event_rsvp_event_id = e.event_id
		WHERE e.event_club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		AND (NOT $4 OR e.event_created_by_community_ambassador = TRUE)
	`

	var accepted, checkIns int
	if err := d.db.QueryRowContext(ctx, query, clubID, from, to, caOnly).Scan(&accepted, &checkIns); err != nil {
		return 0, 0, fmt.Errorf("failed to get total check-ins and accepted members: %w", err)
	}

	return accepted, checkIns, nil
}

func (d *Database) GetEventCheckInAcceptedCounts(ctx context.Context, clubID string, from time.Time, to time.Time, caOnly bool) ([]EventNumbers, error) {
	query := `
		SELECT e.event_campfire_live_event_id, e.event_campfire_live_event_name,
            COUNT(e.event_id) AS events,
			COUNT(CASE WHEN er.event_rsvp_status = 'ACCEPTED' OR er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS accepted,
			COUNT(CASE WHEN er.event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM events e
		JOIN event_rsvps er ON e.event_id = er.event_rsvp_event_id
		WHERE e.event_club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		AND (NOT $4 OR e.event_created_by_community_ambassador = TRUE)
		GROUP BY e.event_id
	`

	var numbers []EventNumbers
	if err := d.db.SelectContext(ctx, &numbers, query, clubID, from, to, caOnly); err != nil {
		return nil, fmt.Errorf("failed to get event check-ins and accepted members: %w", err)
	}

	return numbers, nil
}
