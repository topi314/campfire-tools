package tracker

import (
	"log/slog"
	"net/http"
)

type TrackerCodeVars struct {
	Code string
}

func (h *handler) TrackerCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PathValue("code")

	if err := h.Templates().ExecuteTemplate(w, "tracker_code.gohtml", TrackerCodeVars{
		Code: code,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("error", err.Error()))
	}
}

func (h *handler) PostTrackerCode(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()
	// code := r.PathValue("code")

}
