package database

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            string    `db:"id"`
	Name          string    `db:"name"`
	Details       string    `db:"details"`
	CoverPhotoURL string    `db:"cover_photo_url"`
	EventTime     time.Time `db:"event_time"`
	EventEndTime  time.Time `db:"event_end_time"`

	CampfireLiveEventID   string `db:"campfire_live_event_id"`
	CampfireLiveEventName string `db:"campfire_live_event_name"`

	ClubID        string `db:"club_id"`
	ClubName      string `db:"club_name"`
	ClubAvatarURL string `db:"club_avatar_url"`

	RawJSON json.RawMessage `db:"raw_json"`
}

type TopEvent struct {
	Event
	Accepted int `db:"accepted"`
	CheckIns int `db:"check_ins"`
}

type MemberEvent struct {
	Event
	Status string `db:"status"`
}

type Club struct {
	ClubID        string `db:"club_id"`
	ClubName      string `db:"club_name"`
	ClubAvatarURL string `db:"club_avatar_url"`
}

type ClubMember struct {
	ID          string `db:"id"`
	Username    string `db:"username"`
	DisplayName string `db:"display_name"`
	AvatarURL   string `db:"avatar_url"`
}

func (m ClubMember) GetDisplayName() string {
	displayName := m.DisplayName
	if displayName == "" {
		displayName = m.Username
	}
	if displayName == "" {
		displayName = "<unknown>"
	}
	return displayName
}

type Member struct {
	ClubMember
	Status  string `db:"status"`
	EventID string `db:"event_id"`
}

type EventMember struct {
	Member
	EventName string `db:"event_name"`
}

type TopMember struct {
	Member
	CheckIns int `db:"check_ins"`
}

type Session struct {
	ID        string    `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
}
