package database

import (
	"context"
	"fmt"
)

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

func (d *Database) GetRaffleWinners(ctx context.Context, raffleID int) ([]RaffleWinner, error) {
	query := `
		SELECT * FROM raffle_winners WHERE raffle_winner_raffle_id = $1
	`

	var winners []RaffleWinner
	if err := d.db.SelectContext(ctx, &winners, query, raffleID); err != nil {
		return nil, err
	}

	return winners, nil
}

func (d *Database) InsertRaffleWinner(ctx context.Context, raffleID int, memberID string) error {
	query := `
		INSERT INTO raffle_winners (raffle_winner_raffle_id, raffle_winner_member_id)
		VALUES ($1, $2)
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, memberID); err != nil {
		return err
	}

	return nil
}
