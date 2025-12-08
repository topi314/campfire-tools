package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/topi314/campfire-tools/internal/tsync"
	"github.com/topi314/campfire-tools/internal/xerrors"
	"github.com/topi314/campfire-tools/server/campfire"
)

type TrackerEventImportVars struct {
	SelectedEventID string
	Errors          []string
}

func (h *handler) TrackerEventImport(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerEventImport(w, r)
}

func (h *handler) renderTrackerEventImport(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()
	query := r.URL.Query()

	selected := query.Get("events")
	if selected == "" {
		selected = r.FormValue("events")
	}

	if err := h.Templates().ExecuteTemplate(w, "tracker_event_import.gohtml", TrackerEventImportVars{
		SelectedEventID: selected,
		Errors:          errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker import template", slog.Any("err", err))
	}
}

func (h *handler) TrackerEventDoImport(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	events := strings.TrimSpace(r.FormValue("events"))

	slog.InfoContext(ctx, "Received tracker add request", slog.String("url", r.URL.String()), slog.String("events", events))

	if events == "" {
		h.renderTrackerEventImport(w, r, "Missing 'events' parameter")
		return
	}

	allEvents := strings.FieldsFunc(events, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\r' || r == ';'
	})

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
		h.renderTrackerEventImport(w, r, errorMessages...)
		return
	}

	slog.InfoContext(ctx, "Successfully added events and members", slog.Int("count", len(allEvents)))
	http.Redirect(w, r, "/tracker", http.StatusFound)
}

func (h *handler) importAllEvents(ctx context.Context, eventIDs []string) error {
	var (
		events []campfire.Event
		mu     sync.Mutex
	)

	var eg tsync.ErrorGroup
	for _, eventID := range eventIDs {
		eg.Go(func() error {
			event, err := h.fetchEvent(ctx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", eventID, err)
			}
			if event.ID == "" {
				return nil
			}

			mu.Lock()
			events = append(events, *event)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	if len(events) == 0 {
		return errors.New("no valid events to import")
	}

	slog.InfoContext(ctx, "Fetched all events", slog.Int("count", len(events)))

	return h.bulkProcessEvents(ctx, events)
}
