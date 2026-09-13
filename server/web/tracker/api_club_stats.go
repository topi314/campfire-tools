package tracker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/eventcategory"
	"github.com/topi314/campfire-tools/server/database"
)

const (
	loyaltyActiveThreshold = 5
	topEventsLimit         = 5
	maxLoyaltyTiers        = 6
	coreTierMinFraction    = 0.33 // 33% of top check-ins
)

type APIClubStatsResponse struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Stats       APIClubStats `json:"stats"`
}

type APIClubStats struct {
	GeneratedFrom string              `json:"generated_from"`
	DataAsOf      string              `json:"data_as_of"`
	Club          APIClubStatsClub    `json:"club"`
	Totals        APIClubStatsTotals  `json:"totals"`
	Monthly       []APIClubStatsMonth `json:"monthly"`
	TopEvents     []APIClubStatsEvent `json:"top_events"`
	EventTypes    []APIClubStatsType  `json:"event_types"`
	Loyalty       APIClubStatsLoyalty `json:"loyalty"`
}

type APIClubStatsClub struct {
	Name string `json:"name"`
}

type APIClubStatsTotals struct {
	Events              int                `json:"events"`
	UniqueParticipants  int                `json:"unique_participants"`
	TotalRSVPs          int                `json:"total_rsvps"`
	TotalCheckIns       int                `json:"total_check_ins"`
	TotalAccepted       int                `json:"total_accepted"`
	TotalDeclined       int                `json:"total_declined"`
	AvgCheckInsPerEvent int                `json:"avg_check_ins_per_event"`
	CheckInRate         float64            `json:"check_in_rate"`
	FirstEventDate      *time.Time         `json:"first_event_date,omitempty"`
	LastEventDate       *time.Time         `json:"last_event_date,omitempty"`
	BiggestEvent        *APIClubStatsEvent `json:"biggest_event,omitempty"`
	FirstEvent          *APIClubStatsEvent `json:"first_event,omitempty"`
}

type APIClubStatsMonth struct {
	Month                  string `json:"month"`
	Events                 int    `json:"events"`
	CheckIns               int    `json:"check_ins"`
	RSVPs                  int    `json:"rsvps"`
	NewParticipants        int    `json:"new_participants"`
	CumulativeParticipants int    `json:"cumulative_participants"`
}

type APIClubStatsEvent struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Date     time.Time `json:"date"`
	Address  string    `json:"address"`
	RSVPs    int       `json:"rsvps"`
	CheckIns int       `json:"check_ins"`
	Accepted int       `json:"accepted"`
	Declined int       `json:"declined"`
	Type     string    `json:"type"`
}

type APIClubStatsType struct {
	Type        string `json:"type"`
	Count       int    `json:"count"`
	CheckIns    int    `json:"check_ins"`
	AvgCheckIns int    `json:"avg_check_ins"`
}

type APIClubStatsLoyalty struct {
	EverCheckedIn   int                      `json:"ever_checked_in"`
	ActiveThreshold int                      `json:"active_threshold"`
	ActiveMembers   int                      `json:"active_members"`
	MedianActive    int                      `json:"median_active"`
	AvgActive       int                      `json:"avg_active"`
	Tiers           []APIClubStatsTier       `json:"tiers"`
	Elite           APIClubStatsLoyaltyElite `json:"elite"`
}

type APIClubStatsTier struct {
	Key    string `json:"key"`
	Range  string `json:"range"`
	Casual bool   `json:"casual"`
	People int    `json:"people"`
}

type APIClubStatsLoyaltyElite struct {
	AtLeast50  int `json:"at_least_50"`
	AtLeast100 int `json:"at_least_100"`
	AtLeast150 int `json:"at_least_150"`
}

type loyaltyTierDef struct {
	Key      string
	Range    string
	Casual   bool
	Min      int
	Max      int
	Champion bool
}

func tierThreshold(max int, fraction float64) int {
	if max <= 1 {
		return 1
	}
	min := int(math.Ceil(fraction * float64(max)))
	if min < 1 {
		return 1
	}
	if min >= max {
		return max - 1
	}
	return min
}

func formatCheckInRange(min, max int) string {
	if min == max {
		return fmt.Sprintf("%d", min)
	}
	return fmt.Sprintf("%d–%d", min, max)
}

var loyaltyTierKeys = []struct {
	Key    string
	Casual bool
}{
	{Key: "intro", Casual: true},
	{Key: "casual", Casual: true},
	{Key: "regular", Casual: false},
	{Key: "core", Casual: false},
	{Key: "legend", Casual: false},
}

