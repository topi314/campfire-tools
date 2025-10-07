package database

import (
	"context"
	"fmt"
	"time"
)

type CampfireToken struct {
	ID        int       `db:"campfire_token_id"`
	Token     string    `db:"campfire_token_token"`
	ExpiresAt time.Time `db:"campfire_token_expires_at"`
	Email     string    `db:"campfire_token_email"`
}

func (d *Database) InsertCampfireToken(ctx context.Context, token CampfireToken) error {
	query := `INSERT INTO campfire_tokens (campfire_token_token, campfire_token_expires_at, campfire_token_email) VALUES ($1, $2, $3) RETURNING campfire_token_id`

	var id int
	err := d.db.GetContext(ctx, &id, query, token.Token, token.ExpiresAt, token.Email)
	return err
}

func (d *Database) GetCampfireTokens(ctx context.Context) ([]CampfireToken, error) {
	query := `SELECT * FROM campfire_tokens ORDER BY campfire_token_expires_at DESC`

	var tokens []CampfireToken
	if err := d.db.SelectContext(ctx, &tokens, query); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (d *Database) GetNextCampfireToken(ctx context.Context) (*CampfireToken, error) {
	query := `SELECT * FROM campfire_tokens WHERE campfire_token_expires_at > $1 ORDER BY campfire_token_expires_at LIMIT 1`

	now := time.Now().Add(time.Minute)

	var campfireToken CampfireToken
	if err := d.db.GetContext(ctx, &campfireToken, query, now); err != nil {
		return nil, err
	}

	return &campfireToken, nil
}

func (d *Database) DeleteExpiredCampfireTokens(ctx context.Context) (int, error) {
	res, err := d.db.ExecContext(ctx, "DELETE FROM campfire_tokens WHERE campfire_token_expires_at < now()")
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired campfire tokens: %w", err)
	}

	rows, err := res.RowsAffected()
	return int(rows), err
}

func (d *Database) GetCampfireTokensExpiringSoon(ctx context.Context, within time.Duration) ([]CampfireToken, error) {
	query := `SELECT * FROM campfire_tokens WHERE campfire_token_expires_at > $1 AND campfire_token_expires_at < $2 ORDER BY campfire_token_expires_at`

	now := time.Now()
	later := now.Add(within)

	var tokens []CampfireToken
	if err := d.db.SelectContext(ctx, &tokens, query, now, later); err != nil {
		return nil, err
	}
	return tokens, nil
}
