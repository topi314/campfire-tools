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
	err := d.db.GetContext(ctx, &session, "SELECT * FROM sessions WHERE session_id = $1", sessionID)
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
		INSERT INTO sessions (session_id, session_created_at, session_expires_at, session_user_id)
		VALUES (:session_id, :session_created_at, :session_expires_at, :session_user_id)
	`
	_, err := d.db.NamedExecContext(ctx, query, session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (d *Database) DeleteExpiredSessions(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM sessions WHERE session_expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}
