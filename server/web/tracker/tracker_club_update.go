package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
)

func (h *handler) TrackerClubUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	if session.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	clubID := r.PathValue("club_id")
	pinned := parseBoolQuery(r.Form, "pinned", false)
	autoEventImport := parseBoolQuery(r.Form, "auto_event_import", false)

	_, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pinned {
		if err = h.DB.AddDiscordUserPinnedClub(ctx, session.UserID, clubID); err != nil {
			slog.ErrorContext(ctx, "Failed to pin club for user", slog.String("user_id", session.UserID), slog.String("club_id", clubID), slog.Any("err", err))
			http.Error(w, "Failed to pin club", http.StatusInternalServerError)
			return
		}
	} else {
		if err = h.DB.RemoveDiscordUserPinnedClub(ctx, session.UserID, clubID); err != nil {
			slog.ErrorContext(ctx, "Failed to unpin club for user", slog.String("user_id", session.UserID), slog.String("club_id", clubID), slog.Any("err", err))
			http.Error(w, "Failed to unpin club", http.StatusInternalServerError)
			return
		}
	}

	if err = h.DB.UpdateClubAutoEventImport(ctx, clubID, autoEventImport); err != nil {
		http.Error(w, "Failed to update club settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/club/%s", clubID), http.StatusSeeOther)
}
