package server

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/topi314/campfire-tools/server/campfire"
)

func (s *Server) Export(w http.ResponseWriter, r *http.Request) {
	s.renderExport(w, "")
}

func (s *Server) ExportCSV(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received export request: %s", r.URL.Path)
	meetupURL := r.FormValue("url")
	if meetupURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	includeMissingMembersStr := r.FormValue("include_missing_members")
	var includeMissingMembers bool
	if includeMissingMembersStr != "" {
		parsed, err := strconv.ParseBool(includeMissingMembersStr)
		if err != nil {
			http.Error(w, "Invalid 'include_missing_members' parameter", http.StatusBadRequest)
			return
		}
		includeMissingMembers = parsed
	}

	event, err := s.client.FetchEvent(meetupURL)
	if err != nil {
		s.renderExport(w, fmt.Sprintf("Failed to fetch event: %s", err.Error()))
		return
	}

	if event == nil || len(event.Event.RSVPStatuses) == 0 {
		s.renderExport(w, fmt.Sprintf("Event not found or no checked-in members found"))
		return
	}

	records := [][]string{
		{"id", "name", "status"},
	}
	for _, rsvpStatus := range event.Event.RSVPStatuses {
		member, ok := campfire.FindMemberName(rsvpStatus.UserID, *event)
		if !ok && !includeMissingMembers {
			continue
		}

		records = append(records, []string{
			rsvpStatus.UserID,
			member,
			rsvpStatus.RSVPStatus,
		})
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
	if err = csv.NewWriter(w).WriteAll(records); err != nil {
		log.Printf("Failed to write CSV records: %s", err.Error())
		return
	}
}

func (s *Server) renderExport(w http.ResponseWriter, errorMessage string) {
	if err := s.templates.ExecuteTemplate(w, "export.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
