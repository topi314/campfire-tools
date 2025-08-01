package web

import (
	"fmt"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func newClub(club database.ClubWithCreator) Club {
	return Club{
		ID:                           club.Club.ID,
		Name:                         club.Club.Name,
		AvatarURL:                    imageURL(club.Club.AvatarURL, 48),
		Creator:                      newMember(club.Member, club.Club.ID),
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
		Club:   newClub(database.ClubWithCreator{Club: club.Club}),
		Events: club.Events,
	}
}

type ClubWithEvents struct {
	Club
	Events int
}

func newEvent(event database.EventWithCreator) Event {
	return Event{
		ID:                           event.Event.ID,
		Name:                         event.Event.Name,
		URL:                          fmt.Sprintf("/tracker/event/%s", event.Event.ID),
		CoverPhotoURL:                imageURL(event.Event.CoverPhotoURL, 128),
		Creator:                      newMember(event.Member, event.Event.ClubID),
		Details:                      event.Event.Details,
		Time:                         event.Event.Time,
		EndTime:                      event.Event.EndTime,
		CampfireLiveEventID:          event.Event.CampfireLiveEventID,
		CampfireLiveEventName:        event.Event.CampfireLiveEventName,
		CreatedByCommunityAmbassador: event.Event.CreatedByCommunityAmbassador,
		ImportedAt:                   event.Event.ImportedAt,
	}
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

func newMember(member database.Member, clubID string) Member {
	return Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: getDisplayName(member.DisplayName, member.Username),
		AvatarURL:   imageURL(member.AvatarURL, 32),
		URL:         fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
	}
}

func newMemberFromCampfire(member campfire.Member, clubID string) Member {
	return Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: getDisplayName(member.DisplayName, member.Username),
		AvatarURL:   imageURL(member.AvatarURL, 32),
		URL:         fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
	}
}

type Member struct {
	ID          string
	Username    string
	DisplayName string
	AvatarURL   string
	URL         string
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
		Member:     newMember(winner.Member, clubID),
		Confirmed:  winner.Confirmed,
		Previous:   winner.Past,
		ConfirmURL: confirmURL,
	}
}

type Winner struct {
	Member
	Confirmed  bool
	Previous   bool
	ConfirmURL string
}
