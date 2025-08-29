CREATE TABLE campfire_tokens
(
    id         BIGSERIAL PRIMARY KEY,
    token      VARCHAR   NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    email      VARCHAR   NOT NULL
);