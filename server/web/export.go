package web

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/topi314/campfire-tools/internal/xstrconv"
	"github.com/topi314/campfire-tools/server/campfire"
)

func (h *handler) Export(w http.ResponseWriter, r *http.Request) {
	h.renderExport(w, r, "")
}

func (h *handler) DoExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meetupURLs := r.FormValue("urls")
	includeMissingMembersStr := r.FormValue("include_missing_members")
	combineCSVsStr := r.FormValue("combine_csv")

	slog.Info("Received export request", slog.String("url", r.URL.String()), slog.String("meetup_urls", meetupURLs), slog.String("include_missing_members", includeMissingMembersStr), slog.String("combine_csv", combineCSVsStr))

	if meetupURLs == "" {
		h.renderExport(w, r, "Missing 'urls' parameter")
		return
	}

	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := xstrconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			h.renderExport(w, r, "Invalid 'include_missing_members' parameter")
			return
		}
		includeMissingMembers = parsed
	}

	var combineCSVs bool
	if combineCSVsStr != "" {
		parsed, err := xstrconv.ParseBool(combineCSVsStr)
		if err != nil {
			h.renderExport(w, r, "Invalid 'combine_csv' parameter")
			return
		}
		combineCSVs = parsed
	}

	urls := strings.Split(meetupURLs, "\n")
	if len(urls) > 50 {
		h.renderExport(w, r, fmt.Sprintf("please limit the number of URLs to 50, got %d.", len(urls)))
		return
	}

	eg, ctx := errgroup.WithContext(ctx)
	var events []campfire.Event
	var mu sync.Mutex
	for _, url := range urls {
		meetupURL := strings.TrimSpace(url)
		if meetupURL == "" {
			continue
		}

		eg.Go(func() error {
			event, err := h.Campfire.ResolveEvent(ctx, meetupURL)
			if err != nil {
				if errors.Is(err, campfire.ErrUnsupportedMeetup) {
					return nil
				}

				return fmt.Errorf("failed to fetch event from URL %q: %w", meetupURL, err)
			}

			// ignore events without RSVP statuses
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
		slog.ErrorContext(ctx, "Failed to fetch events", slog.Any("err", err))
		h.renderExport(w, r, "Failed to fetch events: "+err.Error())
		return
	}

	if len(events) == 0 {
		slog.ErrorContext(ctx, "No events found for the provided URLs")
		h.renderExport(w, r, "No events found for the provided URLs")
		return
	}

	slog.InfoContext(ctx, "Fetched events", slog.Int("events", len(events)))

	var allRecords [][][]string
	if combineCSVs {
		records := [][]string{
			{"id", "username", "display_name", "status", "event_id", "event_name"},
		}
		for _, event := range events {
			for _, rsvpStatus := range event.RSVPStatuses {
				member, ok := campfire.FindMember(rsvpStatus.UserID, event)
				if !ok && !includeMissingMembers {
					continue
				}

				records = append(records, []string{
					rsvpStatus.UserID,
					member.Username,
					member.DisplayName,
					rsvpStatus.RSVPStatus,
					event.ID,
					event.Name,
				})
			}
		}
		allRecords = append(allRecords, records)
	} else {
		for _, event := range events {
			records := [][]string{
				{"id", "username", "display_name", "status"},
			}
			for _, rsvpStatus := range event.RSVPStatuses {
				member, ok := campfire.FindMember(rsvpStatus.UserID, event)
				if !ok && !includeMissingMembers {
					continue
				}

				records = append(records, []string{
					rsvpStatus.UserID,
					member.Username,
					member.DisplayName,
					rsvpStatus.RSVPStatus,
				})
			}
			allRecords = append(allRecords, records)
		}
	}

	h.exportRecords(ctx, w, allRecords, combineCSVs)
}

func (h *handler) renderExport(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	if err := h.Templates().ExecuteTemplate(w, "export.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render export template", slog.Any("err", err))
	}
}

func (h *handler) exportRecords(ctx context.Context, w http.ResponseWriter, allRecords [][][]string, combineCSVs bool) {
	if combineCSVs {
		records := allRecords[0]
		slog.InfoContext(ctx, "Combined CSV records", slog.Int("records", len(records)))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		if err := csv.NewWriter(w).WriteAll(records); err != nil {
			slog.ErrorContext(ctx, "Failed to write CSV records", slog.Any("err", err))
			return
		}
		return
	}

	if len(allRecords) == 1 {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		if err := csv.NewWriter(w).WriteAll(allRecords[0]); err != nil {
			slog.ErrorContext(ctx, "Failed to write CSV records", slog.Any("err", err))
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
			slog.ErrorContext(ctx, "Failed to create zip entry", slog.String("filename", filename), slog.Any("err", err))
			return
		}

		if err = csv.NewWriter(f).WriteAll(records); err != nil {
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
