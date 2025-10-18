package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/web/models"
)

const (
	OriginLeagueGoal    = 1
	GreatLeagueGoal     = 61
	UltraLeagueGoal     = 250
	MasterLeagueGoal    = 750
	LegendaryLeagueGoal = 1500
)

type TrackerClubStatsVars struct {
	models.Club
	EventsFilter

	EventCategories models.EventCategories
	LeagueGoals     LeagueGoals
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
	BiggestEvent       *models.TopEvent
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

	from := xquery.ParseTime(query, "from", time.Time{})
	to := xquery.ParseTime(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
	}

	onlyCAEvents := xquery.ParseBool(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")
	categoriesClosed := xquery.ParseBool(query, "event-categories-closed", false)
	leagueGoalsClosed := xquery.ParseBool(query, "league-goals-closed", false)
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

	_, totalCheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetEventCheckInAcceptedCounts(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		http.Error(w, "Failed to fetch event check-in and accepted counts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	eventCategories := make(map[string]models.EventCategory)
	for _, event := range events {
		category := h.getEventCategories(event.CampfireLiveEventName)

		eventCategory, ok := eventCategories[category]
		if !ok {
			eventCategory = models.EventCategory{
				Name:     category,
				Events:   0,
				CheckIns: 0,
				Accepted: 0,
			}
		}

		eventCategory.Events++
		eventCategory.Accepted += event.Accepted
		eventCategory.CheckIns += event.CheckIns
		eventCategory.CheckInRate = models.CalcCheckInRate(eventCategory.Accepted, eventCategory.CheckIns)
		eventCategory.TotalCheckInRate = models.CalcCheckInRate(totalCheckIns, eventCategory.CheckIns)
		eventCategories[category] = eventCategory
	}

	categories := slices.Collect(maps.Values(eventCategories))
	slices.SortFunc(categories, func(a, b models.EventCategory) int {
		if a.CheckIns == b.CheckIns {
			return a.Accepted - b.Accepted
		}
		return b.CheckIns - a.CheckIns
	})

	quarterFrom, quarterTo := xtime.GetRangeFromQuarter(leagueGoalQuarter)

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, totalCACheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, quarterFrom, quarterTo, true, eventCreator)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	biggestEvent, err := h.DB.GetBiggestCheckInEvent(ctx, clubID, quarterFrom, quarterTo, true)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Failed to fetch biggest check-in event: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var trackerBiggestEvent *models.TopEvent
	if biggestEvent != nil {
		trackerBiggestEvent = &models.TopEvent{
			Event: models.Event{
				ID:            biggestEvent.ID,
				Name:          biggestEvent.Name,
				URL:           fmt.Sprintf("/tracker/event/%s", biggestEvent.ID),
				CoverPhotoURL: models.ImageURL(biggestEvent.CoverPhotoURL, 60),
			},
			Accepted:    biggestEvent.Accepted,
			CheckIns:    biggestEvent.CheckIns,
			CheckInRate: models.CalcCheckInRate(biggestEvent.Accepted, biggestEvent.CheckIns),
		}
	}

	totalCAProjectedCheckIns, quarterDays, quarterDaysRemaining := models.CalcCAProjectedCheckIns(quarterFrom, quarterTo, totalCACheckIns)

	leagueGoals := []LeagueGoal{
		{
			Name:       "Origin League",
			Goal:       OriginLeagueGoal,
			Progress:   models.CalcCheckInProgress(OriginLeagueGoal, totalCACheckIns),
			Projection: OriginLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Great League",
			Goal:       GreatLeagueGoal,
			Progress:   models.CalcCheckInProgress(GreatLeagueGoal, totalCACheckIns),
			Projection: GreatLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Ultra League",
			Goal:       UltraLeagueGoal,
			Progress:   models.CalcCheckInProgress(UltraLeagueGoal, totalCACheckIns),
			Projection: UltraLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Master League",
			Goal:       MasterLeagueGoal,
			Progress:   models.CalcCheckInProgress(MasterLeagueGoal, totalCACheckIns),
			Projection: MasterLeagueGoal <= totalCAProjectedCheckIns,
		},
		{
			Name:       "Legendary League",
			Goal:       LegendaryLeagueGoal,
			Progress:   models.CalcCheckInProgress(LegendaryLeagueGoal, totalCACheckIns),
			Projection: LegendaryLeagueGoal <= totalCAProjectedCheckIns,
		},
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		Club: models.NewClub(*club),
		EventsFilter: EventsFilter{
			FilterURL:            r.URL.Path,
			From:                 from,
			To:                   to,
			OnlyCAEvents:         onlyCAEvents,
			Quarters:             xtime.GetQuarters(),
			EventCreators:        eventCreators,
			SelectedEventCreator: eventCreator,
		},
		EventCategories: models.EventCategories{
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
			DaysElapsedPercent: models.CalcQuarterProgress(quarterDays, quarterDaysRemaining),
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
