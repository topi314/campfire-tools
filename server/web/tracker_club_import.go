package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

// Example URL: https://campfire.onelink.me/eBr8?af_dp=campfire://&af_force_deeplink=true&deep_link_sub1=cj1jbHVicyZjPWI2MzJmYzhlLTBiNDEtNDlkZS1hZGUyLTIxYjBjZDgxZGI2OSZpPXRydWU=

// Regex matching urls like: https://campfire.onelink.me/ with query parameters
var clubURLRegex = regexp.MustCompile(`https://campfire\.onelink\.me/[a-zA-Z0-9]+(?:\?[^ ]*)?`)

type TrackerClubImportVars struct {
	SelectedClubID string
	Errors         []string
}

func (h *handler) TrackerClubImport(w http.ResponseWriter, r *http.Request) {
	h.renderTrackerClubImport(w, r)
}

func (h *handler) renderTrackerClubImport(w http.ResponseWriter, r *http.Request, errorMessages ...string) {
	ctx := r.Context()
	query := r.URL.Query()

	selected := query.Get("club")
	if selected == "" {
		selected = r.FormValue("club")
	}

	if err := h.Templates().ExecuteTemplate(w, "tracker_club_import.gohtml", TrackerClubImportVars{
		SelectedClubID: selected,
		Errors:         errorMessages,
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
		if errors.Is(err, campfire.ErrNotFound) {
			h.renderTrackerClubImport(w, r, "No club found for input")
			return
		}
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to get club: %s", err))
		return
	}
	if club.ID == "" {
		h.renderTrackerClubImport(w, r, "Failed to retrieve club")
		return
	}

	slog.DebugContext(ctx, "Retrieved club info", slog.String("club_id", club.ID), slog.String("club_name", club.Name))
	events, err := h.Campfire.GetPastMeetups(ctx, club.ID)
	if err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to get past meetups for %q: %s", club.ID, err))
		return
	}

	if err = h.bulkProcessEvents(ctx, events); err != nil {
		h.renderTrackerClubImport(w, r, fmt.Sprintf("Failed to import club events: %s", err))
		return
	}

	slog.InfoContext(ctx, "Successfully imported events", slog.Int("count", len(events)))
	http.Redirect(w, r, fmt.Sprintf("/tracker/club/%s", club.ID), http.StatusFound)
}

func (h *handler) bulkProcessEvents(ctx context.Context, allEvents []campfire.Event) error {
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
