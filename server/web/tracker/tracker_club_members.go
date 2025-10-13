package tracker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/xtime"
)

type TrackerClubMembersVars struct {
	Club
	EventsFilter

	Members []TopMember
}

func (h *handler) TrackerClubMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	from := parseTimeQuery(query, "from", time.Time{})
	to := parseTimeQuery(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the day
	}
	onlyCAEvents := parseBoolQuery(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	members, err := h.DB.GetTopMembersByClub(ctx, clubID, from, to, onlyCAEvents, eventCreator, -1)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerMembers := make([]TopMember, len(members))
	for i, member := range members {
		trackerMembers[i] = newTopMember(member, clubID, 32)
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_members.gohtml", TrackerClubMembersVars{
		Club: newClub(*club),
		EventsFilter: EventsFilter{
			FilterURL:            r.URL.Path,
			From:                 from,
			To:                   to,
			OnlyCAEvents:         onlyCAEvents,
			Quarters:             xtime.GetQuarters(),
			EventCreators:        eventCreators,
			SelectedEventCreator: eventCreator,
		},
		Members: trackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club members template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}

func (h *handler) getEventCreators(ctx context.Context, clubID string) ([]Member, error) {
	members, err := h.DB.GetClubEventCreators(ctx, clubID)
	if err != nil {
		return nil, err
	}

	eventCreators := make([]Member, len(members))
	for i, m := range members {
		eventCreators[i] = newMember(m, clubID, 32)
	}
	return eventCreators, nil
}
