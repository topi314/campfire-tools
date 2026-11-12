package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type ChecklistWithCounts struct {
	Checklist
	Total    int `db:"total"`
	Received int `db:"received"`
}

type ChecklistEntryRow struct {
	MemberID          string         `db:"member_id"`
	Username          string         `db:"member_username"`
	DisplayName       string         `db:"member_display_name"`
	AvatarURL         string         `db:"member_avatar_url"`
	Received          bool           `db:"checklist_entry_received"`
	ReceivedBy        sql.NullString `db:"checklist_entry_received_by"`
	ReceivedAt        sql.NullTime   `db:"checklist_entry_received_at"`
	MarkerID          sql.NullString `db:"marker_id"`
	MarkerUsername    sql.NullString `db:"marker_username"`
	MarkerDisplayName sql.NullString `db:"marker_display_name"`
	MarkerAvatarURL   sql.NullString `db:"marker_avatar_url"`
}

func (d *Database) GetChecklistsByUserID(ctx context.Context, userID string) ([]ChecklistWithCounts, error) {
	query := `
		SELECT
			checklists.*,
			COUNT(checklist_entries.checklist_entry_member_id) AS total,
			COUNT(checklist_entries.checklist_entry_member_id) FILTER (WHERE checklist_entries.checklist_entry_received) AS received
		FROM checklists
		JOIN checklist_users ON checklist_users.checklist_user_checklist_id = checklists.checklist_id
		LEFT JOIN checklist_entries ON checklist_entries.checklist_entry_checklist_id = checklists.checklist_id
		WHERE checklist_users.checklist_user_user_id = $1
		GROUP BY checklists.checklist_id
		ORDER BY checklists.checklist_created_at DESC, checklists.checklist_id DESC
	`

	var checklists []ChecklistWithCounts
	if err := d.db.SelectContext(ctx, &checklists, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get checklists by user ID: %w", err)
	}
	return checklists, nil
}

func (d *Database) GetChecklistByID(ctx context.Context, id int) (*Checklist, error) {
	var checklist Checklist
	if err := d.db.GetContext(ctx, &checklist, "SELECT * FROM checklists WHERE checklist_id = $1", id); err != nil {
		return nil, err
	}
	return &checklist, nil
}

func (d *Database) GetChecklistByToken(ctx context.Context, token string) (*Checklist, error) {
	var checklist Checklist
	if err := d.db.GetContext(ctx, &checklist, "SELECT * FROM checklists WHERE checklist_token = $1", token); err != nil {
		return nil, err
	}
	return &checklist, nil
}

func (d *Database) InsertChecklist(ctx context.Context, checklist Checklist) (int, error) {
	if checklist.Events == nil {
		checklist.Events = pq.StringArray{}
	}

	query := `
		INSERT INTO checklists (checklist_token, checklist_title, checklist_created_by, checklist_only_checked_in, checklist_events)
		VALUES (:checklist_token, :checklist_title, :checklist_created_by, :checklist_only_checked_in, :checklist_events)
		RETURNING checklist_id
	`

	query, args, err := d.db.BindNamed(query, checklist)
	if err != nil {
		return 0, fmt.Errorf("failed to bind checklist insert: %w", err)
	}

	var id int
	if err = d.db.GetContext(ctx, &id, query, args...); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) UpdateChecklistTitle(ctx context.Context, id int, title string) error {
	if _, err := d.db.ExecContext(ctx, "UPDATE checklists SET checklist_title = $2 WHERE checklist_id = $1", id, title); err != nil {
		return fmt.Errorf("failed to update checklist title: %w", err)
	}
	return nil
}

func (d *Database) AppendChecklistEvents(ctx context.Context, checklistID int, eventIDs []string) error {
	query := `
		UPDATE checklists
		SET checklist_events = (
			SELECT ARRAY(SELECT DISTINCT unnest(checklist_events || $2::varchar[]))
		)
		WHERE checklist_id = $1
	`
	if _, err := d.db.ExecContext(ctx, query, checklistID, pq.Array(eventIDs)); err != nil {
		return fmt.Errorf("failed to append checklist events: %w", err)
	}
	return nil
}

func (d *Database) RemoveChecklistEvents(ctx context.Context, checklistID int, eventIDs []string) error {
	query := `
		UPDATE checklists
		SET checklist_events = COALESCE((
			SELECT ARRAY(
				SELECT e FROM unnest(checklist_events) AS e
				WHERE e <> ALL($2::varchar[])
			)
		), '{}')
		WHERE checklist_id = $1
	`
	if _, err := d.db.ExecContext(ctx, query, checklistID, pq.Array(eventIDs)); err != nil {
		return fmt.Errorf("failed to remove checklist events: %w", err)
	}
	return nil
}

