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
	Club

	From time.Time
	To   time.Time

	TopCounts []int

	TopMembers      TopMembers
	TopEvents       TopEvents
	EventCategories EventCategories
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

	topMembers, err := s.db.GetTopMembersByClub(ctx, clubID, from, to, membersCount)
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

	totalCheckIns, totalAccepted, err := s.db.GetClubTotalCheckInsAccepted(ctx, clubID, from, to)
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

	events, err := s.db.GetEventCheckInAccepted(ctx, clubID, from, to)
	if err != nil {
		http.Error(w, "Failed to fetch check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	eventCategories := make(map[string]EventCategory)
	for _, event := range events {
		category := s.getEventCategories(event.CampfireLiveEventName)

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

	categories := slices.Collect(maps.Values(eventCategories))
	slices.SortFunc(categories, func(a, b EventCategory) int {
		if a.CheckIns == b.CheckIns {
			return a.Accepted - b.Accepted
		}
		return b.CheckIns - a.CheckIns
	})

	if err = s.templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		Club: Club{
			ClubID:        club.ID,
			ClubName:      club.Name,
			ClubAvatarURL: imageURL(club.AvatarURL),
		},

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
			Categories: categories,
		},
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club stats template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

const (
	EventCategoryOther   = "Other"
	EventCategoryNoEvent = "No Event"
)

var AllEventCategories = map[string][]string{
	"Raid Day":       {"Raid Day"},
	"Raid Hour":      {"Raid Hour"},
	"Max Monday":     {"Max Monday"},
	"Research Day":   {"Research Day"},
	"Hatch Day":      {"Hatch Day"},
	"Community Day":  {"Community Day"},
	"Spotlight Hour": {"Spotlight Hour"},
	"Max Battle":     {"Max Battle Weekend", "Max Weekend", "Gigantamax", "GMAX"},
	"GO Tour":        {"GO Tour"},
	"GO Fest":        {"GO Fest"},
}

func (s *Server) getEventCategories(eventName string) string {
	eventName = strings.ToLower(eventName)
	if eventName == "" {
		return EventCategoryNoEvent
	}
	for name, names := range AllEventCategories {
		for _, n := range names {
			if strings.Contains(eventName, strings.ToLower(n)) {
				return name
			}
		}
	}
	if s.cfg.WarnUnknownEventCategories {
		slog.Warn("Unknown event category", slog.String("event_name", eventName))
	}
	return EventCategoryOther
}
