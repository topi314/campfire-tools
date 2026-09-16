package tracker

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/topi314/campfire-tools/internal/eventcategory"
	"github.com/topi314/campfire-tools/server/web/models"
)

type TrackerClubEventVars struct {
	models.Event

	Club             models.Club
	ClubCategories   []string
	IsCustomCategory bool
	CheckedInMembers []models.Member
	AcceptedMembers  []models.Member
	Sort             string
	TotalAccepted    int
	TotalCheckIns    int
	TotalCheckInRate float64
}

func (h *handler) TrackerClubEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	eventID := r.PathValue("event_id")
	sortBy := query.Get("sort")
	switch sortBy {
	case "name", "name-desc":
	default:
		sortBy = "name"
	}

	event, err := h.DB.GetEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch event", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	club, err := h.DB.GetClub(ctx, event.ClubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch club", slog.String("club_id", event.ClubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	clubCategories, err := h.DB.GetClubEventCategories(ctx, event.ClubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch club event categories", slog.String("club_id", event.ClubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event categories: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if event.Category != "" {
		found := false
		for _, category := range clubCategories {
			if category == event.Category {
				found = true
				break
			}
		}
		if !found {
			clubCategories = eventcategory.Sort(append(clubCategories, event.Category))
		}
	}

	checkedInMembers, err := h.DB.GetCheckedInMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch checked-in members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch top members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	checkedInTrackerMembers := make([]models.Member, len(checkedInMembers))
	for i, member := range checkedInMembers {
		checkedInTrackerMembers[i] = models.NewMember(member, event.ClubID, 32)
	}
	sortEventMembers(checkedInTrackerMembers, sortBy)

	acceptedMembers, err := h.DB.GetAcceptedMembersByEvent(ctx, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch accepted members", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to fetch accepted members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	acceptedTrackerMembers := make([]models.Member, len(acceptedMembers))
	for i, member := range acceptedMembers {
		acceptedTrackerMembers[i] = models.NewMember(member, event.ClubID, 32)
	}
	sortEventMembers(acceptedTrackerMembers, sortBy)

	clubModel := models.NewClub(*club)
	eventModel := models.NewEventWithCreator(*event, clubModel.AvatarURL)
	totalCheckIns := len(checkedInTrackerMembers)
	totalAccepted := totalCheckIns + len(acceptedTrackerMembers)

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_event.gohtml", TrackerClubEventVars{
		Event:            eventModel,
		Club:             clubModel,
		ClubCategories:   clubCategories,
		IsCustomCategory: len(clubCategories) == 0,
		CheckedInMembers: checkedInTrackerMembers,
		AcceptedMembers:  acceptedTrackerMembers,
		Sort:             sortBy,
		TotalAccepted:    totalAccepted,
		TotalCheckIns:    totalCheckIns,
		TotalCheckInRate: models.CalcCheckInRate(totalAccepted, totalCheckIns),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club event template", slog.String("event_id", eventID), slog.Any("err", err))
	}
}

func (h *handler) TrackerClubEventCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventID := r.PathValue("event_id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	category := strings.TrimSpace(r.FormValue("event-category"))
	if category == "" || category == "__custom__" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	if _, err := h.DB.GetEvent(ctx, eventID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.DB.UpdateEventCategory(ctx, eventID, category); err != nil {
		slog.ErrorContext(ctx, "Failed to update event category", slog.String("event_id", eventID), slog.Any("err", err))
		http.Error(w, "Failed to update event category: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tracker/event/"+eventID, http.StatusSeeOther)
}

func sortEventMembers(members []models.Member, sortBy string) {
	slices.SortFunc(members, func(a, b models.Member) int {
		if a.IsCommunityAmbassador != b.IsCommunityAmbassador {
			if a.IsCommunityAmbassador {
				return -1
			}
			return 1
		}

		cmp := strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
		if cmp == 0 {
			cmp = strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
		}
		if cmp == 0 {
			cmp = strings.Compare(a.ID, b.ID)
		}
		if sortBy == "name-desc" {
			return -cmp
		}
		return cmp
	})
}
