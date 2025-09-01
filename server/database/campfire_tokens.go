package database

import (
	"context"
	"time"
)

type CampfireToken struct {
	ID        int       `db:"id"`
	Token     string    `db:"token"`
	ExpiresAt time.Time `db:"expires_at"`
	Email     string    `db:"email"`
}

func (d *Database) InsertCampfireToken(token string, expiresAt time.Time, email string) error {
	query := `INSERT INTO campfire_tokens (token, expires_at, email) VALUES ($1, $2, $3) RETURNING id`

	var id int
	err := d.db.QueryRow(query, token, expiresAt, email).Scan(&id)
	return err
}

func (d *Database) GetCampfireToken(token string) (*CampfireToken, error) {
	query := `SELECT id, token, expires_at, email FROM campfire_tokens WHERE token = $1`

	var campfireToken CampfireToken
	if err := d.db.Get(&campfireToken, query, token); err != nil {
		return nil, err
	}
	return &campfireToken, nil
}

func (d *Database) DeleteCampfireToken(id int) error {
	query := `DELETE FROM campfire_tokens WHERE id = $1`
	_, err := d.db.Exec(query, id)
	return err
}

func (d *Database) GetNextCampfireToken(ctx context.Context) (*CampfireToken, error) {
	query := `SELECT id, token, expires_at, email FROM campfire_tokens WHERE expires_at > $1 ORDER BY expires_at LIMIT 1`

	now := time.Now().Add(time.Minute)

	var campfireToken CampfireToken
	if err := d.db.GetContext(ctx, &campfireToken, query, now); err != nil {
		return nil, err
	}

	return &campfireToken, nil
}
