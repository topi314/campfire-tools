CREATE TABLE discord_users
(
    discord_user_id           VARCHAR PRIMARY KEY,
    discord_user_username     VARCHAR UNIQUE NOT NULL,
    discord_user_display_name VARCHAR        NOT NULL,
    discord_user_avatar_url   VARCHAR        NOT NULL,
    discord_user_imported_at  TIMESTAMP      NOT NULL DEFAULT now()
);

CREATE TABLE discord_user_pinned_clubs
(
    discord_user_pinned_club_user_id   VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    discord_user_pinned_club_club_id   VARCHAR REFERENCES clubs (club_id) ON DELETE CASCADE,
    discord_user_pinned_club_pinned_at TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (discord_user_pinned_club_user_id, discord_user_pinned_club_club_id)
);

CREATE TABLE rewards
(
    reward_id          BIGSERIAL PRIMARY KEY,
    reward_name        VARCHAR   NOT NULL,
    reward_description TEXT      NOT NULL,
    reward_created_by  VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    reward_created_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE reward_members
(
    reward_member_reward_id       BIGINT REFERENCES rewards (reward_id) ON DELETE CASCADE,
    reward_member_discord_user_id VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    reward_member_added_at        TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (reward_member_reward_id, reward_member_discord_user_id)
);

CREATE TABLE reward_codes
(
    reward_code_id          BIGSERIAL PRIMARY KEY,
    reward_code_code        VARCHAR(255) UNIQUE NOT NULL,
    reward_code_reward_id   BIGINT REFERENCES rewards (reward_id) ON DELETE CASCADE,
    reward_code_imported_at TIMESTAMP DEFAULT now(),
    reward_code_imported_by VARCHAR             REFERENCES discord_users (discord_user_id) ON DELETE SET NULL,
    reward_code_redeem_code VARCHAR(255),
    reward_code_redeemed_at TIMESTAMP,
    reward_code_redeemed_by VARCHAR             REFERENCES discord_users (discord_user_id) ON DELETE SET NULL
);

DELETE
FROM sessions;

ALTER TABLE sessions
    DROP COLUMN session_user_id;

ALTER TABLE sessions
    ADD COLUMN session_user_id VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE CASCADE;

ALTER TABLE clubs
    ADD COLUMN club_verification_channel_id VARCHAR;

CREATE TABLE reward_users
(
    reward_user_id            BIGSERIAL PRIMARY KEY,
    reward_user_created_at    TIMESTAMP NOT NULL DEFAULT now(),
    reward_user_member_id     VARCHAR   REFERENCES members (member_id) ON DELETE SET NULL,
    reward_user_password_hash VARCHAR   NOT NULL,
    reward_user_password_salt VARCHAR   NOT NULL
);

CREATE TABLE reward_sessions
(
    reward_session_id             BIGSERIAL PRIMARY KEY,
    reward_session_created_at     TIMESTAMP NOT NULL DEFAULT now(),
    reward_session_expires_at     TIMESTAMP NOT NULL,
    reward_session_reward_user_id BIGINT REFERENCES reward_users (reward_user_id) ON DELETE CASCADE
);