func buildLoyaltyTierDefs(topCheckIns int) []loyaltyTierDef {
	if topCheckIns <= 0 {
		return nil
	}

	champion := loyaltyTierDef{
		Key:      "champion",
		Range:    fmt.Sprintf("%d", topCheckIns),
		Champion: true,
	}

	if topCheckIns == 1 {
		return []loyaltyTierDef{champion}
	}

	defs := []loyaltyTierDef{
		{Key: "intro", Range: "1", Casual: true, Min: 1, Max: 1},
	}

	casualEnd := min(4, topCheckIns)
	if casualEnd >= 2 {
		defs = append(defs, loyaltyTierDef{
			Key: "casual", Range: formatCheckInRange(2, casualEnd), Casual: true, Min: 2, Max: casualEnd,
		})
	}

	// Reserve topCheckIns for champion; split 5..upperMax across regular, core, and legend.
	upperMax := topCheckIns - 1
	if upperMax < 5 {
		return append(defs, champion)
	}

	upperKeys := loyaltyTierKeys[2:]
	maxUpper := maxLoyaltyTiers - len(defs) - 1
	numUpper := min(len(upperKeys), maxUpper)

	start := 5
	for i := 0; i < numUpper && start <= upperMax; i++ {
		remaining := upperMax - start + 1
		bandsLeft := numUpper - i
		width := int(math.Ceil(float64(remaining) / float64(bandsLeft)))
		end := start + width - 1
		if i == numUpper-1 || end > upperMax {
			end = upperMax
		}

		key := upperKeys[i]
		defs = append(defs, loyaltyTierDef{
			Key:    key.Key,
			Range:  formatCheckInRange(start, end),
			Casual: key.Casual,
			Min:    start,
			Max:    end,
		})
		start = end + 1
	}

	shrinkCoreTierLowerBarrier(defs, topCheckIns)

	return append(defs, champion)
}

func shrinkCoreTierLowerBarrier(defs []loyaltyTierDef, topCheckIns int) {
	coreMin := tierThreshold(topCheckIns, coreTierMinFraction)
	if coreMin < 5 {
		return
	}

	var regularIdx, coreIdx, legendIdx = -1, -1, -1
	for i, def := range defs {
		switch def.Key {
		case "regular":
			regularIdx = i
		case "core":
			coreIdx = i
		case "legend":
			legendIdx = i
		}
	}
	if coreIdx == -1 || coreMin >= defs[coreIdx].Min {
		return
	}

	coreWidth := defs[coreIdx].Max - defs[coreIdx].Min + 1
	defs[coreIdx].Min = coreMin
	defs[coreIdx].Max = coreMin + coreWidth - 1
	defs[coreIdx].Range = formatCheckInRange(defs[coreIdx].Min, defs[coreIdx].Max)

	if regularIdx != -1 {
		defs[regularIdx].Max = coreMin - 1
		defs[regularIdx].Range = formatCheckInRange(defs[regularIdx].Min, defs[regularIdx].Max)
	}
	if legendIdx != -1 {
		defs[legendIdx].Min = defs[coreIdx].Max + 1
		defs[legendIdx].Range = formatCheckInRange(defs[legendIdx].Min, defs[legendIdx].Max)
	}
}

func (h *handler) APIClubStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clubID := r.PathValue("club_id")

	slog.InfoContext(ctx, "Received API club stats request",
		slog.String("url", r.URL.String()),
		slog.String("club_id", clubID),
	)

	if clubID == "" {
		http.Error(w, "Club ID is required", http.StatusBadRequest)
		return
	}

	if response, ok := h.clubStatsCache.get(clubID); ok {
		h.writeClubStatsResponse(ctx, w, response)
		return
	}

	response, err := h.computeClubStats(ctx, clubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to compute club stats", slog.Any("error", err), slog.String("club_id", clubID))
		http.Error(w, "Failed to compute club stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.clubStatsCache.set(clubID, response)
	h.writeClubStatsResponse(ctx, w, response)
}

func (h *handler) writeClubStatsResponse(ctx context.Context, w http.ResponseWriter, response APIClubStatsResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "Failed to encode club stats to JSON", slog.Any("error", err))
	}
}

