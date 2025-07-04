package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-tools/internal/xstrconv"
	"github.com/topi314/campfire-tools/server/database"
)

type TrackerClubExportVars struct {
	ClubName      string
	ClubAvatarURL string
	ClubID        string
	Events        []Event
	Error         string
}

func (s *Server) TrackerClubExport(w http.ResponseWriter, r *http.Request) {
	s.renderTrackerClubExport(w, r, "")
}

func (s *Server) renderTrackerClubExport(w http.ResponseWriter, r *http.Request, errorMessage string) {
	ctx := r.Context()

	clubID := r.PathValue("club_id")

	club, err := s.db.GetClub(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to get club: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := s.db.GetEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch events for club", slog.String("club_id", clubID), slog.Any("err", err))
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	trackerEvents := make([]Event, len(events))
	for i, event := range events {
		trackerEvents[i] = Event{
			ID:            event.ID,
			Name:          event.Name,
			URL:           fmt.Sprintf("/tracker/event/%s", event.ID),
			CoverPhotoURL: imageURL(event.CoverPhotoURL),
		}
	}

	if err = s.templates().ExecuteTemplate(w, "tracker_club_export.gohtml", TrackerClubExportVars{
		ClubName:      club.ClubName,
		ClubAvatarURL: imageURL(club.ClubAvatarURL),
		ClubID:        club.ClubID,
		Events:        trackerEvents,
		Error:         errorMessage,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker club export template", slog.Any("err", err))
	}
}

func (s *Server) DoTrackerClubExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "Failed to parse form", slog.Any("err", err))
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	events := r.Form["events"]
	includeMissingMembersStr := r.FormValue("include_missing_members")
	combineCSVsStr := r.FormValue("combine_csv")

	slog.Info("Received Tracker Club export request", slog.String("url", r.URL.String()), slog.Any("event_ids", events), slog.String("include_missing_members", includeMissingMembersStr), slog.String("combine_csv", combineCSVsStr))

	if len(events) == 0 {
		s.renderTrackerClubExport(w, r, "Missing 'events' parameter")
		return
	}

	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := xstrconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			s.renderTrackerClubExport(w, r, "Invalid 'include_missing_members' parameter")
			return
		}
		includeMissingMembers = parsed
	}

	var combineCSVs bool
	if combineCSVsStr != "" {
		parsed, err := xstrconv.ParseBool(combineCSVsStr)
		if err != nil {
			s.renderTrackerClubExport(w, r, "Invalid 'combine_csv' parameter")
			return
		}
		combineCSVs = parsed
	}

	var allMembers [][]database.EventMember
	for _, eventID := range events {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			continue
		}

		members, err := s.db.GetCheckedInMembersByEvent(ctx, eventID)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get event members", slog.String("id", eventID), slog.Any("err", err))
			continue
		}

		if len(members) == 0 {
			continue
		}

		allMembers = append(allMembers, members)
	}

	if len(allMembers) == 0 {
		s.renderTrackerClubExport(w, r, "No events found for the provided IDs")
		return
	}

	slog.InfoContext(ctx, "Fetched events", slog.Int("events", len(allMembers)))

	var allRecords [][][]string
	if combineCSVs {
		records := [][]string{
			{"id", "name", "status", "event_id", "event_name"},
		}
		for _, members := range allMembers {
			for _, member := range members {
				if member.DisplayName == "" && !includeMissingMembers {
					continue
				}

				records = append(records, []string{
					member.ID,
					member.DisplayName,
					member.Status,
					member.EventID,
					member.EventName,
				})
			}
		}
		allRecords = append(allRecords, records)
	} else {
		for _, members := range allMembers {
			records := [][]string{
				{"id", "name", "status"},
			}
			for _, member := range members {
				if member.DisplayName == "" && !includeMissingMembers {
					continue
				}

				records = append(records, []string{
					member.ID,
					member.DisplayName,
					member.Status,
				})
			}
			allRecords = append(allRecords, records)
		}
	}

	s.exportRecords(ctx, w, allRecords, true)
}
