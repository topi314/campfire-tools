package tracker

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

func (h *handler) TrackerEventStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	eventKey := query.Get("event")

	// Deduplicate and drop the empty selections coming from the growing club dropdowns.
	selected := make(map[string]bool)
	var clubIDs []string
	for _, id := range query["club_id"] {
		if id == "" || selected[id] {
			continue
		}
		selected[id] = true
		clubIDs = append(clubIDs, id)
	}

	event, eventOK := findConfiguredEvent(eventKey)
	var liveEventIDs []string
	if eventOK {
		liveEventIDs = event.LiveEventIDs()
	}
	runStats := len(clubIDs) > 0 && eventOK && len(liveEventIDs) > 0

	// The club list (for the dropdowns) and the RSVP data are independent, so
	// fetch them concurrently.
	var (
		clubRefs []database.ClubRef
		rsvps    []database.LiveEventMemberRSVP
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		clubRefs, err = h.DB.GetClubOptions(egCtx)
		return err
	})
	if runStats {
		eg.Go(func() error {
			var err error
			rsvps, err = h.DB.GetLiveEventRSVPs(egCtx, clubIDs, liveEventIDs)
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		http.Error(w, "Failed to fetch event stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	session := auth.GetSession(r)
	vars := models.EventStatsVars{
		User:             models.NewDiscordUser(session.DiscordUser),
		SelectedEventKey: eventKey,
	}

	// Selected clubs in the order they appear in the club list (stable matrix order).
	var selectedClubs []models.ClubOption
	for _, club := range clubRefs {
		c := models.ClubOption{
			ID:       club.ID,
			Name:     club.Name,
			Selected: selected[club.ID],
		}
		vars.Clubs = append(vars.Clubs, c)
		if c.Selected {
			selectedClubs = append(selectedClubs, c)
			vars.SelectedClubIDs = append(vars.SelectedClubIDs, c.ID)
			vars.ClubNames = append(vars.ClubNames, c.Name)
		}
	}

	for _, e := range ConfiguredEvents {
		vars.Events = append(vars.Events, models.EventOption{
			Key:      e.Key,
			Name:     e.Name,
			Selected: e.Key == eventKey,
		})
	}

	if len(clubIDs) > 0 && eventKey != "" {
		if !eventOK {
			vars.Errors = append(vars.Errors, "Unknown event selected.")
		} else if len(liveEventIDs) == 0 {
			vars.Errors = append(vars.Errors, "This event has no live event IDs configured yet.")
		}
	}

	if runStats && len(selectedClubs) > 0 {
		buildEventStats(&vars, event, selectedClubs, rsvps)
	}

	if err := h.Templates().ExecuteTemplate(w, "tracker_event_stats.gohtml", vars); err != nil {
		slog.ErrorContext(ctx, "Failed to render event stats template", slog.Any("err", err))
	}
}

// buildEventStats fills the combined, per-club and "who checked in where"
// sections of vars from the aggregated (club, live event, member) RSVP rows.
func buildEventStats(vars *models.EventStatsVars, event ConfiguredEvent, selectedClubs []models.ClubOption, rsvps []database.LiveEventMemberRSVP) {
	// Member view models are built once and shared across all sections. The
	// combined/where sections use a club-agnostic member URL.
	memberModels := make(map[string]models.Member)
	for _, rsvp := range rsvps {
		if _, ok := memberModels[rsvp.Member.ID]; !ok {
			memberModels[rsvp.Member.ID] = models.NewMember(rsvp.Member, "", 32)
		}
	}

	vars.EventName = event.Name
	vars.MultiClub = len(selectedClubs) > 1
	vars.Combined = buildClubStats("combined", "Combined", event, rsvps, memberModels)

	if vars.MultiClub {
		for i, club := range selectedClubs {
			var clubRows []database.LiveEventMemberRSVP
			for _, rsvp := range rsvps {
				if rsvp.ClubID == club.ID {
					clubRows = append(clubRows, rsvp)
				}
			}
			stats := buildClubStats(fmt.Sprintf("club-%d", i+1), club.Name, event, clubRows, memberModels)
			stats.ClubID = club.ID
			vars.PerClub = append(vars.PerClub, stats)
		}

		vars.Where = buildWhereRows(selectedClubs, rsvps, memberModels)
	}

	vars.HasResults = true
}

// buildClubStats computes the per-day, overall and attended-all metrics for a
// single scope (a club subset or the whole combined set). anchorPrefix keeps
// the collapsible list anchor IDs unique across scopes.
func buildClubStats(anchorPrefix string, name string, event ConfiguredEvent, rsvps []database.LiveEventMemberRSVP, memberModels map[string]models.Member) models.ClubStats {
	// byLiveEvent: live event ID -> member ID -> best status rank.
	byLiveEvent := make(map[string]map[string]int)
	for _, rsvp := range rsvps {
		set, ok := byLiveEvent[rsvp.LiveEventID]
		if !ok {
			set = make(map[string]int)
			byLiveEvent[rsvp.LiveEventID] = set
		}
		if rsvp.StatusRank > set[rsvp.Member.ID] {
			set[rsvp.Member.ID] = rsvp.StatusRank
		}
	}

	stats := models.ClubStats{Name: name}

	overallCheckIn := make(map[string]bool)
	overallAccepted := make(map[string]bool)
	checkedInDays := make(map[string]int)
	daysWithID := 0

	for i, day := range event.Days {
		if day.LiveEventID == "" {
			continue
		}
		daysWithID++

		var checkInIDs, acceptedIDs []string
		for memberID, rank := range byLiveEvent[day.LiveEventID] {
			if rank >= 1 {
				acceptedIDs = append(acceptedIDs, memberID)
				overallAccepted[memberID] = true
			}
			if rank >= 2 {
				checkInIDs = append(checkInIDs, memberID)
				overallCheckIn[memberID] = true
				checkedInDays[memberID]++
			}
		}

		stats.Days = append(stats.Days, models.DayStat{
			Label:    day.Label,
			CheckIns: metricGroup(fmt.Sprintf("%s-day-%d-checkins", anchorPrefix, i+1), day.Label+" - Check-Ins", checkInIDs, memberModels),
			Accepted: metricGroup(fmt.Sprintf("%s-day-%d-accepted", anchorPrefix, i+1), day.Label+" - Accepted", acceptedIDs, memberModels),
		})
	}

	stats.Overall = models.DayStat{
		Label:    "Overall (unique)",
		CheckIns: metricGroup(anchorPrefix+"-overall-checkins", "Overall Unique Check-Ins", keys(overallCheckIn), memberModels),
		Accepted: metricGroup(anchorPrefix+"-overall-accepted", "Overall Unique Accepted", keys(overallAccepted), memberModels),
	}

	var attendedAllIDs []string
	if daysWithID > 0 {
		for memberID, days := range checkedInDays {
			if days == daysWithID {
				attendedAllIDs = append(attendedAllIDs, memberID)
			}
		}
	}
	stats.AttendedAll = metricGroup(anchorPrefix+"-attended-all", "Checked In On All Days", attendedAllIDs, memberModels)

	return stats
}

// buildWhereRows produces one row per member that checked in at any of the
// selected clubs, marking which clubs they checked in at (any day).
func buildWhereRows(selectedClubs []models.ClubOption, rsvps []database.LiveEventMemberRSVP, memberModels map[string]models.Member) []models.WhereRow {
	clubIndex := make(map[string]int, len(selectedClubs))
	for i, club := range selectedClubs {
		clubIndex[club.ID] = i
	}

	checkedIn := make(map[string][]bool)
	for _, rsvp := range rsvps {
		if rsvp.StatusRank < 2 {
			continue
		}
		idx, ok := clubIndex[rsvp.ClubID]
		if !ok {
			continue
		}
		row, ok := checkedIn[rsvp.Member.ID]
		if !ok {
			row = make([]bool, len(selectedClubs))
			checkedIn[rsvp.Member.ID] = row
		}
		row[idx] = true
	}

	rows := make([]models.WhereRow, 0, len(checkedIn))
	for memberID, marks := range checkedIn {
		count := 0
		for _, m := range marks {
			if m {
				count++
			}
		}
		rows = append(rows, models.WhereRow{
			Member:    memberModels[memberID],
			CheckedIn: marks,
			ClubCount: count,
			MultiClub: count > 1,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ClubCount != rows[j].ClubCount {
			return rows[i].ClubCount > rows[j].ClubCount
		}
		if rows[i].Member.DisplayName != rows[j].Member.DisplayName {
			return rows[i].Member.DisplayName < rows[j].Member.DisplayName
		}
		return rows[i].Member.ID < rows[j].Member.ID
	})

	return rows
}

func metricGroup(anchorID, title string, memberIDs []string, memberModels map[string]models.Member) models.MetricGroup {
	members := make([]models.Member, 0, len(memberIDs))
	for _, id := range memberIDs {
		members = append(members, memberModels[id])
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].DisplayName != members[j].DisplayName {
			return members[i].DisplayName < members[j].DisplayName
		}
		return members[i].ID < members[j].ID
	})

	return models.MetricGroup{
		AnchorID: anchorID,
		Title:    title,
		Count:    len(members),
		Members:  members,
	}
}

func keys(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}
