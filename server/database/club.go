package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubs(ctx context.Context) ([]Club, error) {
	var clubs []Club
	if err := d.db.SelectContext(ctx, &clubs, "SELECT club_id, club_name, club_avatar_url FROM events GROUP BY club_id, club_name, club_avatar_url ORDER BY club_name"); err != nil {
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
