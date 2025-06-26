package database

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrSessionExpired = errors.New("session expired")

func (d *Database) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	err := d.db.GetContext(ctx, &session, "SELECT * FROM sessions WHERE id = $1", sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if the session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (d *Database) CreateSession(ctx context.Context, session Session) error {
	query := `
	INSERT INTO sessions (id, created_at, expires_at)
	VALUES (:id, :created_at, :expires_at)`
	_, err := d.db.NamedExecContext(ctx, query, session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (d *Database) DeleteExpiredSessions(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}
