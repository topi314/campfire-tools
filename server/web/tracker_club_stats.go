package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/xstrconv"
)

const (
	OriginLeagueGoal    = 1
	GreatLeagueGoal     = 61
	UltraLeagueGoal     = 250
	MasterLeagueGoal    = 750
	LegendaryLeagueGoal = 1500
)

var quarters = []Quarter{
	{
		Name:  "Q3 2025",
		Value: "q3-2025",
	},
	{
		Name:  "Q2 2025",
		Value: "q2-2025",
	},
	{
		Name:  "Q1 2025",
		Value: "q1-2025",
	},
	{
		Name:  "Q4 2024",
		Value: "q4-2024",
	},
	{
		Name:  "Q3 2024",
		Value: "q3-2024",
	},
	{
		Name:  "Q2 2024",
		Value: "q2-2024",
	},
	{
		Name:  "Q1 2024",
		Value: "q1-2024",
	},
	{
		Name:  "Q4 2023",
		Value: "q4-2023",
	},
	{
		Name:  "Q3 2023",
		Value: "q3-2023",
	},
	{
		Name:  "Q2 2023",
		Value: "q2-2023",
	},
	{
		Name:  "Q1 2023",
		Value: "q1-2023",
	},
}

type TrackerClubStatsVars struct {
	Club

	EventsFilter

	TopCounts []int

	TopMembers      TopMembers
	TopEvents       TopEvents
	EventCategories EventCategories
	LeagueGoals     LeagueGoals
}

type EventsFilter struct {
	FilterURL    string
	From         time.Time
	To           time.Time
	OnlyCAEvents bool

	Quarters []Quarter
}

type LeagueGoals struct {
	Open               bool
	Goals              []LeagueGoal
	Quarter            string
	TotalCheckIns      int
	ProjectedCheckIns  int
	Days               int
	DaysElapsed        int
	DaysRemaining      int
	DaysElapsedPercent float64
	BiggestEvent       *TopEvent
}

type Quarter struct {
	Name  string
	Value string
}

type LeagueGoal struct {
	Name       string
	Goal       int
	Progress   float64
	Projection bool
}

