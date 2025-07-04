package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

func (s *Server) Tracker(w http.ResponseWriter, r *http.Request) {
	s.renderTracker(w, r)
}

func (s *Server) renderTracker(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()

	clubs, err := s.db.GetClubs(ctx)
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

	if err = s.templates().ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		Clubs:  trackerClubs,
		Errors: errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker template", slog.Any("err", err))
	}
}

func (s *Server) TrackerAdd(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	events := strings.TrimSpace(r.FormValue("events"))

	slog.InfoContext(ctx, "Received tracker add request", slog.String("url", r.URL.String()), slog.String("events", events))

	if events == "" {
		s.renderTracker(w, r, "Missing 'events' parameter")
		return
	}

	var allEvents []string
	for _, event := range strings.Split(events, "\n") {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}
		allEvents = append(allEvents, event)
	}

	var errs []error
	if len(allEvents) > 50 {
		errs = append(errs, fmt.Errorf("please limit the number of events to 50, got %d. Only the first 50 will be processed", len(allEvents)))
		allEvents = allEvents[:50]
	}

	now := time.Now()
	var eg tsync.ErrorGroup
	for _, event := range allEvents {
		eg.Go(func() error {
			var (
				fullEvent *campfire.FullEvent
				err       error
			)

			if strings.HasPrefix(event, "https://") {
				fullEvent, err = s.campfire.FetchEvent(ctx, event)
			} else {
				fullEvent, err = s.fetchFullEvent(ctx, event)
			}
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", event, err)
			}

			if len(fullEvent.Event.RSVPStatuses) == 0 {
				return nil
			}

			if fullEvent.Event.EventEndTime.After(now) {
				return fmt.Errorf("event has not ended yet: %s", fullEvent.Event.Name)
			}

			rawJSON, _ := json.Marshal(event)

			if err = s.db.AddEvent(ctx, database.Event{
				ID:                    fullEvent.Event.ID,
				Name:                  fullEvent.Event.Name,
				Details:               fullEvent.Event.Details,
				CoverPhotoURL:         fullEvent.Event.CoverPhotoURL,
				EventTime:             fullEvent.Event.EventTime,
				EventEndTime:          fullEvent.Event.EventEndTime,
				CampfireLiveEventID:   fullEvent.Event.CampfireLiveEventID,
				CampfireLiveEventName: fullEvent.Event.CampfireLiveEvent.EventName,
				ClubID:                fullEvent.Event.ClubID,
				ClubName:              fullEvent.Event.Club.Name,
				ClubAvatarURL:         fullEvent.Event.Club.AvatarURL,
				RawJSON:               rawJSON,
			}); err != nil {
				if errors.Is(err, database.ErrDuplicate) {
					return nil
				}
				return fmt.Errorf("failed to add event: %s", err.Error())
			}

			slog.InfoContext(ctx, "Event added", slog.String("name", fullEvent.Event.Name), slog.String("id", fullEvent.Event.ID))

			var members []database.Member
			for _, rsvpStatus := range fullEvent.Event.RSVPStatuses {
				member, _ := campfire.FindMember(rsvpStatus.UserID, *fullEvent)

				members = append(members, database.Member{
					ClubMember: database.ClubMember{
						ID:          rsvpStatus.UserID,
						Username:    member.Username,
						DisplayName: member.DisplayName,
						AvatarURL:   member.AvatarURL,
					},
					Status:  rsvpStatus.RSVPStatus,
					EventID: fullEvent.Event.ID,
				})
			}
			if err = s.db.AddMembers(ctx, members); err != nil {
				return fmt.Errorf("failed to add members: %w", err)
			}

			slog.InfoContext(ctx, "Members added for event", slog.String("name", fullEvent.Event.Name), slog.String("id", fullEvent.Event.ID), slog.Int("count", len(members)))
			return nil
		})
	}

	if errs = append(errs, eg.Wait()...); len(errs) > 0 {
		var errorMessages []string
		for _, err := range errs {
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
				slog.ErrorContext(ctx, "Failed to add event or members", "err", err)
			}
		}
		s.renderTracker(w, r, errorMessages...)
		return
	}

	slog.InfoContext(ctx, "Successfully added events and members", slog.Int("count", len(allEvents)))
	http.Redirect(w, r, "/tracker", http.StatusFound)
}
