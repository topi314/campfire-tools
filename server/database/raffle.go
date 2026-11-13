package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

func (d *Database) GetRafflesByUserID(ctx context.Context, userID string) ([]Raffle, error) {
	query := `
		SELECT DISTINCT raffles.*
		FROM raffles
		LEFT JOIN raffle_users ON raffle_users.raffle_user_raffle_id = raffles.raffle_id
			AND raffle_users.raffle_user_user_id = $1
		WHERE raffles.raffle_user_id = $1
			OR raffle_users.raffle_user_user_id IS NOT NULL
		ORDER BY raffles.raffle_created_at DESC, raffles.raffle_id DESC
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

func (d *Database) GetRaffleByToken(ctx context.Context, token string) (*Raffle, error) {
	var raffle Raffle
	if err := d.db.GetContext(ctx, &raffle, "SELECT * FROM raffles WHERE raffle_token = $1", token); err != nil {
		return nil, err
	}
	return &raffle, nil
}

func (d *Database) GetRaffleEvents(ctx context.Context, raffleID int) ([]Event, error) {
	query := `
		SELECT events.*
		FROM events
		JOIN raffles ON $1 = raffles.raffle_id
		WHERE events.event_id = ANY(raffles.raffle_events)
	`

	var events []Event
	if err := d.db.SelectContext(ctx, &events, query, raffleID); err != nil {
		return nil, fmt.Errorf("failed to get raffle events: %w", err)
	}

	return events, nil
}

func (d *Database) AppendRaffleEvents(ctx context.Context, raffleID int, eventIDs []string) error {
	query := `
		UPDATE raffles
		SET raffle_events = (
			SELECT ARRAY(SELECT DISTINCT unnest(raffle_events || $2::varchar[]))
		)
		WHERE raffle_id = $1
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, pq.Array(eventIDs)); err != nil {
		return fmt.Errorf("failed to append raffle events: %w", err)
	}

	return nil
}

