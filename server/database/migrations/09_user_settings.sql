CREATE TABLE user_settings
(
    user_setting_user_id        VARCHAR NOT NULL PRIMARY KEY,
    user_setting_pinned_club_id VARCHAR REFERENCES clubs (club_id) ON DELETE SET NULL
);