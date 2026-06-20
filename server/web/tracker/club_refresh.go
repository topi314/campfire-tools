package tracker

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) TrackerClubRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	club, err := h.Campfire.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.DB.InsertMembers(ctx, []database.Member{{
		ID:          club.Creator.ID,
		Username:    club.Creator.Username,
		DisplayName: club.Creator.DisplayName,
		AvatarURL:   club.Creator.AvatarURL,
		RawJSON:     club.Creator.Raw,
	}}); err != nil {
		slog.ErrorContext(ctx, "Failed to insert club creator", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to insert club creator: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.DB.InsertClubs(ctx, []database.Club{{
		ID:                           club.ID,
		Name:                         club.Name,
		AvatarURL:                    club.AvatarURL,
		CreatorID:                    club.Creator.ID,
		CreatedByCommunityAmbassador: club.CreatedByCommunityAmbassador,
		RawJSON:                      club.Raw,
	}}); err != nil {
		slog.ErrorContext(ctx, "Failed to insert club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to insert club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tracker/club/"+clubID, http.StatusSeeOther)
}
