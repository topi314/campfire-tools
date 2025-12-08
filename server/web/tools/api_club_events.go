package tools

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (h *handler) APIClubEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	slog.InfoContext(ctx, "Received API club events request", slog.String("url", r.URL.String()), slog.Any("club_id", clubID))

	if clubID == "" {
		http.Error(w, "Club ID is required", http.StatusBadRequest)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID, time.Time{}, time.Time{}, false, "")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get events for club", slog.Any("error", err), slog.String("club_id", clubID))
		http.Error(w, "Failed to get events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(events) == 0 {
		http.Error(w, "No events found for the specified club", http.StatusNotFound)
		return
	}

	var campfireEvents []campfire.Event
	for _, event := range events {
		var campfireEvent campfire.Event
		if err = json.Unmarshal(event.RawJSON, &campfireEvent); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal event", slog.Any("error", err), slog.String("event_id", event.ID))
			http.Error(w, "Failed to process event data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		campfireEvents = append(campfireEvents, campfireEvent)
	}

	exportAllEvents(ctx, w, campfireEvents)
}
