package tracker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

func (h *handler) ConfirmRaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	memberID := r.PathValue("member_id")
	pastWinnersOpenStr := r.FormValue("past_winners")
	blockedMembersOpenStr := r.FormValue("blocked_members")
	confirmedOpenStr := r.FormValue("confirmed")

	slog.InfoContext(ctx, "Received raffle winner request",
		slog.String("url", r.URL.String()),
		slog.String("raffle_id", raffleIDStr),
		slog.String("member_id", memberID),
		slog.String("past_winners_open", pastWinnersOpenStr),
		slog.String("blocked_members_open", blockedMembersOpenStr),
	)

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, raffleIDStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	pastWinnersOpen, _ := strconv.ParseBool(pastWinnersOpenStr)
	blockedMembersOpen, _ := strconv.ParseBool(blockedMembersOpenStr)
	confirmedOpen, _ := strconv.ParseBool(confirmedOpenStr)

	session := auth.GetSession(r)
	updated, err := h.DB.ConfirmRaffleWinner(ctx, raffleID, memberID, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to confirm raffle winner in database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to confirm raffle winner: "+err.Error())
		return
	}

	if updated {
		if r.Header.Get("HX-Request") != "" {
			h.renderRaffleLive(w, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen, RaffleNotice{}, true)
			return
		}
		redirectRaffle(w, r, raffle.Token, clubID, rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, confirmedOpen))
		return
	}

	notice := h.raffleConfirmNotice(ctx, raffleID, memberID)
	if r.Header.Get("HX-Request") != "" {
		h.renderRaffleLive(w, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen, notice, true)
		return
	}
	redirectRaffle(w, r, raffle.Token, clubID, rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, confirmedOpen))
}

func (h *handler) UnconfirmRaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")
	raffleIDStr := r.PathValue("raffle_id")
	memberID := r.PathValue("member_id")
	pastWinnersOpen, _ := strconv.ParseBool(r.FormValue("past_winners"))
	blockedMembersOpen, _ := strconv.ParseBool(r.FormValue("blocked_members"))
	confirmedOpen, _ := strconv.ParseBool(r.FormValue("confirmed"))

	raffleID, raffle, err := h.getAuthorizedRaffle(ctx, r, raffleIDStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = h.DB.UnconfirmRaffleWinner(ctx, raffleID, memberID); err != nil {
		slog.ErrorContext(ctx, "Failed to unconfirm raffle winner", slog.Any("err", err))
		http.Error(w, "Failed to unmark raffle winner: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		h.renderRaffleLive(w, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen, RaffleNotice{}, false)
		return
	}
	redirectRaffle(w, r, raffle.Token, clubID, rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, confirmedOpen))
}

func (h *handler) raffleConfirmNotice(ctx context.Context, raffleID int, memberID string) RaffleNotice {
	state, err := h.DB.GetRaffleConfirmState(ctx, raffleID, memberID)
	if err != nil || state == nil {
		return RaffleNotice{Person: "This winner", Gone: true}
	}
	notice := RaffleNotice{Person: state.DisplayName}
	if state.DisplayName == "" {
		notice.Person = "This winner"
	}
	if !state.Confirmed.Valid || !state.Confirmed.Bool {
		notice.Gone = true
		return notice
	}
	if state.MarkerName.Valid {
		notice.Marker = state.MarkerName.String
	}
	if state.ConfirmedAt.Valid {
		notice.At = state.ConfirmedAt.Time
		notice.HasAt = true
	}
	return notice
}

func raffleURL(token, clubID string) string {
	if clubID != "" {
		return fmt.Sprintf("/tracker/club/%s/raffle/%s", clubID, token)
	}
	return "/raffle/" + token
}

func raffleSharePath(token string) string {
	return "/raffle/" + token + "/share"
}