func (h *handler) computeClubStats(ctx context.Context, clubID string) (APIClubStatsResponse, error) {
	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		return APIClubStatsResponse{}, err
	}

	totals, err := h.DB.GetClubStatsTotals(ctx, clubID)
	if err != nil {
		return APIClubStatsResponse{}, err
	}

	events, err := h.DB.GetClubStatsEvents(ctx, clubID)
	if err != nil {
		return APIClubStatsResponse{}, err
	}

	monthlyRows, err := h.DB.GetClubStatsMonthly(ctx, clubID)
	if err != nil {
		return APIClubStatsResponse{}, err
	}

	memberCheckIns, err := h.DB.GetClubMemberCheckInCounts(ctx, clubID)
	if err != nil {
		return APIClubStatsResponse{}, err
	}

	now := time.Now().UTC()
	stats := APIClubStats{
		GeneratedFrom: fmt.Sprintf("%d events", totals.Events),
		DataAsOf:      now.Format("2006-01-02"),
		Club: APIClubStatsClub{
			Name: club.Name,
		},
		Totals:     buildClubStatsTotals(totals, events),
		Monthly:    buildClubStatsMonthly(monthlyRows),
		TopEvents:  buildTopEvents(events),
		EventTypes: buildEventTypes(events),
		Loyalty:    buildLoyalty(memberCheckIns),
	}

	return APIClubStatsResponse{
		GeneratedAt: now,
		Stats:       stats,
	}, nil
}

func buildClubStatsTotals(totals *database.ClubStatsTotals, events []database.ClubStatsEventRow) APIClubStatsTotals {
	result := APIClubStatsTotals{
		Events:             totals.Events,
		UniqueParticipants: totals.UniqueParticipants,
		TotalRSVPs:         totals.TotalRSVPs,
		TotalCheckIns:      totals.TotalCheckIns,
		TotalAccepted:      totals.TotalAccepted,
		TotalDeclined:      totals.TotalDeclined,
	}

	if totals.Events > 0 {
		result.AvgCheckInsPerEvent = int(math.Round(float64(totals.TotalCheckIns) / float64(totals.Events)))
	}

	if totals.TotalRSVPs > 0 {
		result.CheckInRate = math.Round(float64(totals.TotalCheckIns)/float64(totals.TotalRSVPs)*1000) / 1000
	}

	if !totals.FirstEventDate.IsZero() {
		first := totals.FirstEventDate.UTC()
		result.FirstEventDate = &first
	}

	if !totals.LastEventDate.IsZero() {
		last := totals.LastEventDate.UTC()
		result.LastEventDate = &last
	}

	if len(events) > 0 {
		firstEvent := toAPIClubStatsEvent(events[0])
		result.FirstEvent = &firstEvent

		biggest := events[0]
		for _, event := range events[1:] {
			if event.CheckIns > biggest.CheckIns ||
				(event.CheckIns == biggest.CheckIns && event.RSVPs > biggest.RSVPs) {
				biggest = event
			}
		}
		biggestEvent := toAPIClubStatsEvent(biggest)
		result.BiggestEvent = &biggestEvent
	}

	return result
}

func buildClubStatsMonthly(rows []database.ClubStatsMonthRow) []APIClubStatsMonth {
	monthly := make([]APIClubStatsMonth, len(rows))
	cumulative := 0
	for i, row := range rows {
		cumulative += row.NewPeople
		monthly[i] = APIClubStatsMonth{
			Month:                  row.Month,
			Events:                 row.Events,
			CheckIns:               row.CheckIns,
			RSVPs:                  row.RSVPs,
			NewParticipants:        row.NewPeople,
			CumulativeParticipants: cumulative,
		}
	}
	return monthly
}

func buildTopEvents(events []database.ClubStatsEventRow) []APIClubStatsEvent {
	sorted := slices.Clone(events)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CheckIns != sorted[j].CheckIns {
			return sorted[i].CheckIns > sorted[j].CheckIns
		}
		if sorted[i].RSVPs != sorted[j].RSVPs {
			return sorted[i].RSVPs > sorted[j].RSVPs
		}
		return sorted[i].Time.After(sorted[j].Time)
	})

	limit := min(topEventsLimit, len(sorted))
	topEvents := make([]APIClubStatsEvent, limit)
	for i := range limit {
		topEvents[i] = toAPIClubStatsEvent(sorted[i])
	}
	return topEvents
}

func buildEventTypes(events []database.ClubStatsEventRow) []APIClubStatsType {
	typeAgg := make(map[string]*APIClubStatsType)
	for _, event := range events {
		eventType := apiEventTypeFromCategory(eventCategoryForStats(event))
		agg, ok := typeAgg[eventType]
		if !ok {
			agg = &APIClubStatsType{Type: eventType}
			typeAgg[eventType] = agg
		}
		agg.Count++
		agg.CheckIns += event.CheckIns
	}

	types := make([]APIClubStatsType, 0, len(typeAgg))
	for _, agg := range typeAgg {
		if agg.Count > 0 {
			agg.AvgCheckIns = int(math.Round(float64(agg.CheckIns) / float64(agg.Count)))
		}
		types = append(types, *agg)
	}

	sort.Slice(types, func(i, j int) bool {
		if types[i].CheckIns != types[j].CheckIns {
			return types[i].CheckIns > types[j].CheckIns
		}
		return types[i].Count > types[j].Count
	})

	return types
}

