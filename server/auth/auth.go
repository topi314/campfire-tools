package auth

import (
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/topi314/campfire-tools/server/database"
)

const MaxLoginFlowDuration = 230 * time.Minute

type loginState struct {
	RedirectURL string
	CreatedAt   time.Time
}

func (s loginState) IsExpired() bool {
	return time.Since(s.CreatedAt) > MaxLoginFlowDuration
}

func New(cfg Config) *Auth {
	a := &Auth{
		cfg: cfg,
		oauth2Cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoints.Discord,
			RedirectURL:  cfg.PublicURL + "/tracker/login/callback",
			Scopes:       []string{"identify", "guilds"},
		},
		states: make(map[string]loginState),
	}

	go a.cleanupStates()

	return a
}

type Auth struct {
	cfg       Config
	db        *database.Database
	oauth2Cfg *oauth2.Config
	states    map[string]loginState
	statesMu  sync.Mutex
}

func (a *Auth) Config() *oauth2.Config {
	return a.oauth2Cfg
}

func (a *Auth) NewState(redirectURL string) string {
	a.statesMu.Lock()
	defer a.statesMu.Unlock()

	state := RandomStr(32)
	a.states[state] = loginState{
		RedirectURL: redirectURL,
		CreatedAt:   time.Now(),
	}
	return state
}

func (a *Auth) GetState(state string) (string, bool) {
	a.statesMu.Lock()
	defer a.statesMu.Unlock()

	lState, ok := a.states[state]
	if ok {
		delete(a.states, state)
	}

	if lState.IsExpired() {
		return "", false
	}

	return lState.RedirectURL, ok
}

func (a *Auth) cleanupStates() {
	for {
		a.doCleanupStates()
		time.Sleep(10 * time.Minute)
	}
}

func (a *Auth) doCleanupStates() {
	a.statesMu.Lock()
	defer a.statesMu.Unlock()

	for state, lState := range a.states {
		if lState.IsExpired() {
			delete(a.states, state)
		}
	}
}
