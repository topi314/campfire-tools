package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

type RaffleVars struct {
	Raffles         []Raffle
	SelectedEventID string
	Error           string
}

type RaffleResultVars struct {
	Raffle
	ClubID          string
	RerunRaffleURL  string
	Error           string
	Winners         []Winner
	PastWinners     []Winner
	PastWinnersOpen bool
	BackURL         string
}

func (h *handler) Raffle(w http.ResponseWriter, r *http.Request) {
	session := auth.GetSession(r)

	var raffles []database.Raffle
	if session.UserID != "" {
		var err error
		raffles, err = h.DB.GetRafflesByUserID(r.Context(), session.UserID)
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to get raffles from database", slog.Any("err", err))
			h.renderRaffle(w, r, nil, "Failed to get raffles: "+err.Error())
			return
		}
	}

	dbRaffles := make([]Raffle, 0, len(raffles))
	for _, raffle := range raffles {
		dbRaffles = append(dbRaffles, newRaffle(raffle))
	}
	h.renderRaffle(w, r, dbRaffles, "")
}

func (h *handler) RunRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	events := strings.TrimSpace(r.FormValue("events"))
	eventIDs := r.Form["ids"]
	winnerCount := parseIntQuery(r.Form, "winner_count", 1)
	onlyCheckedIn := parseBoolQuery(r.Form, "only_checked_in", false)
	singleEntry := parseBoolQuery(r.Form, "single_entry", false)

	slog.InfoContext(ctx, "Received raffle request",
		slog.String("url", r.URL.String()),
		slog.String("events", events),
		slog.Int("winner_count", winnerCount),
		slog.Bool("only_checked_in", onlyCheckedIn),
		slog.Bool("single_entry", singleEntry),
	)

	if events == "" && len(eventIDs) == 0 {
		h.renderRaffle(w, r, nil, "Missing 'events' parameter")
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
	if len(allEvents) > 50 {
		h.renderRaffle(w, r, nil, fmt.Sprintf("please limit the number of events to 50, got %d.", len(allEvents)))
		return
	}
	allEvents = append(allEvents, eventIDs...)

	eg, egCtx := errgroup.WithContext(ctx)
	var allEventIDs []string
	var mu sync.Mutex
	for _, event := range allEvents {
		eg.Go(func() error {
			eventID, err := h.fetchEventID(egCtx, event)
			if err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", event, err)
			}

			mu.Lock()
			defer mu.Unlock()

			allEventIDs = append(allEventIDs, eventID)

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to fetch events: "+err.Error())
		return
	}

	session := auth.GetSession(r)

	raffle := database.Raffle{
		UserID:        session.UserID,
		Events:        allEventIDs,
		WinnerCount:   winnerCount,
		OnlyCheckedIn: onlyCheckedIn,
		SingleEntry:   singleEntry,
	}

	winners, err := h.raffleWinners(ctx, raffle, nil)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to run raffle", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to run raffle: "+err.Error())
		return
	}

	raffleID, err := h.DB.InsertRaffle(ctx, raffle)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to insert raffle into database", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to create raffle: "+err.Error())
		return
	}

	if err = h.processRaffleWinners(ctx, raffleID, winners, true); err != nil {
		slog.ErrorContext(ctx, "Failed to process raffle winners", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to process raffle winners: "+err.Error())
		return
	}

	redirectRaffle(w, r, raffleID, clubID, "")
}

func (h *handler) RerunRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	pastWinnersOpenStr := r.FormValue("past_winners")

	slog.InfoContext(ctx, "Received rerun raffle request",
		slog.String("url", r.URL.String()),
		slog.String("raffle_id", raffleIDStr),
		slog.String("past_winners_open", pastWinnersOpenStr),
	)

	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	pastWinnersOpen, _ := strconv.ParseBool(pastWinnersOpenStr)

	raffle, err := h.DB.GetRaffleByID(ctx, raffleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	session := auth.GetSession(r)
	if raffle.UserID != "" && raffle.UserID != session.UserID {
		h.NotFound(w, r)
		return
	}

	pastWinners, err := h.DB.GetRaffleWinners(ctx, raffleID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get past raffle winners from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to get past raffle winners: "+err.Error())
		return
	}

	winners, err := h.raffleWinners(ctx, *raffle, pastWinners)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to rerun raffle", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to rerun raffle: "+err.Error())
		return
	}

	if err = h.processRaffleWinners(ctx, raffleID, winners, false); err != nil {
		slog.ErrorContext(ctx, "Failed to process raffle winners", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to process raffle winners: "+err.Error())
		return
	}

	var rawQuery string
	if pastWinnersOpen {
		rawQuery = "past-winners=true"
	}

	redirectRaffle(w, r, raffleID, clubID, rawQuery)
}

