package database

import (
	"context"
	"fmt"
	"time"
)

type RewardPool struct {
	ID          int    `db:"reward_pool_id"`
	Name        string `db:"reward_pool_name"`
	Description string `db:"reward_pool_description"`
	CreatedBy   string `db:"reward_pool_created_by"`
	CreatedAt   string `db:"reward_pool_created_at"`
}

type RewardPoolWithCreator struct {
	RewardPool
	DiscordUser
}

func (d *Database) GetRewardPools(ctx context.Context, userID string) ([]RewardPoolWithCreator, error) {
	// get all reward pools and the creator, filter if the user id is insite reward_pool_users
	query := `
		SELECT reward_pools.*, discord_users.*
		FROM reward_pools
		JOIN discord_users ON reward_pool_created_by = discord_user_id
		LEFT JOIN reward_pool_users ON reward_pool_user_reward_pool_id = reward_pool_id
		WHERE discord_user_id = $1 OR reward_pool_created_by = $1
		GROUP BY reward_pool_id, discord_user_id, reward_pool_created_at
		ORDER BY reward_pool_created_at DESC
	`
	var rewardPools []RewardPoolWithCreator
	if err := d.db.SelectContext(ctx, &rewardPools, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get reward pools: %w", err)
	}

	return rewardPools, nil
}

func (d *Database) InsertRewardPool(ctx context.Context, pool RewardPool) (int, error) {
	query := `
		INSERT INTO reward_pools (reward_pool_name, reward_pool_description, reward_pool_created_by)
		VALUES (:reward_pool_name, :reward_pool_description, :reward_pool_created_by)
		RETURNING reward_pool_id
	`

	query, args, err := d.db.BindNamed(query, pool)
	if err != nil {
		return 0, fmt.Errorf("failed to bind query: %w", err)
	}

	var poolID int
	if err = d.db.GetContext(ctx, &poolID, query, args...); err != nil {
		return 0, err
	}

	return poolID, nil
}

type RewardCode struct {
	ID           int        `db:"reward_code_id"`
	Code         string     `db:"reward_code_code"`
	RewardPoolID int        `db:"reward_code_reward_pool_id"`
	ImportedAt   time.Time  `db:"reward_code_imported_at"`
	ImportedBy   string     `db:"reward_code_imported_by"`
	RedeemCode   *string    `db:"reward_code_redeem_code"`
	RedeemedAt   *time.Time `db:"reward_code_redeemed_at"`
	RedeemedBy   *string    `db:"reward_code_redeemed_by"`
}

func (d *Database) InsertRewardCodes(ctx context.Context, poolID int, codes []string, userID string) error {
	var dbCodes []RewardCode
	for _, code := range codes {
		dbCodes = append(dbCodes, RewardCode{
			Code:         code,
			RewardPoolID: poolID,
			ImportedBy:   userID,
		})
	}
	query := `
		INSERT INTO reward_codes (reward_code_code, reward_code_reward_pool_id, reward_code_imported_by)
		VALUES (:reward_code_code, :reward_code_reward_pool_id, :reward_code_imported_by)
		ON CONFLICT (reward_code_code) DO NOTHING
	`

	_, err := d.db.NamedExecContext(ctx, query, dbCodes)
	if err != nil {
		return fmt.Errorf("failed to insert reward codes: %w", err)
	}

	return nil
}

func (d *Database) GetRewardPool(ctx context.Context, poolID int, userID string) (*RewardPool, error) {
	query := `
		SELECT reward_pools.*
		FROM reward_pools
		LEFT JOIN reward_pool_users ON reward_pool_user_reward_pool_id = reward_pool_id
		WHERE reward_pool_id = $1 AND (reward_pool_created_by = $2 OR reward_pool_user_user_id = $2)
		GROUP BY reward_pool_id
	`
	var pool RewardPool
	if err := d.db.GetContext(ctx, &pool, query, poolID, userID); err != nil {
		return nil, fmt.Errorf("failed to get reward pool: %w", err)
	}

	return &pool, nil
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

func (d *Database) GetRewardCodes(ctx context.Context, poolID int) ([]RewardCodeWithUser, error) {
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
		WHERE reward_code_reward_pool_id = $1
		ORDER BY reward_code_imported_at DESC, reward_code_id DESC
	`
	var codes []RewardCodeWithUser
	if err := d.db.SelectContext(ctx, &codes, query, poolID); err != nil {
		return nil, fmt.Errorf("failed to get reward codes: %w", err)
	}

	return codes, nil
}
