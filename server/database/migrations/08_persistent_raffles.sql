CREATE TABLE raffles
(
    raffle_id              BIGSERIAL PRIMARY KEY,
    raffle_user_id         VARCHAR   NOT NULL,
    raffle_events          VARCHAR[] NOT NULL DEFAULT '{}',
    raffle_winner_count    INTEGER   NOT NULL DEFAULT 1,
    raffle_only_checked_in BOOLEAN   NOT NULL DEFAULT TRUE,
    raffle_single_entry    BOOLEAN   NOT NULL DEFAULT TRUE,
    raffle_created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE raffle_winners
(
    raffle_winner_raffle_id BIGINT  NOT NULL REFERENCES raffles (raffle_id) ON DELETE CASCADE,
    raffle_winner_member_id VARCHAR NOT NULL REFERENCES members (member_id),
    PRIMARY KEY (raffle_winner_raffle_id, raffle_winner_member_id)
);

ALTER TABLE sessions
    ADD COLUMN session_user_id VARCHAR NOT NULL DEFAULT '';