ALTER TABLE event_rsvps
    RENAME COLUMN rsvp_event_id TO event_rsvp_event_id;
ALTER TABLE event_rsvps
    RENAME COLUMN rsvp_member_id TO event_rsvp_member_id;
ALTER TABLE event_rsvps
    RENAME COLUMN rsvp_status TO event_rsvp_status;
ALTER TABLE event_rsvps
    RENAME COLUMN rsvp_imported_at TO event_rsvp_imported_at;
