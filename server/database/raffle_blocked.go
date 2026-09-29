package database

import (
	"context"
	"fmt"
)

func (d *Database) AddDiscordUserRaffleBlockedMember(ctx context.Context, userID, memberID string) error {
	query := `
		INSERT INTO discord_user_raffle_blocked_members (
			discord_user_raffle_blocked_member_user_id,
			discord_user_raffle_blocked_member_member_id
		)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	if _, err := d.db.ExecContext(ctx, query, userID, memberID); err != nil {
		return fmt.Errorf("failed to add raffle blocked member for discord user: %w", err)
	}

	return nil
}

func (d *Database) RemoveDiscordUserRaffleBlockedMember(ctx context.Context, userID, memberID string) error {
	query := `
		DELETE FROM discord_user_raffle_blocked_members
		WHERE discord_user_raffle_blocked_member_user_id = $1
		  AND discord_user_raffle_blocked_member_member_id = $2
	`

	if _, err := d.db.ExecContext(ctx, query, userID, memberID); err != nil {
		return fmt.Errorf("failed to remove raffle blocked member for discord user: %w", err)
	}

	return nil
}

func (d *Database) GetDiscordUserRaffleBlockedMemberIDs(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT discord_user_raffle_blocked_member_member_id
		FROM discord_user_raffle_blocked_members
		WHERE discord_user_raffle_blocked_member_user_id = $1
		ORDER BY discord_user_raffle_blocked_member_blocked_at
	`

	var memberIDs []string
	if err := d.db.SelectContext(ctx, &memberIDs, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get raffle blocked member IDs for discord user: %w", err)
	}

	return memberIDs, nil
}

func (d *Database) GetDiscordUserRaffleBlockedMembers(ctx context.Context, userID string) ([]Member, error) {
	query := `
		SELECT members.*
		FROM discord_user_raffle_blocked_members
		JOIN members ON discord_user_raffle_blocked_member_member_id = member_id
		WHERE discord_user_raffle_blocked_member_user_id = $1
		ORDER BY discord_user_raffle_blocked_member_blocked_at
	`

	var members []Member
	if err := d.db.SelectContext(ctx, &members, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get raffle blocked members for discord user: %w", err)
	}

	return members, nil
}
