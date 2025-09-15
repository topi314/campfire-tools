package web

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func newClub(club database.ClubWithCreator) Club {
	return Club{
		ID:                           club.Club.ID,
		Name:                         club.Club.Name,
		AvatarURL:                    imageURL(club.Club.AvatarURL, 48),
		Creator:                      newMember(club.Member, club.Club.ID, 32),
		CreatedByCommunityAmbassador: club.Club.CreatedByCommunityAmbassador,
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
	ImportedAt                   time.Time
	URL                          string
}

func newClubWithEvents(club database.ClubWithEvents) ClubWithEvents {
	return ClubWithEvents{
		Club: newClub(database.ClubWithCreator{
			Club: club.Club,
		}),
		Events: club.Events,
	}
}

type ClubWithEvents struct {
	Club
	Events int
}

func newEvent(event database.Event, iconSize int) Event {
	return Event{
		ID:            event.ID,
		Name:          event.Name,
		URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
		CoverPhotoURL: imageURL(event.CoverPhotoURL, iconSize),
		Creator: Member{
			ID: event.CreatorID,
		},
		Details:                      event.Details,
		Time:                         event.Time,
		EndTime:                      event.EndTime,
		CampfireLiveEventID:          event.CampfireLiveEventID,
		CampfireLiveEventName:        event.CampfireLiveEventName,
		CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
		ImportedAt:                   event.ImportedAt,
	}
}

func newEventWithCheckIns(event database.EventWithCheckIns, iconSize int) Event {
	e := newEvent(event.Event, iconSize)
	e.Accepted = event.Accepted
	e.CheckIns = event.CheckIns
	return e
}

func newEventWithCreator(event database.EventWithCreator) Event {
	e := newEvent(event.Event, 48)
	e.Creator = newMember(event.Member, event.Event.ClubID, 32)
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
	CampfireLiveEventID          string
	CampfireLiveEventName        string
	Creator                      Member
	CreatedByCommunityAmbassador bool
	ImportedAt                   time.Time
	Accepted                     int
	CheckIns                     int
}

type TopMembers struct {
	Count   int
	Open    bool
	Members []TopMember
}

type TopEvents struct {
	Count            int
	Open             bool
	Events           []TopEvent
	TotalAccepted    int
	TotalCheckIns    int
	TotalCheckInRate float64
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

func newMember(member database.Member, clubID string, iconSize int) Member {
	if member.ID == "" {
		return Member{}
	}

	var campfireMember campfire.Member
	if err := json.Unmarshal(member.RawJSON, &campfireMember); err != nil {
		panic(fmt.Errorf("failed to unmarshal member: %w", err))
	}

	return newMemberFromCampfire(campfireMember, clubID, iconSize)
}

func newMemberFromCampfire(member campfire.Member, clubID string, iconSize int) Member {
	return Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: getDisplayName(member.DisplayName, member.Username),
		AvatarURL:   imageURL(member.AvatarURL, iconSize),
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

type TopMember struct {
	Member
	Accepted    int
	CheckIns    int
	CheckInRate float64
}

type TopEvent struct {
	Event
	Accepted    int
	CheckIns    int
	CheckInRate float64
}

func newRaffle(raffle database.Raffle) Raffle {
	return Raffle{
		ID:            raffle.ID,
		UserID:        raffle.UserID,
		Events:        raffle.Events,
		WinnerCount:   raffle.WinnerCount,
		OnlyCheckedIn: raffle.OnlyCheckedIn,
		SingleEntry:   raffle.SingleEntry,
		CreatedAt:     raffle.CreatedAt,
		URL:           fmt.Sprintf("/raffle/%d", raffle.ID),
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

func newWinner(winner database.RaffleWinnerWithMember, clubID string) Winner {
	var confirmURL string
	if clubID != "" {
		confirmURL = fmt.Sprintf("/tracker/club/%s/raffle/%d/confirm/%s", clubID, winner.RaffleID, winner.Member.ID)
	} else {
		confirmURL = fmt.Sprintf("/raffle/%d/confirm/%s", winner.RaffleID, winner.Member.ID)
	}

	return Winner{
		Member:     newMember(winner.Member, clubID, 32),
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

func newToken(token database.CampfireToken) Token {
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

func newClubImportJob(job database.ClubImportJobWithClub) ClubImportJob {
	return ClubImportJob{
		ID: job.ClubImportJob.ID,
		Club: newClub(database.ClubWithCreator{
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
