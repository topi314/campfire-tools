ALTER TABLE events
    ADD COLUMN event_category VARCHAR NOT NULL DEFAULT '';

-- Backfill from campfire live event name using the same patterns as eventcategory.FromName
UPDATE events
SET event_category = CASE
    WHEN TRIM(COALESCE(event_campfire_live_event_name, '')) = '' THEN 'No Event'
    WHEN event_campfire_live_event_name ILIKE '%GOWA%' OR event_campfire_live_event_name ILIKE '%GO Wild Area%' THEN 'GO Wild Area'
    WHEN event_campfire_live_event_name ILIKE '%GO Fest%' THEN 'GO Fest'
    WHEN event_campfire_live_event_name ILIKE '%GO Tour%' THEN 'GO Tour'
    WHEN event_campfire_live_event_name ILIKE '%Community Day%' OR event_campfire_live_event_name ILIKE '%Community Classic Day%' THEN 'Community Day'
    WHEN event_campfire_live_event_name ILIKE '%Max Battle Weekend%'
        OR event_campfire_live_event_name ILIKE '%Max Battle Day%'
        OR event_campfire_live_event_name ILIKE '%Max Weekend%'
        OR event_campfire_live_event_name ILIKE '%Gigantamax%'
        OR event_campfire_live_event_name ILIKE '%GMAX%' THEN 'Max Battle'
    WHEN event_campfire_live_event_name ILIKE '%Research Day%' THEN 'Research Day'
    WHEN event_campfire_live_event_name ILIKE '%Hatch Day%' THEN 'Hatch Day'
    WHEN event_campfire_live_event_name ILIKE '%Friendship Friday%' THEN 'Friendship Friday'
    WHEN event_campfire_live_event_name ILIKE '%Raid Day%' OR event_campfire_live_event_name ILIKE '%Mega Raid%' THEN 'Raid Day'
    WHEN event_campfire_live_event_name ILIKE '%Raid Hour%' THEN 'Raid Hour'
    WHEN event_campfire_live_event_name ILIKE '%Max Monday%' THEN 'Max Monday'
    WHEN event_campfire_live_event_name ILIKE '%Spotlight Hour%' THEN 'Spotlight Hour'
    ELSE 'Other'
END
WHERE event_category = '';
