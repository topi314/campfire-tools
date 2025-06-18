package server

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"

	"github.com/topi314/campfire-tools/server/campfire"
)

type RaffleVars struct {
	Winners   []string
	RaffleURL string
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	s.renderRaffle(w, "")
}

func (s *Server) RaffleResult(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received raffle request: %s", r.URL.Path)
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		s.renderRaffle(w, "Missing 'url' parameter. Please specify the event URL.")
		return
	}

	stringCount := r.FormValue("count")
	if stringCount == "" {
		s.renderRaffle(w, "Missing 'count' parameter. Please specify the number of winners to draw.")
		return
	}
	count, err := strconv.Atoi(stringCount)
	if err != nil || count <= 0 {
		s.renderRaffle(w, "Invalid 'count' parameter. It must be a positive number.")
		return
	}

	event, err := s.client.FetchEvent(meetupURL)
	if err != nil {
		s.renderRaffle(w, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderRaffle(w, fmt.Sprintf("Event not found or no checked-in members found"))
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
		s.renderRaffle(w, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err = s.templates.ExecuteTemplate(w, "raffle_result.gohtml", RaffleVars{
		Winners:   winners,
		RaffleURL: r.URL.Path + "?" + r.URL.RawQuery,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderRaffle(w http.ResponseWriter, errorMessage string) {
	if err := s.templates.ExecuteTemplate(w, "raffle.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
