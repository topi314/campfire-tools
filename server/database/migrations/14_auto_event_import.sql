ALTER TABLE clubs
    ADD COLUMN club_auto_event_import         BOOLEAN   NOT NULL DEFAULT TRUE,
    ADD COLUMN club_last_auto_event_import_at TIMESTAMP NOT NULL DEFAULT '0001-01-01 00:00:00+00';

ALTER TABLE events
    ADD COLUMN event_finished BOOLEAN NOT NULL DEFAULT TRUE;