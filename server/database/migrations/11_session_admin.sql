ALTER TABLE sessions
    ADD COLUMN session_admin BOOLEAN DEFAULT FALSE;

ALTER TABLE campfire_tokens
    RENAME COLUMN id TO campfire_token_id;

ALTER TABLE campfire_tokens
    RENAME COLUMN token TO campfire_token_token;

ALTER TABLE campfire_tokens
    RENAME COLUMN expires_at TO campfire_token_expires_at;

ALTER TABLE campfire_tokens
    RENAME COLUMN email TO campfire_token_email;
