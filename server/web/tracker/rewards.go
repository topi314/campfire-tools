package tracker

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardsVars struct {
	RewardPools []models.RewardPool
}

func (h *handler) TrackerRewards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	rewardPools, err := h.DB.GetRewardPools(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward pools", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward pools", http.StatusInternalServerError)
		return
	}

	trackerRewardPools := make([]models.RewardPool, len(rewardPools))
	for i, pool := range rewardPools {
		trackerRewardPools[i] = models.NewRewardPool(pool.RewardPool, 0, 0)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_rewards.gohtml", TrackerRewardsVars{
		RewardPools: trackerRewardPools,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}
