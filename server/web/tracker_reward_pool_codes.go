package web

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerRewardPoolCodesVars struct {
	RewardPool
	Codes []RewardCode
}

func (h *handler) TrackerRewardPoolCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	poolID, err := strconv.Atoi(r.PathValue("pool_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	rewardPool, err := h.DB.GetRewardPool(ctx, poolID, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward pools", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward pools", http.StatusInternalServerError)
		return
	}

	codes, err := h.DB.GetRewardCodes(ctx, poolID)
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
		trackerCodes[i] = newRewardCode(code.RewardCode, code.ImportedByUser, redeemedBy)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_pool.gohtml", TrackerRewardPoolVars{
		RewardPool: newRewardPool(*rewardPool, 0, 0),
		Codes:      trackerCodes,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}
