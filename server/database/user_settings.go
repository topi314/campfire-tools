package database

import (
	"context"
	"fmt"
)

func (d *Database) SetUserSetting(ctx context.Context, setting UserSetting) error {
	query := `
		INSERT INTO user_settings (user_setting_user_id, user_setting_pinned_club_id)
		VALUES (:user_setting_user_id, :user_setting_pinned_club_id)
		ON CONFLICT (user_setting_user_id) DO UPDATE
		SET user_setting_pinned_club_id = EXCLUDED.user_setting_pinned_club_id
	`

	if _, err := d.db.NamedExecContext(ctx, query, setting); err != nil {
		return fmt.Errorf("failed to set user setting: %w", err)
	}
	return nil
}
