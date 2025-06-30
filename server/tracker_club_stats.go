package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/internal/xstrconv"
)

type TrackerClubStatsVars struct {
	ClubName      string
	ClubAvatarURL string
	ClubID        string

	TopCounts       []int
	TopMembersCount int
	TopMembersOpen  bool
	TopMembers      []TopMember
	TopEventsCount  int
	TopEventsOpen   bool
	TopEvents       []TopEvent
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
	Event
	Accepted int
	CheckIns int
}

func (s *Server) TrackerClubStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")
	query := r.URL.Query()
	membersStr := query.Get("members")
	eventsStr := query.Get("events")
	topMembersClosedStr := query.Get("members-closed")
	topEventsClosedStr := query.Get("events-closed")

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

	var topMembersClosed bool
	if topMembersClosedStr != "" {
		parsedTopMembersClosed, err := xstrconv.ParseBool(topMembersClosedStr)
		if err == nil {
			topMembersClosed = parsedTopMembersClosed
		}
	}

	var topEventsClosed bool
	if topEventsClosedStr != "" {
		parsedTopEventsClosed, err := xstrconv.ParseBool(topEventsClosedStr)
		if err == nil {
			topEventsClosed = parsedTopEventsClosed
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
			Event: Event{
				ID:            event.ID,
				Name:          event.Name,
				URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
				CoverPhotoURL: imageURL(event.CoverPhotoURL),
			},
			Accepted: event.Accepted,
			CheckIns: event.CheckIns,
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_stats.gohtml", TrackerClubStatsVars{
		ClubName:        club.ClubName,
		ClubAvatarURL:   imageURL(club.ClubAvatarURL),
		ClubID:          club.ClubID,
		TopCounts:       []int{10, 25, 50, 75, 100},
		TopMembersCount: membersCount,
		TopMembersOpen:  !topMembersClosed,
		TopMembers:      trackerTopMembers,
		TopEventsCount:  eventsCount,
		TopEventsOpen:   !topEventsClosed,
		TopEvents:       trackerTopEvents,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club stats template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}
