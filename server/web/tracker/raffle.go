package tracker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/lib/pq"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type RaffleVars struct {
	Raffles         []models.Raffle
	SelectedEventID string
	Error           string
}

type RaffleNotice struct {
	Person string
	Marker string
	At     time.Time
	HasAt  bool
	Gone   bool
}

type RaffleResultVars struct {
	models.Raffle
	Events             []models.Event
	ClubID             string
	RerunRaffleURL     string
	AddEventsURL       string
	ShareURL           string
	PollURL            string
	Users              []models.DiscordUser
	Error              string
	HasNotice          bool
	Notice             RaffleNotice
	SwapNotice         bool
	Winners            []models.Winner
	ConfirmedWinners   []models.Winner
	ConfirmedOpen      bool
	PastWinners        []models.Winner
	PastWinnersOpen    bool
	BlockedMembers     []models.Member
	BlockedMemberID    map[string]bool
	BlockedMembersOpen bool
	CanBlock           bool
	BlockAction        string
	UnblockAction      string
	BackURL            string
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

	dbRaffles := make([]models.Raffle, 0, len(raffles))
	for _, raffle := range raffles {
		dbRaffles = append(dbRaffles, models.NewRaffle(raffle))
	}
	h.renderRaffle(w, r, dbRaffles, "")
}

func (h *handler) RunRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderRaffle(w, r, nil, "Name is required")
		return
	}
	if len([]rune(title)) > 200 {
		h.renderRaffle(w, r, nil, "Name must be 200 characters or fewer")
		return
	}

	events := strings.TrimSpace(r.FormValue("events"))
	eventIDs := r.Form["ids"]
	winnerCount := xquery.ParseInt(r.Form, "winner_count", 1)
	onlyCheckedIn := xquery.ParseBool(r.Form, "only_checked_in", false)
	singleEntry := xquery.ParseBool(r.Form, "single_entry", false)

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
		Title:         title,
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

	raffle, err = h.insertRaffle(ctx, raffle)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to insert raffle into database", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to create raffle: "+err.Error())
		return
	}
	if session.UserID != "" {
		if err = h.DB.AddRaffleUser(ctx, raffle.ID, session.UserID); err != nil {
			slog.ErrorContext(ctx, "Failed to add raffle creator", slog.Any("err", err))
			h.renderRaffle(w, r, nil, "Failed to create raffle: "+err.Error())
			return
		}
	}
	raffleID := raffle.ID

	if err = h.processRaffleWinners(ctx, raffleID, winners, true); err != nil {
		slog.ErrorContext(ctx, "Failed to process raffle winners", slog.Any("err", err))
		h.renderRaffle(w, r, nil, "Failed to process raffle winners: "+err.Error())
		return
	}

	redirectRaffle(w, r, raffle.Token, "", "")
}

func (h *handler) RerunRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	pastWinnersOpenStr := r.FormValue("past_winners")
	blockedMembersOpenStr := r.FormValue("blocked_members")

	slog.InfoContext(ctx, "Received rerun raffle request",
		slog.String("url", r.URL.String()),
		slog.String("raffle_id", raffleIDStr),
		slog.String("past_winners_open", pastWinnersOpenStr),
		slog.String("blocked_members_open", blockedMembersOpenStr),
	)

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, raffleIDStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	pastWinnersOpen, _ := strconv.ParseBool(pastWinnersOpenStr)
	blockedMembersOpen, _ := strconv.ParseBool(blockedMembersOpenStr)

	if err = h.DB.RunRaffleDraw(ctx, raffleID, func(existing []database.RaffleWinnerWithMember) ([]string, error) {
		winners, drawErr := h.raffleWinners(ctx, *raffle, existing)
		if drawErr != nil {
			return nil, drawErr
		}
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
			if insertErr := h.DB.InsertMembers(ctx, members); insertErr != nil {
				return nil, insertErr
			}
		}
		memberIDs := make([]string, 0, len(winners))
		for _, winner := range winners {
			memberIDs = append(memberIDs, winner.ID)
		}
		return memberIDs, nil
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to rerun raffle", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to rerun raffle: "+err.Error())
		return
	}

	redirectRaffle(w, r, raffle.Token, clubID, rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, xquery.ParseBool(r.Form, "confirmed", false)))
}

