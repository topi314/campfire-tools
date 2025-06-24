package server

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (s *Server) Export(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderExport(w, "")
	case http.MethodPost:
		s.doExport(w, r)
	}
}

func (s *Server) doExport(w http.ResponseWriter, r *http.Request) {
	meetupURLs := r.FormValue("urls")
	includeMissingMembersStr := r.FormValue("include_missing_members")
	combineCSVsStr := r.FormValue("combine_csv")

	slog.Info("Received export request", slog.String("url", r.URL.String()), slog.String("meetup_urls", meetupURLs), slog.String("include_missing_members", includeMissingMembersStr), slog.String("combine_csv", combineCSVsStr))

	if meetupURLs == "" {
		s.renderExport(w, "Missing 'urls' parameter")
		return
	}

	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := strconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			s.renderExport(w, "Invalid 'include_missing_members' parameter")
			return
		}
		includeMissingMembers = parsed
	}

	var combineCSVs bool
	if combineCSVsStr != "" {
		parsed, err := strconv.ParseBool(combineCSVsStr)
		if err != nil {
			s.renderExport(w, "Invalid 'combine_csv' parameter")
			return
		}
		combineCSVs = parsed
	}

	urls := strings.Split(meetupURLs, "\n")
	if len(urls) > 50 {
		s.renderExport(w, fmt.Sprintf("please limit the number of URLs to 50, got %d.", len(urls)))
		return
	}

	eg, ctx := errgroup.WithContext(r.Context())
	var events []campfire.FullEvent
	var mu sync.Mutex
	for _, url := range urls {
		meetupURL := strings.TrimSpace(url)
		if meetupURL == "" {
			continue
		}

		eg.Go(func() error {
			event, err := s.client.FetchEvent(ctx, meetupURL)
			if err != nil {
				if errors.Is(err, campfire.ErrUnsupportedMeetup) {
					return nil
				}

				return fmt.Errorf("failed to fetch event from URL %q: %w", meetupURL, err)
			}

			// ignore events without RSVP statuses
			if len(event.Event.RSVPStatuses) == 0 {
				return nil
			}

			mu.Lock()
			defer mu.Unlock()
			events = append(events, *event)

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(r.Context(), "Failed to fetch events", slog.Any("err", err))
		s.renderExport(w, "Failed to fetch events: "+err.Error())
		return
	}

	if len(events) == 0 {
		slog.ErrorContext(r.Context(), "No events found for the provided URLs")
		s.renderExport(w, "No events found for the provided URLs")
		return
	}

	slog.InfoContext(r.Context(), "Fetched events", slog.Int("events", len(events)))

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

		slog.InfoContext(r.Context(), "Combined CSV records", slog.Int("records", len(records)))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		if err := csv.NewWriter(w).WriteAll(records); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write CSV records", slog.Any("err", err))
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
			slog.ErrorContext(r.Context(), "Failed to write CSV records", slog.Any("err", err))
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
			slog.ErrorContext(r.Context(), "Failed to create zip entry", slog.String("filename", filename), slog.Any("err", err))
			return
		}

		if err = csv.NewWriter(f).WriteAll(records); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write CSV records", slog.String("filename", filename), slog.Any("err", err))
			return
		}
	}
	if err := zw.Close(); err != nil {
		slog.ErrorContext(r.Context(), "Failed to close zip writer: %s", err.Error())
		return
	}

	slog.InfoContext(r.Context(), "Export completed successfully", slog.Int("files", len(allRecords)))
}

func (s *Server) renderExport(w http.ResponseWriter, errorMessage string) {
	if err := s.templates.ExecuteTemplate(w, "export.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
