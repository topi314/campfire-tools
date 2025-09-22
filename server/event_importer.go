package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

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

	importErr := s.importActiveClubEvents(ctx, club.ID)

	if err = s.DB.UpdateClubLastAutoEventImported(ctx, club.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to update club last auto event import", slog.String("club_id", club.ID), slog.Any("err", err))
	}

	return importErr
}

func (s *Server) importActiveClubEvents(ctx context.Context, clubID string) error {
	events, _, err := s.Campfire.GetFutureEvents(ctx, clubID, nil)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "Found future events to import", slog.String("club_id", clubID), slog.Int("events", len(events)))

	for _, event := range events {
		// Skip if event already exists
		if _, err = s.DB.GetEvent(ctx, event.ID); err == nil {
			continue
		}

		members, _, err := s.Campfire.GetEventMembers(ctx, event.ID, nil)
		if err != nil {
			return err
		}

		if processErr := s.ProcessEventImport(ctx, event, members, true); processErr != nil {
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

func (s *Server) ProcessFullEventImport(ctx context.Context, event campfire.Event, skipClub bool) error {
	members := make([]campfire.Member, 0, len(event.Members.Edges))
	for _, member := range event.Members.Edges {
		members = append(members, member.Node)
	}
	return s.ProcessEventImport(ctx, event, members, skipClub)
}

func (s *Server) ProcessEventImport(ctx context.Context, event campfire.Event, members []campfire.Member, skipClub bool) error {
	clubMembers := []database.Member{
		{
			ID:          event.Creator.ID,
			Username:    event.Creator.Username,
			DisplayName: event.Creator.DisplayName,
			AvatarURL:   event.Creator.AvatarURL,
			RawJSON:     event.Creator.Raw,
		},
	}
	if !slices.ContainsFunc(clubMembers, func(item database.Member) bool {
		return item.ID == event.Club.Creator.ID
	}) && !skipClub {
		clubMembers = append(clubMembers, database.Member{
			ID:          event.Club.Creator.ID,
			Username:    event.Club.Creator.Username,
			DisplayName: event.Club.Creator.DisplayName,
			AvatarURL:   event.Club.Creator.AvatarURL,
			RawJSON:     event.Club.Creator.Raw,
		})
	}

	if err := s.DB.InsertMembers(ctx, clubMembers); err != nil {
		return fmt.Errorf("failed to insert creator member: %w", err)
	}

	if !skipClub {
		if err := s.DB.InsertClubs(ctx, []database.Club{
			{
				ID:                           event.Club.ID,
				Name:                         event.Club.Name,
				AvatarURL:                    event.Club.AvatarURL,
				CreatorID:                    event.Club.Creator.ID,
				CreatedByCommunityAmbassador: event.Club.CreatedByCommunityAmbassador,
				RawJSON:                      event.Club.Raw,
			},
		}); err != nil {
			return fmt.Errorf("failed to insert club: %w", err)
		}
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