func (d *Database) RemoveRaffleEvents(ctx context.Context, raffleID int, eventIDs []string) error {
	query := `
		UPDATE raffles
		SET raffle_events = (
			SELECT ARRAY(
				SELECT e FROM unnest(raffle_events) AS e
				WHERE e <> ALL($2::varchar[])
			)
		)
		WHERE raffle_id = $1
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, pq.Array(eventIDs)); err != nil {
		return fmt.Errorf("failed to remove raffle events: %w", err)
	}

	return nil
}

func (d *Database) UpdateRaffleSettings(ctx context.Context, raffleID int, title string, winnerCount int, onlyCheckedIn, singleEntry bool) error {
	query := `
		UPDATE raffles
		SET raffle_title = $2,
			raffle_winner_count = $3,
			raffle_only_checked_in = $4,
			raffle_single_entry = $5
		WHERE raffle_id = $1
	`
	if _, err := d.db.ExecContext(ctx, query, raffleID, title, winnerCount, onlyCheckedIn, singleEntry); err != nil {
		return fmt.Errorf("failed to update raffle settings: %w", err)
	}
	return nil
}

func (d *Database) InsertRaffle(ctx context.Context, raffle Raffle) (int, error) {
	query := `
		INSERT INTO raffles (raffle_token, raffle_user_id, raffle_title, raffle_events, raffle_winner_count, raffle_only_checked_in, raffle_single_entry)
		VALUES (:raffle_token, :raffle_user_id, :raffle_title, :raffle_events, :raffle_winner_count, :raffle_only_checked_in, :raffle_single_entry)
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
		SELECT
			raffle_winners.*,
			members.*,
			(
				SELECT COUNT(*)
				FROM event_rsvps
				WHERE event_rsvp_event_id = ANY(raffle_events)
				AND event_rsvp_member_id = raffle_winner_member_id
				AND event_rsvp_status = 'CHECKED_IN'
			) AS check_ins,
			(
				SELECT COUNT(*)
				FROM event_rsvps
				WHERE event_rsvp_event_id = ANY(raffle_events)
				AND event_rsvp_member_id = raffle_winner_member_id
				AND event_rsvp_status IN ('CHECKED_IN', 'ACCEPTED')
			) AS accepted,
			discord_users.discord_user_id AS marker_id,
			discord_users.discord_user_username AS marker_username,
			discord_users.discord_user_display_name AS marker_display_name,
			discord_users.discord_user_avatar_url AS marker_avatar_url
		FROM raffle_winners
		JOIN members ON raffle_winner_member_id = member_id
		JOIN raffles ON raffle_winner_raffle_id = raffle_id
		LEFT JOIN discord_users ON discord_users.discord_user_id = raffle_winners.raffle_winner_confirmed_by
		WHERE raffle_winner_raffle_id = $1
		ORDER BY raffle_winner_created_at DESC
	`

	var winners []RaffleWinnerWithMember
	if err := d.db.SelectContext(ctx, &winners, query, raffleID); err != nil {
		return nil, fmt.Errorf("failed to get raffle winners: %w", err)
	}

	return winners, nil
}

func (d *Database) DeleteCurrentRaffleWinner(ctx context.Context, raffleID int, memberID string) error {
	query := `
		DELETE FROM raffle_winners
		WHERE raffle_winner_raffle_id = $1
		  AND raffle_winner_member_id = $2
		  AND raffle_winner_confirmed = FALSE
		  AND raffle_winner_past = FALSE
	`

	if _, err := d.db.ExecContext(ctx, query, raffleID, memberID); err != nil {
		return fmt.Errorf("failed to delete current raffle winner: %w", err)
	}

	return nil
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

func (d *Database) ConfirmRaffleWinner(ctx context.Context, raffleID int, memberID, userID string) (bool, error) {
	query := `
		UPDATE raffle_winners
		SET raffle_winner_confirmed = TRUE,
			raffle_winner_confirmed_by = NULLIF($3, ''),
			raffle_winner_confirmed_at = now()
		WHERE raffle_winner_raffle_id = $1
			AND raffle_winner_member_id = $2
			AND raffle_winner_confirmed = FALSE
			AND raffle_winner_past = FALSE
	`

	result, err := d.db.ExecContext(ctx, query, raffleID, memberID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to confirm raffle winner: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to confirm raffle winner: %w", err)
	}
	return updated > 0, nil
}

func (d *Database) UnconfirmRaffleWinner(ctx context.Context, raffleID int, memberID string) (bool, error) {
	query := `
		UPDATE raffle_winners
		SET raffle_winner_confirmed = FALSE,
			raffle_winner_confirmed_by = NULL,
			raffle_winner_confirmed_at = NULL,
			raffle_winner_past = FALSE
		WHERE raffle_winner_raffle_id = $1
			AND raffle_winner_member_id = $2
			AND raffle_winner_confirmed = TRUE
	`

	result, err := d.db.ExecContext(ctx, query, raffleID, memberID)
	if err != nil {
		return false, fmt.Errorf("failed to unconfirm raffle winner: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to unconfirm raffle winner: %w", err)
	}
	return updated > 0, nil
}

type RaffleConfirmState struct {
	DisplayName string         `db:"member_display_name"`
	Confirmed   sql.NullBool   `db:"raffle_winner_confirmed"`
	Past        sql.NullBool   `db:"raffle_winner_past"`
	ConfirmedAt sql.NullTime   `db:"raffle_winner_confirmed_at"`
	MarkerName  sql.NullString `db:"marker_name"`
}

func (d *Database) GetRaffleConfirmState(ctx context.Context, raffleID int, memberID string) (*RaffleConfirmState, error) {
	query := `
		SELECT
			members.member_display_name,
			raffle_winners.raffle_winner_confirmed,
			raffle_winners.raffle_winner_past,
			raffle_winners.raffle_winner_confirmed_at,
			COALESCE(NULLIF(discord_users.discord_user_display_name, ''), discord_users.discord_user_username) AS marker_name
		FROM members
		LEFT JOIN raffle_winners ON raffle_winners.raffle_winner_member_id = members.member_id
			AND raffle_winners.raffle_winner_raffle_id = $1
		LEFT JOIN discord_users ON discord_users.discord_user_id = raffle_winners.raffle_winner_confirmed_by
		WHERE members.member_id = $2
	`
	var state RaffleConfirmState
	if err := d.db.GetContext(ctx, &state, query, raffleID, memberID); err != nil {
		return nil, err
	}
	return &state, nil
}

func (d *Database) AddRaffleUser(ctx context.Context, raffleID int, userID string) error {
	query := `
		INSERT INTO raffle_users (raffle_user_raffle_id, raffle_user_user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	if _, err := d.db.ExecContext(ctx, query, raffleID, userID); err != nil {
		return fmt.Errorf("failed to add raffle user: %w", err)
	}
	return nil
}

func (d *Database) IsRaffleUser(ctx context.Context, raffleID int, userID string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM raffle_users
			WHERE raffle_user_raffle_id = $1
			AND raffle_user_user_id = $2
		)
	`
	if err := d.db.GetContext(ctx, &exists, query, raffleID, userID); err != nil {
		return false, fmt.Errorf("failed to check raffle user: %w", err)
	}
	return exists, nil
}

func (d *Database) GetRaffleUsers(ctx context.Context, raffleID int) ([]DiscordUser, error) {
	query := `
		SELECT
			discord_users.discord_user_id,
			discord_users.discord_user_username,
			discord_users.discord_user_display_name,
			discord_users.discord_user_avatar_url,
			discord_users.discord_user_imported_at
		FROM raffle_users
		JOIN discord_users ON discord_users.discord_user_id = raffle_users.raffle_user_user_id
		WHERE raffle_users.raffle_user_raffle_id = $1
		ORDER BY discord_users.discord_user_display_name ASC, discord_users.discord_user_username ASC
	`
	var users []DiscordUser
	if err := d.db.SelectContext(ctx, &users, query, raffleID); err != nil {
		return nil, fmt.Errorf("failed to get raffle users: %w", err)
	}
	return users, nil
}

// RunRaffleDraw holds the raffle row so two people cannot draw the same person.
// Anyone already drawn and still current, or already confirmed, is left out of this draw.
func (d *Database) RunRaffleDraw(ctx context.Context, raffleID int, draw func(existing []RaffleWinnerWithMember) ([]string, error)) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start raffle draw: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `SELECT raffle_id FROM raffles WHERE raffle_id = $1 FOR UPDATE`, raffleID); err != nil {
		return fmt.Errorf("failed to lock raffle: %w", err)
	}

	var rows []RaffleWinner
	if err = tx.SelectContext(ctx, &rows, `
		SELECT raffle_winner_raffle_id, raffle_winner_member_id, raffle_winner_confirmed, raffle_winner_past, raffle_winner_created_at
		FROM raffle_winners
		WHERE raffle_winner_raffle_id = $1
	`, raffleID); err != nil {
		return fmt.Errorf("failed to read raffle winners: %w", err)
	}

	existing := make([]RaffleWinnerWithMember, len(rows))
	locked := make(map[string]struct{}, len(rows))
	for i, row := range rows {
		existing[i].RaffleWinner = row
		existing[i].Member.ID = row.MemberID
		if row.Confirmed || !row.Past {
			locked[row.MemberID] = struct{}{}
		}
	}

	memberIDs, err := draw(existing)
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(memberIDs))
	seen := make(map[string]struct{}, len(memberIDs))
	for _, memberID := range memberIDs {
		if _, ok := locked[memberID]; ok {
			continue
		}
		if _, ok := seen[memberID]; ok {
			continue
		}
		seen[memberID] = struct{}{}
		filtered = append(filtered, memberID)
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM raffle_winners
		WHERE raffle_winner_raffle_id = $1
			AND raffle_winner_confirmed = FALSE
			AND raffle_winner_past = FALSE
	`, raffleID); err != nil {
		return fmt.Errorf("failed to clear current raffle winners: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE raffle_winners
		SET raffle_winner_past = TRUE
		WHERE raffle_winner_raffle_id = $1
			AND raffle_winner_past = FALSE
	`, raffleID); err != nil {
		return fmt.Errorf("failed to keep confirmed raffle winners: %w", err)
	}
	for _, memberID := range filtered {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO raffle_winners (raffle_winner_raffle_id, raffle_winner_member_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, raffleID, memberID); err != nil {
			return fmt.Errorf("failed to insert raffle winner: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to finish raffle draw: %w", err)
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
