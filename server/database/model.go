package database

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"

	"github.com/topi314/campfire-tools/internal/xpgtype"
)

type Club struct {
	ID                           string          `db:"club_id"`
	Name                         string          `db:"club_name"`
	AvatarURL                    string          `db:"club_avatar_url"`
	CreatorID                    string          `db:"club_creator_id"`
	CreatedByCommunityAmbassador bool            `db:"club_created_by_community_ambassador"`
	ImportedAt                   time.Time       `db:"club_imported_at"`
	RawJSON                      json.RawMessage `db:"club_raw_json"`
	AutoEventImport              bool            `db:"club_auto_event_import"`
	LastAutoEventImportedAt      time.Time       `db:"club_last_auto_event_imported_at"`
}

type Event struct {
	ID                           string          `db:"event_id"`
	Name                         string          `db:"event_name"`
	Details                      string          `db:"event_details"`
	Address                      string          `db:"event_address"`
	Location                     string          `db:"event_location"`
	CreatorID                    string          `db:"event_creator_id"`
	CoverPhotoURL                string          `db:"event_cover_photo_url"`
	Time                         time.Time       `db:"event_time"`
	EndTime                      time.Time       `db:"event_end_time"`
	Finished                     bool            `db:"event_finished"`
	DiscordInterested            int             `db:"event_discord_interested"`
	CreatedByCommunityAmbassador bool            `db:"event_created_by_community_ambassador"`
	CampfireLiveEventID          string          `db:"event_campfire_live_event_id"`
	CampfireLiveEventName        string          `db:"event_campfire_live_event_name"`
	ClubID                       string          `db:"event_club_id"`
	ImportedAt                   time.Time       `db:"event_imported_at"`
	RawJSON                      json.RawMessage `db:"event_raw_json"`
	LastAutoImportedAt           time.Time       `db:"event_last_auto_imported_at"`
}

type Member struct {
	ID          string          `db:"member_id"`
	Username    string          `db:"member_username"`
	DisplayName string          `db:"member_display_name"`
	AvatarURL   string          `db:"member_avatar_url"`
	ImportedAt  time.Time       `db:"member_imported_at"`
	RawJSON     json.RawMessage `db:"member_raw_json"`
}

type ClubWithEvents struct {
	Club
	Events int `db:"events"`
}

type ClubWithCreator struct {
	Club
	Member
}

type EventWithCreator struct {
	Event
	Member
}

type EventWithCheckIns struct {
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
	Member
	EventRSVP
}

type EventNumbers struct {
	CampfireLiveEventID   string `db:"event_campfire_live_event_id"`
	CampfireLiveEventName string `db:"event_campfire_live_event_name"`
	Events                int    `db:"events"`
	CheckIns              int    `db:"check_ins"`
	Accepted              int    `db:"accepted"`
}

type EventRSVP struct {
	EventID    string    `db:"event_rsvp_event_id"`
	MemberID   string    `db:"event_rsvp_member_id"`
	Status     string    `db:"event_rsvp_status"`
	ImportedAt time.Time `db:"event_rsvp_imported_at"`
}

type Raffle struct {
	ID            int            `db:"raffle_id"`
	UserID        string         `db:"raffle_user_id"`
	Events        pq.StringArray `db:"raffle_events"`
	WinnerCount   int            `db:"raffle_winner_count"`
	OnlyCheckedIn bool           `db:"raffle_only_checked_in"`
	SingleEntry   bool           `db:"raffle_single_entry"`
	CreatedAt     time.Time      `db:"raffle_created_at"`
}

type RaffleWinner struct {
	RaffleID  int       `db:"raffle_winner_raffle_id"`
	MemberID  string    `db:"raffle_winner_member_id"`
	Confirmed bool      `db:"raffle_winner_confirmed"`
	Past      bool      `db:"raffle_winner_past"`
	CreatedAt time.Time `db:"raffle_winner_created_at"`
}

type RaffleWinnerWithMember struct {
	RaffleWinner
	Member
	Accepted int `db:"accepted"`
	CheckIns int `db:"check_ins"`
}

type Session struct {
	ID        string    `db:"session_id"`
	CreatedAt time.Time `db:"session_created_at"`
	ExpiresAt time.Time `db:"session_expires_at"`
	UserID    string    `db:"session_user_id"`
	Admin     bool      `db:"session_admin"`
}

type UserSetting struct {
	UserID       string  `db:"user_setting_user_id"`
	PinnedClubID *string `db:"user_setting_pinned_club_id"`
}

type SessionWithUserSetting struct {
	Session
	UserSettingUserID *string `db:"user_setting_user_id"`
	PinnedClubID      *string `db:"user_setting_pinned_club_id"`
}

type ClubImportJobStatus string

const (
	ClubImportJobStatusPending   ClubImportJobStatus = "pending"
	ClubImportJobStatusCompleted ClubImportJobStatus = "completed"
	ClubImportJobStatusFailed    ClubImportJobStatus = "failed"
)

type ClubImportJob struct {
	ID          int                              `db:"club_import_job_id"`
	ClubID      string                           `db:"club_import_job_club_id"`
	CreatedAt   time.Time                        `db:"club_import_job_created_at"`
	CompletedAt time.Time                        `db:"club_import_job_completed_at"`
	LastTriedAt time.Time                        `db:"club_import_job_last_tried_at"`
	Status      ClubImportJobStatus              `db:"club_import_job_status"`
	State       xpgtype.JSON[ClubImportJobState] `db:"club_import_job_state"`
	Error       string                           `db:"club_import_job_error"`
}

type ClubImportJobWithClub struct {
	ClubImportJob
	Club
}

type ClubImportJobState struct {
	Events       []EventState `json:"events"`
	EventCursor  *string      `json:"event_cursor"`
	Members      []Member     `json:"members"`
	MemberCursor *string      `json:"member_cursor"`
}

type EventState struct {
	Event   Event       `json:"event"`
	Creator Member      `json:"creator"`
	RSVPs   []EventRSVP `json:"rsvps"`
}
