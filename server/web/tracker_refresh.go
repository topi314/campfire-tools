package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) TrackerRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	slog.InfoContext(ctx, "Received refresh request", slog.String("url", r.URL.String()))
	query := r.URL.Query()
	if query.Get("password") != h.Cfg.Auth.RefreshPassword {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	events, err := h.DB.GetAllEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get all events", slog.Any("err", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "Successfully retrieved all events", slog.Int("count", len(events)))
	var failed int
	for i, event := range events {
		if err = h.refreshEvent(ctx, event); err != nil {
			slog.ErrorContext(ctx, "Failed to refresh event", slog.String("event_id", event.ID), slog.Int("index", i+1), slog.Int("total", len(events)), slog.Any("err", err))
			failed++
			continue
		}
		slog.InfoContext(ctx, "Successfully refreshed event", slog.String("event_id", event.ID), slog.Int("index", i+1), slog.Int("total", len(events)))
		<-time.After(1 * time.Second)
	}
	if failed > 0 {
		slog.WarnContext(ctx, "Some events failed to refresh", slog.Int("failed_count", failed))
	}

	if _, err = fmt.Fprintf(w, "Refreshed %d events successfully, %d failed", len(events)-failed, failed); err != nil {
		slog.ErrorContext(ctx, "Failed to write refresh response", slog.Any("err", err))
		return
	}
}

func (h *handler) refreshEvent(ctx context.Context, oldEvent database.Event) error {
	event, err := h.Campfire.GetEvent(ctx, oldEvent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch full event: %w", err)
	}

	return h.ProcessFullEventImport(ctx, *event, false)
}
