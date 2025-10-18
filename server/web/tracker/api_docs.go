package tracker

import (
	"log/slog"
	"net/http"
)

type APIDocsVars struct {
	BaseURL string
}

func (h *handler) APIDocs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "tracker_api_docs.gohtml", APIDocsVars{
		BaseURL: h.Cfg.Auth.PublicTrackerURL,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render API docs template", slog.Any("err", err))
		return
	}
}