func (h *handler) RaffleShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := strings.TrimSpace(r.PathValue("raffle_id"))
	if token == "" {
		h.NotFound(w, r)
		return
	}

	raffle, err := h.DB.GetRaffleByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get shared raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}
	if err = h.DB.AddRaffleUser(ctx, raffle.ID, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to record raffle open", slog.Any("err", err))
		http.Error(w, "Failed to open raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}
	redirectRaffle(w, r, raffle.Token, "", "")
}

func (h *handler) RaffleWinnersPoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")
	query := r.URL.Query()
	_, raffle, err := h.getAuthorizedRaffle(ctx, r, r.PathValue("raffle_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to poll raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pastWinnersOpen := xquery.ParseBool(query, "past-winners", false) || xquery.ParseBool(query, "past_winners", false)
	blockedMembersOpen := xquery.ParseBool(query, "blocked-members", false) || xquery.ParseBool(query, "blocked_members", false)
	confirmedOpen := xquery.ParseBool(query, "confirmed", false)
	h.renderRaffleLive(w, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen, RaffleNotice{}, false)
}

func (h *handler) renderRaffleLive(w http.ResponseWriter, r *http.Request, raffle *database.Raffle, clubID string, pastWinnersOpen, blockedMembersOpen, confirmedOpen bool, notice RaffleNotice, swapNotice bool) {
	ctx := r.Context()
	vars, err := h.raffleLiveVars(ctx, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle winners", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if notice.Person != "" {
		vars.HasNotice = true
		vars.Notice = notice
	}
	vars.SwapNotice = swapNotice
	if err = h.Templates().ExecuteTemplate(w, "raffle_live", vars); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle winners", slog.Any("err", err))
	}
}

func splitRaffleWinners(rows []database.RaffleWinnerWithMember, clubID, token string) (current, confirmed, past []models.Winner) {
	for _, row := range rows {
		winner := models.NewWinner(row, clubID, token)
		switch {
		case row.Past:
			past = append(past, winner)
		default:
			current = append(current, winner)
		}
	}
	return current, confirmed, past
}

func rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, confirmedOpen bool) string {
	var rawQuery string
	add := func(name string, open bool) {
		if !open {
			return
		}
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += name + "=true"
	}
	add("past-winners", pastWinnersOpen)
	add("blocked-members", blockedMembersOpen)
	add("confirmed", confirmedOpen)
	return rawQuery
}

func (h *handler) raffleLiveVars(ctx context.Context, r *http.Request, raffle *database.Raffle, clubID string, pastWinnersOpen, blockedMembersOpen, confirmedOpen bool) (RaffleResultVars, error) {
	allWinners, err := h.DB.GetRaffleWinners(ctx, raffle.ID)
	if err != nil {
		return RaffleResultVars{}, err
	}
	winners, confirmedWinners, pastWinners := splitRaffleWinners(allWinners, clubID, raffle.Token)

	var blockedMembers []models.Member
	if raffle.UserID != "" {
		blocked, blockedErr := h.DB.GetDiscordUserRaffleBlockedMembers(ctx, raffle.UserID)
		if blockedErr != nil {
			return RaffleResultVars{}, blockedErr
		}
		blockedMembers = make([]models.Member, len(blocked))
		for i, member := range blocked {
			blockedMembers[i] = models.NewImportedMember(member, 32)
		}
	}

	access, err := h.DB.GetRaffleUsers(ctx, raffle.ID)
	if err != nil {
		return RaffleResultVars{}, err
	}
	users := make([]models.DiscordUser, 0, len(access))
	for _, user := range access {
		users = append(users, models.NewDiscordUser(user))
	}

	return RaffleResultVars{
		Raffle:             models.NewRaffle(*raffle),
		ClubID:             clubID,
		PollURL:            raffleURL(raffle.Token, clubID) + "/winners/poll",
		Users:              users,
		Winners:            winners,
		ConfirmedWinners:   confirmedWinners,
		ConfirmedOpen:      confirmedOpen,
		PastWinners:        pastWinners,
		PastWinnersOpen:    pastWinnersOpen,
		BlockedMembers:     blockedMembers,
		BlockedMemberID:    blockedMemberIDs(blockedMembers),
		BlockedMembersOpen: blockedMembersOpen,
		CanBlock:           raffleCanBlock(r, raffle),
		BlockAction:        raffleURL(raffle.Token, clubID) + "/block/",
		UnblockAction:      raffleURL(raffle.Token, clubID) + "/unblock/",
	}, nil
}

