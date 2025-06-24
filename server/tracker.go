package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/tsync"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerVars struct {
	Clubs  []TrackerClub
	Errors []string
}

type TrackerClub struct {
	ID        string
	Name      string
	AvatarURL string
	URL       string
}

type TrackerClubVars struct {
	ClubName        string
	ClubAvatarURL   string
	ClubID          string
	TopMemberCounts []int
	TopMemberCount  int
	TopMembers      []TrackerTopMember
	Events          []TrackerEvent
	Error           string
}

type TrackerTopMember struct {
	ID          string
	DisplayName string
	EventCount  int
	URL         string
}

type TrackerEvent struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
}

func (s *Server) Tracker(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderTracker(w, r)
	case http.MethodPost:
		s.trackerAdd(w, r)
	}
}

func (s *Server) TrackerClub(w http.ResponseWriter, r *http.Request) {
	s.renderTrackerClub(w, r, "")
}

func (s *Server) trackerAdd(w http.ResponseWriter, r *http.Request) {
	meetupURLs := r.FormValue("urls")

	slog.InfoContext(r.Context(), "Received tracker add request", slog.String("url", r.URL.String()), slog.String("urls", meetupURLs))

	if meetupURLs == "" {
		s.renderTracker(w, r, "Missing 'urls' parameter")
		return
	}

	var errs []error
	urls := strings.Split(meetupURLs, "\n")
	if len(urls) > 50 {
		urls = urls[:50]
		errs = append(errs, fmt.Errorf("please limit the number of URLs to 50, got %d. Only the first 50 will be processed", len(urls)))
	}

	now := time.Now()
	var eg tsync.ErrorGroup
	for _, url := range urls {
		meetupURL := strings.TrimSpace(url)
		if meetupURL == "" {
			continue
		}

		eg.Go(func() error {
			event, err := s.client.FetchEvent(context.Background(), meetupURL)
			if err != nil {
				return fmt.Errorf("failed to fetch event from URL %q: %w", meetupURL, err)
			}

			if event.Event.EventEndTime.After(now) {
				return fmt.Errorf("event has not ended yet: %s", event.Event.Name)
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
				if errors.Is(err, database.ErrDuplicate) {
					return nil
				}
				return fmt.Errorf("failed to add event: %s", err.Error())
			}

			slog.InfoContext(r.Context(), "Event added", slog.String("name", event.Event.Name), slog.String("id", event.Event.ID))

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
				return fmt.Errorf("failed to add members: %w", err)
			}

			slog.InfoContext(r.Context(), "Members added for event", slog.String("name", event.Event.Name), slog.String("id", event.Event.ID), slog.Int("count", len(members)))
			return nil
		})
	}

	if errs = append(errs, eg.Wait()...); len(errs) > 0 {
		var errorMessages []string
		for _, err := range errs {
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
				slog.ErrorContext(r.Context(), "Failed to add event or members", "err", err)
			}
		}
		s.renderTracker(w, r, errorMessages...)
		return
	}

	http.Redirect(w, r, "/tracker", http.StatusFound)
}

func (s *Server) renderTracker(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
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
			AvatarURL: imageURL(club.ClubAvatarURL),
			URL:       fmt.Sprintf("/tracker/club/%s", club.ClubID),
		}
	}

	if err = s.templates.ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		Clubs:  trackerClubs,
		Errors: errorMessages,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render tracker template", "err", err)
	}
}

func (s *Server) renderTrackerClub(w http.ResponseWriter, r *http.Request, errorMessage string) {
	clubID := r.PathValue("club_id")
	query := r.URL.Query()
	topCountStr := query.Get("top_count")
	topCount := 10
	if topCountStr != "" {
		var err error
		topCount, err = strconv.Atoi(topCountStr)
		if err != nil || topCount <= 0 {
			http.Error(w, "Invalid top_count parameter: "+topCountStr, http.StatusBadRequest)
			return
		}
	}

	club, err := s.database.GetClub(context.Background(), clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topMembers, err := s.database.GetTopClubMembers(context.Background(), clubID, topCount)
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
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/events/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates.ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		ClubName:        club.ClubName,
		ClubAvatarURL:   imageURL(club.ClubAvatarURL),
		ClubID:          club.ClubID,
		TopMemberCounts: []int{10, 25, 50, 75, 100},
		TopMemberCount:  topCount,
		TopMembers:      trackerTopMembers,
		Events:          trackerEvents,
		Error:           errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