func (h *handler) PostRaffleTitle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")

	_, raffle, err := h.getAuthorizedRaffle(ctx, r, r.PathValue("raffle_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err = r.ParseForm(); err != nil {
		http.Redirect(w, r, raffleURL(raffle.Token, clubID)+"/events?error=Failed+to+parse+form", http.StatusSeeOther)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if len([]rune(title)) > 200 {
		http.Redirect(w, r, raffleURL(raffle.Token, clubID)+"/events?error="+url.QueryEscape("Name must be 200 characters or fewer"), http.StatusSeeOther)
		return
	}
	winnerCount := xquery.ParseInt(r.Form, "winner_count", 0)
	if winnerCount < 1 {
		http.Redirect(w, r, raffleURL(raffle.Token, clubID)+"/events?error="+url.QueryEscape("Number of winners must be at least 1"), http.StatusSeeOther)
		return
	}
	onlyCheckedIn := xquery.ParseBool(r.Form, "only_checked_in", false)
	singleEntry := xquery.ParseBool(r.Form, "single_entry", false)
	if err = h.DB.UpdateRaffleSettings(ctx, raffle.ID, title, winnerCount, onlyCheckedIn, singleEntry); err != nil {
		slog.ErrorContext(ctx, "Failed to update raffle settings", slog.Any("err", err))
		http.Redirect(w, r, raffleURL(raffle.Token, clubID)+"/events?error=Failed+to+save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, raffleURL(raffle.Token, clubID)+"/events", http.StatusSeeOther)
}

func (h *handler) GetRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	pastWinnersOpen := xquery.ParseBool(query, "past-winners", false)
	blockedMembersOpen := xquery.ParseBool(query, "blocked-members", false)

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("raffle_id", raffleIDStr))

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, raffleIDStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	raffleEvents, err := h.fetchRaffleRenderEvents(ctx, clubID, raffle.Events)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch raffle events", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle events: "+err.Error())
		return
	}
	renderEvents := raffleEvents

	allWinners, err := h.DB.GetRaffleWinners(ctx, raffleID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get raffle winners from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to get raffle winners: "+err.Error())
		return
	}

	winners, confirmedWinners, pastWinners := splitRaffleWinners(allWinners, clubID, raffle.Token)

	var blockedMembers []models.Member
	if raffle.UserID != "" {
		blocked, blockedErr := h.DB.GetDiscordUserRaffleBlockedMembers(ctx, raffle.UserID)
		if blockedErr != nil {
			slog.ErrorContext(ctx, "Failed to get raffle blocked members", slog.Any("err", blockedErr))
			h.renderRaffleResult(w, r, *raffle, clubID, "Failed to get blocked members: "+blockedErr.Error())
			return
		}
		blockedMembers = make([]models.Member, len(blocked))
		for i, member := range blocked {
			blockedMembers[i] = models.NewImportedMember(member, 32)
		}
	}

	var backURL string
	var addEventsURL string
	if clubID != "" {
		backURL = fmt.Sprintf("/tracker/club/%s/raffle", clubID)
		addEventsURL = raffleURL(raffle.Token, clubID) + "/events"
	} else {
		backURL = "/raffle"
		addEventsURL = raffleURL(raffle.Token, "") + "/events"
	}

	access, err := h.DB.GetRaffleUsers(ctx, raffle.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get raffle users", slog.Any("err", err))
		h.renderRaffleResult(w, r, *raffle, clubID, "Failed to get raffle users: "+err.Error())
		return
	}
	users := make([]models.DiscordUser, 0, len(access))
	for _, user := range access {
		users = append(users, models.NewDiscordUser(user))
	}

	if err = h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", RaffleResultVars{
		Raffle:             models.NewRaffle(*raffle),
		Events:             renderEvents,
		ClubID:             clubID,
		RerunRaffleURL:     r.URL.Path,
		AddEventsURL:       addEventsURL,
		ShareURL:           absoluteURL(r, raffleSharePath(raffle.Token)),
		PollURL:            raffleURL(raffle.Token, clubID) + "/winners/poll",
		Users:              users,
		Error:              strings.TrimSpace(query.Get("error")),
		Winners:            winners,
		ConfirmedWinners:   confirmedWinners,
		ConfirmedOpen:      xquery.ParseBool(query, "confirmed", false),
		PastWinners:        pastWinners,
		PastWinnersOpen:    pastWinnersOpen,
		BlockedMembers:     blockedMembers,
		BlockedMemberID:    blockedMemberIDs(blockedMembers),
		BlockedMembersOpen: blockedMembersOpen,
		CanBlock:           raffleCanBlock(r, raffle),
		BlockAction:        raffleURL(raffle.Token, clubID) + "/block/",
		UnblockAction:      raffleURL(raffle.Token, clubID) + "/unblock/",
		BackURL:            backURL,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}

