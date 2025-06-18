CREATE TABLE events
(
    id      VARCHAR PRIMARY KEY,
    name    VARCHAR NOT NULL,
    details TEXT NOT NULL
);

CREATE TABLE members
(
    id           VARCHAR PRIMARY KEY,
    display_name VARCHAR NOT NULL,
    status       VARCHAR NOT NULL,
    event_id     VARCHAR NOT NULL
);