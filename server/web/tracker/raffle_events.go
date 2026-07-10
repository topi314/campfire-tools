package tracker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type RaffleAddEventsVars struct {
	RaffleID   int
	ClubID     string
	BackURL    string
	FormAction string
	Events     []models.Event
	Existing   []models.Event
	Error      string
	models.Club
	EventsFilter
}

func (h *handler) AddRaffleEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, r.PathValue("raffle_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	existingEvents, err := h.fetchRaffleRenderEvents(ctx, clubID, raffle.Events)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch raffle events", slog.Any("err", err))
		http.Error(w, "Failed to get raffle events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var backURL string
	var formAction string
	if clubID != "" {
		backURL = fmt.Sprintf("/tracker/club/%s/raffle/%d", clubID, raffleID)
		formAction = fmt.Sprintf("/tracker/club/%s/raffle/%d/events", clubID, raffleID)
		h.renderClubAddRaffleEvents(w, r, clubID, raffleID, backURL, formAction, raffle.Events, existingEvents, "")
		return
	}

	backURL = fmt.Sprintf("/raffle/%d", raffleID)
	formAction = fmt.Sprintf("/raffle/%d/events", raffleID)
	if err = h.Templates().ExecuteTemplate(w, "raffle_add_events.gohtml", RaffleAddEventsVars{
		RaffleID:   raffleID,
		BackURL:    backURL,
		FormAction: formAction,
		Existing:   existingEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render add raffle events template", slog.Any("err", err))
	}
}

func (h *handler) PostAddRaffleEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, r.PathValue("raffle_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events := strings.TrimSpace(r.FormValue("events"))
	eventIDs := r.Form["ids"]

	if events == "" && len(eventIDs) == 0 {
		h.renderAddRaffleEventsError(w, r, clubID, raffleID, raffle.Events, "Missing events to add")
		return
	}

	var allEvents []string
	for _, event := range strings.Split(events, "\n") {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}
		allEvents = append(allEvents, event)
	}
	allEvents = append(allEvents, eventIDs...)

	if len(allEvents) > 50 {
		h.renderAddRaffleEventsError(w, r, clubID, raffleID, raffle.Events, fmt.Sprintf("please limit the number of events to add to 50, got %d.", len(allEvents)))
		return
	}

	eg, egCtx := errgroup.WithContext(ctx)
	var newEventIDs []string
	var mu sync.Mutex
	for _, event := range allEvents {
		eg.Go(func() error {
			eventID, err := h.fetchEventID(egCtx, event)
			if err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", event, err)
			}

			mu.Lock()
			defer mu.Unlock()
			newEventIDs = append(newEventIDs, eventID)
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		h.renderAddRaffleEventsError(w, r, clubID, raffleID, raffle.Events, "Failed to fetch events: "+err.Error())
		return
	}

	mergedCount := countUniqueEventIDs(raffle.Events, newEventIDs)
	if mergedCount > 50 {
		h.renderAddRaffleEventsError(w, r, clubID, raffleID, raffle.Events, fmt.Sprintf("raffle cannot have more than 50 events, would have %d.", mergedCount))
		return
	}

	if err = h.DB.AppendRaffleEvents(ctx, raffleID, newEventIDs); err != nil {
		slog.ErrorContext(ctx, "Failed to append raffle events", slog.Any("err", err))
		h.renderAddRaffleEventsError(w, r, clubID, raffleID, raffle.Events, "Failed to add events: "+err.Error())
		return
	}

	redirectRaffle(w, r, raffleID, clubID, "")
}

func (h *handler) getAuthorizedRaffle(ctx context.Context, r *http.Request, raffleIDStr string) (int, *database.Raffle, error) {
	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		return 0, nil, sql.ErrNoRows
	}

	raffle, err := h.DB.GetRaffleByID(ctx, raffleID)
	if err != nil {
		return 0, nil, err
	}

	session := auth.GetSession(r)
	if raffle.UserID != "" && raffle.UserID != session.UserID {
		return 0, nil, sql.ErrNoRows
	}

	return raffleID, raffle, nil
}

func (h *handler) renderAddRaffleEventsError(w http.ResponseWriter, r *http.Request, clubID string, raffleID int, existingEventIDs []string, errorMessage string) {
	ctx := r.Context()

	existingEvents, err := h.fetchRaffleRenderEvents(ctx, clubID, existingEventIDs)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch raffle events", slog.Any("err", err))
		http.Error(w, "Failed to get raffle events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var backURL string
	var formAction string
	if clubID != "" {
		backURL = fmt.Sprintf("/tracker/club/%s/raffle/%d", clubID, raffleID)
		formAction = fmt.Sprintf("/tracker/club/%s/raffle/%d/events", clubID, raffleID)
		h.renderClubAddRaffleEvents(w, r, clubID, raffleID, backURL, formAction, existingEventIDs, existingEvents, errorMessage)
		return
	}

	backURL = fmt.Sprintf("/raffle/%d", raffleID)
	formAction = fmt.Sprintf("/raffle/%d/events", raffleID)
	if err = h.Templates().ExecuteTemplate(w, "raffle_add_events.gohtml", RaffleAddEventsVars{
		RaffleID:   raffleID,
		BackURL:    backURL,
		FormAction: formAction,
		Existing:   existingEvents,
		Error:      errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render add raffle events template", slog.Any("err", err))
	}
}

func (h *handler) renderClubAddRaffleEvents(w http.ResponseWriter, r *http.Request, clubID string, raffleID int, backURL, formAction string, existingEventIDs []string, renderExisting []models.Event, errorMessage string) {
	ctx := r.Context()
	query := r.URL.Query()

	from := xquery.ParseTime(query, "from", time.Time{})
	to := xquery.ParseTime(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59)
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

	existingSet := make(map[string]struct{}, len(existingEventIDs))
	for _, id := range existingEventIDs {
		existingSet[id] = struct{}{}
	}

	availableEvents := make([]models.Event, 0, len(events))
	for _, event := range events {
		if _, exists := existingSet[event.Event.ID]; exists {
			continue
		}
		availableEvents = append(availableEvents, models.NewEventWithCheckIns(event, 32))
	}

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_raffle_add_events.gohtml", RaffleAddEventsVars{
		RaffleID:   raffleID,
		ClubID:     clubID,
		BackURL:    backURL,
		FormAction: formAction,
		Club:       models.NewClub(*club),
		EventsFilter: EventsFilter{
			FilterURL:            r.URL.Path,
			From:                 from,
			To:                   to,
			OnlyCAEvents:         onlyCAEvents,
			Quarters:             xtime.GetQuarters(),
			EventCreators:        eventCreators,
			SelectedEventCreator: eventCreator,
		},
		Events:   availableEvents,
		Existing: renderExisting,
		Error:    errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render club add raffle events template", slog.Any("err", err))
	}
}

func countUniqueEventIDs(existing, additional []string) int {
	seen := make(map[string]struct{}, len(existing)+len(additional))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	for _, id := range additional {
		seen[id] = struct{}{}
	}
	return len(seen)
}
