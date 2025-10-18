package cauth

import (
	"sync"
	"time"

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
		cfg:    cfg,
		states: make(map[string]loginState),
	}

	go a.cleanupStates()

	return a
}

type Auth struct {
	cfg      Config
	db       *database.Database
	states   map[string]loginState
	statesMu sync.Mutex
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
