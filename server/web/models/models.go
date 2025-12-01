package models

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func NewClub(club database.ClubWithCreator) Club {
	return Club{
		ID:                           club.Club.ID,
		Name:                         club.Club.Name,
		AvatarURL:                    ImageURL(club.Club.AvatarURL, 48),
		Creator:                      NewMember(club.Member, club.Club.ID, 32),
		CreatedByCommunityAmbassador: club.Club.CreatedByCommunityAmbassador,
		AutoEventImport:              club.Club.AutoEventImport,
		LastAutoEventImportedAt:      club.Club.LastAutoEventImportedAt,
		ImportedAt:                   club.Club.ImportedAt,
		URL:                          fmt.Sprintf("/tracker/club/%s", club.Club.ID),
	}
}

type Club struct {
	ID                           string
	Name                         string
	AvatarURL                    string
	Creator                      Member
	CreatedByCommunityAmbassador bool
	AutoEventImport              bool
	LastAutoEventImportedAt      time.Time
	ImportedAt                   time.Time
	URL                          string
}

func NewClubWithEvents(club database.ClubWithEvents) ClubWithEvents {
	return ClubWithEvents{
		Club: NewClub(database.ClubWithCreator{
			Club: club.Club,
		}),
		Events: club.Events,
	}
}

func NewPinnedClubWithEvents(club database.ClubWithEvents) ClubWithEvents {
	c := NewClubWithEvents(club)
	c.Pinned = true
	return c
}

type ClubWithEvents struct {
	Club
	Events int
	Pinned bool
}

func NewEvent(event database.Event, iconSize int) Event {
	return Event{
		ID:            event.ID,
		Name:          event.Name,
		URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
		CoverPhotoURL: ImageURL(event.CoverPhotoURL, iconSize),
		Creator: Member{
			ID: event.CreatorID,
		},
		Details:                      event.Details,
		Time:                         event.Time,
		EndTime:                      event.EndTime,
		Finished:                     event.Finished,
		CampfireLiveEventID:          event.CampfireLiveEventID,
		CampfireLiveEventName:        event.CampfireLiveEventName,
		CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
		ImportedAt:                   event.ImportedAt,
	}
}

func NewEventWithCheckIns(event database.EventWithCheckIns, iconSize int) Event {
	e := NewEvent(event.Event, iconSize)
	e.Accepted = event.Accepted
	e.CheckIns = event.CheckIns
	return e
}

func NewEventWithCreator(event database.EventWithCreator) Event {
	e := NewEvent(event.Event, 48)
	e.Creator = NewMember(event.Member, event.Event.ClubID, 32)
	return e
}

type Event struct {
	ID                           string
	Name                         string
	URL                          string
	CoverPhotoURL                string
	Details                      string
	Time                         time.Time
	EndTime                      time.Time
	Finished                     bool
	CampfireLiveEventID          string
	CampfireLiveEventName        string
	Creator                      Member
	CreatedByCommunityAmbassador bool
	ImportedAt                   time.Time
	Accepted                     int
	CheckIns                     int
}

type EventCategories struct {
	Open       bool
	Categories []EventCategory
}

type EventCategory struct {
	Name             string
	Events           int
	Accepted         int
	CheckIns         int
	CheckInRate      float64
	TotalCheckInRate float64
}

func GetDisplayName(displayName string, username string) string {
	if displayName == "" {
		displayName = username
	}
	if displayName == "" {
		displayName = "<unknown>"
	}
	return displayName
}

func NewMember(member database.Member, clubID string, iconSize int) Member {
	if member.ID == "" {
		return Member{}
	}

	var campfireMember campfire.Member
	if err := json.Unmarshal(member.RawJSON, &campfireMember); err != nil {
		panic(fmt.Errorf("failed to unmarshal member: %w", err))
	}

	return NewMemberFromCampfire(campfireMember, clubID, iconSize)
}

func NewMemberFromCampfire(member campfire.Member, clubID string, iconSize int) Member {
	return Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: GetDisplayName(member.DisplayName, member.Username),
		AvatarURL:   ImageURL(member.AvatarURL, iconSize),
		IsCommunityAmbassador: slices.ContainsFunc(member.Badges, func(badge campfire.Badge) bool {
			return badge.Alias == "PGO_COMMUNITY_AMBASSADOR"
		}),
		URL: fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
	}
}

type Member struct {
	ID                    string
	Username              string
	DisplayName           string
	AvatarURL             string
	IsCommunityAmbassador bool
	URL                   string
}

type Badge struct {
	Alias     string
	BadgeType string
}

