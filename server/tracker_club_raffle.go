package server

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

type TrackerClubRaffleVars struct {
	ClubName        string
	ClubAvatarURL   string
	ClubID          string
	Events          []Event
	SelectedEventID string
	Error           string
}

func (s *Server) TrackerClubRaffle(w http.ResponseWriter, r *http.Request) {
	s.renderTrackerClubRaffle(w, r, "")
}

func (s *Server) renderTrackerClubRaffle(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	eventID := query.Get("event")

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
		return
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

	if err = s.templates().ExecuteTemplate(w, "tracker_club_raffle.gohtml", TrackerClubRaffleVars{
		ClubName:        club.ClubName,
		ClubAvatarURL:   imageURL(club.ClubAvatarURL),
		ClubID:          club.ClubID,
		Events:          trackerEvents,
		SelectedEventID: eventID,
		Error:           errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club raffle template", slog.Any("err", err))
	}
}

func (s *Server) DoTrackerClubRaffle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form", slog.Any("err", err))
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	events := r.Form["events"]
	stringCount := r.FormValue("count")

	slog.InfoContext(ctx, "Received raffle request", slog.String("url", r.URL.String()), slog.String("event_ids", events), slog.String("count", stringCount))

	if len(events) == 0 {
		s.renderTrackerClubExport(w, r, "Missing 'events' parameter")
		return
	}

	count := 1
	if stringCount != "" {
		parsed, err := strconv.Atoi(stringCount)
		if err != nil || parsed <= 0 {
			s.renderRaffle(w, r, "Invalid 'count' parameter. It must be a positive number.")
			return
		}
		count = parsed
	}

	eg, ctx := errgroup.WithContext(ctx)
	var allMembers []Member
	for _, eventID := range events {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			continue
		}

		members, err := s.db.GetCheckedInMembersByEvent(ctx, eventID)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get event members", slog.String("id", eventID), slog.Any("err", err))
			continue
		}

		for _, member := range members {
			// Skip if the user is already in the members map
			if i := slices.IndexFunc(allMembers, func(m Member) bool {
				return m.ID == member.ID
			}); i != -1 {
				continue
			}

			allMembers = append(allMembers, Member{
				ID:          member.ID,
				Username:    member.Username,
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
			})
		}
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		s.renderRaffle(w, r, "Failed to fetch events: "+err.Error())
		return
	}

	winners := make([]Member, 0, count)
	for {
		if len(allMembers) == 0 || len(winners) >= count {
			break
		}
		num := rand.N(len(allMembers))
		member := allMembers[num]
		allMembers = slices.Delete(allMembers, num, num+1) // Remove selected member to avoid duplicates

		winners = append(winners, Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.DisplayName,
			AvatarURL:   imageURL(member.AvatarURL),
		})
	}

	if len(winners) == 0 {
		s.renderRaffle(w, r, "No winners found. Please check the event URL and ensure there are checked-in members.")
		return
	}

	if err := s.templates().ExecuteTemplate(w, "raffle_result.gohtml", DoRaffleVars{
		Winners: winners,
		URLs:    meetupURLs,
		Count:   count,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}
