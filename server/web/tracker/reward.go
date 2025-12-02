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

func CodeURL(code string) string {
	return fmt.Sprintf("https://store.pokemongo.com/offer-redemption?passcode=%s", code)
}

type TrackerRewardVars struct {
	models.Reward
	Codes []RewardCode
}

func newRewardCode(rewardID int, code database.RewardCode, importedBy database.DiscordUser, redeemedBy *database.DiscordUser) RewardCode {
	var user *DiscordUser
	if redeemedBy != nil {
		u := newDiscordUser(*redeemedBy)
		user = &u
	}
	return RewardCode{
		ID:            code.ID,
		URL:           fmt.Sprintf("/tracker/rewards/%d/codes/%d", rewardID, code.ID),
		Code:          code.Code,
		QRURL:         fmt.Sprintf("/tracker/rewards/%d/codes/%d/qr", rewardID, code.ID),
		RedeemCodeURL: CodeURL(code.Code),
		ImportedAt:    code.ImportedAt,
		ImportedBy:    newDiscordUser(importedBy),
		RedeemCode:    code.RedeemCode,
		RedeemedAt:    code.RedeemedAt,
		RedeemedBy:    user,
	}
}

type RewardCode struct {
	ID            int
	URL           string
	Code          string
	QRURL         string
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

func (h *handler) TrackerReward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("error", err.Error()))
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
