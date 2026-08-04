package tracker

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-tools/internal/xquery"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/web/models"
)

type UpcomingClubEvent struct {
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
	Accepted                     int                     `json:"accepted"`
	CheckedIn                    int                     `json:"checked_in"`
}

func (h *handler) APIClubEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	clubID := r.PathValue("club_id")
	upcoming := xquery.ParseBool(query, "upcoming", false)

	slog.InfoContext(ctx, "Received API club events request",
		slog.String("url", r.URL.String()),
		slog.Any("club_id", clubID),
		slog.Bool("upcoming", upcoming),
	)

	if clubID == "" {
		http.Error(w, "Club ID is required", http.StatusBadRequest)
		return
	}

	if upcoming {
		h.apiClubUpcomingEvents(w, r, clubID)
		return
	}

	events, err := h.DB.GetEvents(ctx, clubID, time.Time{}, time.Time{}, false, "")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get events for club", slog.Any("error", err), slog.String("club_id", clubID))
		http.Error(w, "Failed to get events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(events) == 0 {
		http.Error(w, "No events found for the specified club", http.StatusNotFound)
		return
	}

	var campfireEvents []campfire.Event
	for _, event := range events {
		var campfireEvent campfire.Event
		if err = json.Unmarshal(event.RawJSON, &campfireEvent); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal event", slog.Any("error", err), slog.String("event_id", event.ID))
			http.Error(w, "Failed to process event data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		campfireEvents = append(campfireEvents, campfireEvent)
	}

	exportAllEvents(ctx, w, campfireEvents)
}

func (h *handler) apiClubUpcomingEvents(w http.ResponseWriter, r *http.Request, clubID string) {
	ctx := r.Context()
	query := r.URL.Query()

	tz := xquery.ParseString(query, "timezone", "UTC")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		http.Error(w, "Invalid timezone: "+tz, http.StatusBadRequest)
		return
	}

	events, err := h.DB.GetUpcomingClubEvents(ctx, clubID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get upcoming events for club", slog.Any("error", err), slog.String("club_id", clubID))
		http.Error(w, "Failed to get upcoming events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	upcomingEvents := make([]UpcomingClubEvent, 0, len(events))
	for _, event := range events {
		var campfireEvent campfire.Event
		if err = json.Unmarshal(event.RawJSON, &campfireEvent); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal event", slog.Any("error", err), slog.String("event_id", event.ID))
			http.Error(w, "Failed to process event data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		exportEvent := toExportEvent(campfireEvent, loc)
		exportEvent.CoverPhotoURL = h.absoluteImageURL(exportEvent.CoverPhotoURL)
		exportEvent.Club.AvatarURL = h.absoluteImageURL(exportEvent.Club.AvatarURL)
		exportEvent.Club.Creator.AvatarURL = h.absoluteImageURL(exportEvent.Club.Creator.AvatarURL)

		upcomingEvents = append(upcomingEvents, UpcomingClubEvent{
			ID:                           exportEvent.ID,
			Name:                         exportEvent.Name,
			Address:                      exportEvent.Address,
			CoverPhotoURL:                exportEvent.CoverPhotoURL,
			Details:                      exportEvent.Details,
			URL:                          exportEvent.URL,
			Time:                         exportEvent.Time,
			EndTime:                      exportEvent.EndTime,
			Club:                         exportEvent.Club,
			Creator:                      exportEvent.Creator,
			DiscordInterested:            exportEvent.DiscordInterested,
			CreatedByCommunityAmbassador: exportEvent.CreatedByCommunityAmbassador,
			Badges:                       exportEvent.Badges,
			CampfireLiveEvent:            exportEvent.CampfireLiveEvent,
			Accepted:                     event.Accepted,
			CheckedIn:                    event.CheckIns,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(upcomingEvents); err != nil {
		slog.ErrorContext(ctx, "Failed to encode upcoming club events to JSON", slog.Any("error", err))
		return
	}

	slog.InfoContext(ctx, "Upcoming club events export completed successfully", slog.Int("events", len(upcomingEvents)))
}

func (h *handler) absoluteImageURL(imageURL string) string {
	proxied := models.ImageURL(imageURL, 0)
	if proxied == "" {
		return ""
	}
	return h.Cfg.Server.PublicTrackerURL + proxied
}
