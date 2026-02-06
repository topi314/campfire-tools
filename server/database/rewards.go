package database

import (
	"context"
	"fmt"
	"time"

	"github.com/topi314/campfire-tools/internal/xrand"
)

type Reward struct {
	ID            int    `db:"reward_id"`
	Name          string `db:"reward_name"`
	Description   string `db:"reward_description"`
	CreatedBy     string `db:"reward_created_by"`
	CreatedAt     string `db:"reward_created_at"`
	TotalCodes    int    `db:"reward_total_codes"`
	RedeemedCodes int    `db:"reward_redeemed_codes"`
}

type RewardCode struct {
	ID           int        `db:"reward_code_id"`
	Code         string     `db:"reward_code_code"`
	RewardID     int        `db:"reward_code_reward_id"`
	ImportedAt   time.Time  `db:"reward_code_imported_at"`
	ImportedBy   string     `db:"reward_code_imported_by"`
	RedeemCode   string     `db:"reward_code_redeem_code"`
	RedeemedAt   *time.Time `db:"reward_code_redeemed_at"`
	RedeemedBy   *string    `db:"reward_code_redeemed_by"`
	VisitedCount int        `db:"reward_code_visited_count"`
}

type RewardCodeWithUser struct {
	RewardCode
	ImportedByUser DiscordUser `db:"imported_by_user"`
	RedeemedByUser struct {
		ID          *string `db:"discord_user_id"`
		Username    *string `db:"discord_user_username"`
		DisplayName *string `db:"discord_user_display_name"`
		AvatarURL   *string `db:"discord_user_avatar_url"`
	} `db:"redeemed_by_user"`
}

func (d *Database) GetReward(ctx context.Context, id int, userID string) (*Reward, error) {
	query := `
		SELECT rewards.*,
		       COUNT(reward_codes.reward_code_id) AS reward_total_codes,
		       COUNT(reward_codes.reward_code_redeemed_at) AS reward_redeemed_codes
		FROM rewards
        LEFT JOIN reward_codes ON reward_code_reward_id = reward_id
		LEFT JOIN reward_members ON reward_member_reward_id = reward_id
		WHERE reward_id = $1 AND (reward_created_by = $2 OR reward_member_discord_user_id = $2)
		GROUP BY reward_id
	`
	var reward Reward
	if err := d.db.GetContext(ctx, &reward, query, id, userID); err != nil {
		return nil, fmt.Errorf("failed to get reward: %w", err)
	}

	return &reward, nil
}

func (d *Database) GetRewards(ctx context.Context, userID string) ([]Reward, error) {
	query := `
		SELECT rewards.*,
		       COUNT(reward_codes.reward_code_id) AS reward_total_codes,
		       COUNT(reward_codes.reward_code_redeemed_at) AS reward_redeemed_codes
		FROM rewards
		LEFT JOIN reward_codes ON reward_code_reward_id = reward_id
		LEFT JOIN reward_members ON reward_member_reward_id = reward_id
		WHERE reward_created_by = $1 OR reward_member_discord_user_id = $1
		GROUP BY reward_id
		ORDER BY reward_created_at DESC
	`
	var rewards []Reward
	if err := d.db.SelectContext(ctx, &rewards, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get rewards with counts: %w", err)
	}

	return rewards, nil
}

func (d *Database) InsertReward(ctx context.Context, reward Reward) (int, error) {
	query := `
		INSERT INTO rewards (reward_name, reward_description, reward_created_by)
		VALUES (:reward_name, :reward_description, :reward_created_by)
		RETURNING reward_id
	`

	query, args, err := d.db.BindNamed(query, reward)
	if err != nil {
		return 0, fmt.Errorf("failed to bind query: %w", err)
	}

	var id int
	if err = d.db.GetContext(ctx, &id, query, args...); err != nil {
		return 0, err
	}

	return id, nil
}

func (d *Database) UpdateReward(ctx context.Context, reward Reward) error {
	query := `
		UPDATE rewards
		SET reward_name = :reward_name,
		    reward_description = :reward_description
		WHERE reward_id = :reward_id
	`

	_, err := d.db.NamedExecContext(ctx, query, reward)
	if err != nil {
		return fmt.Errorf("failed to update reward: %w", err)
	}

	return nil
}

