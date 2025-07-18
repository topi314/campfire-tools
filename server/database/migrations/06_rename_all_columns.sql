ALTER TABLE members
    RENAME COLUMN id TO member_id;
ALTER TABLE members
    RENAME COLUMN username TO member_username;
ALTER TABLE members
    RENAME COLUMN display_name TO member_display_name;
ALTER TABLE members
    RENAME COLUMN avatar_url TO member_avatar_url;
ALTER TABLE members
    RENAME COLUMN imported_at TO member_imported_at;
ALTER TABLE members
    RENAME COLUMN raw_json TO member_raw_json;

ALTER TABLE clubs
    RENAME COLUMN id TO club_id;
ALTER TABLE clubs
    RENAME COLUMN name TO club_name;
ALTER TABLE clubs
    RENAME COLUMN avatar_url TO club_avatar_url;
ALTER TABLE clubs
    RENAME COLUMN creator_id TO club_creator_id;
ALTER TABLE clubs
    RENAME COLUMN created_by_community_ambassador TO club_created_by_community_ambassador;
ALTER TABLE clubs
    RENAME COLUMN imported_at TO club_imported_at;
ALTER TABLE clubs
    RENAME COLUMN raw_json TO club_raw_json;

ALTER TABLE events
    RENAME COLUMN id TO event_id;
ALTER TABLE events
    RENAME COLUMN name TO event_name;
ALTER TABLE events
    RENAME COLUMN details TO event_details;
ALTER TABLE events
    RENAME COLUMN address TO event_address;
ALTER TABLE events
    RENAME COLUMN location TO event_location;
ALTER TABLE events
    RENAME COLUMN creator_id TO event_creator_id;
ALTER TABLE events
    RENAME COLUMN cover_photo_url TO event_cover_photo_url;
ALTER TABLE events
    RENAME COLUMN discord_interested TO event_discord_interested;
ALTER TABLE events
    RENAME COLUMN created_by_community_ambassador TO event_created_by_community_ambassador;
ALTER TABLE events
    RENAME COLUMN campfire_live_event_id TO event_campfire_live_event_id;
ALTER TABLE events
    RENAME COLUMN campfire_live_event_name TO event_campfire_live_event_name;
ALTER TABLE events
    RENAME COLUMN club_id TO event_club_id;
ALTER TABLE events
    RENAME COLUMN imported_at TO event_imported_at;
ALTER TABLE events
    RENAME COLUMN raw_json TO event_raw_json;

ALTER TABLE event_rsvps
    RENAME COLUMN event_id TO rsvp_event_id;
ALTER TABLE event_rsvps
    RENAME COLUMN member_id TO rsvp_member_id;
ALTER TABLE event_rsvps
    RENAME COLUMN status TO rsvp_status;
ALTER TABLE event_rsvps
    RENAME COLUMN imported_at TO rsvp_imported_at;

ALTER TABLE sessions
    RENAME COLUMN id TO session_id;
ALTER TABLE sessions
    RENAME COLUMN created_at TO session_created_at;
ALTER TABLE sessions
    RENAME COLUMN expires_at TO session_expires_at;
