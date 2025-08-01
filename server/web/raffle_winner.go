package web

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

	raffleIDStr := r.PathValue("raffle_id")
	memberID := r.PathValue("member_id")

	slog.InfoContext(ctx, "Received raffle winner request",
		slog.String("url", r.URL.String()),
		slog.String("raffle_id", raffleIDStr),
		slog.String("member_id", memberID),
	)

	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if _, err = h.DB.GetRaffleByID(ctx, raffleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, "Failed to get raffle: "+err.Error())
		return
	}

	if err = h.DB.ConfirmRaffleWinner(ctx, raffleID, memberID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "Raffle winner not found", slog.String("member_id", memberID), slog.Int("raffle_id", raffleID))
			h.renderRaffleResult(w, r, database.Raffle{}, "Raffle winner not found")
			return
		}
		slog.ErrorContext(ctx, "Failed to confirm raffle winner in database", slog.Any("err", err))
		h.renderRaffleResult(w, r, database.Raffle{}, "Failed to confirm raffle winner: "+err.Error())
		return
	}

	redirectRaffle(w, r, raffleID, r.URL.RawQuery)
}

func redirectRaffle(w http.ResponseWriter, r *http.Request, raffleID int, rawQuery string) {
	http.Redirect(w, r, withQuery(fmt.Sprintf("/raffle/%d", raffleID), rawQuery), http.StatusSeeOther)
}

func withQuery(url string, query string) string {
	if query == "" {
		return url
	}
	return fmt.Sprintf("%s?%s", url, query)
}
