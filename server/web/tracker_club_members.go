package web

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"
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

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	members, err := h.DB.GetTopMembersByClub(ctx, clubID, from, to, onlyCAEvents, -1)
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
			FilterURL:    r.URL.Path,
			From:         from,
			To:           to,
			OnlyCAEvents: onlyCAEvents,
			Quarters:     quarters,
		},
		Members: trackerMembers,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club members template", slog.String("club_id", clubID), slog.Any("err", err))
	}
}
