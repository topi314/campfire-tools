package server

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/topi314/campfire-tools/server/campfire"
)

type DoRaffleVars struct {
	Winners []string
	URL     string
	Count   int
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	s.renderRaffle(w, r, "")
}

func (s *Server) DoRaffle(w http.ResponseWriter, r *http.Request) {
	meetupURL := strings.TrimSpace(r.FormValue("url"))
	stringCount := r.FormValue("count")

	slog.InfoContext(r.Context(), "Received raffle request", slog.String("url", r.URL.String()), slog.String("meetup_url", meetupURL), slog.String("count", stringCount))

	if meetupURL == "" {
		s.renderRaffle(w, r, "Missing 'url' parameter. Please specify the event URL.")
		return
	}

	if stringCount == "" {
		s.renderRaffle(w, r, "Missing 'count' parameter. Please specify the number of winners to draw.")
		return
	}
	count, err := strconv.Atoi(stringCount)
	if err != nil || count <= 0 {
		s.renderRaffle(w, r, "Invalid 'count' parameter. It must be a positive number.")
		return
	}

	event, err := s.client.FetchEvent(r.Context(), meetupURL)
	if err != nil {
		s.renderRaffle(w, r, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderRaffle(w, r, fmt.Sprintf("Event not found or no checked-in members found"))
		return
	}

	status := event.Event.RSVPStatuses
	winners := make([]string, 0, count)
	for {
		if len(status) == 0 || len(winners) >= count {
			break
		}
		num := rand.N(len(status))

		st := status[num]
		if st.RSVPStatus != "CHECKED_IN" {
			status = slices.Delete(status, num, num+1) // Remove non-checked-in member
			continue
		}

		member, ok := campfire.FindMemberName(st.UserID, *event)
		status = slices.Delete(status, num, num+1) // Remove selected member to avoid duplicates
		if !ok {
			continue
		}
		winners = append(winners, member)
	}

	if len(winners) == 0 {
		s.renderRaffle(w, r, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err = s.templates().ExecuteTemplate(w, "raffle_result.gohtml", DoRaffleVars{
		Winners: winners,
		URL:     meetupURL,
		Count:   count,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render raffle result template", slog.Any("err", err))
	}
}

func (s *Server) renderRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	if err := s.templates().ExecuteTemplate(w, "raffle.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render raffle template", slog.Any("err", err))
	}
}
