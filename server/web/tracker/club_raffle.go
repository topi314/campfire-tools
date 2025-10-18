package tracker

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerClubRaffleVars struct {
	models.Club
	EventsFilter
	Events          []models.Event
	SelectedEventID string
	Error           string
}

func (h *handler) TrackerClubRaffle(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubRaffle(w, r, "")
}

func (h *handler) renderTrackerClubRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	eventID := query.Get("event")
	from := xquery.ParseTime(query, "from", time.Time{})
	to := xquery.ParseTime(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
	}
	onlyCAEvents := xquery.ParseBool(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID, from, to, onlyCAEvents, eventCreator)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]models.Event, len(events))
	for i, event := range events {
		trackerEvents[i] = models.NewEventWithCheckIns(event, 32)
	}

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_raffle.gohtml", TrackerClubRaffleVars{
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
		Events:          trackerEvents,
		SelectedEventID: eventID,
		Error:           errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club raffle template", slog.Any("err", err))
	}
}
