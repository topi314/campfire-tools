package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) TrackerClubPin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	_, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)
	if session.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var pinnedClubID *string
	if strings.HasSuffix(r.URL.Path, "/pin") {
		pinnedClubID = &clubID
	}

	if err = h.DB.SetUserSetting(ctx, database.UserSetting{
		UserID:       session.UserID,
		PinnedClubID: pinnedClubID,
	}); err != nil {
		http.Error(w, "Failed to set pinned club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/club/%s", clubID), http.StatusSeeOther)
}
