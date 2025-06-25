package database

import (
	"context"
	"fmt"
)

func (d *Database) AddMembers(ctx context.Context, members []Member) error {
	_, err := d.db.NamedExecContext(ctx, "INSERT INTO members (id, display_name, status, event_id) VALUES (:id, :display_name, :status, :event_id)", members)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}
	return nil
}

func (d *Database) GetMembersByEvent(ctx context.Context, eventID string) ([]EventMember, error) {
	var members []EventMember
	query := `
		SELECT m.id, m.display_name, m.status, e.name AS event_name
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE m.event_id = $1
		`
	if err := d.db.SelectContext(ctx, &members, query, eventID); err != nil {
		return nil, fmt.Errorf("failed to get members by event: %w", err)
	}
	return members, nil
}

// GetTopClubMembers retrieves the top members of a club based on the amount of events they attended.
// It returns a list of members with their id, display name and event count.
func (d *Database) GetTopClubMembers(ctx context.Context, clubID string, limit int) ([]TopMember, error) {
	var members []TopMember
	query := `
		SELECT m.id, m.display_name, COUNT(e.id) AS event_count
		FROM members m
		JOIN events e ON m.event_id = e.id
		WHERE e.club_id = $1 AND m.status = 'CHECKED_IN'
		GROUP BY m.id, m.display_name
		ORDER BY event_count DESC, m.display_name DESC
		LIMIT $2`
	if err := d.db.SelectContext(ctx, &members, query, clubID, limit); err != nil {
		return nil, fmt.Errorf("failed to get top club members: %w", err)
	}
	return members, nil
}
