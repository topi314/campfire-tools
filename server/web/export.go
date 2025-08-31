package web

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

const (
	FieldUserID                            = "user_id"
	FieldUsername                          = "username"
	FieldDisplayName                       = "display_name"
	FieldRSVPStatus                        = "rsvp_status"
	FieldEventID                           = "event_id"
	FieldEventName                         = "event_name"
	FieldEventURL                          = "event_url"
	FieldEventTime                         = "event_time"
	FieldEventClubID                       = "event_club_id"
	FieldEventCreatorUserID                = "event_creator_user_id"
	FieldEventCreatorUsername              = "event_creator_username"
	FieldEventCreatorDisplayName           = "event_creator_display_name"
	FieldEventDiscordInterested            = "event_discord_interested"
	FieldEventCreatedByCommunityAmbassador = "event_created_by_community_ambassador"
	FieldEventCampfireLiveEventID          = "event_campfire_live_event_id"
	FieldEventCampfireLiveEventName        = "event_campfire_live_event_name"
)

var defaultFields = []string{
	FieldUserID,
	FieldUsername,
	FieldDisplayName,
	FieldRSVPStatus,
	FieldEventID,
	FieldEventName,
}

type ExportVars struct {
	SelectedEventID string
	Error           string
}

func (h *handler) Export(w http.ResponseWriter, r *http.Request) {
	h.renderExport(w, r, "")
}

func (h *handler) renderExport(w http.ResponseWriter, r *http.Request, errorMessage string) {
	if strings.HasPrefix(r.URL.Path, "/tracker/club/") {
		h.renderTrackerClubExport(w, r, errorMessage)
		return
	}

	query := r.URL.Query()
	ctx := r.Context()

	eventID := query.Get("event")

	if err := h.Templates().ExecuteTemplate(w, "export.gohtml", ExportVars{
		SelectedEventID: eventID,
		Error:           errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render export template", slog.Any("err", err))
	}
}

func (h *handler) DoExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form", slog.Any("err", err))
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	events := strings.TrimSpace(r.FormValue("events"))
	eventIDs := r.Form["ids"]
	includeMissingMembers := parseBoolQuery(r.Form, "include_missing_members", false)
	combineCSVs := parseBoolQuery(r.Form, "combine_csv", false)
	includedFields := r.Form["included_fields"]
	if len(includedFields) == 0 {
		includedFields = defaultFields
	}

	slog.Info("Received export request",
		slog.String("url", r.URL.String()),
		slog.String("events", events),
		slog.Bool("include_missing_members", includeMissingMembers),
		slog.Bool("combine_csv", combineCSVs),
		slog.Any("included_fields", includedFields),
	)

	if events == "" && len(eventIDs) == 0 {
		h.renderExport(w, r, "Missing 'events' parameter")
		return
	}

	var allEvents []string
	for _, event := range strings.Split(events, "\n") {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}
		allEvents = append(allEvents, event)
	}

	if len(allEvents) > 50 {
		h.renderExport(w, r, fmt.Sprintf("please limit the number of URLs to 50, got %d.", len(allEvents)))
		return
	}

	allEvents = append(allEvents, eventIDs...)

	campfireEvents, err := h.getAllEvents(ctx, allEvents)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get all events", slog.Any("err", err))
		h.renderExport(w, r, fmt.Sprintf("Failed to get all events: %s", err.Error()))
		return
	}

	slog.InfoContext(ctx, "Fetched events", slog.Int("events", len(events)))

	var allRecords []Records
	if combineCSVs {
		records := [][]string{
			includedFields,
		}

		for _, event := range campfireEvents {
			records = append(records, getRecords(event, includeMissingMembers, includedFields)...)
		}

		allRecords = append(allRecords, Records{
			name:    exportName(),
			records: records,
		})
	} else {
		for _, event := range campfireEvents {
			records := [][]string{
				includedFields,
			}

			records = append(records, getRecords(event, includeMissingMembers, includedFields)...)

			allRecords = append(allRecords, Records{
				name:    fmt.Sprintf("export_%s_%s", event.ID, cleanFilename(event.Name)),
				records: records,
			})
		}
	}

	h.exportRecords(ctx, w, allRecords, combineCSVs)
}

