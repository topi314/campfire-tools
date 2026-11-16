package tracker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

const (
	checklistEventLimit = 50
	checklistTitleLimit = 200
)

type ChecklistSummary struct {
	ID        int
	Title     string
	URL       string
	Received  int
	Total     int
	CreatedAt time.Time
}

type ChecklistIndexVars struct {
	Checklists []ChecklistSummary
	Error      string
}

type ChecklistPerson struct {
	Member         models.Member
	SearchDisplay  string
	SearchUsername string
	Received       bool
	ReceivedAtUnix int64
	ReceivedAt     time.Time
	HasReceivedAt  bool
	Marker         models.DiscordUser
	HasMarker      bool
	CheckIns       int
}

type ChecklistNotice struct {
	Person string
	Marker string
	At     time.Time
	HasAt  bool
}

type ChecklistPage struct {
	ID              int
	Title           string
	ShareURL        string
	EntriesURL      string
	ActionBase      string
	ManageEventsURL string
	BackURL         string
	CanEdit         bool
	Public          bool
	OnlyCheckedIn   bool
	CheckedOpen     bool
	CreatedAt       time.Time
	People          []ChecklistPerson
	Received        int
	Total           int
	Events          []models.Event
	Users           []models.DiscordUser
	HasNotice       bool
	Notice          ChecklistNotice
	SwapNotice      bool
	Error           string
}

type TrackerClubChecklistVars struct {
	models.Club
	EventsFilter
	Events []models.Event
	Error  string
}

func (h *handler) ChecklistIndex(w http.ResponseWriter, r *http.Request) {
	h.renderChecklistIndex(w, r, strings.TrimSpace(r.URL.Query().Get("error")))
}

func (h *handler) renderChecklistIndex(w http.ResponseWriter, r *http.Request, errorMessage string) {
	if strings.HasPrefix(r.URL.Path, "/tracker/club/") {
		h.renderTrackerClubChecklist(w, r, errorMessage)
		return
	}

	session := auth.GetSession(r)
	ctx := r.Context()
	var checklists []ChecklistSummary
	if session.UserID != "" {
		rows, err := h.DB.GetChecklistsByUserID(ctx, session.UserID)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get checklists", slog.Any("err", err))
			http.Error(w, "Failed to get checklists: "+err.Error(), http.StatusInternalServerError)
			return
		}

		checklists = make([]ChecklistSummary, len(rows))
		for i, row := range rows {
			checklists[i] = ChecklistSummary{
				ID:        row.ID,
				Title:     row.Title,
				URL:       checklistPath(row.Token),
				Received:  row.Received,
				Total:     row.Total,
				CreatedAt: row.CreatedAt,
			}
		}
	}

	if err := h.Templates().ExecuteTemplate(w, "checklist_index.gohtml", ChecklistIndexVars{
		Checklists: checklists,
		Error:      errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render checklist index", slog.Any("err", err))
	}
}

func (h *handler) TrackerClubChecklist(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubChecklist(w, r, "")
}

func (h *handler) renderTrackerClubChecklist(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	from := xquery.ParseTime(query, "from", time.Time{})
	to := xquery.ParseTime(query, "to", time.Time{})
	if !to.IsZero() {
		to = to.Add(time.Hour*23 + time.Minute*59 + time.Second*59)
	}
	onlyCAEvents := xquery.ParseBool(query, "only-ca-events", false)
	eventCreator := query.Get("event-creator")
	eventCategory := query.Get("event-category")

	club, err := h.DB.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID, from, to, onlyCAEvents, eventCreator, eventCategory)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club checklist", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	clubModel := models.NewClub(*club)
	eventClubAvatarURL := models.ImageURL(club.Club.AvatarURL, 32)
	trackerEvents := make([]models.Event, len(events))
	for i, event := range events {
		trackerEvents[i] = models.NewEventWithCheckIns(event, 32, eventClubAvatarURL)
	}

	eventCreators, err := h.getEventCreators(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event creators for club checklist", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event creators: "+err.Error(), http.StatusInternalServerError)
		return
	}

	eventCategories, err := h.DB.GetClubEventCategories(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch event categories for club checklist", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch event categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_checklist.gohtml", TrackerClubChecklistVars{
		Club: clubModel,
		EventsFilter: EventsFilter{
			FilterURL:             r.URL.Path,
			From:                  from,
			To:                    to,
			OnlyCAEvents:          onlyCAEvents,
			Quarters:              xtime.GetQuarters(),
			EventCreators:         eventCreators,
			SelectedEventCreator:  eventCreator,
			CategoryOptions:       eventCategories,
			SelectedEventCategory: eventCategory,
		},
		Events: trackerEvents,
		Error:  errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render club checklist", slog.Any("err", err))
	}
}

