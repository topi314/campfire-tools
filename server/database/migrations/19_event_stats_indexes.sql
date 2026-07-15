CREATE INDEX IF NOT EXISTS events_club_id_idx
    ON events (event_club_id);

CREATE INDEX IF NOT EXISTS events_campfire_live_event_id_idx
    ON events (event_campfire_live_event_id)
    WHERE event_campfire_live_event_id <> '';
