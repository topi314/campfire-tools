package database

import (
	"encoding/json"
	"time"
)

type Club struct {
	ID                           string          `db:"id"`
	Name                         string          `db:"name"`
	AvatarURL                    string          `db:"avatar_url"`
	CreatorID                    string          `db:"creator_id"`
	CreatedByCommunityAmbassador bool            `db:"created_by_community_ambassador"`
	ImportedAt                   time.Time       `db:"imported_at"`
	RawJSON                      json.RawMessage `db:"raw_json"`
}

type Event struct {
	ID                           string          `db:"id"`
	Name                         string          `db:"name"`
	Details                      string          `db:"details"`
	Address                      string          `db:"address"`
	Location                     string          `db:"location"`
	CreatorID                    string          `db:"creator_id"`
	CoverPhotoURL                string          `db:"cover_photo_url"`
	EventTime                    time.Time       `db:"event_time"`
	EventEndTime                 time.Time       `db:"event_end_time"`
	DiscordInterested            int             `db:"discord_interested"`
	CreatedByCommunityAmbassador bool            `db:"created_by_community_ambassador"`
	CampfireLiveEventID          string          `db:"campfire_live_event_id"`
	CampfireLiveEventName        string          `db:"campfire_live_event_name"`
	ClubID                       string          `db:"club_id"`
	ImportedAt                   time.Time       `db:"imported_at"`
	RawJSON                      json.RawMessage `db:"raw_json"`
}

type Member struct {
	ID          string          `db:"id"`
	Username    string          `db:"username"`
	DisplayName string          `db:"display_name"`
	AvatarURL   string          `db:"avatar_url"`
	ImportedAt  time.Time       `db:"imported_at"`
	RawJSON     json.RawMessage `db:"raw_json"`
}

type TopEvent struct {
	Event
	Accepted int `db:"accepted"`
	CheckIns int `db:"check_ins"`
}

type TopMember struct {
	Member
	Accepted int `db:"accepted"`
	CheckIns int `db:"check_ins"`
}

type EventMember struct {
	Event
	Member
	EventRSVP
}

type EventNumbers struct {
	CampfireLiveEventID   string `db:"campfire_live_event_id"`
	CampfireLiveEventName string `db:"campfire_live_event_name"`
	Events                int    `db:"events"`
	CheckIns              int    `db:"check_ins"`
	Accepted              int    `db:"accepted"`
}

type EventRSVP struct {
	EventID    string    `db:"event_id"`
	MemberID   string    `db:"member_id"`
	Status     string    `db:"status"`
	ImportedAt time.Time `db:"imported_at"`
}

type Session struct {
	ID        string    `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
}
