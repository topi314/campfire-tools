package rewards

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"

	"github.com/topi314/campfire-tools/internal/xio"
	"github.com/topi314/campfire-tools/internal/xrand"
)

func (h *handler) Code(w http.ResponseWriter, r *http.Request) {
	code := xrand.RandCode()

	http.Redirect(w, r, fmt.Sprintf("/code/%s", code), http.StatusFound)
}

type CodeVars struct {
	Code string
}

func (h *handler) GetCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PathValue("code")

	if err := h.Templates().ExecuteTemplate(w, "code.gohtml", CodeVars{
		Code: code,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}

func (h *handler) QRCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PathValue("code")

	qr, err := qrcode.New(h.Cfg.Server.PublicRewardsURL + "/tracker/code/" + code)
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
