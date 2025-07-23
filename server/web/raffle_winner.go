package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (h *handler) RaffleWinner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form data", slog.Any("err", err))
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	raffleIDStr := r.PathValue("raffle_id")

	slog.InfoContext(ctx, "Received raffle winner request", slog.String("url", r.URL.String()), slog.String("raffle_id", raffleIDStr))

	memberID := r.FormValue("member_id")
	winnersStr := r.FormValue("winners")

	raffleID, err := strconv.Atoi(raffleIDStr)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	raffle, err := h.DB.GetRaffleByID(ctx, raffleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to get raffle from database", slog.Any("err", err))
		h.renderRaffle(w, r, "Failed to get raffle: "+err.Error())
		return
	}

	if err = h.DB.InsertRaffleWinner(ctx, raffleID, memberID); err != nil {
		slog.ErrorContext(ctx, "Failed to insert raffle winner into database", slog.Any("err", err))
		h.renderRaffle(w, r, "Failed to insert raffle winner: "+err.Error())
		return
	}

	eg, egCtx := errgroup.WithContext(ctx)
	var winners []campfire.Member
	var mu sync.Mutex
	for _, eventID := range raffle.Events {
		eg.Go(func() error {
			event, err := h.fetchEvent(egCtx, eventID)
			if err != nil {
				return fmt.Errorf("failed to fetch event %q: %w", eventID, err)
			}

			mu.Lock()
			defer mu.Unlock()

			for _, member := range mem

			for _, edge := range event.Members.Edges {
				if edge.Node.ID == memberID {
					winners = append(winners, edge.Node)
					break
				}
			}

			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for raffle", slog.Any("err", err))
		h.renderRaffle(w, r, "Failed to fetch events: "+err.Error())
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "raffle_result.gohtml", RaffleResultVars{}); err != nil {
		slog.ErrorContext(ctx, "Failed to render raffle result template", slog.Any("err", err))
	}
}
