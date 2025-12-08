package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

func (h *handler) APIImportEvents(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	slog.InfoContext(ctx, "Received API import events request", slog.String("url", r.URL.String()))

	var events []string
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(events) == 0 {
		http.Error(w, "Missing or empty events list", http.StatusBadRequest)
		return
	}

	if err := h.importAllEvents(ctx, events); err != nil {
		slog.ErrorContext(ctx, "Failed to import events", slog.Any("error", err))
		http.Error(w, "Failed to import events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "Successfully imported events", slog.Int("count", len(events)))
	w.WriteHeader(http.StatusNoContent)
}
