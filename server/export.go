package server

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (s *Server) Export(w http.ResponseWriter, r *http.Request) {
	s.renderExport(w, "")
}

func (s *Server) ExportCSV(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received export request: %s", r.URL.Path)
	meetupURLs := r.FormValue("urls")
	if meetupURLs == "" {
		s.renderExport(w, "Missing 'urls' parameter")
		return
	}

	includeMissingMembersStr := r.FormValue("include_missing_members")
	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := strconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			s.renderExport(w, "Invalid 'include_missing_members' parameter")
			return
		}
		includeMissingMembers = parsed
	}

	combineCSVsStr := r.FormValue("combine_csv")
	var combineCSVs bool
	if combineCSVsStr != "" {
		parsed, err := strconv.ParseBool(combineCSVsStr)
		if err != nil {
			s.renderExport(w, "Invalid 'combine_csv' parameter")
			return
		}
		combineCSVs = parsed
	}

	eg, ctx := errgroup.WithContext(r.Context())
	var events []campfire.FullEvent
	var mu sync.Mutex
	for _, url := range strings.Split(meetupURLs, "\n") {
		meetupURL := strings.TrimSpace(url)
		if meetupURL == "" {
			continue
		}

		eg.Go(func() error {
			event, err := s.client.FetchEvent(ctx, meetupURL)
			if err != nil {
				return fmt.Errorf("failed to fetch event from URL %s: %w", meetupURL, err)
			}

			if event == nil || len(event.Event.RSVPStatuses) == 0 {
				return fmt.Errorf("no RSVPs found for event at URL %s", meetupURL)
			}

			mu.Lock()
			defer mu.Unlock()
			events = append(events, *event)

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.Printf("Failed to fetch events: %s", err.Error())
		s.renderExport(w, "Failed to fetch events: "+err.Error())
		return
	}

	if len(events) == 0 {
		log.Println("No events found for the provided URLs")
		s.renderExport(w, "No events found for the provided URLs")
		return
	}

	log.Printf("Fetched %d events", len(events))

	if combineCSVs {
		records := [][]string{
			{"id", "name", "status", "event_id", "event_name"},
		}
		for _, event := range events {
			for _, rsvpStatus := range event.Event.RSVPStatuses {
				member, ok := campfire.FindMemberName(rsvpStatus.UserID, event)
				if !ok && !includeMissingMembers {
					continue
				}

				records = append(records, []string{
					rsvpStatus.UserID,
					member,
					rsvpStatus.RSVPStatus,
					event.Event.ID,
					event.Event.Name,
				})
			}
		}

		log.Printf("Combined CSV records: %d", len(records))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		if err := csv.NewWriter(w).WriteAll(records); err != nil {
			log.Printf("Failed to write CSV records: %s", err.Error())
			return
		}
		return
	}

	var allRecords [][][]string
	for _, event := range events {
		records := [][]string{
			{"id", "name", "status"},
		}
		for _, rsvpStatus := range event.Event.RSVPStatuses {
			member, ok := campfire.FindMemberName(rsvpStatus.UserID, event)
			if !ok && !includeMissingMembers {
				continue
			}

			records = append(records, []string{
				rsvpStatus.UserID,
				member,
				rsvpStatus.RSVPStatus,
			})
		}
		allRecords = append(allRecords, records)
	}

	if len(allRecords) == 1 {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		if err := csv.NewWriter(w).WriteAll(allRecords[0]); err != nil {
			log.Printf("Failed to write CSV records: %s", err.Error())
			return
		}
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=export.zip")
	zw := zip.NewWriter(w)
	for i, records := range allRecords {
		filename := fmt.Sprintf("export_%d.csv", i+1)
		f, err := zw.Create(filename)
		if err != nil {
			log.Printf("Failed to create zip entry %s: %s", filename, err.Error())
			return
		}

		if err = csv.NewWriter(f).WriteAll(records); err != nil {
			log.Printf("Failed to write CSV records for %s: %s", filename, err.Error())
			return
		}
	}
	if err := zw.Close(); err != nil {
		log.Printf("Failed to close zip writer: %s", err.Error())
		return
	}

	log.Printf("Export completed successfully, %d files created", len(allRecords))
}

func (s *Server) renderExport(w http.ResponseWriter, errorMessage string) {
	if err := s.templates.ExecuteTemplate(w, "export.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
