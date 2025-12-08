package tools

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardCodesVar struct {
	models.Reward
	Code          *models.RewardCode
	NextCodeURL   string
	RewardCodeURL string
	ReserveURL    string
	ReservedUntil time.Time
}

func (h *handler) TrackerRewardCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("reward_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	code, err := h.DB.GetNextRewardCode(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	var (
		trackerCode   *models.RewardCode
		nextCodeURL   string
		rewardCodeURL string
		reserveURL    string
	)
	if code != nil {
		if err = h.DB.ReserveRewardCode(ctx, code.ID, session.UserID); err != nil {
			slog.ErrorContext(ctx, "Failed to reserve reward code", slog.String("err", err.Error()))
			http.Error(w, "Failed to reserve reward code", http.StatusInternalServerError)
			return
		}

		code, err = h.DB.GetRewardCode(ctx, code.ID)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get reward code", slog.String("err", err.Error()))
			http.Error(w, "Failed to get reward code", http.StatusInternalServerError)
			return
		}

		c := models.NewRewardCode(code.RewardCode, code.ImportedByUser, code.RedeemedByUser, code.ReservedByUser)
		trackerCode = &c
		nextCodeURL = fmt.Sprintf("/tracker/rewards/%d/codes/%d/next", reward.ID, trackerCode.ID)
		rewardCodeURL = models.RewardCodeURL(h.Cfg.Server.PublicRewardsURL, code.RedeemCode)
		reserveURL = fmt.Sprintf("/tracker/rewards/%d/codes/%d/reserve", reward.ID, trackerCode.ID)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_codes.gohtml", TrackerRewardCodesVar{
		Reward:        models.NewReward(*reward),
		Code:          trackerCode,
		NextCodeURL:   nextCodeURL,
		RewardCodeURL: rewardCodeURL,
		ReserveURL:    reserveURL,
		ReservedUntil: time.Now().Add(time.Minute),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}

func (h *handler) TrackerRewardCodeNext(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("reward_id"))
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

type TrackerRewardCodeReserveVar struct {
	ReservedBy    models.DiscordUser
	ReservedUntil time.Time
}

func (h *handler) PostTrackerRewardCodeReserve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("reward_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if _, err = h.DB.GetReward(ctx, id, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	codeID, err := strconv.Atoi(r.PathValue("code_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	rewardCode, err := h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward code", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward code", http.StatusInternalServerError)
		return
	}
	if rewardCode.RedeemedAt != nil {
		http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d/codes", id), http.StatusSeeOther)
		return
	}

	if err = h.DB.ReserveRewardCode(ctx, codeID, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to reserve reward code", slog.String("err", err.Error()))
		http.Error(w, "Failed to reserve reward code", http.StatusInternalServerError)
		return
	}

	rewardCode, err = h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward code", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward code", http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_code_reserve.gohtml", TrackerRewardCodeReserveVar{
		ReservedBy:    *models.NewOptDiscordUser(rewardCode.ReservedByUser),
		ReservedUntil: time.Now().Add(time.Minute),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker reward code reserve template", slog.String("err", err.Error()))
	}
}
