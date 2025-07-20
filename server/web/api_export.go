package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/omit"
	"github.com/topi314/campfire-tools/server/campfire"
)

type ExportMember struct {
	UserID      omit.Omit[string] `json:"user_id,omitzero"`
	Username    omit.Omit[string] `json:"username,omitzero"`
	DisplayName omit.Omit[string] `json:"display_name,omitzero"`
	RSVPStatus  omit.Omit[string] `json:"rsvp_status,omitzero"`
	Event       ExportEvent       `json:"event,omitzero"`
}

type ExportEvent struct {
	ID                           omit.Omit[string]    `json:"id,omitzero"`
	Name                         omit.Omit[string]    `json:"name,omitzero"`
	URL                          omit.Omit[string]    `json:"url,omitzero"`
	Time                         omit.Omit[time.Time] `json:"time,omitzero"`
	ClubID                       omit.Omit[string]    `json:"club_id,omitzero"`
	Creator                      ExportEventCreator   `json:"creator,omitzero"`
	DiscordInterested            omit.Omit[int]       `json:"discord_interested,omitzero"`
	CreatedByCommunityAmbassador omit.Omit[bool]      `json:"created_by_community_ambassador,omitzero"`
	CampfireLiveEventID          omit.Omit[string]    `json:"campfire_live_event_id,omitzero"`
	CampfireLiveEventName        omit.Omit[string]    `json:"campfire_live_event_name,omitzero"`
}

type ExportEventCreator struct {
	UserID      omit.Omit[string] `json:"user_id,omitzero"`
	Username    omit.Omit[string] `json:"username,omitzero"`
	DisplayName omit.Omit[string] `json:"display_name,omitzero"`
}

func (h *handler) APIExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	events := parseStringSliceQuery(query, "events", nil)
	includeMissingMembers := parseBoolQuery(query, "include_missing_members", false)
	includedFields := parseStringSliceQuery(query, "included_fields", jsonDefaultFields)

	slog.InfoContext(ctx, "Received API export request",
		slog.String("url", r.URL.String()),
		slog.Any("events", events),
		slog.Bool("include_missing_members", includeMissingMembers),
		slog.Any("included_fields", includedFields),
	)

	if len(events) == 0 {
		http.Error(w, "Missing or empty 'events' parameter", http.StatusBadRequest)
		return
	}

	campfireEvents, err := h.getAllEvents(ctx, events)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get all events for export", slog.Any("error", err))
		http.Error(w, "Failed to get events for export: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(campfireEvents) == 0 {
		http.Error(w, "No valid events found for export", http.StatusBadRequest)
		return
	}

	var exportMembers []ExportMember
	for _, event := range campfireEvents {
		for _, rsvpStatus := range event.RSVPStatuses {
			member, ok := campfire.FindMember(rsvpStatus.UserID, event)
			if !ok && !includeMissingMembers {
				continue
			}

			var exportMember ExportMember
			for _, field := range includedFields {
				switch field {
				case FieldUserID:
					exportMember.UserID = omit.New(rsvpStatus.UserID)
				case FieldUsername:
					exportMember.Username = omit.New(member.Username)
				case FieldDisplayName:
					exportMember.DisplayName = omit.New(member.DisplayName)
				case FieldRSVPStatus:
					exportMember.RSVPStatus = omit.New(rsvpStatus.RSVPStatus)
				case FieldEventID:
					exportMember.Event.ID = omit.New(event.ID)
				case FieldEventName:
					exportMember.Event.Name = omit.New(event.Name)
				case FieldEventURL:
					exportMember.Event.URL = omit.New(eventURL(event.ID))
				case FieldEventTime:
					exportMember.Event.Time = omit.New(event.EventTime)
				case FieldEventClubID:
					exportMember.Event.ClubID = omit.New(event.ClubID)
				case FieldEventCreatorUserID:
					exportMember.Event.Creator.UserID = omit.New(event.Creator.ID)
				case FieldEventCreatorUsername:
					exportMember.Event.Creator.Username = omit.New(event.Creator.Username)
				case FieldEventCreatorDisplayName:
					exportMember.Event.Creator.DisplayName = omit.New(event.Creator.DisplayName)
				case FieldEventDiscordInterested:
					exportMember.Event.DiscordInterested = omit.New(event.DiscordInterested)
				case FieldEventCreatedByCommunityAmbassador:
					exportMember.Event.CreatedByCommunityAmbassador = omit.New(event.CreatedByCommunityAmbassador)
				case FieldEventCampfireLiveEventID:
					exportMember.Event.CampfireLiveEventID = omit.New(event.CampfireLiveEventID)
				case FieldEventCampfireLiveEventName:
					exportMember.Event.CampfireLiveEventName = omit.New(event.CampfireLiveEvent.EventName)
				}
			}
			exportMembers = append(exportMembers, exportMember)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(exportMembers); err != nil {
		slog.ErrorContext(ctx, "Failed to encode export members to JSON", slog.Any("error", err))
		return
	}

	slog.InfoContext(ctx, "Export completed successfully", slog.Int("member_count", len(exportMembers)))
}
