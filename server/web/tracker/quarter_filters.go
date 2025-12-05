package tracker

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/web/models"
)

type EventsFilter struct {
	FilterURL    string
	From         time.Time
	To           time.Time
	OnlyCAEvents bool

	Quarters             []xtime.Quarter
	EventCreators        []models.Member
	SelectedEventCreator string
}

func (h *handler) GetQuarterFilters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	quarter := query.Get("quarter")

	from, to := xtime.GetRangeFromQuarter(quarter)

	if err := h.Templates().ExecuteTemplate(w, "tracker_quarter_filters.gohtml", EventsFilter{
		FilterURL: r.URL.Path,
		From:      from,
		To:        to,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render quarter filters template", slog.Any("err", err))
	}
}
