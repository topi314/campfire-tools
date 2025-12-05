package models

import (
	"cmp"
	"fmt"
	"time"

	"github.com/topi314/campfire-tools/server/database"
)

func CodeURL(code string) string {
	return fmt.Sprintf("https://store.pokemongo.com/offer-redemption?passcode=%s", code)
}

func RewardCodeURL(publicURL string, code string) string {
	return fmt.Sprintf("%s/redeem?code=%s", publicURL, code)
}

func NewRewardCode(code database.RewardCode, importedBy database.DiscordUser, redeemedBy *database.DiscordUser) RewardCode {
	var user *DiscordUser
	if redeemedBy != nil {
		u := NewDiscordUser(*redeemedBy)
		user = &u
	}
	return RewardCode{
		ID:         code.ID,
		URL:        fmt.Sprintf("/tracker/rewards/%d/codes/%d", code.RewardID, code.ID),
		Code:       code.Code,
		QRURL:      fmt.Sprintf("/tracker/rewards/%d/codes/%d/qr", code.RewardID, code.ID),
		ImportedAt: code.ImportedAt,
		ImportedBy: NewDiscordUser(importedBy),
		RedeemCode: code.RedeemCode,
		RedeemedAt: code.RedeemedAt,
		RedeemedBy: user,
	}
}

type RewardCode struct {
	ID         int
	URL        string
	Code       string
	QRURL      string
	ImportedAt time.Time
	ImportedBy DiscordUser
	RedeemCode string
	RedeemedAt *time.Time
	RedeemedBy *DiscordUser
}

func (c RewardCode) IsRedeemed() bool {
	return c.RedeemedAt != nil
}

func (c RewardCode) RedeemCodeURL() string {
	return CodeURL(c.Code)
}

func NewDiscordUser(user database.DiscordUser) DiscordUser {
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

func (u DiscordUser) EffectiveName() string {
	if u.DisplayName == "" {
		return u.Username
	}
	return u.DisplayName
}
