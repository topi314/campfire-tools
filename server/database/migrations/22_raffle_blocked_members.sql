CREATE TABLE discord_user_raffle_blocked_members
(
    discord_user_raffle_blocked_member_user_id    VARCHAR REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    discord_user_raffle_blocked_member_member_id  VARCHAR REFERENCES members (member_id) ON DELETE CASCADE,
    discord_user_raffle_blocked_member_blocked_at TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (discord_user_raffle_blocked_member_user_id, discord_user_raffle_blocked_member_member_id)
);
