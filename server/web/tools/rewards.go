package tools

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardsVars struct {
	Rewards []models.Reward
}

func (h *handler) TrackerRewards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	// rewards, err := h.DB.GetRewards(ctx, session.UserID)
	rewards, err := h.DB.GetRewards(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get rewards", slog.String("err", err.Error()))
		http.Error(w, "Failed to get rewards", http.StatusInternalServerError)
		return
	}

	trackerRewards := make([]models.Reward, len(rewards))
	for i, reward := range rewards {
		trackerRewards[i] = models.NewReward(reward)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_rewards.gohtml", TrackerRewardsVars{
		Rewards: trackerRewards,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}