func (d *Database) AddChecklistUser(ctx context.Context, checklistID int, userID string) error {
	query := `
		INSERT INTO checklist_users (checklist_user_checklist_id, checklist_user_user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	if _, err := d.db.ExecContext(ctx, query, checklistID, userID); err != nil {
		return fmt.Errorf("failed to add checklist user: %w", err)
	}
	return nil
}

func (d *Database) IsChecklistUser(ctx context.Context, checklistID int, userID string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM checklist_users
			WHERE checklist_user_checklist_id = $1
			AND checklist_user_user_id = $2
		)
	`
	if err := d.db.GetContext(ctx, &exists, query, checklistID, userID); err != nil {
		return false, fmt.Errorf("failed to check checklist user: %w", err)
	}
	return exists, nil
}

func (d *Database) GetChecklistEntries(ctx context.Context, checklistID int) ([]ChecklistEntryRow, error) {
	query := `
		SELECT
			members.member_id,
			members.member_username,
			members.member_display_name,
			members.member_avatar_url,
			checklist_entries.checklist_entry_received,
			checklist_entries.checklist_entry_received_by,
			checklist_entries.checklist_entry_received_at,
			discord_users.discord_user_id AS marker_id,
			discord_users.discord_user_username AS marker_username,
			discord_users.discord_user_display_name AS marker_display_name,
			discord_users.discord_user_avatar_url AS marker_avatar_url
		FROM checklist_entries
		JOIN members ON members.member_id = checklist_entries.checklist_entry_member_id
		LEFT JOIN discord_users ON discord_users.discord_user_id = checklist_entries.checklist_entry_received_by
		WHERE checklist_entries.checklist_entry_checklist_id = $1
		ORDER BY checklist_entries.checklist_entry_received ASC, members.member_display_name ASC, members.member_username ASC
	`

	var rows []ChecklistEntryRow
	if err := d.db.SelectContext(ctx, &rows, query, checklistID); err != nil {
		return nil, fmt.Errorf("failed to get checklist entries: %w", err)
	}
	return rows, nil
}

func (d *Database) InsertChecklistEntries(ctx context.Context, checklistID int, memberIDs []string) error {
	if len(memberIDs) == 0 {
		return nil
	}

	query := `
		INSERT INTO checklist_entries (checklist_entry_checklist_id, checklist_entry_member_id)
		SELECT $1, member_id
		FROM unnest($2::varchar[]) AS member_id
		ON CONFLICT DO NOTHING
	`
	if _, err := d.db.ExecContext(ctx, query, checklistID, pq.Array(memberIDs)); err != nil {
		return fmt.Errorf("failed to insert checklist entries: %w", err)
	}
	return nil
}

func (d *Database) DeleteChecklistEntry(ctx context.Context, checklistID int, memberID string) error {
	if _, err := d.db.ExecContext(ctx, `
		DELETE FROM checklist_entries
		WHERE checklist_entry_checklist_id = $1
		AND checklist_entry_member_id = $2
	`, checklistID, memberID); err != nil {
		return fmt.Errorf("failed to delete checklist entry: %w", err)
	}
	return nil
}

// MarkChecklistEntryReceived marks a waiting person as received.
// updated is false when they were already received. A missing row returns sql.ErrNoRows.
func (d *Database) MarkChecklistEntryReceived(ctx context.Context, checklistID int, memberID, userID string, at time.Time) (updated bool, err error) {
	result, err := d.db.ExecContext(ctx, `
		UPDATE checklist_entries
		SET checklist_entry_received = TRUE,
			checklist_entry_received_by = $3,
			checklist_entry_received_at = $4
		WHERE checklist_entry_checklist_id = $1
		AND checklist_entry_member_id = $2
		AND checklist_entry_received = FALSE
	`, checklistID, memberID, userID, at)
	if err != nil {
		return false, fmt.Errorf("failed to mark checklist entry received: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}

	exists, err := d.checklistEntryExists(ctx, checklistID, memberID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, sql.ErrNoRows
	}
	return false, nil
}

func (d *Database) UnmarkChecklistEntryReceived(ctx context.Context, checklistID int, memberID string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE checklist_entries
		SET checklist_entry_received = FALSE,
			checklist_entry_received_by = NULL,
			checklist_entry_received_at = NULL
		WHERE checklist_entry_checklist_id = $1
		AND checklist_entry_member_id = $2
	`, checklistID, memberID)
	if err != nil {
		return fmt.Errorf("failed to unmark checklist entry: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		exists, err := d.checklistEntryExists(ctx, checklistID, memberID)
		if err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
	}
	return nil
}

func (d *Database) checklistEntryExists(ctx context.Context, checklistID int, memberID string) (bool, error) {
	var exists bool
	err := d.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1
			FROM checklist_entries
			WHERE checklist_entry_checklist_id = $1
			AND checklist_entry_member_id = $2
		)
	`, checklistID, memberID)
	if err != nil {
		return false, fmt.Errorf("failed to check checklist entry: %w", err)
	}
	return exists, nil
}
