package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (d *Database) AddMembers(ctx context.Context, members []Member) error {
	query := `
		INSERT INTO members (id, username, display_name, avatar_url, status, event_id)
		VALUES (:id, :username, :display_name, :avatar_url, :status, :event_id)
		ON CONFLICT (id, event_id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			status = EXCLUDED.status
		`

	_, err := d.db.NamedExecContext(ctx, query, members)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}
	return nil
}

func (d *Database) GetCheckedInMembersByEvent(ctx context.Context, eventID string) ([]EventMember, error) {
	query := `
		SELECT m.id, m.username, m.display_name, m.avatar_url, m.status, e.name AS event_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE m.event_id = $1 AND m.status = 'CHECKED_IN'
		`

	var members []EventMember
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get members by event: %w", err)
	}
	return members, nil
}

func (d *Database) GetAcceptedMembersByEvent(ctx context.Context, eventID string) ([]EventMember, error) {
	query := `
		SELECT m.id, m.username, m.display_name, m.avatar_url, m.status, e.name AS event_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE m.event_id = $1 AND m.status = 'ACCEPTED'
		`

	var members []EventMember
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get accepted members by event: %w", err)
	}
	return members, nil
}

// GetTopClubMembers retrieves the top members of a club based on the amount of events they attended.
// It returns a list of members with their id, display name and event count.
func (d *Database) GetTopClubMembers(ctx context.Context, clubID string, limit int) ([]TopMember, error) {
	query := `
		SELECT DISTINCT(m.id),
			m.username,
			m.display_name,
			m.avatar_url,
		  COUNT(e.id) OVER (PARTITION BY m.id) AS check_ins
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE e.club_id = $1 AND m.status = 'CHECKED_IN'
		ORDER BY check_ins DESC, m.id, m.display_name
		LIMIT $2
		`

	var members []TopMember
	if err := d.db.SelectContext(ctx, &members, query, clubID, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club members: %w", err)
	}
	return members, nil
}

func (d *Database) GetClubMember(ctx context.Context, clubID string, memberID string) (*ClubMember, error) {
	query := `
		SELECT m.id, m.username, m.display_name, m.avatar_url
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE e.club_id = $1 AND m.id = $2
		ORDER BY e.event_time
		LIMIT  1
		`

	var member ClubMember
	if err := d.db.GetContext(ctx, &member, query, clubID, memberID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("member not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get club member: %w", err)
	}

	return &member, nil
}
