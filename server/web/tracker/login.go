package tracker

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/topi314/campfire-tools/server/auth"
)

type TrackerLoginVars struct {
	LoginURL string
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	redirect := query.Get("rd")
	if redirect == "" {
		redirect = "/tracker"
	}

	state := h.Auth.NewState(redirect)

	scopes := strings.Join(h.Auth.Config().Scopes, " ")
	opts := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("scope", scopes)}

	expiration := time.Now().Add(auth.MaxLoginFlowDuration)
	addOauthCookie(w, state, expiration)

	if err := h.Templates().ExecuteTemplate(w, "tracker_login.gohtml", TrackerLoginVars{
		LoginURL: h.Auth.Config().AuthCodeURL(state, opts...),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render index template", slog.String("err", err.Error()))
	}
}
