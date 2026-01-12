package homepage

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
)

func (h *handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	homepage, err := h.GetHomepage(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.Templates().ExecuteTemplate(w, "homepage_index.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}