func (h *handler) getAllEvents(ctx context.Context, allEvents []string) ([]campfire.Event, error) {
	var (
		events []campfire.Event
		mu     sync.Mutex
	)

	eg, ctx := errgroup.WithContext(ctx)
	for _, eventID := range allEvents {
		eg.Go(func() error {
			event, err := h.fetchEvent(ctx, eventID)
			if err != nil {
				if errors.Is(err, campfire.ErrUnsupportedMeetup) {
					return nil
				}

				return fmt.Errorf("failed to fetch event %q: %w", eventID, err)
			}

			if len(event.RSVPStatuses) == 0 {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			events = append(events, *event)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}

	if len(events) == 0 {
		return nil, errors.New("no valid events found in the provided list")
	}

	return events, nil
}

func getRecords(event campfire.Event, includeMissingMembers bool, fields []string) [][]string {
	var records [][]string
	for _, rsvpStatus := range event.RSVPStatuses {
		member, ok := campfire.FindMember(rsvpStatus.UserID, event)
		if !ok && !includeMissingMembers {
			continue
		}
		var record []string
		for _, field := range fields {
			switch field {
			case FieldUserID:
				record = append(record, rsvpStatus.UserID)
			case FieldUsername:
				record = append(record, member.Username)
			case FieldDisplayName:
				record = append(record, member.DisplayName)
			case FieldRSVPStatus:
				record = append(record, rsvpStatus.RSVPStatus)
			case FieldEventID:
				record = append(record, event.ID)
			case FieldEventName:
				record = append(record, event.Name)
			case FieldEventURL:
				record = append(record, eventURL(event.ID))
			case FieldEventTime:
				record = append(record, event.EventTime.Format(time.RFC3339))
			case FieldEventClubID:
				record = append(record, event.ClubID)
			case FieldEventCreatorUserID:
				record = append(record, event.Creator.ID)
			case FieldEventCreatorUsername:
				record = append(record, event.Creator.Username)
			case FieldEventCreatorDisplayName:
				record = append(record, event.Creator.DisplayName)
			case FieldEventDiscordInterested:
				record = append(record, strconv.Itoa(event.DiscordInterested))
			case FieldEventCreatedByCommunityAmbassador:
				record = append(record, strconv.FormatBool(event.CreatedByCommunityAmbassador))
			case FieldEventCampfireLiveEventID:
				record = append(record, event.CampfireLiveEventID)
			case FieldEventCampfireLiveEventName:
				record = append(record, event.CampfireLiveEvent.EventName)
			}
		}

		records = append(records, record)
	}
	return records
}

func eventURL(id string) string {
	return fmt.Sprintf("https://campfire.nianticlabs.com/discover/meetup/%s", id)
}

type Records struct {
	name    string
	records [][]string
}

func (h *handler) exportRecords(ctx context.Context, w http.ResponseWriter, allRecords []Records, combineCSVs bool) {
	if combineCSVs || len(allRecords) == 1 {
		records := allRecords[0]
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", records.name))
		if err := csv.NewWriter(w).WriteAll(records.records); err != nil {
			slog.ErrorContext(ctx, "Failed to write CSV records", slog.Any("err", err))
			return
		}
		slog.InfoContext(ctx, "Export completed successfully", slog.Int("records", len(records.records)))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", exportName()))
	zw := zip.NewWriter(w)
	for _, records := range allRecords {
		filename := fmt.Sprintf("%s.csv", records.name)
		f, err := zw.Create(filename)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to create zip entry", slog.String("filename", filename), slog.Any("err", err))
			return
		}

		if err = csv.NewWriter(f).WriteAll(records.records); err != nil {
			slog.ErrorContext(ctx, "Failed to write CSV records", slog.String("filename", filename), slog.Any("err", err))
			return
		}
	}
	if err := zw.Close(); err != nil {
		slog.ErrorContext(ctx, "Failed to close zip writer: %s", err.Error())
		return
	}

	slog.InfoContext(ctx, "Export completed successfully", slog.Int("files", len(allRecords)))
}

func exportName() string {
	return fmt.Sprintf("export_%s", time.Now().Format("20060102_150405"))
}

func cleanFilename(name string) string {
	// Clean the filename to ensure it is safe for use in file systems
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "|", "")
	return strings.ToLower(name)
}