func (h *handler) TrackerClubStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")
	query := r.URL.Query()

	from := parseTimeQuery(query, "from", time.Time{})
	to := parseTimeQuery(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
	}

	membersCount := parseIntQuery(query, "members", 10)
	eventsCount := parseIntQuery(query, "events", 10)
	onlyCAEvents := parseBoolQuery(query, "only-ca-events", false)
	topMembersClosed := parseBoolQuery(query, "members-closed", false)
	topEventsClosed := parseBoolQuery(query, "events-closed", false)
	categoriesClosed := parseBoolQuery(query, "event-categories-closed", false)
	leagueGoalsClosed := parseBoolQuery(query, "league-goals-closed", false)
	leagueGoalQuarter := query.Get("league-goal-quarter")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topMembers, err := h.DB.GetTopMembersByClub(ctx, clubID, from, to, onlyCAEvents, membersCount)
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
				DisplayName: getDisplayName(member.DisplayName, member.Username),
				AvatarURL:   imageURL(member.AvatarURL, 60),
				URL:         fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
			},
			Accepted:    member.Accepted,
			CheckIns:    member.CheckIns,
			CheckInRate: calcCheckInRate(member.Accepted, member.CheckIns),
		}
	}

	topEvents, err := h.DB.GetTopEventsByClub(ctx, clubID, from, to, onlyCAEvents, eventsCount)
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
				CoverPhotoURL: imageURL(event.CoverPhotoURL, 60),
			},
			Accepted:    event.Accepted,
			CheckIns:    event.CheckIns,
			CheckInRate: calcCheckInRate(event.Accepted, event.CheckIns),
		}
	}

	totalAccepted, totalCheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, onlyCAEvents)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	totalCheckInRate := calcCheckInRate(totalAccepted, totalCheckIns)

	events, err := h.DB.GetEventCheckInAcceptedCounts(ctx, clubID, from, to, onlyCAEvents)
	if err != nil {
		http.Error(w, "Failed to fetch event check-in and accepted counts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	eventCategories := make(map[string]EventCategory)
	for _, event := range events {
		category := h.getEventCategories(event.CampfireLiveEventName)

		eventCategory, ok := eventCategories[category]
		if !ok {
			eventCategory = EventCategory{
				Name:     category,
				Events:   0,
				CheckIns: 0,
				Accepted: 0,
			}
		}

		eventCategory.Events++
		eventCategory.Accepted += event.Accepted
		eventCategory.CheckIns += event.CheckIns
		eventCategory.CheckInRate = calcCheckInRate(eventCategory.Accepted, eventCategory.CheckIns)
		eventCategory.TotalCheckInRate = calcCheckInRate(totalCheckIns, eventCategory.CheckIns)
		eventCategories[category] = eventCategory
	}

	categories := slices.Collect(maps.Values(eventCategories))
	slices.SortFunc(categories, func(a, b EventCategory) int {
		if a.CheckIns == b.CheckIns {
			return a.Accepted - b.Accepted
		}
		return b.CheckIns - a.CheckIns
	})

	quarterFrom, quarterTo := getRangeFromQuarter(leagueGoalQuarter)

	_, totalCACheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, quarterFrom, quarterTo, true)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	biggestEvent, err := h.DB.GetBiggestCheckInEvent(ctx, clubID, quarterFrom, quarterTo, true)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Failed to fetch biggest check-in event: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var trackerBiggestEvent *TopEvent
	if biggestEvent != nil {
		trackerBiggestEvent = &TopEvent{
			Event: Event{
				ID:            biggestEvent.ID,
				Name:          biggestEvent.Name,
				URL:           fmt.Sprintf("/tracker/event/%s", biggestEvent.ID),
				CoverPhotoURL: imageURL(biggestEvent.CoverPhotoURL, 60),
			},
			Accepted:    biggestEvent.Accepted,
			CheckIns:    biggestEvent.CheckIns,
			CheckInRate: calcCheckInRate(biggestEvent.Accepted, biggestEvent.CheckIns),
		}
	}

	totalCAProjectedCheckIns, quarterDays, quarterDaysRemaining := calcCAProjectedCheckIns(quarterFrom, quarterTo, totalCACheckIns)

	leagueGoals := []LeagueGoal{
		{
			Name:       "Origin League",
			Goal:       OriginLeagueGoal,
			Progress:   calcCheckInProgress(OriginLeagueGoal, totalCACheckIns),
			Projection: OriginLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Great League",
			Goal:       GreatLeagueGoal,
			Progress:   calcCheckInProgress(GreatLeagueGoal, totalCACheckIns),
			Projection: GreatLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Ultra League",
			Goal:       UltraLeagueGoal,
			Progress:   calcCheckInProgress(UltraLeagueGoal, totalCACheckIns),
			Projection: UltraLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Master League",
			Goal:       MasterLeagueGoal,
			Progress:   calcCheckInProgress(MasterLeagueGoal, totalCACheckIns),
			Projection: MasterLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Legendary League",
			Goal:       LegendaryLeagueGoal,
			Progress:   calcCheckInProgress(LegendaryLeagueGoal, totalCACheckIns),
			Projection: LegendaryLeagueGoal <= totalCAProjectedCheckIns,
		},
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		Club: newClub(*club),
		EventsFilter: EventsFilter{
			FilterURL:    r.URL.Path,
			From:         from,
			To:           to,
			OnlyCAEvents: onlyCAEvents,
			Quarters:     quarters,
		},
		TopCounts: []int{10, 25, 50, 75, 100},
		TopMembers: TopMembers{
			Count:   membersCount,
			Open:    !topMembersClosed,
			Members: trackerTopMembers,
		},
		TopEvents: TopEvents{
			Count:            eventsCount,
			Open:             !topEventsClosed,
			Events:           trackerTopEvents,
			TotalCheckIns:    totalCheckIns,
			TotalAccepted:    totalAccepted,
			TotalCheckInRate: totalCheckInRate,
		},
		EventCategories: EventCategories{
			Open:       !categoriesClosed,
			Categories: categories,
		},
		LeagueGoals: LeagueGoals{
			Open:               !leagueGoalsClosed,
			Goals:              leagueGoals,
			Quarter:            leagueGoalQuarter,
			TotalCheckIns:      totalCACheckIns,
			ProjectedCheckIns:  totalCAProjectedCheckIns,
			Days:               quarterDays,
			DaysElapsed:        quarterDays - quarterDaysRemaining,
			DaysRemaining:      quarterDaysRemaining,
			DaysElapsedPercent: calcQuarterProgress(quarterDays, quarterDaysRemaining),
			BiggestEvent:       trackerBiggestEvent,
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
	"Raid Day":       {"Raid Day", "Mega Raid"},
	"Raid Hour":      {"Raid Hour"},
	"Max Monday":     {"Max Monday"},
	"Research Day":   {"Research Day"},
	"Hatch Day":      {"Hatch Day"},
	"Community Day":  {"Community Day", "Community Classic Day"},
	"Spotlight Hour": {"Spotlight Hour"},
	"Max Battle":     {"Max Battle Weekend", "Max Battle Day", "Max Weekend", "Gigantamax", "GMAX"},
	"GO Tour":        {"GO Tour"},
	"GO Fest":        {"GO Fest"},
	"GO Wild Area":   {"GOWA"},
}

func (h *handler) getEventCategories(eventName string) string {
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
	if h.Cfg.WarnUnknownEventCategories {
		slog.Warn("Unknown event category", slog.String("event_name", eventName))
	}
	return EventCategoryOther
}

func calcCheckInRate(accepted int, checkIns int) float64 {
	if checkIns == 0 {
		return 0
	}
	return math.RoundToEven(float64(checkIns) / float64(accepted) * 100)
}

func calcQuarterProgress(days int, daysRemaining int) float64 {
	if days == 0 {
		return 0
	}
	if daysRemaining <= 0 {
		return 100
	}
	return math.RoundToEven(float64(days-daysRemaining) / float64(days) * 100)
}

func calcCheckInProgress(goal int, checkIns int) float64 {
	if goal == 0 {
		return 0
	}
	if checkIns >= goal {
		return 100
	}
	return math.RoundToEven(float64(checkIns) / float64(goal) * 100)
}

func calcCAProjectedCheckIns(from time.Time, to time.Time, totalCheckIns int) (int, int, int) {
	duration := to.Sub(from)
	if duration <= 0 {
		return 0, 0, 0 // No projection if the duration is zero or negative
	}

	days := int(duration.Hours() / 24)
	if days == 0 {
		return totalCheckIns, 0, 0 // No projection if the duration is less than a day
	}

	now := time.Now()
	nowDuration := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Sub(from)

	daysElapsed := int(nowDuration.Hours() / 24)

	daysRemaining := int(max(float64(days-daysElapsed), 0))

	// project for the remaining days in the quarter
	projectedCheckIns := totalCheckIns
	if daysRemaining > 0 {
		projectedCheckIns = int(math.Round(float64(totalCheckIns) / float64(daysElapsed) * float64(days)))
	}

	return projectedCheckIns, days, daysRemaining
}

func parseTimeQuery(query url.Values, name string, defaultValue time.Time) time.Time {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseBoolQuery(query url.Values, name string, defaultValue bool) bool {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := xstrconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseIntQuery(query url.Values, name string, defaultValue int) int {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func parseStringQuery(query url.Values, name string, defaultValue string) string {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseStringSliceQuery(query url.Values, name string, defaultValue []string) []string {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	var result []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func getRangeFromQuarter(value string) (time.Time, time.Time) {
	value = strings.ToLower(value)

	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return getCurrentQuarter()
	}

	quarter := parts[0]
	yearStr := parts[1]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return getCurrentQuarter()
	}

	var startMonth time.Month
	switch quarter {
	case "q1":
		startMonth = time.January
	case "q2":
		startMonth = time.April
	case "q3":
		startMonth = time.July
	case "q4":
		startMonth = time.October
	default:
		return getCurrentQuarter()
	}
	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the quarter

	return start, end
}

func getCurrentQuarter() (time.Time, time.Time) {
	now := time.Now()
	year := now.Year()
	month := now.Month()

	var startMonth time.Month
	switch month {
	case time.January, time.February, time.March:
		startMonth = time.January
	case time.April, time.May, time.June:
		startMonth = time.April
	case time.July, time.August, time.September:
		startMonth = time.July
	default:
		startMonth = time.October
	}

	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 3, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the quarter

	return start, end
}
