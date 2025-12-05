package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"

	"github.com/topi314/campfire-tools/internal/xio"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardCodeVars struct {
	models.Reward
	models.RewardCode
	BackURL         string
	MarkAsUsedURL   string
	MarkAsUnusedURL string
	URL             string
	RewardCodeURL   string
}

func (h *handler) TrackerRewardCode(w http.ResponseWriter, r *http.Request) {
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

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	code, err := h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	var redeemedBy *database.DiscordUser
	if code.RedeemedByUser.ID != nil {
		redeemedBy = &database.DiscordUser{
			ID:          *code.RedeemedByUser.ID,
			Username:    *code.RedeemedByUser.Username,
			DisplayName: *code.RedeemedByUser.DisplayName,
			AvatarURL:   *code.RedeemedByUser.AvatarURL,
		}
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_code.gohtml", TrackerRewardCodeVars{
		Reward:          models.NewReward(*reward),
		RewardCode:      models.NewRewardCode(code.RewardCode, code.ImportedByUser, redeemedBy),
		BackURL:         fmt.Sprintf("/tracker/rewards/%d", id),
		MarkAsUsedURL:   fmt.Sprintf("/tracker/rewards/%d/codes/%d/mark-used", id, codeID),
		MarkAsUnusedURL: fmt.Sprintf("/tracker/rewards/%d/codes/%d/mark-unused", id, codeID),
		URL:             fmt.Sprintf("/tracker/rewards/%d/codes/%d", id, codeID),
		RewardCodeURL:   models.RewardCodeURL(h.Cfg.Server.PublicRewardsURL, code.RedeemCode),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}

func (h *handler) TrackerRewardCodeQR(w http.ResponseWriter, r *http.Request) {
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

	code, err := h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("err", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	qr, err := qrcode.New(models.RewardCodeURL(h.Cfg.Server.PublicRewardsURL, code.RedeemCode))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create qrcode", slog.String("err", err.Error()))
		http.Error(w, "Failed to create qrcode", http.StatusInternalServerError)
		return
	}

	qrW := standard.NewWithWriter(xio.NewResponseWriteCloser(w), standard.WithLogoImage(h.Logo),
		standard.WithBgTransparent(),
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
		standard.WithLogoSafeZone(),
		standard.WithLogoSizeMultiplier(2),
	)

	defer func() {
		_ = qrW.Close()
	}()
	if err = qr.Save(qrW); err != nil {
		slog.ErrorContext(ctx, "Failed to save qrcode", slog.String("err", err.Error()))
	}
}

func (h *handler) TrackerRewardCodeDelete(w http.ResponseWriter, r *http.Request) {
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

	if err = h.DB.DeleteRewardCode(ctx, codeID); err != nil {
		slog.ErrorContext(ctx, "Failed to delete reward code", slog.String("err", err.Error()))
		http.Error(w, "Failed to delete reward code", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func (h *handler) TrackerRewardCodeMarkAsUsed(w http.ResponseWriter, r *http.Request) {
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

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func (h *handler) TrackerRewardCodeMarkAsUnused(w http.ResponseWriter, r *http.Request) {
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

	if err = h.DB.UpdateRewardCodeRedeemed(ctx, codeID, nil, nil); err != nil {
		slog.ErrorContext(ctx, "Failed to mark reward code as unused", slog.String("err", err.Error()))
		http.Error(w, "Failed to mark reward code as unused", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}
