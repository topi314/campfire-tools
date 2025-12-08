ALTER TABLE reward_codes
    ADD COLUMN reward_code_reserved_at TIMESTAMP,
    ADD COLUMN reward_code_reserved_by VARCHAR REFERENCES discord_users(discord_user_id) ON DELETE SET NULL;
