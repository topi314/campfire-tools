package database

import (
	"context"
	"fmt"
)

func (d *Database) UpsertDiscordUser(ctx context.Context, user DiscordUser) error {
	query := `
		INSERT INTO discord_users (discord_user_id, discord_user_username, discord_user_display_name, discord_user_avatar_url)
		VALUES (:discord_user_id, :discord_user_username, :discord_user_display_name, :discord_user_avatar_url)
		ON CONFLICT (discord_user_id) DO UPDATE
		SET discord_user_username = EXCLUDED.discord_user_username,
		    discord_user_display_name = EXCLUDED.discord_user_display_name,
		    discord_user_avatar_url = EXCLUDED.discord_user_avatar_url,
		    discord_user_imported_at = now()
	`

	if _, err := d.db.NamedExecContext(ctx, query, user); err != nil {
		return fmt.Errorf("failed to upsert discord user: %w", err)
	}

	return nil
}

func (d *Database) AddDiscordUserPinnedClub(ctx context.Context, userID, clubID string) error {
	query := `
		INSERT INTO discord_user_pinned_clubs (discord_user_pinned_club_user_id, discord_user_pinned_club_club_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	if _, err := d.db.ExecContext(ctx, query, userID, clubID); err != nil {
		return fmt.Errorf("failed to add pinned club for discord user: %w", err)
	}

	return nil
}

func (d *Database) RemoveDiscordUserPinnedClub(ctx context.Context, userID, clubID string) error {
	query := `
		DELETE FROM discord_user_pinned_clubs
		WHERE discord_user_pinned_club_user_id = $1 AND discord_user_pinned_club_club_id = $2
	`

	if _, err := d.db.ExecContext(ctx, query, userID, clubID); err != nil {
		return fmt.Errorf("failed to remove pinned club for discord user: %w", err)
	}

	return nil
}

func (d *Database) GetDiscordUserPinnedClubs(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT discord_user_pinned_club_club_id
		FROM discord_user_pinned_clubs
		WHERE discord_user_pinned_club_user_id = $1
		ORDER BY discord_user_pinned_club_pinned_at
	`

	var clubIDs []string
	if err := d.db.SelectContext(ctx, &clubIDs, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get pinned clubs for discord user: %w", err)
	}

	return clubIDs, nil
}
