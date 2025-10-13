package tracker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/xpgtype"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

// Example URL: https://campfire.onelink.me/eBr8?af_dp=campfire://&af_force_deeplink=true&deep_link_sub1=cj1jbHVicyZjPWI2MzJmYzhlLTBiNDEtNDlkZS1hZGUyLTIxYjBjZDgxZGI2OSZpPXRydWU=

// Regex matching urls like: https://campfire.onelink.me/ with query parameters
var clubURLRegex = regexp.MustCompile(`https://campfire\.onelink\.me/[a-zA-Z0-9]+(?:\?[^ ]*)?`)

type TrackerClubImportVars struct {
	SelectedClubID string
	ImportJobs     []ClubImportJob
	Errors         []string
	IsAdmin        bool
}

func (h *handler) TrackerClubImport(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubImport(w, r)
}

func (h *handler) renderTrackerClubImport(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()
	query := r.URL.Query()

	session := auth.GetSession(r)

	clubImportJobs, err := h.DB.GetClubImportJobs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get club import jobs", slog.Any("err", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var jobs []ClubImportJob
	for _, job := range clubImportJobs {
		jobs = append(jobs, newClubImportJob(job))
	}

	selected := query.Get("club")
	if selected == "" {
		selected = r.FormValue("club")
	}

	if err = h.Templates().ExecuteTemplate(w, "tracker_club_import.gohtml", TrackerClubImportVars{
		SelectedClubID: selected,
		ImportJobs:     jobs,
		Errors:         errorMessages,
		IsAdmin:        session.Admin,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to render tracker import template", slog.Any("err", err))
	}
}

func (h *handler) TrackerClubDoImport(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())

	clubParam := strings.TrimSpace(r.FormValue("club"))

	slog.InfoContext(ctx, "Received tracker club import request", slog.String("url", r.URL.String()), slog.String("club", clubParam))

	if clubParam == "" {
		h.renderTrackerClubImport(w, r, "Missing 'club' parameter")
		return
	}

	var (
		clubURL    string
		possibleID string
	)

	for _, line := range strings.Fields(clubParam) {
		match := clubURLRegex.FindStringSubmatch(line)
		if len(match) > 0 {
			if clubURL != "" || possibleID != "" {
				h.renderTrackerClubImport(w, r, "Multiple club URLs or IDs found")
				return
			}
			clubURL = match[0]
			continue
		}

		if strings.Count(line, "-") == 4 {
			if clubURL != "" || possibleID != "" {
				h.renderTrackerClubImport(w, r, "Multiple club URLs or IDs found")
				return
			}
			possibleID = line
			continue
		}
	}

	var clubID string
	if clubURL != "" {
		resolvedClubID, err := h.Campfire.ResolveClubID(clubURL)
		if err != nil {
			h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to resolve club ID: %s", err))
			return
		}
		clubID = resolvedClubID
	} else if possibleID != "" {
		clubID = possibleID
	} else {
		h.renderTrackerClubImport(w, r, "No valid club URL or ID found")
		return
	}

	slog.DebugContext(ctx, "Resolved club ID", slog.String("club_id", clubID))
	club, err := h.Campfire.GetClub(ctx, clubID)
	if err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to get club: %s", err))
		return
	}

	if club.ID == "" {
		h.renderTrackerClubImport(w, r, "Failed to retrieve club")
		return
	}

	slog.DebugContext(ctx, "Retrieved club info", slog.String("club_id", club.ID), slog.String("club_name", club.Name))

	if err = h.DB.InsertMembers(ctx, []database.Member{{
		ID:          club.Creator.ID,
		Username:    club.Creator.Username,
		DisplayName: club.Creator.DisplayName,
		AvatarURL:   club.Creator.AvatarURL,
		RawJSON:     club.Creator.Raw,
	}}); err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to insert club creator into database: %s", err))
		return
	}

	if err = h.DB.InsertClubs(ctx, []database.Club{{
		ID:                           club.ID,
		Name:                         club.Name,
		AvatarURL:                    club.AvatarURL,
		CreatorID:                    club.Creator.ID,
		CreatedByCommunityAmbassador: club.CreatedByCommunityAmbassador,
		RawJSON:                      club.Raw,
	}}); err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to insert club into database: %s", err))
		return
	}

	_, err = h.DB.InsertClubImportJob(ctx, database.ClubImportJob{
		ClubID:      club.ID,
		CompletedAt: time.Time{},
		LastTriedAt: time.Time{},
		Status:      database.ClubImportJobStatusPending,
		State:       xpgtype.NewJSON(database.ClubImportJobState{}),
	})
	if err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to create club import job: %s", err))
		return
	}

	http.Redirect(w, r, "/tracker/club/import", http.StatusFound)
}

