package database

import (
	"context"
	"fmt"
	"time"

	"github.com/topi314/campfire-tools/internal/xrand"
)

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

func (d *Database) GetRewardWithMembers(ctx context.Context, id int) (*RewardWithMembers, error) {
	query := `
		SELECT rewards.*,
		       COUNT(reward_codes.reward_code_id) AS reward_total_codes,
		       COUNT(reward_codes.reward_code_redeemed_at) AS reward_redeemed_codes,
		       COALESCE(
		           JSONB_AGG(
		               JSONB_BUILD_OBJECT(
		                   'reward_member_reward_id', reward_members.reward_member_reward_id,
		                   'reward_member_discord_user_id', reward_members.reward_member_discord_user_id,
		                   'reward_member_added_at', reward_members.reward_member_added_at,
		                   'discord_user', JSONB_BUILD_OBJECT(
		                       'discord_user_id', discord_users.discord_user_id,
		                       'discord_user_username', discord_users.discord_user_username,
		                       'discord_user_display_name', discord_users.discord_user_display_name,
		                       'discord_user_avatar_url', discord_users.discord_user_avatar_url
		                   )
		               )
		           ) FILTER (WHERE reward_members.reward_member_discord_user_id IS NOT NULL),
		           '[]'::JSONB
		       ) AS members
		FROM rewards
		LEFT JOIN reward_codes ON reward_code_reward_id = reward_id
		LEFT JOIN reward_members ON reward_member_reward_id = reward_id
		LEFT JOIN discord_users ON reward_member_discord_user_id = discord_users.discord_user_id
		WHERE reward_id = $1
		GROUP BY reward_id
	`
	var reward RewardWithMembers
	if err := d.db.GetContext(ctx, &reward, query, id); err != nil {
		return nil, fmt.Errorf("failed to get reward with members: %w", err)
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

func (d *Database) GetRewardCode(ctx context.Context, id int) (*RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       importer.discord_user_imported_at AS "imported_by_user.discord_user_imported_at",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url",
		       redeemer.discord_user_imported_at AS "redeemed_by_user.discord_user_imported_at",
		       reserver.discord_user_id AS "reserved_by_user.discord_user_id",
		       reserver.discord_user_username AS "reserved_by_user.discord_user_username",
		       reserver.discord_user_display_name AS "reserved_by_user.discord_user_display_name",
		       reserver.discord_user_avatar_url AS "reserved_by_user.discord_user_avatar_url",
		       reserver.discord_user_imported_at AS "reserved_by_user.discord_user_imported_at"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		LEFT JOIN discord_users AS reserver ON reward_code_reserved_by = reserver.discord_user_id
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
		       importer.discord_user_imported_at AS "imported_by_user.discord_user_imported_at",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url",
		       redeemer.discord_user_imported_at AS "redeemed_by_user.discord_user_imported_at",
		       reserver.discord_user_id AS "reserved_by_user.discord_user_id",
		       reserver.discord_user_username AS "reserved_by_user.discord_user_username",
		       reserver.discord_user_display_name AS "reserved_by_user.discord_user_display_name",
		       reserver.discord_user_avatar_url AS "reserved_by_user.discord_user_avatar_url",
		       reserver.discord_user_imported_at AS "reserved_by_user.discord_user_imported_at"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		LEFT JOIN discord_users AS reserver ON reward_code_reserved_by = reserver.discord_user_id
		WHERE reward_code_redeem_code = $1
	`
	var code RewardCodeWithUser
	if err := d.db.GetContext(ctx, &code, query, redeemCode); err != nil {
		return nil, fmt.Errorf("failed to get reward code by redeem code: %w", err)
	}

	return &code, nil
}

func (d *Database) GetRewardCodes(ctx context.Context, id int, filter string) ([]RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       importer.discord_user_imported_at AS "imported_by_user.discord_user_imported_at",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url",
		       redeemer.discord_user_imported_at AS "redeemed_by_user.discord_user_imported_at",
		       reserver.discord_user_id AS "reserved_by_user.discord_user_id",
		       reserver.discord_user_username AS "reserved_by_user.discord_user_username",
		       reserver.discord_user_display_name AS "reserved_by_user.discord_user_display_name",
		       reserver.discord_user_avatar_url AS "reserved_by_user.discord_user_avatar_url",
		       reserver.discord_user_imported_at AS "reserved_by_user.discord_user_imported_at"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		LEFT JOIN discord_users AS reserver ON reward_code_reserved_by = reserver.discord_user_id
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

func (d *Database) GetNextRewardCode(ctx context.Context, id int) (*RewardCodeWithUser, error) {
	query := `
		SELECT reward_codes.*, 
		       importer.discord_user_id AS "imported_by_user.discord_user_id",
		       importer.discord_user_username AS "imported_by_user.discord_user_username",
		       importer.discord_user_display_name AS "imported_by_user.discord_user_display_name",
		       importer.discord_user_avatar_url AS "imported_by_user.discord_user_avatar_url",
		       importer.discord_user_imported_at AS "imported_by_user.discord_user_imported_at",
		       redeemer.discord_user_id AS "redeemed_by_user.discord_user_id",
		       redeemer.discord_user_username AS "redeemed_by_user.discord_user_username",
		       redeemer.discord_user_display_name AS "redeemed_by_user.discord_user_display_name",
		       redeemer.discord_user_avatar_url AS "redeemed_by_user.discord_user_avatar_url",
		       redeemer.discord_user_imported_at AS "redeemed_by_user.discord_user_imported_at",
		       reserver.discord_user_id AS "reserved_by_user.discord_user_id",
		       reserver.discord_user_username AS "reserved_by_user.discord_user_username",
		       reserver.discord_user_display_name AS "reserved_by_user.discord_user_display_name",
		       reserver.discord_user_avatar_url AS "reserved_by_user.discord_user_avatar_url",
		       reserver.discord_user_imported_at AS "reserved_by_user.discord_user_imported_at"
		FROM reward_codes
		LEFT JOIN discord_users AS importer ON reward_code_imported_by = importer.discord_user_id
		LEFT JOIN discord_users AS redeemer ON reward_code_redeemed_by = redeemer.discord_user_id
		LEFT JOIN discord_users AS reserver ON reward_code_reserved_by = reserver.discord_user_id
		WHERE reward_code_reward_id = $1
		  AND reward_code_redeemed_at IS NULL
		  AND reward_code_reserved_at IS NULL
		ORDER BY reward_code_imported_at DESC, reward_code_id DESC
		LIMIT 1
	`

	var code RewardCodeWithUser
	if err := d.db.GetContext(ctx, &code, query, id); err != nil {
		return nil, fmt.Errorf("failed to get next reward code: %w", err)
	}

	return &code, nil
}

func (d *Database) ReserveRewardCode(ctx context.Context, id int, userID string) error {
	query := `
		UPDATE reward_codes
		SET reward_code_reserved_at = now(),
		    reward_code_reserved_by = $1
		WHERE reward_code_id = $2
	`

	_, err := d.db.ExecContext(ctx, query, userID, id)
	if err != nil {
		return fmt.Errorf("failed to reserve reward code: %w", err)
	}

	return nil
}

func (d *Database) UnreserveRewardCodes(ctx context.Context) error {
	query := `
		UPDATE reward_codes
		SET reward_code_reserved_at = NULL,
		    reward_code_reserved_by = NULL
		WHERE reward_code_reserved_at IS NOT NULL
		  AND reward_code_redeemed_at IS NULL
		  AND reward_code_reserved_at < NOW() - INTERVAL '1 minute'
	`

	_, err := d.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to unreserve reward codes: %w", err)
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
		    reward_code_redeemed_by = $2,
			reward_code_reserved_at = NULL,
			reward_code_reserved_by = NULL
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

func (d *Database) AddRewardMember(ctx context.Context, rewardID int, discordUserID string) error {
	query := `
		INSERT INTO reward_members (reward_member_reward_id, reward_member_discord_user_id)
		VALUES ($1, $2)
		ON CONFLICT (reward_member_reward_id, reward_member_discord_user_id) DO NOTHING
	`

	_, err := d.db.ExecContext(ctx, query, rewardID, discordUserID)
	if err != nil {
		return fmt.Errorf("failed to add reward member: %w", err)
	}

	return nil
}

func (d *Database) RemoveRewardMember(ctx context.Context, rewardID int, discordUserID string) error {
	query := `
		DELETE FROM reward_members
		WHERE reward_member_reward_id = $1 AND reward_member_discord_user_id = $2
	`

	_, err := d.db.ExecContext(ctx, query, rewardID, discordUserID)
	if err != nil {
		return fmt.Errorf("failed to remove reward member: %w", err)
	}

	return nil
}
