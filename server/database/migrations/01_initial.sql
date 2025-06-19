CREATE TABLE events
(
    id                       VARCHAR PRIMARY KEY,
    name                     VARCHAR   NOT NULL,
    details                  TEXT      NOT NULL,
    cover_photo_url          VARCHAR   NOT NULL,
    event_time               TIMESTAMP NOT NULL,
    event_end_time           TIMESTAMP NOT NULL,

    campfire_live_event_id   VARCHAR   NOT NULL,
    campfire_live_event_name VARCHAR   NOT NULL,

    club_id                  VARCHAR   NOT NULL,
    club_name                VARCHAR   NOT NULL,
    club_avatar_url          VARCHAR   NOT NULL
);

CREATE TABLE members
(
    id           VARCHAR NOT NULL,
    display_name VARCHAR NOT NULL,
    status       VARCHAR NOT NULL,
    event_id     VARCHAR NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    PRIMARY KEY (id, event_id)
);