package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (h *handler) TrackerMigrate(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	slog.InfoContext(ctx, "Received migrate request", slog.String("url", r.URL.String()))
	query := r.URL.Query()
	if query.Get("password") != h.Cfg.Auth.RefreshPassword {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slog.InfoContext(ctx, "Starting migration process")
	events, err := h.DB.GetOldEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get old events", slog.Any("err", err))
		http.Error(w, "Failed to get old events", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "Retrieved old events", slog.Int("count", len(events)))

	for i, oldEvent := range events {
		slog.InfoContext(ctx, "Migrating event", slog.Int("index", i), slog.String("event_id", oldEvent.ID))
		var event campfire.Event
		if err = json.Unmarshal(oldEvent.RawJSON, &event); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal event", slog.Any("err", err), slog.String("event_id", oldEvent.ID))
			http.Error(w, "Failed to unmarshal event", http.StatusInternalServerError)
			return
		}
		if err = h.processEvent(ctx, event); err != nil {
			slog.ErrorContext(ctx, "Failed to process event", slog.Any("err", err), slog.String("event_id", oldEvent.ID))
			http.Error(w, "Failed to process event", http.StatusInternalServerError)
			return
		}
	}

	slog.InfoContext(ctx, "Migration process completed successfully")
	if _, err = w.Write([]byte("Migration completed successfully")); err != nil {
		slog.ErrorContext(ctx, "Failed to write migration response", slog.Any("err", err))
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}
