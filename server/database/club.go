package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubs(ctx context.Context) ([]Club, error) {
	query := `
		SELECT id, name, avatar_url, creator_id, created_by_community_ambassador
        FROM clubs
		ORDER BY name
        `

	var clubs []Club
	if err := d.db.SelectContext(ctx, &clubs, query); err != nil {
		return nil, fmt.Errorf("failed to get clubs: %w", err)
	}

	return clubs, nil
}

func (d *Database) GetClub(ctx context.Context, clubID string) (*Club, error) {
	query := `
		SELECT id, name, avatar_url, creator_id, created_by_community_ambassador
		FROM clubs
		WHERE id = $1
	`

	var club Club
	if err := d.db.GetContext(ctx, &club, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club: %w", err)
	}

	return &club, nil
}

func (d *Database) InsertClub(ctx context.Context, club Club) error {
	query := `
		INSERT INTO clubs (id, name, avatar_url, creator_id, created_by_community_ambassador)
		VALUES (:id, :name, :avatar_url, :creator_id, :created_by_community_ambassador)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		avatar_url = EXCLUDED.avatar_url,
		creator_id = EXCLUDED.creator_id,
		created_by_community_ambassador = EXCLUDED.created_by_community_ambassador
	`

	if _, err := d.db.NamedExecContext(ctx, query, club); err != nil {
		return fmt.Errorf("failed to create or update club: %w", err)
	}

	return nil
}
