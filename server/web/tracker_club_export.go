package web

import (
	"fmt"
	"log/slog"
	"net/http"
)

type TrackerClubExportVars struct {
	Club
	Events []Event
	Error  string
}

func (h *handler) TrackerClubExport(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubExport(w, r, "")
}

func (h *handler) renderTrackerClubExport(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]Event, len(events))
	for i, event := range events {
		trackerEvents[i] = Event{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL, 32),
		}
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_export.gohtml", TrackerClubExportVars{
		Club:   newClub(*club),
		Events: trackerEvents,
		Error:  errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club export template", slog.Any("err", err))
	}
}