func (h *handler) bulkProcessEvents(ctx context.Context, allEvents []campfire.Event) error {
	now := time.Now()
	var (
		members []database.Member
		clubs   []database.Club
		events  []database.Event
		rsvps   []database.EventRSVP
	)

	for _, event := range allEvents {
		for _, member := range event.Members.Edges {
			if containsMember(members, member.Node.ID) {
				continue
			}
			members = append(members, database.Member{
				ID:          member.Node.ID,
				Username:    member.Node.Username,
				DisplayName: member.Node.DisplayName,
				AvatarURL:   member.Node.AvatarURL,
				RawJSON:     member.Node.Raw,
			})
		}

		if !slices.ContainsFunc(clubs, func(c database.Club) bool {
			return c.ID == event.Club.ID
		}) {
			clubs = append(clubs, database.Club{
				ID:                           event.Club.ID,
				Name:                         event.Club.Name,
				AvatarURL:                    event.Club.AvatarURL,
				CreatorID:                    event.Club.Creator.ID,
				CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
				RawJSON:                      event.Club.Raw,
			})

			if !containsMember(members, event.Club.Creator.ID) {
				members = append(members, database.Member{
					ID:          event.Club.Creator.ID,
					Username:    event.Club.Creator.Username,
					DisplayName: event.Club.Creator.DisplayName,
					AvatarURL:   event.Club.Creator.AvatarURL,
					RawJSON:     event.Club.Creator.Raw,
				})
			}
		}

		if !slices.ContainsFunc(events, func(e database.Event) bool {
			return e.ID == event.ID
		}) {
			events = append(events, database.Event{
				ID:                           event.ID,
				Name:                         event.Name,
				Details:                      event.Details,
				Address:                      event.Address,
				Location:                     event.Location,
				CreatorID:                    event.Creator.ID,
				CoverPhotoURL:                event.CoverPhotoURL,
				Time:                         event.EventTime,
				EndTime:                      event.EventEndTime,
				Finished:                     event.EventEndTime.Before(now),
				DiscordInterested:            event.DiscordInterested,
				CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
				CampfireLiveEventID:          event.CampfireLiveEventID,
				CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
				ClubID:                       event.ClubID,
				RawJSON:                      event.Raw,
			})

			for _, rsvpStatus := range event.RSVPStatuses {
				if !containsMember(members, rsvpStatus.UserID) {
					members = append(members, database.Member{
						ID:          rsvpStatus.UserID,
						Username:    "",
						DisplayName: "",
						AvatarURL:   "",
						RawJSON:     []byte("{}"),
					})
				}
				rsvps = append(rsvps, database.EventRSVP{
					EventID:  event.ID,
					MemberID: rsvpStatus.UserID,
					Status:   rsvpStatus.RSVPStatus,
				})
			}
		}
	}

	if err := h.DB.InsertMembers(ctx, members); err != nil {
		return fmt.Errorf("failed to insert members: %w", err)
	}

	if err := h.DB.InsertClubs(ctx, clubs); err != nil {
		return fmt.Errorf("failed to insert clubs: %w", err)
	}

	if err := h.DB.InsertEvents(ctx, events); err != nil {
		return fmt.Errorf("failed to insert events: %w", err)
	}

	if err := h.DB.InsertEventRSVPs(ctx, rsvps); err != nil {
		return fmt.Errorf("failed to add event RSVPs: %w", err)
	}

	slog.InfoContext(ctx, "Members added for events",
		slog.Int("member_count", len(members)),
		slog.Int("club_count", len(clubs)),
		slog.Int("event_count", len(events)),
		slog.Int("rsvp_count", len(rsvps)),
	)

	return nil
}

func containsMember(members []database.Member, memberID string) bool {
	return slices.ContainsFunc(members, func(m database.Member) bool {
		return m.ID == memberID
	})
}
