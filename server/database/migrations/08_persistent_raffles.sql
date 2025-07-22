CREATE TABLE raffles
(
    raffle_id         BIGSERIAL PRIMARY KEY,
    raffle_created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    raffle_user_id    VARCHAR   NOT NULL
);

CREATE TABLE raffle_events
(
    raffle_event_raffle_id BIGINT  NOT NULL REFERENCES raffles (raffle_id) ON DELETE CASCADE,
    raffle_event_event_id  VARCHAR NOT NULL,
    PRIMARY KEY (raffle_event_raffle_id, raffle_event_event_id)
);

CREATE TABLE raffle_winners
(
    raffle_winner_raffle_id BIGINT  NOT NULL REFERENCES raffles (raffle_id) ON DELETE CASCADE,
    raffle_winner_member_id VARCHAR NOT NULL REFERENCES members (member_id),
    PRIMARY KEY (raffle_winner_raffle_id, raffle_winner_member_id)
);

ALTER TABLE sessions
    ADD COLUMN session_user_id VARCHAR NOT NULL DEFAULT '';