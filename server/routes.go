package server

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var meetupURLRegex = regexp.MustCompile(`https://niantic-social.nianticlabs.com/public/meetup/[a-zA-Z0-9-]+`)

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, "")
}

func (s *Server) Export(w http.ResponseWriter, r *http.Request) {
	s.renderExport(w, "")
}

type RaffleVars struct {
	Winners   []string
	RaffleURL string
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received raffle request: %s", r.URL.Path)
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	stringCount := r.FormValue("count")
	if stringCount == "" {
		http.Error(w, "Missing 'count' parameter", http.StatusBadRequest)
		return
	}
	count, err := strconv.Atoi(stringCount)
	if err != nil || count <= 0 {
		http.Error(w, "Invalid 'count' parameter", http.StatusBadRequest)
		return
	}

	event, err := s.getEvent(meetupURL)
	if err != nil {
		s.renderIndex(w, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderIndex(w, fmt.Sprintf("Event not found or no checked-in members found"))
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

		member, ok := s.findMemberName(st.UserID, *event)
		status = slices.Delete(status, num, num+1) // Remove selected member to avoid duplicates
		if !ok {
			continue
		}
		winners = append(winners, member)
	}

	if len(winners) == 0 {
		s.renderIndex(w, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err = s.Templates.ExecuteTemplate(w, "raffle.gohtml", RaffleVars{
		Winners:   winners,
		RaffleURL: r.URL.Path + "?" + r.URL.RawQuery,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) ExportCSV(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received export request: %s", r.URL.Path)
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	includeMissingMembersStr := r.FormValue("include_missing_members")
	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := strconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			http.Error(w, "Invalid 'include_missing_members' parameter", http.StatusBadRequest)
			return
		}
		includeMissingMembers = parsed
	}

	event, err := s.getEvent(meetupURL)
	if err != nil {
		s.renderExport(w, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderExport(w, fmt.Sprintf("Event not found or no checked-in members found"))
		return
	}

	records := [][]string{
		{"id", "name", "status"},
	}
	for _, rsvpStatus := range event.Event.RSVPStatuses {
		member, ok := s.findMemberName(rsvpStatus.UserID, *event)
		if !ok && !includeMissingMembers {
			continue
		}

		records = append(records, []string{
			rsvpStatus.UserID,
			member,
			rsvpStatus.RSVPStatus,
		})
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
	if err = csv.NewWriter(w).WriteAll(records); err != nil {
		log.Printf("Failed to write CSV records: %s", err.Error())
		return
	}
}

func (s *Server) getEvent(meetupURL string) (*FullEvent, error) {
	var campfireEventID string
	if !strings.HasPrefix(meetupURL, "https://campfire.nianticlabs.com/discover/meetup/") {
		if strings.HasPrefix(meetupURL, "https://cmpf.re/") {
			var err error
			meetupURL, err = s.resolveShortURL(meetupURL)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve short URL: %w", err)
			}
		}

		if !strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup/") {
			return nil, errors.New("invalid URL. Must start with 'https://niantic-social.nianticlabs.com/public/meetup/' or 'https://cmpf.re/' or 'https://campfire.nianticlabs.com/discover/meetup/'")
		}
		eventID := path.Base(meetupURL)
		if eventID == "" {
			return nil, errors.New("could not extract event ID from URL")
		}

		events, err := s.FetchEvents(eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch event: %w", err)
		}

		if len(events.PublicMapObjectsByID) == 0 {
			return nil, errors.New("event not found")
		}

		firstEvent := events.PublicMapObjectsByID[0]

		if firstEvent.ID != eventID {
			return nil, fmt.Errorf("event ID mismatch: expected %s, got %s", campfireEventID, firstEvent.Event.ID)
		}
		campfireEventID = firstEvent.Event.ID
	} else {
		campfireEventID = path.Base(meetupURL)
	}
	if campfireEventID == "" {
		return nil, fmt.Errorf("invalid URL: %s", meetupURL)
	}

	event, err := s.FetchFullEvent(campfireEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event: %w", err)
	}

	return event, nil
}

func (s *Server) resolveShortURL(shortURL string) (string, error) {
	rs, err := s.Client.Get(shortURL)
	if err != nil {
		return "", fmt.Errorf("failed to resolve short URL: %w", err)
	}
	defer rs.Body.Close()
	if rs.StatusCode != http.StatusOK {
		return "", fmt.Errorf("short URL resolution failed with status: %s", rs.Status)
	}

	html, err := io.ReadAll(rs.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	url := meetupURLRegex.FindString(string(html))
	if url == "" {
		return "", fmt.Errorf("no valid meetup URL found in response")
	}
	return url, nil
}

func (s *Server) renderIndex(w http.ResponseWriter, errorMessage string) {
	if err := s.Templates.ExecuteTemplate(w, "index.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderExport(w http.ResponseWriter, errorMessage string) {
	if err := s.Templates.ExecuteTemplate(w, "export.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) findMemberName(id string, event FullEvent) (string, bool) {
	for _, edge := range event.Event.Members.Edges {
		if edge.Node.ID == id {
			return edge.Node.DisplayName, true
		}
	}
	return "", false
}
