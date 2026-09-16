package tracker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type AdminVars struct {
	Tokens                []models.Token
	EventsWithoutCategory int
	Success               string
	Errors                []string
}

func (h *handler) Admin(w http.ResponseWriter, r *http.Request) {
	session := auth.GetSession(r)

	if !session.Admin {
		h.NotFound(w, r)
		return
	}

	h.renderAdmin(w, r)
}

func (h *handler) renderAdmin(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()

	tokens, err := h.DB.GetCampfireTokens(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch tokens: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var tokenList []models.Token
	for _, t := range tokens {
		tokenList = append(tokenList, models.NewToken(t))
	}

	uncategorized, err := h.DB.CountEventsWithoutCategory(ctx)
	if err != nil {
		http.Error(w, "Failed to count uncategorized events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "admin.gohtml", AdminVars{
		Tokens:                tokenList,
		EventsWithoutCategory: uncategorized,
		Success:               r.URL.Query().Get("success"),
		Errors:                errorMessages,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker template", slog.Any("err", err))
	}
}

func (h *handler) AdminTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	if !session.Admin {
		h.NotFound(w, r)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		h.renderAdmin(w, r, "Token cannot be empty")
		return
	}

	campfireToken, err := parseToken(token)
	if err != nil {
		h.renderAdmin(w, r, "Invalid token: "+err.Error())
		return
	}

	if err = h.DB.InsertCampfireToken(ctx, *campfireToken); err != nil {
		h.renderAdmin(w, r, "Failed to insert token: "+err.Error())
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *handler) AdminBackfillEventCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	if !session.Admin {
		h.NotFound(w, r)
		return
	}

	updated, err := h.DB.BackfillEmptyEventCategories(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to backfill event categories", slog.Any("err", err))
		h.renderAdmin(w, r, "Failed to backfill event categories: "+err.Error())
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin?success=%s", url.QueryEscape(fmt.Sprintf("Updated %d events without a category.", updated))), http.StatusSeeOther)
}

func parseToken(token string) (*database.CampfireToken, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	tokenData, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid token data: %w", err)
	}

	var t jwtToken
	if err = json.Unmarshal(tokenData, &t); err != nil {
		return nil, fmt.Errorf("invalid token json: %w", err)
	}

	return &database.CampfireToken{
		Token:     token,
		ExpiresAt: time.Unix(t.Exp, 0),
		Email:     t.Email,
	}, nil
}

type jwtToken struct {
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
}
