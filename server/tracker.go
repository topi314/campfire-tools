package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerVars struct {
	Events []TrackerEvent
	Error  string
}

type TrackerEvent struct {
	ID   string
	Name string
	URL  string
}

func (s *Server) Tracker(w http.ResponseWriter, r *http.Request) {
	s.renderTracker(w, "")
}

func (s *Server) TrackerAdd(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received tracker add request: %s", r.URL.Path)
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	event, err := s.client.FetchEvent(meetupURL)
	if err != nil {
		s.renderTracker(w, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil {
		s.renderTracker(w, fmt.Sprintf("Event not found"))
		return
	}

	if err = s.database.AddEvent(context.Background(), event.Event.ID, event.Event.Name, event.Event.Details); err != nil {
		s.renderTracker(w, fmt.Sprintf("Failed to add event: %s", err.Error()))
		return
	}

	log.Printf("Event added: %s (%s)", event.Event.Name, event.Event.ID)

	var members []database.Member
	for _, rsvpStatus := range event.Event.RSVPStatuses {
		name, _ := campfire.FindMemberName(rsvpStatus.UserID, *event)

		members = append(members, database.Member{
			ID:          rsvpStatus.UserID,
			DisplayName: name,
			Status:      rsvpStatus.RSVPStatus,
			EventID:     event.Event.ID,
		})
	}
	if err = s.database.AddMembers(context.Background(), members); err != nil {
		s.renderTracker(w, fmt.Sprintf("Failed to add members: %s", err.Error()))
		return
	}

	log.Printf("Members added for event: %s (%s)", event.Event.Name, event.Event.ID)
	http.Redirect(w, r, "/tracker", http.StatusFound)
}

func (s *Server) renderTracker(w http.ResponseWriter, errorMessage string) {
	events, err := s.database.GetEvents(context.Background())
	if err != nil {
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]TrackerEvent, len(events))
	for i, event := range events {
		trackerEvents[i] = TrackerEvent{
			ID:   event.ID,
			Name: event.Name,
			URL:  fmt.Sprintf("/tracker/event/%s", event.ID),
		}
	}

	if err = s.templates.ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		Events: trackerEvents,
		Error:  errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
