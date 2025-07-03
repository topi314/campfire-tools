package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type discordGuild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for non-tracker endpoints
		if !strings.HasPrefix(r.URL.Path, "/tracker") {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("session")
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				u := url.URL{
					Path:     "/login",
					RawQuery: url.Values{"rd": {r.URL.Path}}.Encode(),
				}
				http.Redirect(w, r, u.String(), http.StatusFound)
				return
			}
			slog.Error("failed to get session cookie", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		session, err := s.db.GetSession(ctx, cookie.Value)
		if err != nil {
			slog.Error("failed to get session", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		r = r.WithContext(auth.SetSession(ctx, *session))
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	redirect := query.Get("rd")
	if redirect == "" {
		redirect = "/tracker"
	}

	state := s.auth.NewState(redirect)

	scopes := strings.Join(s.auth.Config().Scopes, " ")
	opts := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("scope", scopes)}

	expiration := time.Now().Add(auth.MaxLoginFlowDuration)
	addOauthCookie(w, state, expiration)
	http.Redirect(w, r, s.auth.Config().AuthCodeURL(state, opts...), http.StatusTemporaryRedirect)
}

func (s *Server) LoginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	oauthState, _ := r.Cookie("oauthstate")
	state := query.Get("state")
	code := query.Get("code")

	if oauthState == nil || state != oauthState.Value {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	redirectURL, ok := s.auth.GetState(state)
	if !ok {
		http.Error(w, "Unknown OAuth state", http.StatusBadRequest)
		return
	}

	token, err := s.auth.Config().Exchange(ctx, code)
	if err != nil {
		slog.ErrorContext(ctx, "failed to exchange OAuth code", slog.Any("error", err))
		http.Error(w, "Failed to exchange OAuth code", http.StatusInternalServerError)
		return
	}

	user, err := s.getUser(ctx, token.AccessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get user info from Discord", slog.Any("error", err))
		http.Error(w, "Failed to get user info from Discord", http.StatusInternalServerError)
		return
	}

	if !slices.Contains(s.cfg.Auth.Whitelist, user.ID) {
		guilds, err := s.getUserGuilds(ctx, token.AccessToken)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get user guilds from Discord", slog.Any("error", err))
			http.Error(w, "Failed to get user guilds from Discord", http.StatusInternalServerError)
			return
		}

		if i := slices.IndexFunc(guilds, func(g discordGuild) bool {
			return g.ID == s.cfg.Auth.DiscordGuildID
		}); i == -1 {
			slog.ErrorContext(ctx, "user is not whitelisted or a member of the required Discord guild", slog.String("guild_id", s.cfg.Auth.DiscordGuildID))
			http.Error(w, "You are not whitelisted or a member of the required Discord guild", http.StatusForbidden)
			return
		}
	}

	now := time.Now()
	expiration := now.AddDate(1, 0, 0)
	session := auth.RandomStr(32)
	if err = s.db.CreateSession(ctx, database.Session{
		ID:        session,
		CreatedAt: now,
		ExpiresAt: expiration,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to create session", slog.Any("error", err))
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
	}

	addSessionCookie(w, session, expiration)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) getUser(ctx context.Context, accessToken string) (*discordUser, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := s.httpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}

	var user discordUser
	if err = json.NewDecoder(rs.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

func (s *Server) getUserGuilds(ctx context.Context, accessToken string) ([]discordGuild, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me/guilds", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := s.httpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}

	var guilds []discordGuild
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
		Path:     "/login/callback",
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
		Path:     "/login/callback",
	}

	http.SetCookie(w, &cookie)
}

func addSessionCookie(w http.ResponseWriter, session string, expiration time.Time) {
	cookie := http.Cookie{
		Name:     "session",
		Value:    session,
		Expires:  expiration,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,      // Can use via http reqs
		HttpOnly: true,       // Can't be accessed by JS
		Path:     "/tracker", // Only valid for tracker endpoints
	}

	http.SetCookie(w, &cookie)
	removeOauthCookie(w)
}
