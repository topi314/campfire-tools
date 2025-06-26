package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/internal/xstrconv"
)

type TrackerClubVars struct {
	ClubName        string
	ClubAvatarURL   string
	ClubID          string
	TopCounts       []int
	TopMembersCount int
	TopMembersOpen  bool
	TopMembers      []TopMember
	TopEventsCount  int
	TopEventsOpen   bool
	TopEvents       []TopEvent
	Events          []Event
}

type Member struct {
	ID          string
	Username    string
	DisplayName string
	AvatarURL   string
	URL         string
}

type TopMember struct {
	Member
	CheckIns int
}

type TopEvent struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
	Accepted      int
	CheckIns      int
}

type Event struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
}

func (s *Server) TrackerClub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
		parsedTopMembersOpen, err := xstrconv.ParseBool(topMembersOpenStr)
		if err == nil {
			topMembersOpen = parsedTopMembersOpen
		}
	}

	var topEventsOpen bool
	if topEventsOpenStr != "" {
		parsedTopEventsOpen, err := xstrconv.ParseBool(topEventsOpenStr)
		if err == nil {
			topEventsOpen = parsedTopEventsOpen
		}
	}

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	topMembers, err := s.db.GetTopClubMembers(ctx, clubID, membersCount)
	if err != nil {
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopMembers := make([]TopMember, len(topMembers))
	for i, member := range topMembers {
		trackerTopMembers[i] = TopMember{
			Member: Member{
				ID:          member.ID,
				Username:    member.Username,
				DisplayName: member.GetDisplayName(),
				AvatarURL:   imageURL(member.AvatarURL),
				URL:         fmt.Sprintf("/tracker/club/%s/member/%s", clubID, member.ID),
			},
			CheckIns: member.CheckIns,
		}
	}

	topEvents, err := s.db.GetTopClubEvents(ctx, clubID, eventsCount)
	if err != nil {
		http.Error(w, "Failed to fetch top events: "+err.Error(), http.StatusInternalServerError)
		return
	}
	trackerTopEvents := make([]TopEvent, len(topEvents))
	for i, event := range topEvents {
		trackerTopEvents[i] = TopEvent{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
			Accepted:      event.Accepted,
			CheckIns:      event.CheckIns,
		}
	}

	events, err := s.db.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]Event, len(events))
	for i, event := range events {
		trackerEvents[i] = Event{
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
		slog.ErrorContext(ctx, "Failed to render tracker club template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

func (s *Server) TrackerClubEventsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	events, err := s.db.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var exportEvents []json.RawMessage
	for _, event := range events {
		exportEvents = append(exportEvents, event.RawJSON)
	}

	if err = json.NewEncoder(w).Encode(exportEvents); err != nil {
		slog.ErrorContext(ctx, "Failed to write events export", slog.String("club_id", clubID), slog.Any("err", err))
		return
	}
}
