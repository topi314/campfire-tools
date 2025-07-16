package web

import (
	"log/slog"
	"net/http"
)

func (h *handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "index.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("error", err.Error()))
	}
}
