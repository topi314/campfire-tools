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

	meetupURLs := r.FormValue("urls")

	slog.InfoContext(ctx, "Received tracker add request", slog.String("url", r.URL.String()), slog.String("urls", meetupURLs))

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
			event, err := s.campfire.FetchEvent(ctx, meetupURL)
			if err != nil {
				return fmt.Errorf("failed to fetch event from URL %q: %w", meetupURL, err)
			}

			if event.Event.EventEndTime.After(now) {
				return fmt.Errorf("event has not ended yet: %s", event.Event.Name)
			}

			rawJSON, _ := json.Marshal(event)

			if err = s.db.AddEvent(ctx, database.Event{
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
				RawJSON:               rawJSON,
			}); err != nil {
				if errors.Is(err, database.ErrDuplicate) {
					return nil
				}
				return fmt.Errorf("failed to add event: %s", err.Error())
			}

			slog.InfoContext(ctx, "Event added", slog.String("name", event.Event.Name), slog.String("id", event.Event.ID))

			var members []database.Member
			for _, rsvpStatus := range event.Event.RSVPStatuses {
				member, _ := campfire.FindMember(rsvpStatus.UserID, *event)

				members = append(members, database.Member{
					ClubMember: database.ClubMember{
						ID:          rsvpStatus.UserID,
						Username:    member.Username,
						DisplayName: member.DisplayName,
						AvatarURL:   member.AvatarURL,
					},
					Status:  rsvpStatus.RSVPStatus,
					EventID: event.Event.ID,
				})
			}
			if err = s.db.AddMembers(ctx, members); err != nil {
				return fmt.Errorf("failed to add members: %w", err)
			}

			slog.InfoContext(ctx, "Members added for event", slog.String("name", event.Event.Name), slog.String("id", event.Event.ID), slog.Int("count", len(members)))
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

	slog.InfoContext(ctx, "Successfully added events and members", slog.Int("count", len(urls)))
	http.Redirect(w, r, "/tracker", http.StatusFound)
}
