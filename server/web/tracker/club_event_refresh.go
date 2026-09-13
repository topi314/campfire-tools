package tracker

import (
	"context"
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
		if errors.Is(err, campfire.ErrEventNotFound) {
			dbEvent, dbErr := h.DB.GetEvent(ctx, eventID)
			if dbErr != nil {
				h.NotFound(w, r)
				return
			}
			if delErr := h.DB.DeleteEvent(ctx, eventID); delErr != nil {
				slog.ErrorContext(ctx, "Failed to delete not found event", slog.String("event_id", eventID), slog.Any("err", delErr))
				http.Error(w, "Failed to delete event: "+delErr.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/tracker/club/"+dbEvent.ClubID, http.StatusSeeOther)
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
