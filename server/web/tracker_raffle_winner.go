package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
)

func (h *handler) ConfirmRaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	raffleIDStr := r.PathValue("raffle_id")
	memberID := r.PathValue("member_id")
	pastWinnersOpenStr := r.FormValue("past_winners")

	slog.InfoContext(ctx, "Received raffle winner request",
		slog.String("url", r.URL.String()),
		slog.String("raffle_id", raffleIDStr),
		slog.String("member_id", memberID),
		slog.String("past_winners_open", pastWinnersOpenStr),
	)

	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	pastWinnersOpen, _ := strconv.ParseBool(pastWinnersOpenStr)

	raffle, err := h.DB.GetRaffleByID(ctx, raffleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, clubID, "Failed to get raffle: "+err.Error())
		return
	}

	session := auth.GetSession(r)
	if raffle.UserID != "" && raffle.UserID != session.UserID {
		h.NotFound(w, r)
		return
	}

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

	redirectRaffle(w, r, raffleID, clubID, rawQuery)
}

func redirectRaffle(w http.ResponseWriter, r *http.Request, raffleID int, clubID string, rawQuery string) {
	var redirectURL string
	if clubID != "" {
		redirectURL = fmt.Sprintf("/tracker/club/%s/raffle/%d", clubID, raffleID)
	} else {
		redirectURL = fmt.Sprintf("/tracker/raffle/%d", raffleID)
	}
	http.Redirect(w, r, withQuery(redirectURL, rawQuery), http.StatusSeeOther)
}

func withQuery(url string, query string) string {
	if query == "" {
		return url
	}
	return fmt.Sprintf("%s?%s", url, query)
}
