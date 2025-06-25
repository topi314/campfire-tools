package database

import (
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
}

type Club struct {
	ClubID        string `db:"club_id"`
	ClubName      string `db:"club_name"`
	ClubAvatarURL string `db:"club_avatar_url"`
}

type Member struct {
	ID          string `db:"id"`
	DisplayName string `db:"display_name"`
	Status      string `db:"status"`
	EventID     string `db:"event_id"`
}

type EventMember struct {
	Member
	EventName string `db:"event_name"`
}

type TopMember struct {
	Member
	EventCount int `db:"event_count"`
}
