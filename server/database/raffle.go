package database

import (
	"context"
	"fmt"
)

func (d *Database) GetRafflesByUserID(ctx context.Context, userID string) ([]Raffle, error) {
	query := `
		SELECT * FROM raffles
		WHERE raffle_user_id = $1
		ORDER BY raffle_created_at DESC, raffle_id DESC
	`

	var raffles []Raffle
	if err := d.db.SelectContext(ctx, &raffles, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get raffles by user ID: %w", err)
	}
	return raffles, nil
}

func (d *Database) GetRaffleByID(ctx context.Context, id int) (*Raffle, error) {
	var raffle Raffle
	if err := d.db.GetContext(ctx, &raffle, "SELECT * FROM raffles WHERE raffle_id = $1", id); err != nil {
		return nil, err
	}
	return &raffle, nil
}

func (d *Database) InsertRaffle(ctx context.Context, raffle Raffle) (int, error) {
	query := `
		INSERT INTO raffles (raffle_user_id, raffle_events, raffle_winner_count, raffle_only_checked_in, raffle_single_entry)
		VALUES (:raffle_user_id, :raffle_events, :raffle_winner_count, :raffle_only_checked_in, :raffle_single_entry)
		RETURNING raffle_id
	`

	query, args, err := d.db.BindNamed(query, raffle)
	if err != nil {
		return 0, fmt.Errorf("failed to bind query: %w", err)
	}

	var raffleID int
	if err = d.db.GetContext(ctx, &raffleID, query, args...); err != nil {
		return 0, err
	}

	return raffleID, nil
}

func (d *Database) GetRaffleWinners(ctx context.Context, raffleID int) ([]RaffleWinnerWithMember, error) {
	query := `
		SELECT members.*,
			COUNT(CASE WHEN event_rsvp_status = 'ACCEPTED' or event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS accepted,
			COUNT(CASE WHEN event_rsvp_status = 'CHECKED_IN' THEN 1 END) AS check_ins
		FROM raffle_winners
		JOIN members ON raffle_winner_member_id = member_id
		JOIN event_rsvps ON member_id = event_rsvp_member_id
		WHERE raffle_winner_raffle_id = $1
		GROUP BY member_id, raffle_winner_created_at, member_display_name, member_username
		ORDER BY raffle_winner_created_at DESC, member_display_name, member_username, member_id
	`

	var winners []RaffleWinnerWithMember
	if err := d.db.SelectContext(ctx, &winners, query, raffleID); err != nil {
		return nil, fmt.Errorf("failed to get raffle winners: %w", err)
	}

	return winners, nil
}

func (d *Database) DeleteNotConfirmedRaffleWinners(ctx context.Context, raffleID int) error {
	query := `
		DELETE FROM raffle_winners
		WHERE raffle_winner_raffle_id = $1 AND raffle_winner_confirmed = FALSE
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID); err != nil {
		return fmt.Errorf("failed to delete not confirmed raffle winners: %w", err)
	}

	return nil
}

func (d *Database) InsertRaffleWinners(ctx context.Context, raffleID int, memberIDs []string) error {
	winners := make([]RaffleWinner, len(memberIDs))
	for i, id := range memberIDs {
		winners[i] = RaffleWinner{
			RaffleID: raffleID,
			MemberID: id,
		}
	}

	query := `
		INSERT INTO raffle_winners (raffle_winner_raffle_id, raffle_winner_member_id)
		VALUES (:raffle_winner_raffle_id, :raffle_winner_member_id)
	`

	if _, err := d.db.NamedExecContext(ctx, query, winners); err != nil {
		return fmt.Errorf("failed to insert raffle winners: %w", err)
	}

	return nil
}

func (d *Database) ConfirmRaffleWinner(ctx context.Context, raffleID int, memberID string) error {
	query := `
		UPDATE raffle_winners
		SET raffle_winner_confirmed = TRUE
		WHERE raffle_winner_raffle_id = $1 AND raffle_winner_member_id = $2
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, memberID); err != nil {
		return fmt.Errorf("failed to confirm raffle winner: %w", err)
	}

	return nil
}

func (d *Database) DeleteUnconfirmedRaffleWinners(ctx context.Context, raffleID int) error {
	query := `
		DELETE FROM raffle_winners
		WHERE raffle_winner_raffle_id = $1 AND raffle_winner_confirmed = FALSE
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID); err != nil {
		return fmt.Errorf("failed to delete unconfirmed raffle winners: %w", err)
	}

	return nil
}

func (d *Database) MarkRaffleWinnersAsPast(ctx context.Context, raffleID int) error {
	query := `
		UPDATE raffle_winners
		SET raffle_winner_past = TRUE
		WHERE raffle_winner_raffle_id = $1
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID); err != nil {
		return fmt.Errorf("failed to mark raffle winners as past: %w", err)
	}

	return nil
}
