package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerRewardsEditVars struct {
	models.RewardPool
	Error string
}

func (h *handler) TrackerRewardsEdit(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerRewardsEdit(w, r, "")
}

func (h *handler) renderTrackerRewardsEdit(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("pool_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	rewardPool, err := h.DB.GetRewardPool(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward pool", slog.String("error", err.Error()))
		h.NotFound(w, r)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_rewards_edit.gohtml", TrackerRewardsEditVars{
		RewardPool: models.NewRewardPool(*rewardPool, 0, 0),
		Error:      errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards edit template", slog.String("error", err.Error()))
	}
}

func (h *handler) PostTrackerRewardsEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("pool_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	description := r.FormValue("description")

	if err = h.DB.UpdateRewardPool(ctx, database.RewardPool{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedBy:   session.UserID,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to update reward pool", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to update reward pool")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func (h *handler) DeleteTrackerRewardCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	codeID, err := strconv.Atoi(r.PathValue("code_id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if err = h.DB.DeleteRewardCode(ctx, codeID); err != nil {
		slog.ErrorContext(ctx, "Failed to delete reward code", slog.String("error", err.Error()))
		http.Error(w, "Failed to delete reward code", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards"), http.StatusSeeOther)
}
