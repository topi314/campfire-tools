package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (d *Database) AddMembers(ctx context.Context, members []Member) error {
	_, err := d.db.NamedExecContext(ctx, "INSERT INTO members (id, display_name, status, event_id) VALUES (:id, :display_name, :status, :event_id)", members)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}
	return nil
}

func (d *Database) GetCheckedInMembersByEvent(ctx context.Context, eventID string) ([]EventMember, error) {
	var members []EventMember
	query := `
		SELECT m.id, m.display_name, m.status, e.name AS event_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE m.event_id = $1 AND m.status = 'CHECKED_IN'
		`
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get members by event: %w", err)
	}
	return members, nil
}

func (d *Database) GetRSVPMembersByEvent(ctx context.Context, eventID string) ([]EventMember, error) {
	var members []EventMember
	query := `
		SELECT m.id, m.display_name, m.status, e.name AS event_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE m.event_id = $1 AND m.status = 'ACCEPTED'
		`
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get RSVPed members by event: %w", err)
	}
	return members, nil
}

// GetTopClubMembers retrieves the top members of a club based on the amount of events they attended.
// It returns a list of members with their id, display name and event count.
func (d *Database) GetTopClubMembers(ctx context.Context, clubID string, limit int) ([]TopMember, error) {
	var members []TopMember
	query := `
		SELECT m.id, m.display_name, COUNT(e.id) AS check_ins
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE e.club_id = $1 AND m.status = 'CHECKED_IN'
		GROUP BY m.id, m.display_name
		ORDER BY check_ins DESC, m.display_name DESC
		LIMIT $2`
	if err := d.db.SelectContext(ctx, &members, query, clubID, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club members: %w", err)
	}
	return members, nil
}

func (d *Database) GetClubMember(ctx context.Context, clubID string, memberID string) (*ClubMember, error) {
	var member ClubMember
	query := `
		SELECT m.id, m.display_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE e.club_id = $1 AND m.id = $2
		ORDER BY e.event_time
		LIMIT  1`
	if err := d.db.GetContext(ctx, &member, query, clubID, memberID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("member not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get club member: %w", err)
	}

	return &member, nil
}
