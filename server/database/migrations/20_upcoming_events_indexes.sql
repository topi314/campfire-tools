-- Speeds up club event range / upcoming lookups (club + time filters).
CREATE INDEX IF NOT EXISTS events_club_id_time_idx
    ON events (event_club_id, event_time);

CREATE INDEX IF NOT EXISTS events_club_id_end_time_idx
    ON events (event_club_id, event_end_time);

-- Speeds up accepted/check-in aggregates without scanning declined RSVPs.
CREATE INDEX IF NOT EXISTS event_rsvps_event_id_status_idx
    ON event_rsvps (event_rsvp_event_id, event_rsvp_status);