func (h *handler) GetRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	pastWinnersOpen := parseBoolQuery(query, "past-winners", false)

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("raffle_id", raffleIDStr))

	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	raffle, err := h.DB.GetRaffleByID(ctx, raffleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	session := auth.GetSession(r)
	if raffle.UserID != "" && raffle.UserID != session.UserID {
		h.NotFound(w, r)
		return
	}

	allWinners, err := h.DB.GetRaffleWinners(ctx, raffleID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get raffle winners from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to get raffle winners: "+err.Error())
		return
	}

	var winners []Winner
	var pastWinners []Winner
	for _, winner := range allWinners {
		if winner.Past {
			pastWinners = append(pastWinners, newWinner(winner, clubID))
		} else {
			winners = append(winners, newWinner(winner, clubID))
		}
	}

	var backURL string
	if clubID != "" {
		backURL = fmt.Sprintf("/tracker/club/%s/raffle", clubID)
	} else {
		backURL = "/raffle"
	}

	if err = h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", RaffleResultVars{
		Raffle:          newRaffle(*raffle),
		ClubID:          clubID,
		RerunRaffleURL:  r.URL.Path,
		Winners:         winners,
		PastWinners:     pastWinners,
		PastWinnersOpen: pastWinnersOpen,
		BackURL:         backURL,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}

