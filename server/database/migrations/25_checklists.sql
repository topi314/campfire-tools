CREATE TABLE checklists
(
    checklist_id              BIGSERIAL PRIMARY KEY,
    checklist_token           VARCHAR   NOT NULL UNIQUE,
    checklist_title           VARCHAR   NOT NULL,
    checklist_created_by      VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
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
