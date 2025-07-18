package web

import (
	"fmt"
	"time"

	"github.com/topi314/campfire-tools/server/database"
)

func newClub(club database.ClubWithCreator) Club {
	return Club{
		ClubID:                       club.Club.ID,
		ClubName:                     club.Club.Name,
		ClubAvatarURL:                imageURL(club.Club.AvatarURL, 48),
		Creator:                      newMember(club.Member, club.Club.ID),
		CreatedByCommunityAmbassador: club.Club.CreatedByCommunityAmbassador,
		ImportedAt:                   club.Club.ImportedAt,
	}
}

type Club struct {
	ClubID                       string
	ClubName                     string
	ClubAvatarURL                string
	Creator                      Member
	CreatedByCommunityAmbassador bool
	ImportedAt                   time.Time
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
