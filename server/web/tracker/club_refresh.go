package tracker

import (
	"context"
	"log/slog"
	"net/http"
)

func (h *handler) TrackerClubRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	clubID := r.PathValue("club_id")
	if err := h.ImportActiveClubEvents(ctx, clubID); err != nil {
		slog.ErrorContext(ctx, "Failed to refresh club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to refresh club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tracker/club/"+clubID, http.StatusSeeOther)
}
