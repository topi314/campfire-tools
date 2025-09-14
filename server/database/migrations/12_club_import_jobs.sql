CREATE TABLE club_import_jobs
(
    club_import_job_id            SERIAL PRIMARY KEY,
    club_import_job_club_id       VARCHAR   NOT NULL REFERENCES clubs (club_id) ON DELETE CASCADE,
    club_import_job_created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    club_import_job_completed_at  TIMESTAMP NOT NULL,
    club_import_job_last_tried_at TIMESTAMP NOT NULL,
    club_import_job_status        VARCHAR   NOT NULL,
    club_import_job_state         JSONB     NOT NULL
)