func (h *handler) raffleWinners(ctx context.Context, raffle database.Raffle, pastWinners []database.RaffleWinnerWithMember) ([]campfire.Member, error) {
	var blockedMemberIDs []string
	if raffle.UserID != "" {
		var err error
		blockedMemberIDs, err = h.DB.GetDiscordUserRaffleBlockedMemberIDs(ctx, raffle.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get raffle blocked members: %w", err)
		}
	}

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

				// Skip if the user is blocked from the raffle owner's draws
				if slices.Contains(blockedMemberIDs, rsvpStatus.UserID) {
					continue
				}

				// Confirmed winners stay out. Current winners are locked so another draw cannot pick them.
				if slices.ContainsFunc(pastWinners, func(pastWinner database.RaffleWinnerWithMember) bool {
					return winnerLocked(pastWinner, rsvpStatus.UserID)
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

		// If the member has already won, skip them.
		// This can happen if singleEntry is false.
		if slices.ContainsFunc(winners, func(winner campfire.Member) bool {
			return winner.ID == member.ID
		}) {
			continue
		}

		winners = append(winners, member)
	}

	return winners, nil
}

func (h *handler) fetchRaffleRenderEvents(ctx context.Context, clubID string, eventIDs []string) ([]models.Event, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}

	eg, egCtx := errgroup.WithContext(ctx)
	results := make([]models.Event, len(eventIDs))
	for i, eventID := range eventIDs {
		eg.Go(func() error {
			if dbEvent, err := h.DB.GetEvent(egCtx, eventID); err == nil {
				event := models.NewEvent(dbEvent.Event, 32, "")
				if clubID == "" {
					event.URL = fmt.Sprintf("/event/%s", event.ID)
				}
				results[i] = event
				return nil
			}

			event, err := h.fetchEvent(egCtx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", eventID, err)
			}

			renderEvent := models.Event{
				ID:                           event.ID,
				Name:                         event.Name,
				CoverPhotoURL:                models.ImageURL(event.CoverPhotoURL, 32),
				Details:                      event.Details,
				Time:                         event.EventTime,
				EndTime:                      event.EventEndTime,
				CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			}
			if clubID != "" {
				renderEvent.URL = fmt.Sprintf("/tracker/event/%s", renderEvent.ID)
			} else {
				renderEvent.URL = fmt.Sprintf("/event/%s", renderEvent.ID)
			}
			results[i] = renderEvent
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return results, nil
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
	if err == nil && eventFinished(dbEvent.Event, time.Now()) {
		if !dbEvent.Finished {
			if markErr := h.DB.MarkEventFinished(ctx, dbEvent.Event.ID); markErr != nil {
				slog.ErrorContext(ctx, "Failed to mark event finished", slog.String("event_id", dbEvent.Event.ID), slog.Any("err", markErr))
			}
		}
		return h.unmarshalEvent(ctx, *dbEvent)
	}

	fullEvent, err := h.Campfire.GetEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	if importErr := h.ProcessFullEventImport(ctx, *fullEvent, true); importErr != nil {
		slog.ErrorContext(ctx, "Failed to update event in database", slog.String("event_id", event), slog.Any("err", importErr))
	}
	return fullEvent, nil
}

func eventFinished(event database.Event, now time.Time) bool {
	if event.Finished {
		return true
	}
	return !event.EndTime.IsZero() && !event.EndTime.After(now)
}

func (h *handler) unmarshalEvent(ctx context.Context, event database.EventWithCreator) (*campfire.Event, error) {
	var fullEvent campfire.Event
	if err := json.Unmarshal(event.Event.RawJSON, &fullEvent); err != nil {
		return nil, err
	}

	members, err := h.DB.GetEventMembers(ctx, event.Event.ID)
	if err != nil {
		return nil, err
	}

	fullEvent.RSVPStatuses = nil
	fullEvent.Members.Edges = nil
	fullEvent.Members.TotalCount = 0

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

func (h *handler) insertRaffle(ctx context.Context, raffle database.Raffle) (database.Raffle, error) {
	var lastErr error
	for range 3 {
		token, err := newChecklistToken()
		if err != nil {
			return raffle, err
		}
		raffle.Token = token

		id, err := h.DB.InsertRaffle(ctx, raffle)
		if err == nil {
			raffle.ID = id
			return raffle, nil
		}

		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			lastErr = err
			continue
		}
		return raffle, err
	}
	if lastErr == nil {
		lastErr = errors.New("failed to create raffle")
	}
	return raffle, lastErr
}

func (h *handler) renderRaffle(w http.ResponseWriter, r *http.Request, raffles []models.Raffle, errorMessage string) {
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

	vars := RaffleResultVars{
		Raffle: models.NewRaffle(raffle),
		ClubID: clubID,
		Error:  errorMessage,
	}
	if err := h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", vars); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}
