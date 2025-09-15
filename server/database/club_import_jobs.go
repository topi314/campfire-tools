package database

import (
	"context"
	"fmt"
)

func (d *Database) GetClubImportJobs(ctx context.Context) ([]ClubImportJobWithClub, error) {
	query := `
		SELECT *
		FROM club_import_jobs
		LEFT JOIN clubs ON club_import_job_club_id = club_id
		ORDER BY club_import_job_created_at DESC
	`

	var jobs []ClubImportJobWithClub
	if err := d.db.SelectContext(ctx, &jobs, query); err != nil {
		return nil, fmt.Errorf("failed to get club import jobs: %w", err)
	}

	return jobs, nil
}

func (d *Database) GetNextPendingClubImportJob(ctx context.Context) (*ClubImportJob, error) {
	query := `
		SELECT *
		FROM club_import_jobs
		WHERE club_import_job_status = 'pending'
		ORDER BY club_import_job_last_tried_at , club_import_job_created_at
		LIMIT 1
	`

	var job ClubImportJob
	if err := d.db.GetContext(ctx, &job, query); err != nil {
		return nil, fmt.Errorf("failed to get next pending club import job: %w", err)
	}

	return &job, nil
}

func (d *Database) InsertClubImportJob(ctx context.Context, job ClubImportJob) (int, error) {
	query := `
		INSERT INTO club_import_jobs (club_import_job_club_id, club_import_job_created_at, club_import_job_completed_at, club_import_job_last_tried_at, club_import_job_status, club_import_job_state, club_import_job_error)
		VALUES (:club_import_job_club_id, now(), :club_import_job_completed_at, :club_import_job_last_tried_at, :club_import_job_status, :club_import_job_state, :club_import_job_error)
		RETURNING club_import_job_id
	`

	q, args, err := d.db.BindNamed(query, job)
	if err != nil {
		return 0, fmt.Errorf("failed to bind named parameters: %w", err)
	}

	var id int
	if err = d.db.GetContext(ctx, &id, q, args...); err != nil {
		return 0, fmt.Errorf("failed to insert club import job: %w", err)
	}
	return id, nil
}

func (d *Database) UpdateClubImportJob(ctx context.Context, job ClubImportJob) error {
	query := `
		UPDATE club_import_jobs
		SET club_import_job_club_id = :club_import_job_club_id,
			club_import_job_created_at = :club_import_job_created_at,
			club_import_job_completed_at = :club_import_job_completed_at,
			club_import_job_last_tried_at = :club_import_job_last_tried_at,
			club_import_job_status = :club_import_job_status,
			club_import_job_state = :club_import_job_state,
			club_import_job_error = :club_import_job_error
		WHERE club_import_job_id = :club_import_job_id
	`

	if _, err := d.db.NamedExecContext(ctx, query, job); err != nil {
		return fmt.Errorf("failed to update club import job: %w", err)
	}
	return nil
}
