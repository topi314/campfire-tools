package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"

	"github.com/topi314/campfire-tools/internal/xio"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardCodeVars struct {
	models.Reward
	RewardCode
	BackURL string
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
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	code, err := h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("error", err.Error()))
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
	trackerCode := newRewardCode(id, code.RewardCode, code.ImportedByUser, redeemedBy)

	if err = h.Templates().ExecuteTemplate(w, "tracker_reward_code.gohtml", TrackerRewardCodeVars{
		Reward:     models.NewReward(*reward, 0, 0),
		RewardCode: trackerCode,
		BackURL:    fmt.Sprintf("/tracker/rewards/%d", id),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
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
		slog.ErrorContext(ctx, "Failed to get reward ", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward", http.StatusInternalServerError)
		return
	}

	code, err := h.DB.GetRewardCode(ctx, codeID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward codes", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward codes", http.StatusInternalServerError)
		return
	}

	qr, err := qrcode.New(CodeURL(code.Code))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create qrcode", slog.String("error", err.Error()))
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
		slog.ErrorContext(ctx, "Failed to save qrcode", slog.String("error", err.Error()))
	}
}
