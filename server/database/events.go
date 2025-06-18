package database

import (
	"context"
	"fmt"
)

func (d *Database) AddEvent(ctx context.Context, eventID string, name string, details string) error {
	_, err := d.db.ExecContext(ctx, "INSERT INTO events (id, name, details) VALUES ($1, $2, $3)", eventID, name, details)
	if err != nil {
		return fmt.Errorf("failed to add event: %w", err)
	}
	return nil
}

func (d *Database) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	var event Event
	err := d.db.GetContext(ctx, &event, "SELECT id, name, details FROM events WHERE id = $1", eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	return &event, nil
}

func (d *Database) GetEvents(ctx context.Context) ([]Event, error) {
	var events []Event
	err := d.db.SelectContext(ctx, &events, "SELECT id, name, details FROM events")
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	return events, nil
}

func (d *Database) AddMembers(ctx context.Context, members []Member) error {
	_, err := d.db.NamedExecContext(ctx, "INSERT INTO members (id, display_name, status, event_id) VALUES (:id, :display_name, :status, :event_id)", members)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}
	return nil
}

func (d *Database) GetMembersByEvent(ctx context.Context, eventID string) ([]Member, error) {
	var members []Member
	err := d.db.SelectContext(ctx, &members, "SELECT id, display_name, status, event_id FROM members WHERE event_id = $1", eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get members by event: %w", err)
	}
	return members, nil
}
