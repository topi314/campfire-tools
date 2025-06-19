package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerVars struct {
	Clubs []TrackerClub
	Error string
}

type TrackerClub struct {
	ID        string
	Name      string
	AvatarURL string
	URL       string
}

type TrackerClubVars struct {
	TopMembers []TrackerTopMember
	Events     []TrackerEvent
	Error      string
}

type TrackerTopMember struct {
	ID          string
	DisplayName string
	EventCount  int
	URL         string
}

type TrackerEvent struct {
	ID   string
	Name string
	URL  string
}

func (s *Server) Tracker(w http.ResponseWriter, r *http.Request) {
	s.renderTracker(w, r, "")
}

func (s *Server) TrackerClub(w http.ResponseWriter, r *http.Request) {
	s.renderTrackerClub(w, r, "")
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
		s.renderTracker(w, r, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil {
		s.renderTracker(w, r, fmt.Sprintf("Event not found"))
		return
	}

	if err = s.database.AddEvent(context.Background(), database.Event{
		ID:                    event.Event.ID,
		Name:                  event.Event.Name,
		Details:               event.Event.Details,
		CoverPhotoURL:         event.Event.CoverPhotoURL,
		EventTime:             event.Event.EventTime,
		EventEndTime:          event.Event.EventEndTime,
		CampfireLiveEventID:   event.Event.CampfireLiveEventID,
		CampfireLiveEventName: event.Event.CampfireLiveEvent.EventName,
		ClubID:                event.Event.ClubID,
		ClubName:              event.Event.Club.Name,
		ClubAvatarURL:         event.Event.Club.AvatarURL,
	}); err != nil {
		s.renderTracker(w, r, fmt.Sprintf("Failed to add event: %s", err.Error()))
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
		s.renderTracker(w, r, fmt.Sprintf("Failed to add members: %s", err.Error()))
		return
	}

	log.Printf("Members added for event: %s (%s)", event.Event.Name, event.Event.ID)
	http.Redirect(w, r, "/tracker", http.StatusFound)
}

func (s *Server) renderTracker(w http.ResponseWriter, r *http.Request, errorMessage string) {
	clubs, err := s.database.GetClubs(context.Background())
	if err != nil {
		http.Error(w, "Failed to fetch clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerClubs := make([]TrackerClub, len(clubs))
	for i, club := range clubs {
		trackerClubs[i] = TrackerClub{
			ID:        club.ClubID,
			Name:      club.ClubName,
			AvatarURL: club.ClubAvatarURL,
			URL:       fmt.Sprintf("/tracker/club/%s", club.ClubID),
		}
	}

	if err = s.templates.ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		Clubs: trackerClubs,
		Error: errorMessage,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render tracker template", "err", err)
	}
}

func (s *Server) renderTrackerClub(w http.ResponseWriter, r *http.Request, errorMessage string) {
	clubID := r.PathValue("club_id")

	topMembers, err := s.database.GetTopClubMembers(context.Background(), clubID, 10)
	if err != nil {
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopMembers := make([]TrackerTopMember, len(topMembers))
	for i, member := range topMembers {
		trackerTopMembers[i] = TrackerTopMember{
			ID:          member.ID,
			DisplayName: member.DisplayName,
			EventCount:  member.EventCount,
			URL:         fmt.Sprintf("/tracker/club/%s/members/%s", clubID, member.ID),
		}
	}

	events, err := s.database.GetEvents(context.Background(), clubID)
	if err != nil {
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]TrackerEvent, len(events))
	for i, event := range events {
		trackerEvents[i] = TrackerEvent{
			ID:   event.ID,
			Name: event.Name,
			URL:  fmt.Sprintf("/tracker/events/%s", event.ID),
		}
	}

	if err = s.templates.ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		TopMembers: trackerTopMembers,
		Events:     trackerEvents,
		Error:      errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
