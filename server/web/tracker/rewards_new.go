package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerRewardsNewVars struct {
	Error string
}

func (h *handler) TrackerRewardsNew(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerRewardsNew(w, r, "")
}

func (h *handler) renderTrackerRewardsNew(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "tracker_rewards_new.gohtml", TrackerRewardsNewVars{
		Error: errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("err", err.Error()))
	}
}

func (h *handler) PostTrackerRewardsNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	name := r.FormValue("name")
	description := r.FormValue("description")
	codes := parseCodes(r.FormValue("codes"))
	if len(codes) == 0 {
		h.renderTrackerRewardsNew(w, r, "No codes found in the file")
		return
	}

	id, err := h.DB.InsertReward(ctx, database.Reward{
		Name:        name,
		Description: description,
		CreatedBy:   session.UserID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to insert reward", slog.String("err", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to create reward")
		return
	}

	if err = h.DB.InsertRewardCodes(ctx, id, codes, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to insert reward codes", slog.String("err", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to add reward codes")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func parseCodes(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
	})
}