func (h *handler) CreateChecklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := auth.GetSession(r)

	if err := r.ParseForm(); err != nil {
		h.renderChecklistIndex(w, r, "Failed to parse form data")
		return
	}

	title, titleErr := cleanChecklistTitle(r.FormValue("title"))
	if titleErr != "" {
		h.renderChecklistIndex(w, r, titleErr)
		return
	}

	rawEvents := checklistEventInputs(r)
	if len(rawEvents) > checklistEventLimit {
		h.renderChecklistIndex(w, r, fmt.Sprintf("please limit the number of events to %d, got %d.", checklistEventLimit, len(rawEvents)))
		return
	}

	eventIDs, err := h.resolveChecklistEventIDs(ctx, rawEvents)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to resolve checklist events", slog.Any("err", err))
		h.renderChecklistIndex(w, r, "Failed to fetch events: "+err.Error())
		return
	}

	var createdBy sql.NullString
	if session.UserID != "" {
		createdBy = sql.NullString{String: session.UserID, Valid: true}
	}

	checklistID, err := h.insertChecklist(ctx, database.Checklist{
		Title:         title,
		CreatedBy:     createdBy,
		OnlyCheckedIn: xquery.ParseBool(r.Form, "only_checked_in", false),
		Events:        eventIDs,
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to insert checklist", slog.Any("err", err))
		h.renderChecklistIndex(w, r, "Failed to create checklist: "+err.Error())
		return
	}

	if session.UserID != "" {
		if err = h.DB.AddChecklistUser(ctx, checklistID, session.UserID); err != nil {
			slog.ErrorContext(ctx, "Failed to add checklist creator", slog.Any("err", err))
			h.renderChecklistIndex(w, r, "Failed to create checklist: "+err.Error())
			return
		}
	}

	checklist, err := h.DB.GetChecklistByID(ctx, checklistID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load new checklist", slog.Any("err", err))
		http.Redirect(w, r, "/checklist?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}
	dest := "/checklist/share/" + checklist.Token
	if session.UserID != "" {
		dest = checklistHere(r, checklist)
	}
	if err = h.refreshChecklistEvents(ctx, *checklist); err != nil {
		slog.ErrorContext(ctx, "Failed to import checklist people", slog.Any("err", err))
		http.Redirect(w, r, dest+"?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (h *handler) ChecklistShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, ok := h.checklistByToken(w, r)
	if !ok {
		return
	}

	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return
	}
	if err := h.DB.AddChecklistUser(ctx, checklist.ID, session.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to record checklist open", slog.Any("err", err))
		http.Error(w, "Failed to open checklist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, checklistPath(checklist.Token), http.StatusSeeOther)
}

func (h *handler) GetChecklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}
	if _, err := strconv.Atoi(r.PathValue("checklist_id")); err == nil {
		target := checklistPath(checklist.Token)
		if query := r.URL.RawQuery; query != "" {
			target += "?" + query
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}

	syncErr := h.syncChecklistPeople(ctx, *checklist)
	page, err := h.buildChecklistPage(ctx, r, *checklist, true, strings.TrimSpace(r.URL.Query().Get("already")))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to build checklist", slog.Any("err", err))
		http.Error(w, "Failed to load checklist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	page.Error = strings.TrimSpace(r.URL.Query().Get("error"))
	if page.Error == "" {
		page.Error = syncErr
	}
	h.renderChecklistPage(w, r, page)
}

func (h *handler) ChecklistEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}

	h.syncChecklistPeople(ctx, *checklist)
	page, err := h.buildChecklistPage(ctx, r, *checklist, true, "")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to build checklist entries", slog.Any("err", err))
		http.Error(w, "Failed to load checklist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderChecklistEntries(w, r, page)
}

func (h *handler) PostChecklistToggle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, userID, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	memberID := strings.TrimSpace(r.PathValue("member_id"))
	wantReceived := r.FormValue("received") == "true"
	now := time.Now()

	var updated bool
	var err error
	if wantReceived {
		updated, err = h.DB.MarkChecklistEntryReceived(ctx, checklist.ID, memberID, userID, now)
	} else {
		err = h.DB.UnmarkChecklistEntryReceived(ctx, checklist.ID, memberID)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "Failed to toggle checklist entry", slog.String("member_id", memberID), slog.Any("err", err))
		http.Error(w, "Failed to update checklist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !isHTMX(r) {
		target := checklistHere(r, checklist)
		if wantReceived && !updated {
			target += "?already=" + url.QueryEscape(memberID)
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}

	noticeMemberID := ""
	if wantReceived && !updated {
		noticeMemberID = memberID
	}
	page, err := h.buildChecklistPage(ctx, r, *checklist, true, noticeMemberID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to render checklist toggle", slog.Any("err", err))
		http.Error(w, "Failed to load checklist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	page.SwapNotice = true
	h.renderChecklistEntries(w, r, page)
}

func (h *handler) PostChecklistTitle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+parse+form", http.StatusSeeOther)
		return
	}

	title, titleErr := cleanChecklistTitle(r.FormValue("title"))
	if titleErr != "" {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error="+url.QueryEscape(titleErr), http.StatusSeeOther)
		return
	}
	onlyCheckedIn := xquery.ParseBool(r.Form, "only_checked_in", false)
	if err := h.DB.UpdateChecklistSettings(ctx, checklist.ID, title, onlyCheckedIn); err != nil {
		slog.ErrorContext(ctx, "Failed to update checklist settings", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+save", http.StatusSeeOther)
		return
	}

	updated, err := h.DB.GetChecklistByID(ctx, checklist.ID)
	if err != nil {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}
	if err = h.refreshChecklistEvents(ctx, *updated); err != nil {
		slog.ErrorContext(ctx, "Failed to import people after checklist settings change", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, checklistEventsPage(r, checklist), http.StatusSeeOther)
}

func (h *handler) PostChecklistEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+parse+form", http.StatusSeeOther)
		return
	}

	rawEvents := checklistEventInputs(r)
	if len(rawEvents) == 0 {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Add+at+least+one+event", http.StatusSeeOther)
		return
	}
	if len(rawEvents) > checklistEventLimit {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error="+url.QueryEscape(fmt.Sprintf("please limit the number of events to %d", checklistEventLimit)), http.StatusSeeOther)
		return
	}

	eventIDs, err := h.resolveChecklistEventIDs(ctx, rawEvents)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to resolve checklist events", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+fetch+events", http.StatusSeeOther)
		return
	}
	if countUniqueEventIDs([]string(checklist.Events), eventIDs) > checklistEventLimit {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error="+url.QueryEscape(fmt.Sprintf("a checklist can include at most %d events", checklistEventLimit)), http.StatusSeeOther)
		return
	}
	if err = h.DB.AppendChecklistEvents(ctx, checklist.ID, eventIDs); err != nil {
		slog.ErrorContext(ctx, "Failed to append checklist events", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+add+events", http.StatusSeeOther)
		return
	}

	updated, err := h.DB.GetChecklistByID(ctx, checklist.ID)
	if err != nil {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}
	if err = h.refreshChecklistEvents(ctx, *updated); err != nil {
		slog.ErrorContext(ctx, "Failed to import people for new checklist events", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+import+people", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, checklistEventsPage(r, checklist), http.StatusSeeOther)
}

func (h *handler) PostChecklistEventsRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+parse+form", http.StatusSeeOther)
		return
	}

	eventIDs := r.Form["ids"]
	if eventID := strings.TrimSpace(r.FormValue("event_id")); eventID != "" {
		eventIDs = append(eventIDs, eventID)
	}
	if len(eventIDs) == 0 {
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Select+at+least+one+event", http.StatusSeeOther)
		return
	}
	if err := h.DB.RemoveChecklistEvents(ctx, checklist.ID, eventIDs); err != nil {
		slog.ErrorContext(ctx, "Failed to remove checklist event", slog.Any("err", err))
		http.Redirect(w, r, checklistEventsPage(r, checklist)+"?error=Failed+to+remove+event", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, checklistEventsPage(r, checklist), http.StatusSeeOther)
}

type ChecklistEventsPage struct {
	Title            string
	TitleAction      string
	BackURL          string
	FormAction       string
	RemoveFormAction string
	OnlyCheckedIn    bool
	Events           []models.Event
	Error            string
}

func (h *handler) GetChecklistEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checklist, _, ok := h.checklistEditor(w, r)
	if !ok {
		return
	}

	actionBase := checklistActionBase(r, checklist)
	if err := h.Templates().ExecuteTemplate(w, "checklist_events.gohtml", ChecklistEventsPage{
		Title:            checklist.Title,
		TitleAction:      actionBase + "/title",
		BackURL:          checklistHere(r, checklist),
		FormAction:       actionBase + "/events",
		RemoveFormAction: actionBase + "/events/remove",
		OnlyCheckedIn:    checklist.OnlyCheckedIn,
		Events:           h.checklistEventModels(ctx, checklist.Events),
		Error:            strings.TrimSpace(r.URL.Query().Get("error")),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render checklist events", slog.Any("err", err))
	}
}

func checklistIsPublic(checklist database.Checklist) bool {
	return !checklist.CreatedBy.Valid || checklist.CreatedBy.String == ""
}

func checklistHere(r *http.Request, checklist *database.Checklist) string {
	return checklistPath(checklist.Token)
}

func checklistEventsPage(r *http.Request, checklist *database.Checklist) string {
	return checklistPath(checklist.Token) + "/events/manage"
}

func checklistActionBase(r *http.Request, checklist *database.Checklist) string {
	return checklistPath(checklist.Token)
}

func (h *handler) checklistEditor(w http.ResponseWriter, r *http.Request) (*database.Checklist, string, bool) {
	checklist, session, ok := h.authorizedChecklist(w, r)
	if !ok {
		return nil, "", false
	}
	return checklist, session.UserID, true
}

func (h *handler) authorizedChecklist(w http.ResponseWriter, r *http.Request) (*database.Checklist, database.SessionWithUser, bool) {
	session := auth.GetSession(r)
	if session.UserID == "" {
		h.forceLogin(w, r)
		return nil, session, false
	}

	ctx := r.Context()
	checklist, err := h.checklistByPath(ctx, r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return nil, session, false
		}
		slog.ErrorContext(ctx, "Failed to get checklist", slog.Any("err", err))
		http.Error(w, "Failed to get checklist: "+err.Error(), http.StatusInternalServerError)
		return nil, session, false
	}

	member, err := h.DB.IsChecklistUser(ctx, checklist.ID, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to check checklist access", slog.Any("err", err))
		http.Error(w, "Failed to get checklist: "+err.Error(), http.StatusInternalServerError)
		return nil, session, false
	}
	if !member {
		h.NotFound(w, r)
		return nil, session, false
	}
	return checklist, session, true
}

func (h *handler) checklistByToken(w http.ResponseWriter, r *http.Request) (*database.Checklist, bool) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		h.NotFound(w, r)
		return nil, false
	}

	checklist, err := h.DB.GetChecklistByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.NotFound(w, r)
			return nil, false
		}
		slog.ErrorContext(r.Context(), "Failed to get checklist by token", slog.Any("err", err))
		http.Error(w, "Failed to get checklist: "+err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	return checklist, true
}

func (h *handler) buildChecklistPage(ctx context.Context, r *http.Request, checklist database.Checklist, canEdit bool, noticeMemberID string) (ChecklistPage, error) {
	rows, err := h.DB.GetChecklistEntries(ctx, checklist.ID)
	if err != nil {
		return ChecklistPage{}, err
	}

	access, err := h.DB.GetChecklistUsers(ctx, checklist.ID)
	if err != nil {
		return ChecklistPage{}, err
	}
	users := make([]models.DiscordUser, 0, len(access))
	for _, user := range access {
		users = append(users, models.NewDiscordUser(user))
	}

	page := ChecklistPage{
		ID:            checklist.ID,
		Title:         checklist.Title,
		ShareURL:      absoluteURL(r, "/checklist/share/"+checklist.Token),
		BackURL:       "/checklist",
		CanEdit:       canEdit,
		Public:        checklistIsPublic(checklist),
		OnlyCheckedIn: checklist.OnlyCheckedIn,
		CheckedOpen:   xquery.ParseBool(r.URL.Query(), "checked", false) || xquery.ParseBool(r.Form, "checked", false),
		CreatedAt:     checklist.CreatedAt,
		People:        make([]ChecklistPerson, 0, len(rows)),
		Events:        h.checklistEventModels(ctx, checklist.Events),
		Users:         users,
	}
	page.ActionBase = checklistPath(checklist.Token)
	page.EntriesURL = page.ActionBase + "/entries/poll"
	page.ManageEventsURL = page.ActionBase + "/events/manage"

	for _, row := range rows {
		person := checklistPersonFromRow(row)
		page.Total++
		if person.Received {
			page.Received++
		}
		if noticeMemberID != "" && row.MemberID == noticeMemberID && row.Received {
			page.HasNotice = true
			page.Notice = ChecklistNotice{
				Person: person.Member.DisplayName,
				HasAt:  person.HasReceivedAt,
				At:     person.ReceivedAt,
			}
			if person.HasMarker {
				page.Notice.Marker = person.Marker.EffectiveName()
			}
		}
		page.People = append(page.People, person)
	}
	return page, nil
}

func (h *handler) renderChecklistPage(w http.ResponseWriter, r *http.Request, page ChecklistPage) {
	if err := h.Templates().ExecuteTemplate(w, "checklist.gohtml", page); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render checklist", slog.Any("err", err))
	}
}

