package web

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (h *handler) TrackerClubEventRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := r.PathValue("event_id")

	event, err := h.Campfire.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.bulkProcessEvents(context.WithoutCancel(ctx), []campfire.Event{*event}); err != nil {
		slog.ErrorContext(ctx, "Failed to process event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to process event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tracker/event/"+eventID, http.StatusSeeOther)
}