func (d *Database) DeleteReward(ctx context.Context, codeID int) error {
	query := `
		DELETE FROM rewards
		WHERE reward_id = $1
	`

	_, err := d.db.ExecContext(ctx, query, codeID)
	if err != nil {
		return fmt.Errorf("failed to delete reward: %w", err)
	}

	return nil
}

func (d *Database) InsertRewardCodes(ctx context.Context, id int, codes []string, userID string) error {
	var dbCodes []RewardCode
	for _, code := range codes {
		dbCodes = append(dbCodes, RewardCode{
			Code:       code,
			RewardID:   id,
			ImportedBy: userID,
			RedeemCode: xrand.Code(12),
		})
	}
	query := `
		INSERT INTO reward_codes (reward_code_code, reward_code_reward_id, reward_code_imported_by, reward_code_redeem_code)
		VALUES (:reward_code_code, :reward_code_reward_id, :reward_code_imported_by, :reward_code_redeem_code)
		ON CONFLICT (reward_code_code) DO NOTHING
	`

	_, err := d.db.NamedExecContext(ctx, query, dbCodes)
	if err != nil {
		return fmt.Errorf("failed to insert reward codes: %w", err)
	}

	return nil
}

func (d *Database) UpdateRewardCodeRedeemed(ctx context.Context, id int, at *time.Time, userID *string) error {
	query := `
		UPDATE reward_codes
		SET reward_code_redeemed_at = $1,
		    reward_code_redeemed_by = $2
		WHERE reward_code_id = $3
	`

	_, err := d.db.ExecContext(ctx, query, at, userID, id)
	if err != nil {
		return fmt.Errorf("failed to update reward code: %w", err)
	}

	return nil
}

func (d *Database) DeleteRewardCode(ctx context.Context, id int) error {
	query := `
		DELETE FROM reward_codes
		WHERE reward_code_id = $1
	`

	_, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete reward code: %w", err)
	}

	return nil
}

func (d *Database) GetRewardCode(ctx context.Context, id int) (*RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		WHERE reward_code_id = $1
	`
	var code RewardCodeWithUser
	if err := d.db.GetContext(ctx, &code, query, id); err != nil {
		return nil, fmt.Errorf("failed to get reward code: %w", err)
	}

	return &code, nil
}

func (d *Database) GetRewardCodeByRedeemCode(ctx context.Context, redeemCode string) (*RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		WHERE reward_code_redeem_code = $1
	`
	var code RewardCodeWithUser
	if err := d.db.GetContext(ctx, &code, query, redeemCode); err != nil {
		return nil, fmt.Errorf("failed to get reward code by redeem code: %w", err)
	}

	return &code, nil
}

func (d *Database) GetRewardCodeByRedeemCodeAndIncreaseVisitedCount(ctx context.Context, redeemCode string) (*RewardCodeWithUser, error) {
	code, err := d.GetRewardCodeByRedeemCode(ctx, redeemCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get reward code by redeem code: %w", err)
	}

	if err = d.IncreaseRewardCodeVisitedCount(ctx, code.ID); err != nil {
		return nil, fmt.Errorf("failed to increase reward code visited count: %w", err)
	}

	return code, nil
}

func (d *Database) GetRewardCodes(ctx context.Context, id int, filter string) ([]RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		WHERE reward_code_reward_id = $1
	`

	switch filter {
	case "redeemed":
		query += ` AND reward_code_redeemed_at IS NOT NULL `
	case "unredeemed":
		query += ` AND reward_code_redeemed_at IS NULL `
	}

	query += `ORDER BY reward_code_imported_at DESC, reward_code_id DESC`

	var codes []RewardCodeWithUser
	if err := d.db.SelectContext(ctx, &codes, query, id); err != nil {
		return nil, fmt.Errorf("failed to get reward codes: %w", err)
	}

	return codes, nil
}

func (d *Database) IncreaseRewardCodeVisitedCount(ctx context.Context, id int) error {
	query := `
		UPDATE reward_codes
		SET reward_code_visited_count = reward_code_visited_count + 1
		WHERE reward_code_id = $1
	`

	if _, err := d.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to increase reward code visited count: %w", err)
	}

	return nil
}
