package server

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

type DoRaffleVars struct {
	Winners []Member
	Events  string
	Count   int
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	s.renderRaffle(w, r, "")
}

func (s *Server) DoRaffle(w http.ResponseWriter, r *http.Request) {
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
	stringCount := r.FormValue("count")

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("events", events), slog.String("count", stringCount))

	if events == "" {
		s.renderRaffle(w, r, "Missing 'events' parameter")
		return
	}

	count := 1
	if stringCount != "" {
		parsed, err := strconv.Atoi(stringCount)
		if err != nil || parsed <= 0 {
			s.renderRaffle(w, r, "Invalid 'count' parameter. It must be a positive number.")
			return
		}
		count = parsed
	}

	allEvents := strings.Split(events, "\n")
	for _, event := range allEvents {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}
		allEvents = append(allEvents, event)
	}
	if len(allEvents) > 50 {
		s.renderExport(w, r, fmt.Sprintf("please limit the number of events to 50, got %d.", len(allEvents)))
		return
	}

	eg, ctx := errgroup.WithContext(ctx)
	var members []Member
	var eventIDs []string
	var mu sync.Mutex
	for _, event := range allEvents {
		eg.Go(func() error {
			var (
				fullEvent *campfire.FullEvent
				err       error
			)

			if strings.HasPrefix(event, "https://") {
				fullEvent, err = s.campfire.FetchEvent(ctx, event)
			} else {
				fullEvent, err = s.campfire.FetchFullEvent(ctx, event)
			}
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", event, err)
			}

			if len(fullEvent.Event.RSVPStatuses) == 0 {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			for _, rsvpStatus := range fullEvent.Event.RSVPStatuses {
				// Only consider checked-in members
				if rsvpStatus.RSVPStatus != "CHECKED_IN" {
					continue
				}

				// Skip if the user is already in the members map
				if i := slices.IndexFunc(members, func(m Member) bool {
					return m.ID == rsvpStatus.UserID
				}); i != -1 {
					continue
				}

				// Skip if we don't have the member's information
				member, ok := campfire.FindMember(rsvpStatus.UserID, *fullEvent)
				if !ok {
					continue
				}

				members = append(members, Member{
					ID:          rsvpStatus.UserID,
					Username:    member.Username,
					DisplayName: member.DisplayName,
					AvatarURL:   member.AvatarURL,
				})
			}
			eventIDs = append(eventIDs, fullEvent.Event.ID)

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		s.renderRaffle(w, r, "Failed to fetch events: "+err.Error())
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

		winners = append(winners, Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.DisplayName,
			AvatarURL:   imageURL(member.AvatarURL),
		})
	}

	if len(winners) == 0 {
		s.renderRaffle(w, r, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err := s.templates().ExecuteTemplate(w, "raffle_result.gohtml", DoRaffleVars{
		Winners: winners,
		Events:  strings.Join(eventIDs, "\n"),
		Count:   count,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}

func (s *Server) renderRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	if err := s.templates().ExecuteTemplate(w, "raffle.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle template", slog.Any("err", err))
	}
}
