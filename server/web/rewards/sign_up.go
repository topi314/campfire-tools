package rewards

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type SignUpVars struct {
	Clubs       []models.Club
	DefaultClub string
}

func (h *handler) SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	club := query.Get("club")

	clubs, err := h.DB.GetRewardClubs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward clubs", slog.String("error", err.Error()))
		http.Error(w, "Failed to get reward clubs", http.StatusInternalServerError)
		return
	}

	mClubs := make([]models.Club, len(clubs))
	for i, c := range clubs {
		mClubs[i] = models.NewClub(database.ClubWithCreator{
			Club: c,
		})
	}

	if err = h.Templates().ExecuteTemplate(w, "rewards_sign_up.gohtml", SignUpVars{
		Clubs:       mClubs,
		DefaultClub: club,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render rewards template", slog.String("error", err.Error()))
	}
}

func (h *handler) CampfireLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	redirect := query.Get("rd")
	if redirect == "" {
		redirect = "/inventory"
	}

	clubID := query.Get("club")
	if clubID == "" {
		http.Error(w, "Missing club parameter", http.StatusBadRequest)
		return
	}

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Club not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "Failed to get club for Campfire login", slog.String("error", err.Error()))
		http.Error(w, "Failed to get club for Campfire login", http.StatusInternalServerError)
		return
	}

	if club.ClubVerificationChannelID == nil {
		http.Error(w, "Club does not have a verification channel set up", http.StatusBadRequest)
		return
	}

	slog.InfoContext(ctx, "Initiating Campfire login for sign up", slog.String("club", clubID))

	state := h.CampfireAuth.NewState(redirect)

	u, _ := url.Parse(h.Cfg.CampfireAuth.AuthURL + "/login")
	q := u.Query()
	q.Set("client_id", h.Cfg.CampfireAuth.ClientID)
	q.Set("redirect_uri", h.Cfg.CampfireAuth.PublicURL+"/callback")
	q.Set("club_id", club.Club.ID)
	q.Set("channel_id", *club.ClubVerificationChannelID)
	q.Set("state", state)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *handler) SignUpCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	code := query.Get("code")
	state := query.Get("state")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	redirectURL, ok := h.CampfireAuth.GetState(state)
	if !ok {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	m, err := h.exchangeCode(code)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to exchange code for Campfire member", slog.String("error", err.Error()))
		http.Error(w, "Failed to process login", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "Successfully authenticated Campfire member for sign up", slog.String("member_id", m.ID), slog.String("redirect", redirectURL))

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *handler) PostSignUp(w http.ResponseWriter, r *http.Request) {

}

func (h *handler) exchangeCode(code string) (*campfire.Member, error) {
	rq, err := http.NewRequest(http.MethodGet, h.Cfg.CampfireAuth.AuthURL+"/api/exchange?code="+url.QueryEscape(code), nil)
	if err != nil {
		return nil, err
	}

	rq.SetBasicAuth(h.Cfg.CampfireAuth.ClientID, h.Cfg.CampfireAuth.ClientSecret)

	rs, err := http.DefaultClient.Do(rq)
	if err != nil {
		return nil, err
	}

	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, errors.New("invalid response status: " + rs.Status)
	}

	var m campfire.Member
	if err = json.NewDecoder(rs.Body).Decode(&m); err != nil {
		return nil, err
	}

	return &m, nil
}