func NewTopMember(member database.TopMember, clubID string, size int) TopMember {
	return TopMember{
		Member:      NewMember(member.Member, clubID, size),
		Accepted:    member.Accepted,
		CheckIns:    member.CheckIns,
		CheckInRate: CalcCheckInRate(member.Accepted, member.CheckIns),
	}
}

type TopMember struct {
	Member
	Accepted    int
	CheckIns    int
	CheckInRate float64
}

func NewTopEvent(event database.EventWithCheckIns, iconSize int) TopEvent {
	return TopEvent{
		Event:       NewEvent(event.Event, iconSize),
		Accepted:    event.Accepted,
		CheckIns:    event.CheckIns,
		CheckInRate: CalcCheckInRate(event.Accepted, event.CheckIns),
	}
}

type TopEvent struct {
	Event
	Accepted    int
	CheckIns    int
	CheckInRate float64
}

func NewRaffle(raffle database.Raffle) Raffle {
	return Raffle{
		ID:            raffle.ID,
		UserID:        raffle.UserID,
		Events:        raffle.Events,
		WinnerCount:   raffle.WinnerCount,
		OnlyCheckedIn: raffle.OnlyCheckedIn,
		SingleEntry:   raffle.SingleEntry,
		CreatedAt:     raffle.CreatedAt,
		URL:           fmt.Sprintf("/tracker/raffle/%d", raffle.ID),
	}
}

type Raffle struct {
	ID            int
	UserID        string
	Events        []string
	WinnerCount   int
	OnlyCheckedIn bool
	SingleEntry   bool
	CreatedAt     time.Time
	URL           string
}

func NewWinner(winner database.RaffleWinnerWithMember, clubID string) Winner {
	var confirmURL string
	if clubID != "" {
		confirmURL = fmt.Sprintf("/tracker/club/%s/raffle/%d/confirm/%s", clubID, winner.RaffleID, winner.Member.ID)
	} else {
		confirmURL = fmt.Sprintf("/tracker/raffle/%d/confirm/%s", winner.RaffleID, winner.Member.ID)
	}

	return Winner{
		Member:     NewMember(winner.Member, clubID, 32),
		Accepted:   winner.Accepted,
		CheckIns:   winner.CheckIns,
		Confirmed:  winner.Confirmed,
		Previous:   winner.Past,
		ConfirmURL: confirmURL,
	}
}

type Winner struct {
	Member
	Accepted   int
	CheckIns   int
	Confirmed  bool
	Previous   bool
	ConfirmURL string
}

func NewToken(token database.CampfireToken) Token {
	return Token{
		ID:        token.ID,
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt,
		Email:     token.Email,
	}
}

type Token struct {
	ID        int
	Token     string
	ExpiresAt time.Time
	Email     string
}

func NewClubImportJob(job database.ClubImportJobWithClub) ClubImportJob {
	return ClubImportJob{
		ID: job.ClubImportJob.ID,
		Club: NewClub(database.ClubWithCreator{
			Club: job.Club,
			Member: database.Member{
				ID:      job.Club.CreatorID,
				RawJSON: []byte("{}"),
			},
		}),
		CreatedAt:   job.CreatedAt,
		CompletedAt: job.CompletedAt,
		LastTriedAt: job.LastTriedAt,
		Status:      string(job.Status),
		State:       job.State.V,
		Error:       job.Error,
	}
}

type ClubImportJob struct {
	ID          int
	Club        Club
	CreatedAt   time.Time
	CompletedAt time.Time
	LastTriedAt time.Time
	Status      string
	State       database.ClubImportJobState
	Error       string
}

func ImageURL(imageURL string, size int) string {
	if imageURL == "" {
		return ""
	}

	imageURL = path.Join("/images", path.Base(imageURL))
	if size > 0 {
		imageURL = fmt.Sprintf("%s?size=%d", imageURL, size)
	}

	return imageURL
}

func NewRewardPool(pool database.RewardPool, usedCodes int, totalCodes int) RewardPool {
	return RewardPool{
		ID:          pool.ID,
		URL:         fmt.Sprintf("/tracker/reward-pool/%d", pool.ID),
		CodesURL:    fmt.Sprintf("/tracker/reward-pool/%d/codes", pool.ID),
		EditURL:     fmt.Sprintf("/tracker/reward-pool/%d/edit", pool.ID),
		DeleteURL:   fmt.Sprintf("/tracker/reward-pool/%d/delete", pool.ID),
		Name:        pool.Name,
		Description: pool.Description,
		UsedCodes:   usedCodes,
		TotalCodes:  totalCodes,
	}
}

type RewardPool struct {
	ID          int
	URL         string
	CodesURL    string
	EditURL     string
	DeleteURL   string
	Name        string
	Description string
	UsedCodes   int
	TotalCodes  int
}
