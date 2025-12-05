package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardCodesVar struct {
	models.Reward
	Code          *models.RewardCode
	NextCodeURL   string
	RewardCodeURL string
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
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	codes, err := h.DB.GetRewardCodes(ctx, id, "unredeemed")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	var (
		trackerCode   *models.RewardCode
		nextCodeURL   string
		rewardCodeURL string
	)
	if len(codes) > 0 {
		code := codes[0]

		var redeemedBy *database.DiscordUser
		if code.RedeemedByUser.ID != nil {
			redeemedBy = &database.DiscordUser{
				ID:          *code.RedeemedByUser.ID,
				Username:    *code.RedeemedByUser.Username,
				DisplayName: *code.RedeemedByUser.DisplayName,
				AvatarURL:   *code.RedeemedByUser.AvatarURL,
			}
		}
		c := models.NewRewardCode(code.RewardCode, code.ImportedByUser, redeemedBy)
		trackerCode = &c
		nextCodeURL = fmt.Sprintf("/tracker/rewards/%d/codes/%d/next", reward.ID, trackerCode.ID)
		rewardCodeURL = models.RewardCodeURL(h.Cfg.Server.PublicRewardsURL, code.RedeemCode)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_codes.gohtml", TrackerRewardCodesVar{
		Reward:        models.NewReward(*reward),
		Code:          trackerCode,
		NextCodeURL:   nextCodeURL,
		RewardCodeURL: rewardCodeURL,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}

func (h *handler) TrackerRewardCodeNext(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	codeID, err := strconv.Atoi(r.PathValue("code_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if _, err = h.DB.GetReward(ctx, id, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	if err = h.DB.UpdateRewardCodeRedeemed(ctx, codeID, &now, &session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to mark reward code as used", slog.String("err", err.Error()))
		http.Error(w, "Failed to mark reward code as used", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d/codes", id), http.StatusSeeOther)
}
