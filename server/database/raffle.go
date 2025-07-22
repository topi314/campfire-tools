package database

import (
	"context"
)

func (d *Database) GetRaffleByID(ctx context.Context, id string) (*Raffle, error) {
	var raffle Raffle
	if err := d.db.GetContext(ctx, &raffle, "SELECT * FROM raffles WHERE raffle_id = $1", id); err != nil {
		return nil, err
	}
	return &raffle, nil
}

func (d *Database) InsertRaffle(ctx context.Context) (int, error) {
	query := `
		INSERT INTO raffles (raffle_id, raffle_created_at)
		VALUES ($1, $2)
		RETURNING raffle_id
	`

	var raffleID int
	if err := d.db.GetContext(ctx, &raffleID, query); err != nil {
		return 0, err
	}

	return raffleID, nil
}

func (d *Database) GetRaffleEvents(ctx context.Context, raffleID int) ([]RaffleEvent, error) {
	query := "SELECT * FROM raffle_events WHERE raffle_event_raffle_id = $1"

	var raffleEvents []RaffleEvent
	if err := d.db.SelectContext(ctx, &raffleEvents, query, raffleID); err != nil {
		return nil, err
	}

	return raffleEvents, nil
}

func (d *Database) InsertRaffleEvents(ctx context.Context, raffleID int, eventIDs []string) error {
	query := `
		INSERT INTO raffle_events (raffle_event_raffle_id, raffle_event_event_id)
		VALUES (:raffle_event_raffle_id, :raffle_event_event_id)
		`

	events := make([]RaffleEvent, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		events = append(events, RaffleEvent{
			RaffleID: raffleID,
			EventID:  eventID,
		})
	}
	if _, err := d.db.NamedExecContext(ctx, query, events); err != nil {
		return err
	}

	return nil
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

func (d *Database) InsertRaffleWinner(ctx context.Context, raffleID int, memberID int) error {
	query := `
		INSERT INTO raffle_winners (raffle_winner_raffle_id, raffle_winner_member_id)
		VALUES ($1, $2)
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, memberID); err != nil {
		return err
	}

	return nil
}
