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
	URLs    string
	Count   int
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	s.renderRaffle(w, r, "")
}

func (s *Server) DoRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meetupURLs := r.FormValue("urls")
	stringCount := r.FormValue("count")

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("meetup_urls", meetupURLs), slog.String("count", stringCount))

	if meetupURLs == "" {
		s.renderRaffle(w, r, "Missing 'url' parameter. Please specify the event URL.")
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

	urls := strings.Split(meetupURLs, "\n")
	if len(urls) > 50 {
		s.renderExport(w, r, fmt.Sprintf("please limit the number of URLs to 50, got %d.", len(urls)))
		return
	}

	eg, ctx := errgroup.WithContext(ctx)
	var members []Member
	var mu sync.Mutex
	for _, url := range urls {
		meetupURL := strings.TrimSpace(url)
		if meetupURL == "" {
			continue
		}

		eg.Go(func() error {
			event, err := s.campfire.FetchEvent(ctx, meetupURL)
			if err != nil {
				return fmt.Errorf("failed to fetch event from URL %s: %w", meetupURL, err)
			}

			if len(event.Event.RSVPStatuses) == 0 {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			for _, rsvpStatus := range event.Event.RSVPStatuses {
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
				member, ok := campfire.FindMember(rsvpStatus.UserID, *event)
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
		URLs:    meetupURLs,
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