func (h *handler) renderChecklistEntries(w http.ResponseWriter, r *http.Request, page ChecklistPage) {
	if err := h.Templates().ExecuteTemplate(w, "checklist_entries", page); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render checklist entries", slog.Any("err", err))
	}
}

func (h *handler) checklistEventModels(ctx context.Context, eventIDs []string) []models.Event {
	events := make([]models.Event, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		dbEvent, err := h.DB.GetEvent(ctx, eventID)
		if err != nil {
			events = append(events, models.Event{
				ID:   eventID,
				Name: eventID,
				URL:  "/event/" + eventID,
			})
			continue
		}
		event := models.NewEvent(dbEvent.Event, 32, "")
		event.URL = "/event/" + event.ID
		events = append(events, event)
	}
	return events
}

func (h *handler) syncChecklistPeople(ctx context.Context, checklist database.Checklist) string {
	if err := h.refreshChecklistEvents(ctx, checklist); err != nil {
		slog.ErrorContext(ctx, "Failed to refresh checklist people", slog.Int("checklist_id", checklist.ID), slog.Any("err", err))
		return "Failed to update people from events"
	}
	return ""
}

func (h *handler) refreshChecklistEvents(ctx context.Context, checklist database.Checklist) error {
	if len(checklist.Events) == 0 {
		return nil
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, eventID := range checklist.Events {
		eg.Go(func() error {
			if _, err := h.fetchEvent(egCtx, eventID); err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", eventID, err)
			}
			return nil
		})
	}
	return eg.Wait()
}

