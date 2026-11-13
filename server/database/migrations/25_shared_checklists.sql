CREATE TABLE checklists
(
    checklist_id              BIGSERIAL PRIMARY KEY,
    checklist_token           VARCHAR   NOT NULL UNIQUE,
    checklist_title           VARCHAR   NOT NULL,
    checklist_created_by      VARCHAR   REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    checklist_only_checked_in BOOLEAN   NOT NULL DEFAULT TRUE,
    checklist_events          VARCHAR[] NOT NULL DEFAULT '{}',
    checklist_created_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE checklist_entries
(
    checklist_entry_checklist_id BIGINT    NOT NULL REFERENCES checklists (checklist_id) ON DELETE CASCADE,
    checklist_entry_member_id    VARCHAR   NOT NULL REFERENCES members (member_id),
    checklist_entry_received     BOOLEAN   NOT NULL DEFAULT FALSE,
    checklist_entry_received_by  VARCHAR   REFERENCES discord_users (discord_user_id) ON DELETE SET NULL,
    checklist_entry_received_at  TIMESTAMP,
    PRIMARY KEY (checklist_entry_checklist_id, checklist_entry_member_id)
);

CREATE TABLE checklist_users
(
    checklist_user_checklist_id BIGINT    NOT NULL REFERENCES checklists (checklist_id) ON DELETE CASCADE,
    checklist_user_user_id      VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    checklist_user_opened_at    TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (checklist_user_checklist_id, checklist_user_user_id)
);

CREATE INDEX checklist_users_user_id_idx ON checklist_users (checklist_user_user_id);

ALTER TABLE raffles
    ADD COLUMN raffle_token VARCHAR;

UPDATE raffles
SET raffle_token = md5(raffle_id::text || ':' || random()::text || ':' || clock_timestamp()::text)
WHERE raffle_token IS NULL;

ALTER TABLE raffles
    ALTER COLUMN raffle_token SET NOT NULL;

ALTER TABLE raffles
    ADD CONSTRAINT raffles_raffle_token_key UNIQUE (raffle_token);

ALTER TABLE raffles
    ADD COLUMN raffle_title VARCHAR NOT NULL DEFAULT '';

CREATE TABLE raffle_users
(
    raffle_user_raffle_id BIGINT    NOT NULL REFERENCES raffles (raffle_id) ON DELETE CASCADE,
    raffle_user_user_id   VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    raffle_user_opened_at TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (raffle_user_raffle_id, raffle_user_user_id)
);

CREATE INDEX raffle_users_user_id_idx ON raffle_users (raffle_user_user_id);

INSERT INTO raffle_users (raffle_user_raffle_id, raffle_user_user_id)
SELECT raffle_id, raffle_user_id
FROM raffles
WHERE raffle_user_id <> ''
  AND EXISTS (
      SELECT 1
      FROM discord_users
      WHERE discord_user_id = raffles.raffle_user_id
  )
ON CONFLICT DO NOTHING;

ALTER TABLE raffle_winners
    ADD COLUMN raffle_winner_confirmed_by VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE SET NULL,
    ADD COLUMN raffle_winner_confirmed_at TIMESTAMP;
