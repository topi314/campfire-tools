package tracker

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"

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
		slog.ErrorContext(ctx, "Failed to render tracker rewards template", slog.String("error", err.Error()))
	}
}

func (h *handler) PostTrackerRewardsNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	name := r.FormValue("name")
	description := r.FormValue("description")
	file, _, err := r.FormFile("codes")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get codes file", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to get codes file")
		return
	}
	defer file.Close()

	codes, err := parseCodesFromFile(file)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse codes from file", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to parse codes from file")
		return
	}

	if len(codes) == 0 {
		h.renderTrackerRewardsNew(w, r, "No codes found in the file")
		return
	}

	id, err := h.DB.InsertRewardPool(ctx, database.RewardPool{
		Name:        name,
		Description: description,
		CreatedBy:   session.UserID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to insert reward pool", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to create reward pool")
		return
	}

	if err = h.DB.InsertRewardCodes(ctx, id, codes, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to insert reward codes", slog.String("error", err.Error()))
		h.renderTrackerRewardsNew(w, r, "Failed to add reward codes")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/tracker/rewards/%d", id), http.StatusSeeOther)
}

func parseCodesFromFile(r io.ReadCloser) ([]string, error) {
	defer r.Close()
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	var codes []string
	for i, record := range records {
		if i == 0 {
			// Skip header
			continue
		}
		if len(record) == 0 {
			continue
		}
		codes = append(codes, record[0])
	}
	return codes, nil
}
