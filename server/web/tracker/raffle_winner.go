package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) ConfirmRaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	memberID := r.PathValue("member_id")
	pastWinnersOpenStr := r.FormValue("past_winners")
	blockedMembersOpenStr := r.FormValue("blocked_members")

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

	if err = h.DB.ConfirmRaffleWinner(ctx, raffleID, memberID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "Raffle winner not found", slog.String("member_id", memberID), slog.Int("raffle_id", raffleID))
			h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Raffle winner not found")
			return
		}
		slog.ErrorContext(ctx, "Failed to confirm raffle winner in database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to confirm raffle winner: "+err.Error())
		return
	}

	var rawQuery string
	if pastWinnersOpen {
		rawQuery = "past-winners=true"
	}
	if blockedMembersOpen {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += "blocked-members=true"
	}

	redirectRaffle(w, r, raffle.Token, clubID, rawQuery)
}

func raffleURL(token, clubID string) string {
	if clubID != "" {
		return fmt.Sprintf("/tracker/club/%s/raffle/%s", clubID, token)
	}
	return "/raffle/" + token
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
