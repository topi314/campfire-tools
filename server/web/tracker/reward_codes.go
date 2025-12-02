package tracker

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardCodesVars struct {
	models.Reward
	Codes []RewardCode
}

func (h *handler) TrackerRewardCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	codes, err := h.DB.GetRewardCodes(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	trackerCodes := make([]RewardCode, len(codes))
	for i, code := range codes {
		var redeemedBy *database.DiscordUser
		if code.RedeemedByUser.ID != nil {
			redeemedBy = &database.DiscordUser{
				ID:          *code.RedeemedByUser.ID,
				Username:    *code.RedeemedByUser.Username,
				DisplayName: *code.RedeemedByUser.DisplayName,
				AvatarURL:   *code.RedeemedByUser.AvatarURL,
			}
		}
		trackerCodes[i] = newRewardCode(id, code.RewardCode, code.ImportedByUser, redeemedBy)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward.gohtml", TrackerRewardVars{
		Reward: models.NewReward(*reward, 0, 0),
		Codes:  trackerCodes,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}
