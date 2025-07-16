package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/internal/xstrconv"
	"github.com/topi314/campfire-tools/server/campfire"
)

type DoRaffleVars struct {
	Winners       []Member
	Events        string
	Count         int
	OnlyCheckedIn string
}

func (h *handler) Raffle(w http.ResponseWriter, r *http.Request) {
	h.renderRaffle(w, r, "")
}

func (h *handler) DoRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	events := strings.TrimSpace(r.FormValue("events"))
	if ids := r.Form["ids"]; len(ids) > 0 {
		events += "\n" + strings.Join(ids, "\n")
	}
	countStr := r.FormValue("count")
	onlyCheckedInStr := r.FormValue("only_checked_in")

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("events", events), slog.String("count", countStr))

	if events == "" {
		h.renderRaffle(w, r, "Missing 'events' parameter")
		return
	}

	count := 1
	if countStr != "" {
		parsed, err := strconv.Atoi(countStr)
		if err != nil || parsed <= 0 {
			h.renderRaffle(w, r, "Invalid 'count' parameter. It must be a positive number.")
			return
		}
		count = parsed
	}

	var onlyCheckedIn bool
	if onlyCheckedInStr != "" {
		parsed, err := xstrconv.ParseBool(onlyCheckedInStr)
		if err != nil {
			h.renderRaffle(w, r, "Invalid 'only_checked_in' parameter. It must be 'true' or 'false'.")
			return
		}
		onlyCheckedIn = parsed
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
		h.renderExport(w, r, fmt.Sprintf("please limit the number of events to 50, got %d.", len(allEvents)))
		return
	}

	eg, ctx := errgroup.WithContext(ctx)
	var members []Member
	var eventIDs []string
	var mu sync.Mutex
	for _, eventID := range allEvents {
		eg.Go(func() error {
			event, err := h.fetchEvent(ctx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", eventID, err)
			}

			if len(event.RSVPStatuses) == 0 {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			for _, rsvpStatus := range event.RSVPStatuses {
				// Only consider checked-in members if `onlyCheckedIn` is true
				if rsvpStatus.RSVPStatus == "DECLINED" || (onlyCheckedIn && rsvpStatus.RSVPStatus != "CHECKED_IN") {
					continue
				}

				// Skip if the user is already in the members map
				if i := slices.IndexFunc(members, func(m Member) bool {
					return m.ID == rsvpStatus.UserID
				}); i != -1 {
					continue
				}

				// Skip if we don't have the member's information
				member, ok := campfire.FindMember(rsvpStatus.UserID, *event)
				if !ok {
					continue
				}

				members = append(members, Member{
					ID:          rsvpStatus.UserID,
					Username:    member.Username,
					DisplayName: getDisplayName(member.DisplayName, member.Username),
					AvatarURL:   imageURL(member.AvatarURL, 32),
				})
			}
			eventIDs = append(eventIDs, event.ID)

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		h.renderRaffle(w, r, "Failed to fetch events: "+err.Error())
		return
	}

	winners := make([]Member, 0, count)
	for {
		if len(members) == 0 || len(winners) >= count {
			break
		}
		num := rand.N(len(members))
		member := members[num]
		members = slices.Delete(members, num, num+1) // Remove selected member to avoid duplicates

		winners = append(winners, member)
	}

	if len(winners) == 0 {
		h.renderRaffle(w, r, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err := h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", DoRaffleVars{
		Winners:       winners,
		Events:        strings.Join(eventIDs, "\n"),
		Count:         count,
		OnlyCheckedIn: onlyCheckedInStr,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}

func (h *handler) fetchEvent(ctx context.Context, event string) (*campfire.Event, error) {
	if strings.HasPrefix(event, "https://") {
		eventID, err := h.Campfire.ResolveEventID(ctx, event)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve event ID from URL %q: %w", event, err)
		}
		event = eventID
	}

	dbEvent, err := h.DB.GetEvent(ctx, event)
	if err == nil {
		var fullEvent *campfire.Event
		if err = json.Unmarshal(dbEvent.RawJSON, &fullEvent); err == nil {
			return fullEvent, nil
		}
	}

	return h.Campfire.GetEvent(ctx, event)
}

func (h *handler) renderRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "raffle.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle template", slog.Any("err", err))
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
