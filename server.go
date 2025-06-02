package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"path"
	"slices"
	"strconv"
	"time"
)

func New() *Server {
	return &Server{
		Server: &http.Server{
			Addr: ":8080",
		},
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type Server struct {
	Server *http.Server
	Client *http.Client
}

func (s *Server) Start() {
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server failed: %s\n", err)
		}
	}()
}

func (s *Server) raffle(w http.ResponseWriter, r *http.Request) {
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

	eventID := path.Base(meetupURL)
	if eventID == "" {
		http.Error(w, "Invalid URL: "+meetupURL, http.StatusBadRequest)
		return
	}

	resp, err := s.FetchEvent(eventID)
	if err != nil {
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if resp == nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	status := resp.Data.Event.RSVPStatuses
	if len(status) == 0 {
		http.Error(w, "No RSVP statuses found for the event", http.StatusNotFound)
		return
	}

	// Shuffle members and select winners
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

		member, ok := s.findMemberName(st.UserID, resp)
		status = slices.Delete(status, num, num+1) // Remove selected member to avoid duplicates
		if !ok {
			continue
		}
		winners = append(winners, member)
	}
	if len(winners) == 0 {
		http.Error(w, "No valid winners found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	for i, winner := range winners {
		if i > 0 {
			_, _ = w.Write([]byte("\n"))
		}
		_, _ = fmt.Fprintf(w, "%d. %s", i+1, winner)
	}

	_, _ = w.Write([]byte("\n"))
	if len(winners) < count {
		_, _ = w.Write([]byte("Not enough members to select the requested number of winners.\n"))
	} else {
		_, _ = w.Write([]byte("Selected " + strconv.Itoa(count) + " winners successfully.\n"))
	}
	_, _ = w.Write([]byte("Thank you for using the raffle service!\n"))
}

func (s *Server) findMemberName(id string, resp *Resp) (string, bool) {
	for _, edge := range resp.Data.Event.Members.Edges {
		if edge.Node.ID == id {
			return edge.Node.DisplayName, true
		}
	}
	return "", false
}
