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

type TrackerRewardEditVars struct {
	models.Reward
	Error string
}

func (h *handler) TrackerRewardEdit(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerRewardEdit(w, r, "")
}

func (h *handler) renderTrackerRewardEdit(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	reward, err := h.DB.GetReward(ctx, id, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get reward", slog.String("error", err.Error()))
		h.NotFound(w, r)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_rewards_edit.gohtml", TrackerRewardEditVars{
		Reward: models.NewReward(*reward, 0, 0),
		Error:  errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker rewards edit template", slog.String("error", err.Error()))
	}
}

func (h *handler) PostTrackerRewardEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	description := r.FormValue("description")

	if err = h.DB.UpdateReward(ctx, database.Reward{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedBy:   session.UserID,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to update reward", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to update reward")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func (h *handler) TrackerRewardDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if err = h.DB.DeleteReward(ctx, id); err != nil {
		slog.ErrorContext(ctx, "Failed to delete reward", slog.String("error", err.Error()))
		http.Error(w, "Failed to delete reward", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards"), http.StatusSeeOther)
}
