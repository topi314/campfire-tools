package tracker

import (
	"log/slog"
	"net/http"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

func (h *handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "index.gohtml", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}

type TrackerIndexVars struct {
	User models.DiscordUser
}

func (h *handler) TrackerIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.GetSession(r)

	if err := h.Templates().ExecuteTemplate(w, "tracker_index.gohtml", TrackerIndexVars{
		User: models.NewDiscordUser(session.DiscordUser),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}
