package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubs(ctx context.Context, order string) ([]ClubWithEvents, error) {
	orderQuery := " ORDER BY "
	switch order {
	case "events":
		orderQuery += "events DESC, clubs.club_name ASC"
	default:
		orderQuery += "clubs.club_name ASC"
	}

	query := `
		SELECT clubs.*, COUNT(events.event_id) AS events
		FROM clubs
		LEFT JOIN events ON clubs.club_id = events.event_club_id
		GROUP BY clubs.club_id, clubs.club_name
	` + orderQuery

	var clubs []ClubWithEvents
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

func (d *Database) InsertClubs(ctx context.Context, clubs []Club) error {
	query := `
		INSERT INTO clubs (club_id, club_name, club_avatar_url, club_creator_id, club_created_by_community_ambassador, club_raw_json, club_last_auto_event_imported_at)
		VALUES (:club_id, :club_name, :club_avatar_url, :club_creator_id, :club_created_by_community_ambassador, :club_raw_json, now())
		ON CONFLICT (club_id) DO UPDATE SET
			club_name = EXCLUDED.club_name,
			club_avatar_url = EXCLUDED.club_avatar_url,
			club_creator_id = EXCLUDED.club_creator_id,
			club_created_by_community_ambassador = EXCLUDED.club_created_by_community_ambassador,
			club_imported_at = now(),
			club_raw_json = EXCLUDED.club_raw_json,
			club_last_auto_event_imported_at = now()
	`

	if _, err := d.db.NamedExecContext(ctx, query, clubs); err != nil {
		return fmt.Errorf("failed to create or update club: %w", err)
	}

	return nil
}

func (d *Database) UpdateClub(ctx context.Context, clubID string, autoImport bool, verificationChannelID *string) error {
	query := `
		UPDATE clubs
		SET club_auto_event_import = $1,
			club_verification_channel_id = $2
		WHERE club_id = $3
	`

	if _, err := d.db.ExecContext(ctx, query, autoImport, verificationChannelID, clubID); err != nil {
		return fmt.Errorf("failed to update club auto import: %w", err)
	}

	return nil
}

func (d *Database) UpdateClubLastAutoEventImported(ctx context.Context, clubID string) error {
	query := `
		UPDATE clubs
		SET club_last_auto_event_imported_at = now()
		WHERE club_id = $1
	`

	if _, err := d.db.ExecContext(ctx, query, clubID); err != nil {
		return fmt.Errorf("failed to update club last auto event imported: %w", err)
	}

	return nil
}

func (d *Database) GetNextClubImport(ctx context.Context) (*Club, error) {
	query := `
		SELECT *
		FROM clubs
		WHERE club_auto_event_import = TRUE AND (club_last_auto_event_imported_at < now() - INTERVAL '1 hour')
		ORDER BY club_last_auto_event_imported_at
		LIMIT 1
	`

	var club Club
	if err := d.db.GetContext(ctx, &club, query); err != nil {
		return nil, fmt.Errorf("failed to get next club to import: %w", err)
	}

	return &club, nil
}

func (d *Database) GetClubEventCreators(ctx context.Context, clubID string) ([]Member, error) {
	query := `
		SELECT DISTINCT members.*
		FROM events
		JOIN members ON events.event_creator_id = members.member_id
		WHERE events.event_club_id = $1
		ORDER BY members.member_username ASC
	`

	var creators []Member
	if err := d.db.SelectContext(ctx, &creators, query, clubID); err != nil {
		return nil, fmt.Errorf("failed to get club event creators: %w", err)
	}

	return creators, nil
}

func (d *Database) GetRewardClubs(ctx context.Context) ([]Club, error) {
	query := `
		SELECT *
		FROM clubs
		WHERE club_verification_channel_id IS NOT NULL
		ORDER BY club_name
	`

	var clubs []Club
	if err := d.db.SelectContext(ctx, &clubs, query); err != nil {
		return nil, fmt.Errorf("failed to get reward clubs: %w", err)
	}

	return clubs, nil
}
