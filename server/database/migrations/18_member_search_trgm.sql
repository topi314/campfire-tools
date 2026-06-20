CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX members_username_trgm_idx
    ON members USING gin (member_username gin_trgm_ops)
    WHERE member_username <> '';

CREATE INDEX members_display_name_trgm_idx
    ON members USING gin (member_display_name gin_trgm_ops)
    WHERE member_display_name <> '';
