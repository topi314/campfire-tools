package database

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrSessionExpired = errors.New("session expired")

func (d *Database) GetSession(ctx context.Context, sessionID string) (*SessionWithUserSetting, error) {
	query := `
		SELECT * FROM sessions 
		LEFT JOIN user_settings ON sessions.session_user_id = user_settings.user_setting_user_id
        WHERE session_id = $1
		`

	var session SessionWithUserSetting
	err := d.db.GetContext(ctx, &session, query, sessionID)
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
		INSERT INTO sessions (session_id, session_created_at, session_expires_at, session_user_id, session_admin)
		VALUES (:session_id, :session_created_at, :session_expires_at, :session_user_id, :session_admin)
	`
	_, err := d.db.NamedExecContext(ctx, query, session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (d *Database) DeleteExpiredSessions(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM sessions WHERE session_expires_at < now()")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}
