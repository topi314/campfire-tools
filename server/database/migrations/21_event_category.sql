ALTER TABLE events
    ADD COLUMN event_category VARCHAR NOT NULL DEFAULT '';

-- Speeds club event filters that include category (events/stats/members/export/raffle)
-- and GetClubEventCategories DISTINCT lookups (leftmost prefix).
CREATE INDEX IF NOT EXISTS events_club_id_category_time_idx
    ON events (event_club_id, event_category, event_time);

-- Speeds creator-filtered club event queries (same filter form as category).
CREATE INDEX IF NOT EXISTS events_club_id_creator_time_idx
    ON events (event_club_id, event_creator_id, event_time);