func buildLoyalty(memberCheckIns []database.ClubMemberCheckInCount) APIClubStatsLoyalty {
	sorted := slices.Clone(memberCheckIns)
	slices.SortFunc(sorted, func(a, b database.ClubMemberCheckInCount) int {
		if a.CheckIns != b.CheckIns {
			return b.CheckIns - a.CheckIns
		}
		return strings.Compare(a.MemberID, b.MemberID)
	})

	checkInCounts := make([]int, len(sorted))
	for i, member := range sorted {
		checkInCounts[i] = member.CheckIns
	}

	maxCheckIns := 0
	var championID string
	if len(sorted) > 0 {
		maxCheckIns = sorted[0].CheckIns
		championID = sorted[0].MemberID
	}

	tierDefs := buildLoyaltyTierDefs(maxCheckIns)
	tiers := make([]APIClubStatsTier, len(tierDefs))
	for i, def := range tierDefs {
		people := 0
		if def.Champion {
			people = 1
		} else {
			for _, member := range sorted {
				if member.MemberID == championID {
					continue
				}
				if member.CheckIns >= def.Min && member.CheckIns <= def.Max {
					people++
				}
			}
		}
		tiers[i] = APIClubStatsTier{
			Key:    def.Key,
			Range:  def.Range,
			Casual: def.Casual,
			People: people,
		}
	}

	activeMembers := 0
	elite50, elite100, elite150 := 0, 0, 0
	elite50Min := 50
	elite100Min := 100
	elite150Min := 150
	if maxCheckIns > 0 {
		if scaled := int(math.Ceil(float64(maxCheckIns) * 0.25)); scaled > elite50Min {
			elite50Min = scaled
		}
		if scaled := int(math.Ceil(float64(maxCheckIns) * 0.50)); scaled > elite100Min {
			elite100Min = scaled
		}
		if scaled := int(math.Ceil(float64(maxCheckIns) * 0.75)); scaled > elite150Min {
			elite150Min = scaled
		}
	}
	for _, count := range checkInCounts {
		if count >= loyaltyActiveThreshold {
			activeMembers++
		}
		if count >= elite50Min {
			elite50++
		}
		if count >= elite100Min {
			elite100++
		}
		if count >= elite150Min {
			elite150++
		}
	}

	median := 0
	avg := 0
	if len(checkInCounts) > 0 {
		countsSorted := slices.Clone(checkInCounts)
		slices.Sort(countsSorted)
		mid := len(countsSorted) / 2
		if len(countsSorted)%2 == 0 {
			median = (countsSorted[mid-1] + countsSorted[mid]) / 2
		} else {
			median = countsSorted[mid]
		}

		sum := 0
		for _, count := range countsSorted {
			sum += count
		}
		avg = int(math.Round(float64(sum) / float64(len(countsSorted))))
	}

	return APIClubStatsLoyalty{
		EverCheckedIn:   len(checkInCounts),
		ActiveThreshold: loyaltyActiveThreshold,
		ActiveMembers:   activeMembers,
		MedianActive:    median,
		AvgActive:       avg,
		Tiers:           tiers,
		Elite: APIClubStatsLoyaltyElite{
			AtLeast50:  elite50,
			AtLeast100: elite100,
			AtLeast150: elite150,
		},
	}
}

func toAPIClubStatsEvent(event database.ClubStatsEventRow) APIClubStatsEvent {
	return APIClubStatsEvent{
		ID:       event.ID,
		Name:     event.Name,
		Date:     event.Time.UTC(),
		Address:  event.Address,
		RSVPs:    event.RSVPs,
		CheckIns: event.CheckIns,
		Accepted: event.Accepted,
		Declined: event.Declined,
		Type:     apiEventTypeFromCategory(eventCategoryForStats(event)),
	}
}

func eventCategoryForStats(event database.ClubStatsEventRow) string {
	if event.Category != "" {
		return event.Category
	}
	return eventCategoryFromName(event.CampfireLiveEventName)
}

func apiEventTypeFromCategory(category string) string {
	if category == EventCategoryOther || category == EventCategoryNoEvent || !eventcategory.IsPreset(category) {
		return "other"
	}
	return strings.ReplaceAll(strings.ToLower(category), " ", "_")
}

func apiEventTypeFromName(liveEventName string) string {
	return apiEventTypeFromCategory(eventCategoryFromName(liveEventName))
}
