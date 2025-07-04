package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubs(ctx context.Context) ([]Club, error) {
	query := `
		SELECT club_id, MAX(club_name) AS club_name, MAX(club_avatar_url) AS club_avatar_url
        FROM events
        GROUP BY club_id
        `

	var clubs []Club
	if err := d.db.SelectContext(ctx, &clubs, query); err != nil {
		return nil, fmt.Errorf("failed to get clubs: %w", err)
	}
	return clubs, nil
}

func (d *Database) GetClub(ctx context.Context, clubID string) (*Club, error) {
	var club Club
	if err := d.db.GetContext(ctx, &club, "SELECT club_id, club_name, club_avatar_url FROM events WHERE club_id = $1 LIMIT 1", clubID); err != nil {
		return nil, fmt.Errorf("failed to get club: %w", err)
	}
	return &club, nil
}
