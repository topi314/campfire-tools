package web

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
)

func (h *handler) TrackerClubEventExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	event, err := h.DB.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(event.RawJSON); err != nil {
		slog.ErrorContext(ctx, "Failed to write event export", slog.String("event_id", eventID), slog.Any("err", err))
		return
	}
}
