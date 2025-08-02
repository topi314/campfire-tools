package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/tsync"
	"github.com/topi314/campfire-tools/internal/xerrors"
	"github.com/topi314/campfire-tools/server/auth"
)

type TrackerVars struct {
	PinnedClub *ClubWithEvents
	Clubs      []ClubWithEvents
	Errors     []string
}

func (h *handler) Tracker(w http.ResponseWriter, r *http.Request) {
	h.renderTracker(w, r)
}

func (h *handler) renderTracker(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()

	clubs, err := h.DB.GetClubs(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch clubs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)

	var pinnedClub *ClubWithEvents
	trackerClubs := make([]ClubWithEvents, len(clubs))
	for i, club := range clubs {
		if session.PinnedClubID != nil && *session.PinnedClubID == club.Club.ID {
			c := newClubWithEvents(club)
			pinnedClub = &c
			continue
		}
		trackerClubs[i] = newClubWithEvents(club)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker.gohtml", TrackerVars{
		PinnedClub: pinnedClub,
		Clubs:      trackerClubs,
		Errors:     errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker template", slog.Any("err", err))
	}
}

func (h *handler) TrackerAdd(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	events := strings.TrimSpace(r.FormValue("events"))

	slog.InfoContext(ctx, "Received tracker add request", slog.String("url", r.URL.String()), slog.String("events", events))

	if events == "" {
		h.renderTracker(w, r, "Missing 'events' parameter")
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

	if err := h.importAllEvents(ctx, allEvents); err != nil {
		errs = append(errs, xerrors.Unwrap(err)...)
	}

	if len(errs) > 0 {
		var errorMessages []string
		for _, err := range errs {
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
				slog.ErrorContext(ctx, "Failed to add event or members", "err", err)
			}
		}
		h.renderTracker(w, r, errorMessages...)
		return
	}

	slog.InfoContext(ctx, "Successfully added events and members", slog.Int("count", len(allEvents)))
	http.Redirect(w, r, "/tracker", http.StatusFound)
}

func (h *handler) importAllEvents(ctx context.Context, allEvents []string) error {
	now := time.Now()
	var eg tsync.ErrorGroup
	for _, eventID := range allEvents {
		eg.Go(func() error {
			event, err := h.fetchEvent(ctx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", eventID, err)
			}

			if event.EventEndTime.After(now) {
				return fmt.Errorf("event has not ended yet: %s", event.Name)
			}

			if err = h.processEvent(ctx, *event); err != nil {
				return fmt.Errorf("failed to process event %q: %w", eventID, err)
			}

			return nil
		})
	}

	return eg.Wait()
}
