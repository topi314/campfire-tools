package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"golang.org/x/oauth2"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var session *database.SessionWithUser
		if !strings.HasPrefix(r.URL.Path, "/tracker/login") {
			for _, cookie := range r.CookiesNamed("session") {
				var err error
				session, err = h.DB.GetSession(ctx, cookie.Value)
				if err != nil {
					if !errors.Is(err, sql.ErrNoRows) {
						slog.ErrorContext(ctx, "failed to get session from database", slog.Any("error", err), slog.String("session_id", cookie.Value))
					}
					continue
				}
				break
			}

			if session == nil && strings.HasPrefix(r.URL.Path, "/tracker") {
				h.forceLogin(w, r)
				return
			}
		}

		if session == nil {
			session = &database.SessionWithUser{
				Session: database.Session{
					ID:        "",
					CreatedAt: time.Time{},
					ExpiresAt: time.Time{},
					UserID:    "",
				},
				DiscordUser: database.DiscordUser{
					ID:          "",
					Username:    "",
					DisplayName: "",
					AvatarURL:   "",
					ImportedAt:  time.Time{},
				},
			}
		}

		r = r.WithContext(auth.SetSession(ctx, *session))
		next.ServeHTTP(w, r)
	})
}

func (h *handler) forceLogin(w http.ResponseWriter, r *http.Request) {
	u := url.URL{
		Path:     "/tracker/login",
		RawQuery: url.Values{"rd": {r.URL.Path}}.Encode(),
	}
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *handler) LoginOld(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, h.Auth.Config().AuthCodeURL(state, opts...), http.StatusTemporaryRedirect)
}

func (h *handler) LoginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	oauthState, _ := r.Cookie("oauthstate")
	state := query.Get("state")
	code := query.Get("code")

	if oauthState == nil || state != oauthState.Value {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	redirectURL, ok := h.Auth.GetState(state)
	if !ok {
		http.Error(w, "Unknown OAuth state", http.StatusBadRequest)
		return
	}

	token, err := h.Auth.Config().Exchange(ctx, code)
	if err != nil {
		slog.ErrorContext(ctx, "failed to exchange OAuth code", slog.Any("error", err))
		http.Error(w, "Failed to exchange OAuth code", http.StatusInternalServerError)
		return
	}

	user, err := h.getDiscordUser(ctx, token.AccessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get user info from Discord", slog.Any("error", err))
		http.Error(w, "Failed to get user info from Discord", http.StatusInternalServerError)
		return
	}

	if !slices.Contains(h.Cfg.DiscordAuth.Whitelist, user.ID.String()) {
		guilds, err := h.getDiscordUserGuilds(ctx, token.AccessToken)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get user guilds from Discord", slog.Any("error", err))
			http.Error(w, "Failed to get user guilds from Discord", http.StatusInternalServerError)
			return
		}

		if i := slices.IndexFunc(guilds, func(g discord.OAuth2Guild) bool {
			return g.ID.String() == h.Cfg.DiscordAuth.GuildID
		}); i == -1 {
			slog.ErrorContext(ctx, "user is not whitelisted or a member of the required Discord guild", slog.String("guild_id", h.Cfg.DiscordAuth.GuildID))
			http.Error(w, "You are not whitelisted or a member of the required Discord guild", http.StatusForbidden)
			return
		}
	}

	var admin bool
	if slices.Contains(h.Cfg.DiscordAuth.Admins, user.ID.String()) {
		admin = true
	}

	now := time.Now()
	expiration := now.AddDate(1, 0, 0)
	session := auth.RandomStr(32)
	if err = h.DB.UpsertDiscordUser(ctx, database.DiscordUser{
		ID:          user.ID.String(),
		Username:    user.Username,
		DisplayName: user.EffectiveName(),
		AvatarURL:   user.EffectiveAvatarURL(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to upsert discord user", slog.Any("error", err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	if err = h.DB.CreateSession(ctx, database.Session{
		ID:        session,
		CreatedAt: now,
		ExpiresAt: expiration,
		UserID:    user.ID.String(),
		Admin:     admin,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to create session", slog.Any("error", err))
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	addSessionCookie(w, session, expiration)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *handler) getDiscordUser(ctx context.Context, accessToken string) (*discord.OAuth2User, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := h.HttpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}

	var user discord.OAuth2User
	if err = json.NewDecoder(rs.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

func (h *handler) getDiscordUserGuilds(ctx context.Context, accessToken string) ([]discord.OAuth2Guild, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me/guilds", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := h.HttpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}

	var guilds []discord.OAuth2Guild
	if err = json.NewDecoder(rs.Body).Decode(&guilds); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return guilds, nil
}

func addOauthCookie(w http.ResponseWriter, state string, expiration time.Time) {
	cookie := http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  expiration,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Can use via http reqs
		HttpOnly: true,  // Can't be accessed by JS
		Path:     "/tracker/login/callback",
	}

	http.SetCookie(w, &cookie)
}

func removeOauthCookie(w http.ResponseWriter) {
	cookie := http.Cookie{
		Name:     "oauthstate",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Can use via http reqs
		HttpOnly: true,  // Can't be accessed by JS
		Path:     "/tracker/login/callback",
	}

	http.SetCookie(w, &cookie)
}

func addSessionCookie(w http.ResponseWriter, session string, expiration time.Time) {
	cookie := http.Cookie{
		Name:     "session",
		Value:    session,
		Expires:  expiration,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Can use via http reqs
		HttpOnly: true,  // Can't be accessed by JS
		Path:     "/",   // Only valid for tracker endpoints
	}

	http.SetCookie(w, &cookie)
	removeOauthCookie(w)
}
