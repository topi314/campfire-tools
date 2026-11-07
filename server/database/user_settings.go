package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type UserSettings struct {
	UserID    string `db:"user_setting_user_id"`
	Timezone  string `db:"user_setting_timezone"`
	HourCycle string `db:"user_setting_hour_cycle"`
	Theme     string `db:"user_setting_theme"`
}

func (d *Database) GetUserSettings(ctx context.Context, userID string) (UserSettings, error) {
	var settings UserSettings
	err := d.db.GetContext(ctx, &settings, `
		SELECT user_setting_user_id,
		       COALESCE(user_setting_timezone, '') AS user_setting_timezone,
		       COALESCE(user_setting_hour_cycle, '') AS user_setting_hour_cycle,
		       COALESCE(user_setting_theme, '') AS user_setting_theme
		FROM user_settings
		WHERE user_setting_user_id = $1
	`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return UserSettings{UserID: userID}, nil
	}
	if err != nil {
		return UserSettings{}, fmt.Errorf("failed to get user settings: %w", err)
	}
	return settings, nil
}

func (d *Database) SaveUserSettings(ctx context.Context, settings UserSettings) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO user_settings (user_setting_user_id, user_setting_timezone, user_setting_hour_cycle, user_setting_theme)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''))
		ON CONFLICT (user_setting_user_id) DO UPDATE
		SET user_setting_timezone = EXCLUDED.user_setting_timezone,
		    user_setting_hour_cycle = EXCLUDED.user_setting_hour_cycle,
		    user_setting_theme = EXCLUDED.user_setting_theme
	`, settings.UserID, settings.Timezone, settings.HourCycle, settings.Theme)
	if err != nil {
		return fmt.Errorf("failed to save user settings: %w", err)
	}
	return nil
}
