package tracker

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardPoolVars struct {
	models.RewardPool
	Codes []RewardCode
}

func newRewardCode(pool database.RewardCode, importedBy database.DiscordUser, redeemedBy *database.DiscordUser) RewardCode {
	var user *DiscordUser
	if redeemedBy != nil {
		u := newDiscordUser(*redeemedBy)
		user = &u
	}
	return RewardCode{
		ID:            pool.ID,
		URL:           fmt.Sprintf("/tracker/rewards/%d", pool.ID),
		Code:          pool.Code,
		RedeemCodeURL: fmt.Sprintf("https://store.pokemongo.com/offer-redemption?passcode=%s", pool.Code),
		ImportedAt:    pool.ImportedAt,
		ImportedBy:    newDiscordUser(importedBy),
		RedeemCode:    pool.RedeemCode,
		RedeemedAt:    pool.RedeemedAt,
		RedeemedBy:    user,
	}
}

type RewardCode struct {
	ID            int
	URL           string
	Code          string
	RedeemCodeURL string
	ImportedAt    time.Time
	ImportedBy    DiscordUser
	RedeemCode    *string
	RedeemedAt    *time.Time
	RedeemedBy    *DiscordUser
}

func newDiscordUser(user database.DiscordUser) DiscordUser {
	return DiscordUser{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: cmp.Or(user.DisplayName, user.Username),
		AvatarURL:   user.AvatarURL,
		ImportedAt:  user.ImportedAt,
	}
}

type DiscordUser struct {
	ID          string
	Username    string
	DisplayName string
	AvatarURL   string
	ImportedAt  time.Time
}

func (h *handler) TrackerRewardPool(w http.ResponseWriter, r *http.Request) {
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
		RewardPool: models.NewRewardPool(*rewardPool, 0, 0),
		Codes:      trackerCodes,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}