func (h *handler) resolveChecklistEventIDs(ctx context.Context, raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	eg, egCtx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	var ids []string
	for _, event := range raw {
		eg.Go(func() error {
			eventID, err := h.fetchEventID(egCtx, event)
			if err != nil {
				return fmt.Errorf("failed to fetch event id %q: %w", event, err)
			}
			mu.Lock()
			ids = append(ids, eventID)
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return uniqueStrings(ids), nil
}

func (h *handler) insertChecklist(ctx context.Context, checklist database.Checklist) (int, error) {
	if checklist.Events == nil {
		checklist.Events = pq.StringArray{}
	}

	var lastErr error
	for range 3 {
		token, err := newChecklistToken()
		if err != nil {
			return 0, err
		}
		checklist.Token = token
		id, err := h.DB.InsertChecklist(ctx, checklist)
		if err == nil {
			return id, nil
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			lastErr = err
			continue
		}
		return 0, err
	}
	if lastErr == nil {
		lastErr = errors.New("failed to create checklist")
	}
	return 0, lastErr
}

func checklistPersonFromRow(row database.ChecklistEntryRow) ChecklistPerson {
	member := models.NewImportedMember(database.Member{
		ID:          row.MemberID,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		AvatarURL:   row.AvatarURL,
	}, 32)
	person := ChecklistPerson{
		Member:         member,
		SearchDisplay:  strings.ToLower(member.DisplayName),
		SearchUsername: strings.ToLower(member.Username),
		Received:       row.Received,
		CheckIns:       row.CheckIns,
	}
	if row.ReceivedAt.Valid {
		person.HasReceivedAt = true
		person.ReceivedAt = row.ReceivedAt.Time
		person.ReceivedAtUnix = row.ReceivedAt.Time.Unix()
	}
	if row.MarkerID.Valid {
		person.HasMarker = true
		person.Marker = models.NewDiscordUser(database.DiscordUser{
			ID:          row.MarkerID.String,
			Username:    row.MarkerUsername.String,
			DisplayName: row.MarkerDisplayName.String,
			AvatarURL:   row.MarkerAvatarURL.String,
		})
	}
	return person
}

func checklistEventInputs(r *http.Request) []string {
	var events []string
	for _, event := range strings.Split(r.FormValue("events"), "\n") {
		event = strings.TrimSpace(event)
		if event != "" {
			events = append(events, event)
		}
	}
	for _, id := range r.Form["ids"] {
		id = strings.TrimSpace(id)
		if id != "" {
			events = append(events, id)
		}
	}
	return events
}

func cleanChecklistTitle(title string) (string, string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "Name is required"
	}
	if len([]rune(title)) > checklistTitleLimit {
		return "", fmt.Sprintf("Name must be %d characters or fewer", checklistTitleLimit)
	}
	return title, ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func newChecklistToken() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

func checklistPath(token string) string {
	return "/checklist/" + token
}

func checklistPathID(r *http.Request) int {
	id, _ := strconv.Atoi(r.PathValue("checklist_id"))
	return id
}

func (h *handler) checklistByPath(ctx context.Context, r *http.Request) (*database.Checklist, error) {
	value := strings.TrimSpace(r.PathValue("checklist_id"))
	if value == "" {
		return nil, sql.ErrNoRows
	}
	if _, err := strconv.Atoi(value); err == nil {
		return h.DB.GetChecklistByID(ctx, checklistPathID(r))
	}
	return h.DB.GetChecklistByToken(ctx, value)
}

func absoluteURL(r *http.Request, path string) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host + path
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
