package server

import (
	"fmt"
	"io"
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
	s.renderIndex(w, r, "")
}

type RaffleVars struct {
	Winners   []string
	RaffleURL string
}

func (s *Server) Raffle(w http.ResponseWriter, r *http.Request) {
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	if strings.HasPrefix(meetupURL, "https://cmpf.re/") {
		var err error
		meetupURL, err = s.resolveShortURL(meetupURL)
		if err != nil {
			s.renderIndex(w, r, fmt.Sprintf("Failed to resolve short URL: %s", err.Error()))
			return
		}
	}

	if !strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup/") {
		s.renderIndex(w, r, "Invalid URL. Must start with 'niantic-social.nianticlabs.com/public/meetup/' or 'https://cmpf.re/'")
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

	campfireEventID := path.Base(meetupURL)
	if campfireEventID == "" {
		s.renderIndex(w, r, fmt.Sprintf("Invalid URL: %s", meetupURL))
		return
	}

	events, err := s.FetchEvents(campfireEventID)
	if err != nil {
		s.renderIndex(w, r, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if len(events.PublicMapObjectsByID) == 0 {
		s.renderIndex(w, r, fmt.Sprintf("Event not found"))
		return
	}

	firstEvent := events.PublicMapObjectsByID[0]

	if firstEvent.ID != campfireEventID {
		s.renderIndex(w, r, fmt.Sprintf("Event ID mismatch: expected %s, got %s", campfireEventID, firstEvent.Event.ID))
		return
	}

	event, err := s.FetchFullEvent(firstEvent.Event.ID)
	if err != nil {
		s.renderIndex(w, r, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderIndex(w, r, fmt.Sprintf("Event not found or no checked-in members found"))
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
		s.renderIndex(w, r, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err = s.Templates.ExecuteTemplate(w, "raffle.gohtml", RaffleVars{
		Winners:   winners,
		RaffleURL: r.URL.Path + "?" + r.URL.RawQuery,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
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

func (s *Server) renderIndex(w http.ResponseWriter, r *http.Request, errorMessage string) {
	if err := s.Templates.ExecuteTemplate(w, "index.gohtml", map[string]any{
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
