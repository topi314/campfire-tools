package rewards

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/web/models"
)

type RedeemVars struct {
	Code *models.RewardCode
}

func (h *handler) Redeem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	code := query.Get("code")

	rewardCode, err := h.DB.GetRewardCodeByRedeemCodeAndIncreaseVisitedCount(ctx, code)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.ErrorContext(ctx, "Failed to get reward by redeem code", slog.String("err", err.Error()))
		http.Error(w, "Invalid redeem code", http.StatusBadRequest)
		return
	}

	var rewardsRewardCode *models.RewardCode
	if rewardCode != nil {
		t := models.NewRewardCode(rewardCode.RewardCode, rewardCode.ImportedByUser, nil)
		rewardsRewardCode = &t
	}

	if err = h.Templates().ExecuteTemplate(w, "rewards_redeem.gohtml", RedeemVars{
		Code: rewardsRewardCode,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}
