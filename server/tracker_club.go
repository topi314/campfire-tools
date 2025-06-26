package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

type TrackerClubVars struct {
	ClubName        string
	ClubAvatarURL   string
	ClubID          string
	TopCounts       []int
	TopMembersCount int
	TopMembersOpen  bool
	TopMembers      []TrackerTopMember
	TopEventsCount  int
	TopEventsOpen   bool
	TopEvents       []TrackerTopEvent
	Events          []TrackerEvent
}

type TrackerMember struct {
	ID   string
	Name string
	URL  string
}

type TrackerTopMember struct {
	TrackerMember
	CheckIns int
}

type TrackerTopEvent struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
	RSVP          int
	CheckIns      int
}

type TrackerEvent struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
}

func (s *Server) TrackerClub(w http.ResponseWriter, r *http.Request) {
	clubID := r.PathValue("club_id")
	query := r.URL.Query()
	membersStr := query.Get("members")
	eventsStr := query.Get("events")
	topMembersOpenStr := query.Get("members-open")
	topEventsOpenStr := query.Get("events-open")

	membersCount := 10
	if membersStr != "" {
		parsedMembersCount, err := strconv.Atoi(membersStr)
		if err == nil {
			membersCount = parsedMembersCount
		}
	}

	eventsCount := 10
	if eventsStr != "" {
		parsedEventsCount, err := strconv.Atoi(eventsStr)
		if err == nil {
			eventsCount = parsedEventsCount
		}
	}

	var topMembersOpen bool
	if topMembersOpenStr != "" {
		parsedTopMembersOpen, err := strconv.ParseBool(topMembersOpenStr)
		if err == nil {
			topMembersOpen = parsedTopMembersOpen
		}
	}

	var topEventsOpen bool
	if topEventsOpenStr != "" {
		parsedTopEventsOpen, err := strconv.ParseBool(topEventsOpenStr)
		if err == nil {
			topEventsOpen = parsedTopEventsOpen
		}
	}

	club, err := s.db.GetClub(context.Background(), clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topMembers, err := s.db.GetTopClubMembers(context.Background(), clubID, membersCount)
	if err != nil {
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopMembers := make([]TrackerTopMember, len(topMembers))
	for i, member := range topMembers {
		trackerTopMembers[i] = TrackerTopMember{
			TrackerMember: TrackerMember{
				ID:   member.ID,
				Name: member.DisplayName,
				URL:  fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
			},
			CheckIns: member.CheckIns,
		}
	}

	topEvents, err := s.db.GetTopClubEvents(context.Background(), clubID, eventsCount)
	if err != nil {
		http.Error(w, "Failed to fetch top events: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopEvents := make([]TrackerTopEvent, len(topEvents))
	for i, event := range topEvents {
		trackerTopEvents[i] = TrackerTopEvent{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
			RSVP:          event.RSVP,
			CheckIns:      event.CheckIns,
		}
	}

	events, err := s.db.GetEvents(context.Background(), clubID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]TrackerEvent, len(events))
	for i, event := range events {
		trackerEvents[i] = TrackerEvent{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club.gohtml", TrackerClubVars{
		ClubName:        club.ClubName,
		ClubAvatarURL:   imageURL(club.ClubAvatarURL),
		ClubID:          club.ClubID,
		TopCounts:       []int{10, 25, 50, 75, 100},
		TopMembersCount: membersCount,
		TopMembersOpen:  topMembersOpen,
		TopMembers:      trackerTopMembers,
		TopEventsCount:  eventsCount,
		TopEventsOpen:   topEventsOpen,
		TopEvents:       trackerTopEvents,
		Events:          trackerEvents,
	}); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render tracker club template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}
