CREATE TABLE users
(
    user_id             VARCHAR PRIMARY KEY,
    user_username       VARCHAR UNIQUE NOT NULL,
    user_display_name   VARCHAR        NOT NULL,
    user_pinned_club_id VARCHAR        REFERENCES clubs (club_id) ON DELETE SET NULL,
    user_imported_at    TIMESTAMP      NOT NULL DEFAULT now()
);

CREATE TABLE reward_groups
(
    reward_group_id          SERIAL PRIMARY KEY,
    reward_group_name        VARCHAR   NOT NULL,
    reward_group_description TEXT      NOT NULL,
    reward_group_created_by  VARCHAR   NOT NULL,
    reward_group_created_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE reward_group_users
(
    reward_group_user_reward_group_id INT REFERENCES reward_groups (id) ON DELETE CASCADE,
    reward_group_user_user_id         VARCHAR   NOT NULL,
    reward_group_user_assigned_at     TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (reward_group_user_reward_group_id, reward_group_user_user_id)
);

CREATE TABLE reward_pools
(
    reward_pool_id              SERIAL PRIMARY KEY,
    reward_pool_name            VARCHAR   NOT NULL,
    reward_pool_description     TEXT      NOT NULL,
    reward_pool_reward_group_id INT REFERENCES reward_groups (id) ON DELETE CASCADE,
    reward_pool_created_by      VARCHAR   NOT NULL,
    reward_pool_created_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE reward_codes
(
    reward_code_id             SERIAL PRIMARY KEY,
    reward_code_code           VARCHAR(255) UNIQUE NOT NULL,
    reward_code_reward_pool_id INT REFERENCES reward_pools (id) ON DELETE CASCADE,
    reward_code_imported_at    TIMESTAMP DEFAULT now(),
    reward_code_imported_by    VARCHAR             NOT NULL,
    reward_code_redeem_code    VARCHAR(255),
    reward_code_redeemed_at    TIMESTAMP,
    reward_code_redeemed_by    VARCHAR
);
