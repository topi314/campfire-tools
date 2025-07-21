package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/server/campfire"
)

type ExportEvent struct {
	ID                           string                  `json:"id"`
	Name                         string                  `json:"name"`
	Address                      string                  `json:"address"`
	CoverPhotoURL                string                  `json:"cover_photo_url"`
	Details                      string                  `json:"details"`
	URL                          string                  `json:"url"`
	Time                         time.Time               `json:"time"`
	EndTime                      time.Time               `json:"end_time"`
	Club                         ExportClub              `json:"club"`
	Creator                      ExportMember            `json:"creator"`
	DiscordInterested            int                     `json:"discord_interested"`
	CreatedByCommunityAmbassador bool                    `json:"created_by_community_ambassador"`
	Badges                       []string                `json:"badges"`
	CampfireLiveEvent            ExportCampfireLiveEvent `json:"campfire_live_event"`
	Members                      []ExportRSVPMember      `json:"members"`
}

type ExportClub struct {
	ID                           string       `json:"id"`
	Name                         string       `json:"name"`
	AvatarURL                    string       `json:"avatar_url"`
	Badges                       []string     `json:"badges"`
	CreatedByCommunityAmbassador bool         `json:"created_by_community_ambassador"`
	Creator                      ExportMember `json:"creator"`
}

type ExportCampfireLiveEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExportMember struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url"`
	Badges      []string `json:"badges"`
}

type ExportRSVPMember struct {
	ExportMember
	RSVPStatus string `json:"rsvp_status"`
}

func (h *handler) APIEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	events := parseStringSliceQuery(query, "events", nil)

	slog.InfoContext(ctx, "Received API events request", slog.String("url", r.URL.String()), slog.Any("events", events))

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

	exportAllEvents(ctx, w, campfireEvents)
}

func badges(badges []campfire.Badge) []string {
	var badgeList []string
	for _, badge := range badges {
		badgeList = append(badgeList, badge.BadgeType)
	}
	return badgeList
}

func exportAllEvents(ctx context.Context, w http.ResponseWriter, events []campfire.Event) {
	var exportEvents []ExportEvent
	for _, event := range events {
		exportEvent := ExportEvent{
			ID:            event.ID,
			Name:          event.Name,
			Address:       event.Address,
			CoverPhotoURL: event.CoverPhotoURL,
			Details:       event.Details,
			URL:           eventURL(event.ID),
			Time:          event.EventTime,
			EndTime:       event.EventEndTime,
			Club: ExportClub{
				ID:                           event.ClubID,
				Name:                         event.Club.Name,
				AvatarURL:                    event.Club.AvatarURL,
				Badges:                       event.Club.BadgeGrants,
				CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
				Creator: ExportMember{
					ID:          event.Club.Creator.ID,
					Username:    event.Club.Creator.Username,
					DisplayName: event.Club.Creator.DisplayName,
					AvatarURL:   event.Club.Creator.AvatarURL,
					Badges:      badges(event.Club.Creator.Badges),
				},
			},
			Creator: ExportMember{
				ID:          event.Creator.ID,
				Username:    event.Creator.Username,
				DisplayName: event.Creator.DisplayName,
			},
			DiscordInterested:            event.DiscordInterested,
			CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			Badges:                       event.BadgeGrants,
			CampfireLiveEvent: ExportCampfireLiveEvent{
				ID:   event.CampfireLiveEventID,
				Name: event.CampfireLiveEvent.EventName,
			},
		}

		for _, rsvpStatus := range event.RSVPStatuses {
			member, _ := campfire.FindMember(rsvpStatus.UserID, event)

			exportEvent.Members = append(exportEvent.Members, ExportRSVPMember{
				ExportMember: ExportMember{
					ID:          rsvpStatus.UserID,
					Username:    member.Username,
					DisplayName: member.DisplayName,
					AvatarURL:   member.AvatarURL,
					Badges:      badges(member.Badges),
				},
				RSVPStatus: rsvpStatus.RSVPStatus,
			})
		}
		exportEvents = append(exportEvents, exportEvent)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(exportEvents); err != nil {
		slog.ErrorContext(ctx, "Failed to encode export members to JSON", slog.Any("error", err))
		return
	}

	slog.InfoContext(ctx, "Export completed successfully", slog.Int("events", len(exportEvents)))
}
