package web

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerRewardsVars struct {
	RewardPools []RewardPool
}

func newRewardPool(pool database.RewardPool, usedCodes int, totalCodes int) RewardPool {
	return RewardPool{
		ID:          pool.ID,
		URL:         fmt.Sprintf("/tracker/reward-pool/%d", pool.ID),
		CodesURL:    fmt.Sprintf("/tracker/reward-pool/%d/codes", pool.ID),
		Name:        pool.Name,
		Description: pool.Description,
		UsedCodes:   usedCodes,
		TotalCodes:  totalCodes,
	}
}

type RewardPool struct {
	ID          int
	URL         string
	CodesURL    string
	Name        string
	Description string
	UsedCodes   int
	TotalCodes  int
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

	trackerRewardPools := make([]RewardPool, len(rewardPools))
	for i, pool := range rewardPools {
		trackerRewardPools[i] = newRewardPool(pool.RewardPool, 0, 0)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_rewards.gohtml", TrackerRewardsVars{
		RewardPools: trackerRewardPools,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}