func blockedMemberIDs(members []models.Member) map[string]bool {
	ids := make(map[string]bool, len(members))
	for _, member := range members {
		ids[member.ID] = true
	}
	return ids
}

func raffleCanBlock(r *http.Request, raffle *database.Raffle) bool {
	if raffle == nil || raffle.UserID == "" {
		return false
	}
	return auth.GetSession(r).UserID == raffle.UserID
}

func (h *handler) PostRaffleBlockMember(w http.ResponseWriter, r *http.Request) {
	h.postRaffleBlock(w, r, true)
}

func (h *handler) PostRaffleUnblockMember(w http.ResponseWriter, r *http.Request) {
	h.postRaffleBlock(w, r, false)
}

func (h *handler) postRaffleBlock(w http.ResponseWriter, r *http.Request, block bool) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")
	raffleIDStr := r.PathValue("raffle_id")
	memberID := strings.TrimSpace(r.PathValue("member_id"))

	_, raffle, err := h.getAuthorizedRaffle(ctx, r, raffleIDStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle", slog.Any("err", err))
		http.Error(w, "Failed to get raffle: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !raffleCanBlock(r, raffle) {
		http.Error(w, "Only the raffle owner can change the block list", http.StatusForbidden)
		return
	}
	if memberID == "" {
		http.Error(w, "Member ID is required", http.StatusBadRequest)
		return
	}

	pastWinnersOpen, _ := strconv.ParseBool(r.FormValue("past_winners"))
	blockedMembersOpen, _ := strconv.ParseBool(r.FormValue("blocked_members"))
	confirmedOpen, _ := strconv.ParseBool(r.FormValue("confirmed"))

	if block {
		if _, err = h.DB.GetMember(ctx, memberID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "Member not found", http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "Failed to get member for raffle block list", slog.String("member_id", memberID), slog.Any("err", err))
			http.Error(w, "Failed to block member", http.StatusInternalServerError)
			return
		}
		if err = h.DB.AddDiscordUserRaffleBlockedMember(ctx, raffle.UserID, memberID); err != nil {
			slog.ErrorContext(ctx, "Failed to block raffle member", slog.String("member_id", memberID), slog.Any("err", err))
			http.Error(w, "Failed to block member", http.StatusInternalServerError)
			return
		}
		if err = h.DB.DeleteCurrentRaffleWinner(ctx, raffle.ID, memberID); err != nil {
			slog.ErrorContext(ctx, "Failed to remove blocked raffle winner", slog.String("member_id", memberID), slog.Any("err", err))
			http.Error(w, "Failed to block member", http.StatusInternalServerError)
			return
		}
		blockedMembersOpen = true
	} else if err = h.DB.RemoveDiscordUserRaffleBlockedMember(ctx, raffle.UserID, memberID); err != nil {
		slog.ErrorContext(ctx, "Failed to unblock raffle member", slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to unblock member", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		h.renderRaffleLive(w, r, raffle, clubID, pastWinnersOpen, blockedMembersOpen, confirmedOpen, RaffleNotice{}, false)
		return
	}
	redirectRaffle(w, r, raffle.Token, clubID, rafflePanelQuery(pastWinnersOpen, blockedMembersOpen, confirmedOpen))
}

func redirectRaffle(w http.ResponseWriter, r *http.Request, token, clubID, rawQuery string) {
	http.Redirect(w, r, withQuery(raffleURL(token, clubID), rawQuery), http.StatusSeeOther)
}

func withQuery(url string, query string) string {
	if query == "" {
		return url
	}
	return fmt.Sprintf("%s?%s", url, query)
}
