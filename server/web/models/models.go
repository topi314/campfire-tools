package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
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
		CreatedByCommunityAmbassador: campfire.ClubCreatedByCommunityAmbassadorFromRaw(club.Club.CreatedByCommunityAmbassador, club.Club.RawJSON),
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

func NewEvent(event database.Event, iconSize int, clubAvatarURL string) Event {
	return Event{
		ID:            event.ID,
		Name:          event.Name,
		URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
		CoverPhotoURL: ImageURL(event.CoverPhotoURL, iconSize),
		ClubAvatarURL: clubAvatarURL,
		Creator: Member{
			ID: event.CreatorID,
		},
		Details:                      event.Details,
		Address:                      event.Address,
		Location:                     event.Location,
		MapURL:                       eventMapURL(event.Location, event.Address),
		Time:                         event.Time,
		EndTime:                      event.EndTime,
		Finished:                     event.Finished,
		DiscordInterested:            event.DiscordInterested,
		CampfireLiveEventID:          event.CampfireLiveEventID,
		CampfireLiveEventName:        event.CampfireLiveEventName,
		Category:                     event.Category,
		CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
		ImportedAt:                   event.ImportedAt,
	}
}

func NewEventWithCheckIns(event database.EventWithCheckIns, iconSize int, clubAvatarURL string) Event {
	e := NewEvent(event.Event, iconSize, clubAvatarURL)
	e.Accepted = event.Accepted
	e.CheckIns = event.CheckIns
	return e
}

func NewEventWithCreator(event database.EventWithCreator, clubAvatarURL string) Event {
	e := NewEvent(event.Event, 48, clubAvatarURL)
	e.Creator = NewMember(event.Member, event.Event.ClubID, 32)
	return e
}

type Event struct {
	ID                           string
	Name                         string
	URL                          string
	CoverPhotoURL                string
	ClubAvatarURL                string
	Details                      string
	Address                      string
	Location                     string
	MapURL                       string
	Time                         time.Time
	EndTime                      time.Time
	Finished                     bool
	DiscordInterested            int
	CampfireLiveEventID          string
	CampfireLiveEventName        string
	Category                     string
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

	if len(member.RawJSON) > 0 && string(member.RawJSON) != "{}" {
		var campfireMember campfire.Member
		if err := json.Unmarshal(member.RawJSON, &campfireMember); err != nil {
			panic(fmt.Errorf("failed to unmarshal member: %w", err))
		}
		if campfireMember.ID != "" {
			return NewMemberFromCampfire(campfireMember, clubID, iconSize)
		}
	}

	// Stub members (RSVP-only imports) have no RawJSON profile yet.
	displayName := GetDisplayName(member.DisplayName, member.Username)
	if displayName == "<unknown>" {
		displayName = member.ID
	}
	return Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: displayName,
		AvatarURL:   ImageURL(member.AvatarURL, iconSize),
		URL:         clubMemberURL(clubID, member.ID),
		ProfileURL:  memberProfileURL(member.ID),
	}
}

func memberProfileURL(memberID string) string {
	return fmt.Sprintf("/tracker/members/%s", memberID)
}

func clubMemberURL(clubID, memberID string) string {
	if clubID != "" {
		return fmt.Sprintf("/tracker/club/%s/member/%s", clubID, memberID)
	}
	return memberProfileURL(memberID)
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
		URL:        clubMemberURL(clubID, member.ID),
		ProfileURL: memberProfileURL(member.ID),
	}
}

func NewImportedMember(member database.Member, iconSize int) Member {
	if member.ID == "" {
		return Member{}
	}

	displayName := GetDisplayName(member.DisplayName, member.Username)
	if displayName == "<unknown>" {
		displayName = member.ID
	}

	m := Member{
		ID:          member.ID,
		Username:    member.Username,
		DisplayName: displayName,
		AvatarURL:   ImageURL(member.AvatarURL, iconSize),
		URL:         memberProfileURL(member.ID),
		ProfileURL:  memberProfileURL(member.ID),
	}

	if len(member.RawJSON) > 0 && string(member.RawJSON) != "{}" {
		var campfireMember campfire.Member
		if err := json.Unmarshal(member.RawJSON, &campfireMember); err == nil {
			m.IsCommunityAmbassador = slices.ContainsFunc(campfireMember.Badges, func(badge campfire.Badge) bool {
				return badge.Alias == "PGO_COMMUNITY_AMBASSADOR"
			})
		}
	}

	return m
}

