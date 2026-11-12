package server

import (
	"bytes"
	"html/template"
	"testing"
	"time"

	"github.com/topi314/campfire-tools/server/database"
	"github.com/topi314/campfire-tools/server/web/models"
)

func TestChecklistTemplatesExecute(t *testing.T) {
	root := template.New("templates").Funcs(templateFuncs).Option("missingkey=error")
	template.Must(root.New("live_reload_script").Parse(""))
	tpl := template.Must(root.ParseFS(
		templates,
		"web/templates/*.gohtml",
		"web/tracker/templates/*.gohtml",
		"web/rewards/templates/*.gohtml",
	))

	type summary struct {
		ID        int
		Title     string
		URL       string
		Received  int
		Total     int
		CreatedAt time.Time
	}
	mustExecute(t, tpl, "checklist_index.gohtml", struct {
		Checklists []summary
		Error      string
	}{
		Checklists: []summary{{
			ID:        1,
			Title:     "Stickers",
			URL:       "/checklist/1",
			Received:  1,
			Total:     2,
			CreatedAt: time.Now(),
		}},
	})

	type person struct {
		Member         models.Member
		SearchDisplay  string
		SearchUsername string
		Received       bool
		ReceivedAtUnix int64
		ReceivedAt     time.Time
		HasReceivedAt  bool
		Marker         models.DiscordUser
		HasMarker      bool
	}
	type notice struct {
		Person string
		Marker string
		At     time.Time
		HasAt  bool
	}
	type addResults struct {
		ChecklistID int
		Query       string
		Members     []models.Member
		OnList      map[string]bool
	}
	page := struct {
		ID            int
		Title         string
		ShareURL      string
		EntriesURL    string
		BackURL       string
		CanEdit       bool
		OnlyCheckedIn bool
		People        []person
		Received      int
		Total         int
		Events        []models.Event
		HasNotice     bool
		Notice        notice
		SwapNotice    bool
		Error         string
		Add           addResults
	}{
		ID:            1,
		Title:         "Stickers",
		ShareURL:      "http://example.test/checklist/share/token",
		EntriesURL:    "/checklist/1/entries",
		BackURL:       "/checklist",
		CanEdit:       true,
		OnlyCheckedIn: true,
		People: []person{
			{
				Member:         models.Member{ID: "m1", Username: "jdoe_smith", DisplayName: "Jordan"},
				SearchDisplay:  "jordan",
				SearchUsername: "jdoe_smith",
				Received:       true,
				ReceivedAtUnix: time.Now().Unix(),
				ReceivedAt:     time.Now(),
				HasReceivedAt:  true,
				HasMarker:      true,
				Marker: models.NewDiscordUser(database.DiscordUser{
					ID:          "d1",
					Username:    "sam",
					DisplayName: "Sam",
				}),
			},
			{
				Member:         models.Member{ID: "m2", Username: "alex_q", DisplayName: "Alex"},
				SearchDisplay:  "alex",
				SearchUsername: "alex_q",
			},
			{
				Member:         models.Member{ID: "m3", Username: "jds_user", DisplayName: "Jamie Doe"},
				SearchDisplay:  "jamie doe",
				SearchUsername: "jds_user",
			},
		},
		Received:  1,
		Total:     3,
		Events:    []models.Event{{ID: "e1", Name: "Meetup", URL: "/event/e1"}},
		HasNotice: true,
		Notice: notice{
			Person: "Jordan",
			Marker: "Sam",
			At:     time.Now(),
			HasAt:  true,
		},
		Error: "already",
		Add: addResults{
			ChecklistID: 1,
			Query:       "jo",
			Members:     []models.Member{{ID: "m2", Username: "other", DisplayName: "Other"}},
			OnList:      map[string]bool{"m1": true},
		},
	}
	mustExecute(t, tpl, "checklist.gohtml", page)
	page.SwapNotice = true
	mustExecute(t, tpl, "checklist_entries", page)
	mustExecute(t, tpl, "checklist_add_results", page.Add)

	type creator struct {
		ID       string
		Username string
	}
	type quarter struct {
		Value string
		Name  string
	}
	mustExecute(t, tpl, "tracker_club_checklist.gohtml", struct {
		models.Club
		FilterURL             string
		From                  time.Time
		To                    time.Time
		OnlyCAEvents          bool
		Quarters              []quarter
		EventCreators         []creator
		SelectedEventCreator  string
		CategoryOptions       []string
		SelectedEventCategory string
		Events                []models.Event
		Error                 string
	}{
		Club:      models.Club{ID: "club", Name: "Club", URL: "/tracker/club/club"},
		FilterURL: "/tracker/club/club/checklist",
		Events:    []models.Event{{ID: "e1", Name: "Meetup", CheckIns: 3}},
		Error:     "nope",
	})
}

func mustExecute(t *testing.T, tpl *template.Template, name string, data any) {
	t.Helper()
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if buf.Len() == 0 {
		t.Fatalf("%s rendered empty", name)
	}
}
