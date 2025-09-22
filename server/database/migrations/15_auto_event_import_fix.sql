ALTER TABLE clubs
    RENAME COLUMN club_last_auto_event_import_at TO club_last_auto_event_imported_at;

ALTER TABLE events
    ADD COLUMN event_last_auto_imported_at TIMESTAMP NOT NULL DEFAULT '0001-01-01 00:00:00+00';
