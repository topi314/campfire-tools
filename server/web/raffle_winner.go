package web

import (
	"log/slog"
	"net/http"
)

func (h *handler) RaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	if err := h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", DoRaffleVars{}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}
