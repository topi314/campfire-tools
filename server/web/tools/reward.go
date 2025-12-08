package tools

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardVars struct {
	models.Reward
	Codes  []models.RewardCode
	URL    string
	Filter string
}

func (h *handler) TrackerReward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("reward_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}
	filter := query.Get("filter")
	if filter == "" {
		filter = "unredeemed"
	}

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	codes, err := h.DB.GetRewardCodes(ctx, id, filter)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	trackerCodes := make([]models.RewardCode, len(codes))
	for i, code := range codes {
		trackerCodes[i] = models.NewRewardCode(code.RewardCode, code.ImportedByUser, code.RedeemedByUser, code.ReservedByUser)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward.gohtml", TrackerRewardVars{
		Reward: models.NewReward(*reward),
		Codes:  trackerCodes,
		URL:    fmt.Sprintf("/tracker/rewards/%d", id),
		Filter: filter,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}