type ImportedMember struct {
	Member
	ImportedAt time.Time
}

type Member struct {
	ID                    string
	Username              string
	DisplayName           string
	AvatarURL             string
	IsCommunityAmbassador bool
	URL                   string
	ProfileURL            string
}

type Badge struct {
	Alias     string
	BadgeType string
}

type ClubMemberEvents struct {
	Club   Club
	Events []Event
}

func GroupEventsByClub(rows []database.EventWithClub, iconSize int) []ClubMemberEvents {
	groups := make([]ClubMemberEvents, 0)
	index := make(map[string]int)

	for _, row := range rows {
		clubAvatarURL := ImageURL(row.Club.AvatarURL, iconSize)

		if i, ok := index[row.Club.ID]; ok {
			groups[i].Events = append(groups[i].Events, NewEvent(row.Event, iconSize, clubAvatarURL))
			continue
		}

		index[row.Club.ID] = len(groups)
		groups = append(groups, ClubMemberEvents{
			Club:   NewClub(database.ClubWithCreator{Club: row.Club}),
			Events: []Event{NewEvent(row.Event, iconSize, clubAvatarURL)},
		})
	}

	return groups
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

func NewTopEvent(event database.EventWithCheckIns, iconSize int, clubAvatarURL string) TopEvent {
	return TopEvent{
		Event:       NewEvent(event.Event, iconSize, clubAvatarURL),
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

func NewWinner(winner database.RaffleWinnerWithMember, clubID string) Winner {
	var confirmURL string
	if clubID != "" {
		confirmURL = fmt.Sprintf("/tracker/club/%s/raffle/%d/confirm/%s", clubID, winner.RaffleID, winner.Member.ID)
	} else {
		confirmURL = fmt.Sprintf("/raffle/%d/confirm/%s", winner.RaffleID, winner.Member.ID)
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

func eventMapURL(location string, address string) string {
	if lat, lng, ok := parseLocationCoords(location); ok {
		return fmt.Sprintf("https://www.google.com/maps?q=%s,%s",
			strconv.FormatFloat(lat, 'f', -1, 64),
			strconv.FormatFloat(lng, 'f', -1, 64),
		)
	}
	if address != "" {
		return "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(address)
	}
	return ""
}

func parseLocationCoords(location string) (lat float64, lng float64, ok bool) {
	location = strings.TrimSpace(location)
	if location == "" {
		return 0, 0, false
	}

	// Campfire stores location as [longitude, latitude].
	var coords []float64
	if err := json.Unmarshal([]byte(location), &coords); err == nil && len(coords) >= 2 {
		return coords[1], coords[0], true
	}

	trimmed := strings.Trim(location, "[]() ")
	parts := strings.Split(trimmed, ",")
	if len(parts) < 2 {
		return 0, 0, false
	}

	lng, errLng := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, errLat := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errLat != nil || errLng != nil {
		return 0, 0, false
	}
	return lat, lng, true
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

func NewReward(reward database.Reward) Reward {
	return Reward{
		ID:            reward.ID,
		URL:           fmt.Sprintf("/tracker/rewards/%d", reward.ID),
		CodesURL:      fmt.Sprintf("/tracker/rewards/%d/codes", reward.ID),
		EditURL:       fmt.Sprintf("/tracker/rewards/%d/edit", reward.ID),
		Name:          reward.Name,
		Description:   reward.Description,
		RedeemedCodes: reward.RedeemedCodes,
		TotalCodes:    reward.TotalCodes,
	}
}

type Reward struct {
	ID            int
	URL           string
	CodesURL      string
	EditURL       string
	Name          string
	Description   string
	RedeemedCodes int
	TotalCodes    int
}
