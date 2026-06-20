package tracker

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/server/web/models"
)

const searchMembersLimit = 100

type TrackerMembersVars struct {
	Query   string
	Members []models.ImportedMember
}

type TrackerMemberVars struct {
	models.ImportedMember
	CheckInEventsByClub  []models.ClubMemberEvents
	AcceptedEventsByClub []models.ClubMemberEvents
}

func (h *handler) TrackerMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var trackerMembers []models.ImportedMember
	if query != "" {
		members, err := h.DB.SearchMembers(ctx, query, searchMembersLimit)
		if err != nil {
			http.Error(w, "Failed to search members: "+err.Error(), http.StatusInternalServerError)
			return
		}

		trackerMembers = make([]models.ImportedMember, len(members))
		for i, member := range members {
			trackerMembers[i] = models.ImportedMember{
				Member:     models.NewImportedMember(member, 32),
				ImportedAt: member.ImportedAt,
			}
		}
	}

	if err := h.Templates().ExecuteTemplate(w, "tracker_members.gohtml", TrackerMembersVars{
		Query:   query,
		Members: trackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker members template", slog.Any("err", err))
	}
}

func (h *handler) TrackerMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	memberID := r.PathValue("member_id")

	member, err := h.DB.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to fetch member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	checkInEvents, err := h.DB.GetCheckedInEventsByMember(ctx, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch checked-in events for member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch checked-in events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	acceptedEvents, err := h.DB.GetAcceptedEventsByMember(ctx, memberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch accepted events for member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to fetch accepted events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_member.gohtml", TrackerMemberVars{
		ImportedMember: models.ImportedMember{
			Member:     models.NewImportedMember(*member, 48),
			ImportedAt: member.ImportedAt,
		},
		CheckInEventsByClub:  models.GroupEventsByClub(checkInEvents, 32),
		AcceptedEventsByClub: models.GroupEventsByClub(acceptedEvents, 32),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker member template", slog.String("member_id", memberID), slog.Any("err", err))
	}
}
