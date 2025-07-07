ALTER TABLE events
    RENAME TO events_old;
ALTER TABLE members
    RENAME TO members_old;

CREATE TABLE members
(
    id           VARCHAR NOT NULL PRIMARY KEY,
    username     VARCHAR NOT NULL,
    display_name VARCHAR NOT NULL,
    avatar_url   VARCHAR NOT NULL
);

CREATE TABLE clubs
(
    id                              VARCHAR PRIMARY KEY,
    name                            VARCHAR NOT NULL,
    avatar_url                      VARCHAR NOT NULL,
    creator_id                      VARCHAR NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    created_by_community_ambassador BOOLEAN NOT NULL
);

CREATE TABLE events
(
    id                              VARCHAR PRIMARY KEY,
    name                            VARCHAR   NOT NULL,
    details                         TEXT      NOT NULL,
    address                         VARCHAR   NOT NULL,
    location                        VARCHAR   NOT NULL,
    creator_id                      VARCHAR   NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    cover_photo_url                 VARCHAR   NOT NULL,
    event_time                      TIMESTAMP NOT NULL,
    event_end_time                  TIMESTAMP NOT NULL,
    discord_interested              INTEGER   NOT NULL,
    created_by_community_ambassador BOOLEAN   NOT NULL,
    campfire_live_event_id          VARCHAR   NOT NULL,
    campfire_live_event_name        VARCHAR   NOT NULL,
    club_id                         VARCHAR   NOT NULL REFERENCES clubs (id) ON DELETE CASCADE,
    raw_json                        JSONB     NOT NULL
);

CREATE TABLE event_rsvps
(
    event_id  VARCHAR NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    member_id VARCHAR NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    status    VARCHAR NOT NULL,
    PRIMARY KEY (event_id, member_id)
);

