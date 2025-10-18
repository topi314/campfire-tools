package tracker

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerClubEventsVars struct {
	models.Club
	EventsFilter

	Events           []models.TopEvent
	TotalAccepted    int
	TotalCheckIns    int
	TotalCheckInRate float64
}

func (h *handler) TrackerClubEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	from := xquery.ParseTime(query, "from", time.Time{})
	to := xquery.ParseTime(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
	}
	onlyCAEvents := xquery.ParseBool(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetTopEventsByClub(ctx, clubID, from, to, onlyCAEvents, eventCreator, -1)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]models.TopEvent, len(events))
	for i, event := range events {
		trackerEvents[i] = models.NewTopEvent(event, 32)
	}

	totalAccepted, totalCheckIns, err := h.DB.GetClubTotalCheckInsAccepted(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		http.Error(w, "Failed to fetch total check-ins and accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	totalCheckInRate := models.CalcCheckInRate(totalAccepted, totalCheckIns)

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_events.gohtml", TrackerClubEventsVars{
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
		Events:           trackerEvents,
		TotalCheckIns:    totalCheckIns,
		TotalAccepted:    totalAccepted,
		TotalCheckInRate: totalCheckInRate,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club events template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}
