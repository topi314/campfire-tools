package database

import (
	"context"
	"fmt"
	"time"
)

func (d *Database) GetMember(ctx context.Context, memberID string) (*Member, error) {
	query := `
		SELECT id, username, display_name, avatar_url
		FROM members
		WHERE id = $1
	`

	var member Member
	if err := d.db.GetContext(ctx, &member, query, memberID); err != nil {
		return nil, fmt.Errorf("failed to get member by ID: %w", err)
	}

	return &member, nil
}

func (d *Database) InsertMembers(ctx context.Context, members []Member) error {
	query := `
		INSERT INTO members (id, username, display_name, avatar_url)
		VALUES (:id, :username, :display_name, :avatar_url)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url
	`

	_, err := d.db.NamedExecContext(ctx, query, members)
	if err != nil {
		return fmt.Errorf("failed to create or update members: %w", err)
	}

	return nil
}

func (d *Database) GetEventMembers(ctx context.Context, eventID string) ([]EventMember, error) {
	query := `
		SELECT e.id, e.campfire_live_event_id, e.campfire_live_event_name, er.status
		FROM events e
		JOIN event_rsvps er ON e.id = er.event_id
		WHERE e.id = $1
	`

	var members []EventMember
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get event members: %w", err)
	}

	return members, nil
}

func (d *Database) GetCheckedInMembersByEvent(ctx context.Context, eventID string) ([]Member, error) {
	query := `
		SELECT u.id, u.username, u.display_name, u.avatar_url
		FROM members u
		JOIN event_rsvps er ON u.id = er.member_id
		WHERE er.event_id = $1 AND er.status = 'CHECKED_IN'
	`

	var members []Member
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get checked-in members by event: %w", err)
	}

	return members, nil
}

func (d *Database) GetAcceptedMembersByEvent(ctx context.Context, eventID string) ([]Member, error) {
	query := `
		SELECT u.id, u.username, u.display_name, u.avatar_url
		FROM members u
		JOIN event_rsvps er ON u.id = er.member_id
		WHERE er.event_id = $1 AND er.status = 'ACCEPTED'
	`

	var members []Member
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get accepted members by event: %w", err)
	}

	return members, nil
}

func (d *Database) GetTopMembersByClub(ctx context.Context, clubID string, from time.Time, to time.Time, limit int) ([]TopMember, error) {
	query := `
		SELECT u.id, u.username, u.display_name, u.avatar_url,
			COUNT(CASE WHEN er.status = 'ACCEPTED' THEN 1 END) AS accepted,
			COUNT(CASE WHEN er.status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM event_rsvps er
		JOIN events e ON er.event_id = e.id
		JOIN members u ON er.member_id = u.id
		WHERE e.club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		GROUP BY u.id, u.username, u.display_name, u.avatar_url
		ORDER BY check_ins DESC, accepted DESC
		LIMIT $4
	`

	var members []TopMember
	if err := d.db.SelectContext(ctx, &members, query, clubID, from, to, limit); err != nil {
		return nil, fmt.Errorf("failed to get top members by club: %w", err)
	}

	return members, nil
}

func (d *Database) GetClubTotalCheckInsAccepted(ctx context.Context, clubID string, from time.Time, to time.Time) (int, int, error) {
	query := `
		SELECT COUNT(CASE WHEN er.status = 'CHECKED_IN' THEN 1 END) AS check_ins,
			COUNT(CASE WHEN er.status = 'ACCEPTED' OR er.status = 'CHECKED_IN' THEN 1 END) AS accepted
		FROM event_rsvps er
		JOIN events e ON er.event_id = e.id
		WHERE e.club_id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
	`

	var checkIns, accepted int
	if err := d.db.QueryRowContext(ctx, query, clubID, from, to).Scan(&checkIns, &accepted); err != nil {
		return 0, 0, fmt.Errorf("failed to get total check-ins and accepted members: %w", err)
	}

	return checkIns, accepted, nil
}

func (d *Database) GetEventCheckInAccepted(ctx context.Context, eventID string, from time.Time, to time.Time) ([]EventNumbers, error) {
	query := `
		SELECT e.campfire_live_event_id, e.campfire_live_event_name,
			COUNT(CASE WHEN er.status = 'CHECKED_IN' THEN 1 END) AS check_ins,
			COUNT(CASE WHEN er.status = 'ACCEPTED' OR er.status = 'CHECKED_IN' THEN 1 END) AS accepted
		FROM events e
		JOIN event_rsvps er ON e.id = er.event_id
		WHERE e.id = $1
		AND ($2 = '0001-01-01 00:00:00'::timestamp OR e.event_time >= $2)
		AND ($3 = '0001-01-01 00:00:00'::timestamp OR e.event_time <= $3)
		GROUP BY e.campfire_live_event_id, e.campfire_live_event_name
	`

	var numbers []EventNumbers
	if err := d.db.SelectContext(ctx, &numbers, query, eventID, from, to); err != nil {
		return nil, fmt.Errorf("failed to get event check-ins and accepted members: %w", err)
	}

	return numbers, nil
}
