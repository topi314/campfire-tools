package web

import (
	"context"
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
	"github.com/topi314/campfire-tools/internal/xtime"
)

const (
	OriginLeagueGoal    = 1
	GreatLeagueGoal     = 61
	UltraLeagueGoal     = 250
	MasterLeagueGoal    = 750
	LegendaryLeagueGoal = 1500
)

type TrackerClubStatsVars struct {
	Club
	EventsFilter

	EventCategories EventCategories
	LeagueGoals     LeagueGoals
	DigitalCodes    DigitalCodes
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

type LeagueGoal struct {
	Name       string
	Goal       int
	Progress   float64
	Projection bool
}

type DigitalCodes struct {
	Open   bool
	Months []DigitalCodeMonth
}

type DigitalCodeMonth struct {
	Date              time.Time
	CheckIns          int
	PredictedCheckIns int
	Codes             int
	PredictedCodes    int
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

	onlyCAEvents := parseBoolQuery(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")
	categoriesClosed := parseBoolQuery(query, "event-categories-closed", false)
	digitalCodesClosed := parseBoolQuery(query, "digital-codes-closed", false)
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

	quarterFrom, quarterTo := xtime.GetRangeFromQuarter(leagueGoalQuarter)

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	eventCategories, err := h.calculateEventCategories(ctx, clubID, from, to, onlyCAEvents, eventCreator, categoriesClosed)
	if err != nil {
		http.Error(w, "Failed to fetch event categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	digitalCodes, err := h.calculateDigitalCodes(ctx, clubID, digitalCodesClosed)
	if err != nil {
		http.Error(w, "Failed to fetch digital codes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	goals, err := h.calculateLeagueGoals(ctx, clubID, quarterFrom, quarterTo, leagueGoalQuarter, eventCreator, leagueGoalsClosed)
	if err != nil {
		http.Error(w, "Failed to fetch league goals: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		Club: newClub(*club),
		EventsFilter: EventsFilter{
			FilterURL:            r.URL.Path,
			From:                 from,
			To:                   to,
			OnlyCAEvents:         onlyCAEvents,
			Quarters:             xtime.GetQuarters(),
			EventCreators:        eventCreators,
			SelectedEventCreator: eventCreator,
		},
		EventCategories: *eventCategories,
		DigitalCodes:    *digitalCodes,
		LeagueGoals:     *goals,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club stats template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

func (h *handler) calculateEventCategories(ctx context.Context, clubID string, from time.Time, to time.Time, onlyCAEvents bool, eventCreator string, categoriesClosed bool) (*EventCategories, error) {
	totalAccepted, totalCheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch total check-ins and accepted members: %w", err)
	}

	events, err := h.DB.GetEventCheckInAcceptedCounts(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event check-in and accepted counts: %w", err)
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
	categories = append(categories, EventCategory{
		Name:             "Total",
		Events:           len(events),
		Accepted:         totalAccepted,
		CheckIns:         totalCheckIns,
		CheckInRate:      calcCheckInRate(totalAccepted, totalCheckIns),
		TotalCheckInRate: 100,
	})

	return &EventCategories{
		Open:       !categoriesClosed,
		Categories: categories,
	}, nil
}

func (h *handler) calculateDigitalCodes(ctx context.Context, clubID string, digitalCodesClosed bool) (*DigitalCodes, error) {
	startDate := time.Now().AddDate(0, 1, 0)
	endDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)

	var digitalCodeMonths []DigitalCodeMonth
	for date := startDate; !date.Before(endDate); date = date.AddDate(0, -1, 0) {
		from := date.AddDate(0, -3, 0)
		to := date.AddDate(0, 0, -1).
			Add(time.Hour*23 + time.Minute*59 + time.Second*59)
		_, checkIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, true, "")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch total check-ins and accepted members for digital codes: %w", err)
		}

		predictedCheckIns, _, _ := calcCAProjectedCheckIns(from, to, checkIns)
		codes := int(float64(checkIns/3) * 0.25)
		predictedCodes := int(float64(predictedCheckIns/3) * 0.25)

		digitalCodeMonths = append(digitalCodeMonths, DigitalCodeMonth{
			Date:              date,
			CheckIns:          checkIns,
			PredictedCheckIns: predictedCheckIns,
			Codes:             codes,
			PredictedCodes:    predictedCodes,
		})
	}

	return &DigitalCodes{
		Open:   !digitalCodesClosed,
		Months: digitalCodeMonths,
	}, nil
}

func (h *handler) calculateLeagueGoals(ctx context.Context, clubID string, from time.Time, to time.Time, eventCreator string, leagueGoalQuarter string, leagueGoalsClosed bool) (*LeagueGoals, error) {
	_, totalCACheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, true, eventCreator)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch total check-ins and accepted members: %w", err)
	}

	biggestEvent, err := h.DB.GetBiggestCheckInEvent(ctx, clubID, from, to, true)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to fetch biggest check-in event: %w", err)
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

	totalCAProjectedCheckIns, quarterDays, quarterDaysRemaining := calcCAProjectedCheckIns(from, to, totalCACheckIns)

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

	return &LeagueGoals{
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
	}, nil
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