func (h *handler) raffleWinners(ctx context.Context, raffle database.Raffle, pastWinners []database.RaffleWinnerWithMember) ([]campfire.Member, error) {
	eg, egCtx := errgroup.WithContext(ctx)
	var eventIDs []string
	var members []campfire.Member
	var mu sync.Mutex
	for _, eventID := range raffle.Events {
		eg.Go(func() error {
			event, err := h.fetchEvent(egCtx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", eventID, err)
			}

			mu.Lock()
			defer mu.Unlock()

			eventIDs = append(eventIDs, event.ID)

			if len(event.RSVPStatuses) == 0 {
				return nil
			}

			for _, rsvpStatus := range event.RSVPStatuses {
				// Only consider checked-in members if `onlyCheckedIn` is true
				if rsvpStatus.RSVPStatus == "DECLINED" || (raffle.OnlyCheckedIn && rsvpStatus.RSVPStatus != "CHECKED_IN") {
					continue
				}

				// Skip if the user is already in the members
				if raffle.SingleEntry && slices.ContainsFunc(members, func(member campfire.Member) bool {
					return member.ID == rsvpStatus.UserID
				}) {
					continue
				}

				// Skip if the user is a past winner & confirmed
				if slices.ContainsFunc(pastWinners, func(pastWinner database.RaffleWinnerWithMember) bool {
					return pastWinner.Member.ID == rsvpStatus.UserID && pastWinner.Confirmed
				}) {
					continue
				}

				// Skip if we don't have the member's information
				member, ok := campfire.FindMember(rsvpStatus.UserID, *event)
				if !ok {
					continue
				}

				members = append(members, member)
			}

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	winners := make([]campfire.Member, 0, raffle.WinnerCount)
	for {
		if len(members) == 0 || len(winners) >= raffle.WinnerCount {
			break
		}
		num := rand.N(len(members))
		member := members[num]
		members = slices.Delete(members, num, num+1) // Remove selected member to avoid duplicates

		winners = append(winners, member)
	}

	return winners, nil
}

func (h *handler) processRaffleWinners(ctx context.Context, raffleID int, winners []campfire.Member, create bool) error {
	if len(winners) > 0 {
		members := make([]database.Member, 0, len(winners))
		for _, winner := range winners {
			members = append(members, database.Member{
				ID:          winner.ID,
				Username:    winner.Username,
				DisplayName: winner.DisplayName,
				AvatarURL:   winner.AvatarURL,
				RawJSON:     winner.Raw,
			})
		}
		if err := h.DB.InsertMembers(ctx, members); err != nil {
			return err
		}
	}

	if !create {
		if err := h.DB.DeleteNotConfirmedRaffleWinners(ctx, raffleID); err != nil {
			return err
		}
		if err := h.DB.MarkRaffleWinnersAsPast(ctx, raffleID); err != nil {
			return err
		}
	}

	if len(winners) > 0 {
		winnerIDs := make([]string, 0, len(winners))
		for _, winner := range winners {
			winnerIDs = append(winnerIDs, winner.ID)
		}
		if err := h.DB.InsertRaffleWinners(ctx, raffleID, winnerIDs); err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) fetchEventID(ctx context.Context, event string) (string, error) {
	if strings.HasPrefix(event, "https://") {
		eventID, err := h.Campfire.ResolveEventID(ctx, event)
		if err != nil {
			return "", fmt.Errorf("failed to resolve event ID from URL %q: %w", event, err)
		}
		return eventID, nil
	}

	dbEvent, err := h.DB.GetEvent(ctx, event)
	if err == nil {
		return dbEvent.Event.ID, nil
	}

	campfireEvent, err := h.Campfire.GetEvent(ctx, event)
	if err != nil {
		return "", fmt.Errorf("failed to fetch event %q: %w", event, err)
	}

	return campfireEvent.ID, nil
}

func (h *handler) fetchEvent(ctx context.Context, event string) (*campfire.Event, error) {
	event = strings.TrimSpace(event)

	if strings.HasPrefix(event, "https://") {
		eventID, err := h.Campfire.ResolveEventID(ctx, event)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve event ID from URL %q: %w", event, err)
		}
		event = eventID
	}

	dbEvent, err := h.DB.GetEvent(ctx, event)
	if err == nil && dbEvent.Finished {
		fullEvent, err := h.unmarshalEvent(ctx, *dbEvent)
		if err == nil {
			return fullEvent, nil
		}
	}

	fullEvent, err := h.Campfire.GetEvent(ctx, event)
	if err == nil && dbEvent != nil && !dbEvent.Finished {
		if err = h.ProcessFullEventImport(ctx, *fullEvent, true); err != nil {
			slog.ErrorContext(ctx, "Failed to update event in database", slog.String("event_id", event), slog.Any("err", err))
		}
	}

	return fullEvent, nil
}

func (h *handler) unmarshalEvent(ctx context.Context, event database.EventWithCreator) (*campfire.Event, error) {
	var fullEvent campfire.Event
	if err := json.Unmarshal(event.Event.RawJSON, &fullEvent); err != nil {
		return nil, err
	}

	if len(fullEvent.Members.Edges) > 0 {
		return &fullEvent, nil
	}

	members, err := h.DB.GetEventMembers(ctx, event.Event.ID)
	if err != nil {
		return nil, err
	}

	for _, member := range members {
		fullEvent.RSVPStatuses = append(fullEvent.RSVPStatuses, campfire.RSVPStatus{
			UserID:     member.ID,
			RSVPStatus: member.Status,
		})
		fullEvent.Members.TotalCount++

		var fullMember campfire.Member
		if err = json.Unmarshal(member.RawJSON, &fullMember); err != nil {
			return nil, err
		}
		fullEvent.Members.Edges = append(fullEvent.Members.Edges, campfire.Edge[campfire.Member]{
			Node:   fullMember,
			Cursor: "",
		})
	}

	return &fullEvent, nil
}

func (h *handler) renderRaffle(w http.ResponseWriter, r *http.Request, raffles []Raffle, errorMessage string) {
	if strings.HasPrefix(r.URL.Path, "/tracker/club/") {
		h.renderTrackerClubRaffle(w, r, errorMessage)
		return
	}

	query := r.URL.Query()
	ctx := r.Context()

	eventID := query.Get("event")

	if err := h.Templates().ExecuteTemplate(w, "raffle.gohtml", RaffleVars{
		Raffles:         raffles,
		SelectedEventID: eventID,
		Error:           errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle template", slog.Any("err", err))
	}
}

func (h *handler) renderRaffleResult(w http.ResponseWriter, r *http.Request, raffle database.Raffle, clubID string, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", RaffleResultVars{
		Raffle: newRaffle(raffle),
		ClubID: clubID,
		Error:  errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}

func getDisplayName(displayName string, username string) string {
	if displayName == "" {
		displayName = username
	}
	if displayName == "" {
		displayName = "<unknown>"
	}
	return displayName
}
