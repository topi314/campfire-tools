package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/topi314/campfire-tools/internal/eventcategory"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func (s *Server) importEvents() {
	for {
		s.doImportEvents()
		time.Sleep(10 * time.Second)
	}
}

func (s *Server) doImportEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.doImportNextEvent(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to import next event", slog.Any("err", err))
	}
}

func (s *Server) doImportNextEvent(ctx context.Context) error {
	club, err := s.DB.GetNextClubImport(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	slog.InfoContext(ctx, "Importing events for club", slog.String("club_id", club.ID), slog.String("club_name", club.Name))

	importErr := s.ImportActiveClubEvents(ctx, club.ID)

	if err = s.DB.UpdateClubLastAutoEventImported(ctx, club.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to update club last auto event import", slog.String("club_id", club.ID), slog.Any("err", err))
	}

	return importErr
}

func (s *Server) ImportActiveClubEvents(ctx context.Context, clubID string) error {
	club, events, err := s.Campfire.GetFutureEvents(ctx, clubID, nil)
	if err != nil {
		return err
	}
	if err = s.saveClub(ctx, club); err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "Found future events to import", slog.String("club_id", clubID), slog.Int("events", len(events)))

	for _, event := range events {
		members := eventMembers(event)
		if eventMembersIncomplete(event) {
			members, _, err = s.Campfire.GetEventMembers(ctx, event.ID, nil)
			if err != nil {
				return err
			}
		}

		if processErr := s.ProcessEventImport(ctx, event, members); processErr != nil {
			slog.ErrorContext(ctx, "Failed to insert event RSVPs", slog.String("club_id", clubID), slog.String("event_id", event.ID), slog.Any("err", processErr))
			continue
		}
		slog.InfoContext(ctx, "Imported event",
			slog.String("club_id", clubID),
			slog.String("event_id", event.ID),
			slog.String("event_name", event.Name),
			slog.Int("rsvps", len(event.RSVPStatuses)),
			slog.Int("members", len(members)),
		)
	}

	return nil
}

func eventMembers(event campfire.Event) []campfire.Member {
	members := make([]campfire.Member, 0, len(event.Members.Edges))
	for _, edge := range event.Members.Edges {
		members = append(members, edge.Node)
	}
	return members
}

func eventMembersIncomplete(event campfire.Event) bool {
	return event.Members.PageInfo.HasNextPage || event.Members.TotalCount > len(event.Members.Edges)
}

func (s *Server) saveClub(ctx context.Context, club *campfire.Club) error {
	if club == nil || club.ID == "" {
		return fmt.Errorf("failed to fetch club")
	}

	if err := s.DB.InsertMembers(ctx, []database.Member{{
		ID:          club.Creator.ID,
		Username:    club.Creator.Username,
		DisplayName: club.Creator.DisplayName,
		AvatarURL:   club.Creator.AvatarURL,
		RawJSON:     club.Creator.Raw,
	}}); err != nil {
		return fmt.Errorf("failed to insert club creator: %w", err)
	}

	if err := s.DB.InsertClubs(ctx, []database.Club{{
		ID:                           club.ID,
		Name:                         club.Name,
		AvatarURL:                    club.AvatarURL,
		CreatorID:                    club.Creator.ID,
		CreatedByCommunityAmbassador: club.CreatedByCommunityAmbassador,
		RawJSON:                      club.Raw,
	}}); err != nil {
		return fmt.Errorf("failed to insert club: %w", err)
	}

	return nil
}

func (s *Server) ProcessFullEventImport(ctx context.Context, event campfire.Event) error {
	return s.ProcessEventImport(ctx, event, eventMembers(event))
}

func (s *Server) ProcessEventImport(ctx context.Context, event campfire.Event, members []campfire.Member) error {
	if err := s.DB.InsertMembers(ctx, []database.Member{{
		ID:          event.Creator.ID,
		Username:    event.Creator.Username,
		DisplayName: event.Creator.DisplayName,
		AvatarURL:   event.Creator.AvatarURL,
		RawJSON:     event.Creator.Raw,
	}}); err != nil {
		return fmt.Errorf("failed to insert creator member: %w", err)
	}

	dbEvent := database.Event{
		ID:                           event.ID,
		Name:                         event.Name,
		Details:                      event.Details,
		Address:                      event.Address,
		Location:                     event.Location,
		CreatorID:                    event.Creator.ID,
		CoverPhotoURL:                event.CoverPhotoURL,
		Time:                         event.EventTime,
		EndTime:                      event.EventEndTime,
		Finished:                     event.EventEndTime.Before(time.Now()),
		DiscordInterested:            event.DiscordInterested,
		CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
		CampfireLiveEventID:          event.CampfireLiveEventID,
		CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
		Category:                     eventcategory.FromName(event.CampfireLiveEvent.EventName),
		ClubID:                       event.ClubID,
		RawJSON:                      event.Raw,
	}

	allMembers := make([]database.Member, 0, len(members))
	for _, member := range members {
		allMembers = append(allMembers, database.Member{
			ID:          member.ID,
			Username:    member.Username,
			DisplayName: member.DisplayName,
			AvatarURL:   member.AvatarURL,
			RawJSON:     member.Raw,
		})
	}

	var rsvps []database.EventRSVP
	for _, rsvp := range event.RSVPStatuses {
		rsvps = append(rsvps, database.EventRSVP{
			EventID:  event.ID,
			MemberID: rsvp.UserID,
			Status:   rsvp.RSVPStatus,
		})
		if !slices.ContainsFunc(allMembers, func(m database.Member) bool {
			return m.ID == rsvp.UserID
		}) {
			allMembers = append(allMembers, database.Member{
				ID:          rsvp.UserID,
				Username:    "",
				DisplayName: "",
				AvatarURL:   "",
				RawJSON:     []byte("{}"),
			})
		}
	}

	if err := s.DB.InsertMembers(ctx, allMembers); err != nil {
		return err
	}

	if err := s.DB.InsertEvents(ctx, []database.Event{dbEvent}); err != nil {
		return err
	}

	if err := s.DB.InsertEventRSVPs(ctx, rsvps); err != nil {
		return err
	}

	return nil
}
