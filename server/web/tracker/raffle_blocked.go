package tracker

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/web/models"
)

const raffleBlockedSearchLimit = 100

type RaffleBlockedVars struct {
	Query           string
	BlockedMembers  []models.ImportedMember
	SearchMembers   []models.ImportedMember
	BlockedMemberID map[string]bool
	Error           string
	Success         string
}

func (h *handler) RaffleBlocked(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	errorMsg := strings.TrimSpace(r.URL.Query().Get("error"))
	successMsg := strings.TrimSpace(r.URL.Query().Get("success"))

	blocked, err := h.DB.GetDiscordUserRaffleBlockedMembers(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get raffle blocked members", slog.Any("err", err))
		http.Error(w, "Failed to get blocked members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	blockedMembers := make([]models.ImportedMember, len(blocked))
	blockedMemberID := make(map[string]bool, len(blocked))
	for i, member := range blocked {
		blockedMembers[i] = models.ImportedMember{
			Member:     models.NewImportedMember(member, 32),
			ImportedAt: member.ImportedAt,
		}
		blockedMemberID[member.ID] = true
	}

	var searchMembers []models.ImportedMember
	if query != "" {
		members, searchErr := h.DB.SearchMembers(ctx, query, raffleBlockedSearchLimit)
		if searchErr != nil {
			slog.ErrorContext(ctx, "Failed to search members for raffle block list", slog.Any("err", searchErr))
			http.Error(w, "Failed to search members: "+searchErr.Error(), http.StatusInternalServerError)
			return
		}

		searchMembers = make([]models.ImportedMember, len(members))
		for i, member := range members {
			searchMembers[i] = models.ImportedMember{
				Member:     models.NewImportedMember(member, 32),
				ImportedAt: member.ImportedAt,
			}
		}
	}

	if err = h.Templates().ExecuteTemplate(w, "raffle_blocked.gohtml", RaffleBlockedVars{
		Query:           query,
		BlockedMembers:  blockedMembers,
		SearchMembers:   searchMembers,
		BlockedMemberID: blockedMemberID,
		Error:           errorMsg,
		Success:         successMsg,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle blocked template", slog.Any("err", err))
	}
}

func (h *handler) PostRaffleBlocked(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	memberID := strings.TrimSpace(r.FormValue("member_id"))
	if memberID == "" {
		http.Redirect(w, r, "/raffle/blocked?error=Member+ID+is+required", http.StatusSeeOther)
		return
	}

	if _, err := h.DB.GetMember(ctx, memberID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Redirect(w, r, "/raffle/blocked?error=Member+not+found", http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "Failed to get member for raffle block list", slog.String("member_id", memberID), slog.Any("err", err))
		http.Redirect(w, r, "/raffle/blocked?error=Failed+to+block+member", http.StatusSeeOther)
		return
	}

	if err := h.DB.AddDiscordUserRaffleBlockedMember(ctx, session.UserID, memberID); err != nil {
		slog.ErrorContext(ctx, "Failed to add raffle blocked member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Redirect(w, r, "/raffle/blocked?error=Failed+to+block+member", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/raffle/blocked?success=Member+blocked", http.StatusSeeOther)
}

func (h *handler) PostRaffleBlockedRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	memberID := strings.TrimSpace(r.FormValue("member_id"))
	if memberID == "" {
		http.Redirect(w, r, "/raffle/blocked?error=Member+ID+is+required", http.StatusSeeOther)
		return
	}

	if err := h.DB.RemoveDiscordUserRaffleBlockedMember(ctx, session.UserID, memberID); err != nil {
		slog.ErrorContext(ctx, "Failed to remove raffle blocked member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Redirect(w, r, "/raffle/blocked?error=Failed+to+unblock+member", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/raffle/blocked?success=Member+unblocked", http.StatusSeeOther)
}
