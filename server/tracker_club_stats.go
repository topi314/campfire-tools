package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/xstrconv"
)

type TrackerClubStatsVars struct {
	ClubName      string
	ClubAvatarURL string
	ClubID        string

	From time.Time
	To   time.Time

	TopCounts []int

	TopMembers      TopMembers
	TopEvents       TopEvents
	EventCategories EventCategories
}

type TopMembers struct {
	Count   int
	Open    bool
	Members []TopMember
}

type TopEvents struct {
	Count         int
	Open          bool
	Events        []TopEvent
	TotalCheckIns int
	TotalAccepted int
}

type EventCategories struct {
	Open       bool
	Categories []EventCategory
}

type EventCategory struct {
	Name     string
	CheckIns int
	Accepted int
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
	CheckIns int
}

type TopEvent struct {
	Event
	Accepted int
	CheckIns int
}

func (s *Server) TrackerClubStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")
	query := r.URL.Query()
	fromDate := query.Get("from")
	toDate := query.Get("to")
	membersStr := query.Get("members")
	eventsStr := query.Get("events")
	topMembersClosedStr := query.Get("members-closed")
	topEventsClosedStr := query.Get("events-closed")
	categoriesClosedStr := query.Get("event-categories-closed")

	var from time.Time
	if fromDate != "" {
		fromParsed, err := time.Parse("2006-01-02", fromDate)
		if err == nil {
			from = fromParsed
		}
	}

	var to time.Time
	if toDate != "" {
		toParsed, err := time.Parse("2006-01-02", toDate)
		if err == nil {
			to = toParsed.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
		}
	}

	membersCount := 10
	if membersStr != "" {
		parsedMembersCount, err := strconv.Atoi(membersStr)
		if err == nil {
			membersCount = parsedMembersCount
		}
	}

	eventsCount := 10
	if eventsStr != "" {
		parsedEventsCount, err := strconv.Atoi(eventsStr)
		if err == nil {
			eventsCount = parsedEventsCount
		}
	}

	var topMembersClosed bool
	if topMembersClosedStr != "" {
		parsedTopMembersClosed, err := xstrconv.ParseBool(topMembersClosedStr)
		if err == nil {
			topMembersClosed = parsedTopMembersClosed
		}
	}

	var topEventsClosed bool
	if topEventsClosedStr != "" {
		parsedTopEventsClosed, err := xstrconv.ParseBool(topEventsClosedStr)
		if err == nil {
			topEventsClosed = parsedTopEventsClosed
		}
	}

	var categoriesClosed bool
	if categoriesClosedStr != "" {
		parsedCategoriesClosed, err := xstrconv.ParseBool(categoriesClosedStr)
		if err == nil {
			categoriesClosed = parsedCategoriesClosed
		}
	}

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topMembers, err := s.db.GetTopClubMembers(ctx, clubID, from, to, membersCount)
	if err != nil {
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopMembers := make([]TopMember, len(topMembers))
	for i, member := range topMembers {
		trackerTopMembers[i] = TopMember{
			Member: Member{
				ID:          member.ID,
				Username:    member.Username,
				DisplayName: member.GetDisplayName(),
				AvatarURL:   imageURL(member.AvatarURL),
				URL:         fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
			},
			CheckIns: member.CheckIns,
		}
	}

	totalCheckIns, totalAccepted, err := s.db.GetGlubTotalCheckInsAccepted(ctx, clubID, from, to)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topEvents, err := s.db.GetTopClubEvents(ctx, clubID, from, to, eventsCount)
	if err != nil {
		http.Error(w, "Failed to fetch top events: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopEvents := make([]TopEvent, len(topEvents))
	for i, event := range topEvents {
		trackerTopEvents[i] = TopEvent{
			Event: Event{
				ID:            event.ID,
				Name:          event.Name,
				URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
				CoverPhotoURL: imageURL(event.CoverPhotoURL),
			},
			Accepted: event.Accepted,
			CheckIns: event.CheckIns,
		}
	}

	events, err := s.db.GetGlubCheckInsAccepted(ctx, clubID, from, to)
	if err != nil {
		http.Error(w, "Failed to fetch check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	eventCategories := make(map[string]EventCategory)
	for _, event := range events {
		category := getEventCategories(event.CampfireLiveEventName)

		eventCategory, ok := eventCategories[category]
		if !ok {
			eventCategory = EventCategory{
				Name:     category,
				CheckIns: 0,
				Accepted: 0,
			}
		}

		eventCategory.CheckIns += event.CheckIns
		eventCategory.Accepted += event.Accepted
		eventCategories[category] = eventCategory
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		ClubName:      club.ClubName,
		ClubAvatarURL: imageURL(club.ClubAvatarURL),
		ClubID:        club.ClubID,

		From: from,
		To:   to,

		TopCounts: []int{10, 25, 50, 75, 100},
		TopMembers: TopMembers{
			Count:   membersCount,
			Open:    !topMembersClosed,
			Members: trackerTopMembers,
		},
		TopEvents: TopEvents{
			Count:         eventsCount,
			Open:          !topEventsClosed,
			Events:        trackerTopEvents,
			TotalCheckIns: totalCheckIns,
			TotalAccepted: totalAccepted,
		},
		EventCategories: EventCategories{
			Open:       !categoriesClosed,
			Categories: slices.Collect(maps.Values(eventCategories)),
		},
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club stats template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

const EventCategoryOther = "Other"

var AllEventCategories = []string{
	"Raid Day",
	"Raid Hour",
	"Max Monday",
	"Research Day",
	"Community Day",
	"Spotlight Hour",
	"Elite Raids",
	"Max Battle Weekend",
	"Gigantamax",
	"Pokémon GO Tour",
	"GO Fest",
}

func getEventCategories(eventName string) string {
	for _, category := range AllEventCategories {
		if strings.Contains(eventName, category) {
			return category
		}
	}
	return EventCategoryOther
}
