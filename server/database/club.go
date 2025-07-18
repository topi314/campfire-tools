package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubs(ctx context.Context) ([]Club, error) {
	query := `
		SELECT *
		FROM clubs
		ORDER BY club_name
	`

	var clubs []Club
	if err := d.db.SelectContext(ctx, &clubs, query); err != nil {
		return nil, fmt.Errorf("failed to get clubs: %w", err)
	}

	return clubs, nil
}

func (d *Database) GetClub(ctx context.Context, clubID string) (*ClubWithCreator, error) {
	query := `
		SELECT clubs.*, members.*
		FROM clubs
		JOIN members ON clubs.club_creator_id = members.member_id
		WHERE clubs.club_id = $1
	`

	var club ClubWithCreator
	if err := d.db.GetContext(ctx, &club, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club: %w", err)
	}

	return &club, nil
}

func (d *Database) InsertClub(ctx context.Context, club Club) error {
	query := `
		INSERT INTO clubs (club_id, club_name, club_avatar_url, club_creator_id, club_created_by_community_ambassador, club_raw_json)
		VALUES (:club_id, :club_name, :club_avatar_url, :club_creator_id, :club_created_by_community_ambassador, :club_raw_json)
		ON CONFLICT (club_id) DO UPDATE SET
			club_name = EXCLUDED.club_name,
			club_avatar_url = EXCLUDED.club_avatar_url,
			club_creator_id = EXCLUDED.club_creator_id,
			club_created_by_community_ambassador = EXCLUDED.club_created_by_community_ambassador,
			club_imported_at = NOW(),
			club_raw_json = EXCLUDED.club_raw_json
	`

	if _, err := d.db.NamedExecContext(ctx, query, club); err != nil {
		return fmt.Errorf("failed to create or update club: %w", err)
	}

	return nil
}
