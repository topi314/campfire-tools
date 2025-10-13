package tracker

import (
	"log/slog"
	"net/http"
)

func (h *handler) TrackerIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "tracker_index.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("error", err.Error()))
	}
}
