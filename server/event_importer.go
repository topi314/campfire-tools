package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"time"

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

	if err = s.DB.UpdateClubLastAutoEventImport(ctx, club.ID); err != nil {
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

		now := time.Now()

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
			DiscordInterested:            event.DiscordInterested,
			CreatedByCommunityAmbassador: event.CreatedByCommunityAmbassador,
			CampfireLiveEventID:          event.CampfireLiveEventID,
			CampfireLiveEventName:        event.CampfireLiveEvent.EventName,
			ClubID:                       event.ClubID,
			ImportedAt:                   now,
			RawJSON:                      event.Raw,
		}

		var dbRSVPs []database.EventRSVP
		for _, rsvpStatus := range event.RSVPStatuses {
			dbRSVPs = append(dbRSVPs, database.EventRSVP{
				EventID:    event.ID,
				MemberID:   rsvpStatus.UserID,
				Status:     rsvpStatus.RSVPStatus,
				ImportedAt: now,
			})
		}

		var dbMembers []database.Member
		members, _, err := s.Campfire.GetEventMembers(ctx, event.ID, nil)
		for _, member := range members {
			dbMembers = append(dbMembers, database.Member{
				ID:          member.ID,
				Username:    member.Username,
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
				ImportedAt:  now,
				RawJSON:     member.Raw,
			})
		}
		if err != nil {
			slog.ErrorContext(ctx, "Failed to fetch event members", slog.String("club_id", clubID), slog.String("event_id", event.ID), slog.Any("err", err))
			continue
		}

		if !slices.ContainsFunc(dbMembers, func(m database.Member) bool {
			return m.ID == event.Creator.ID
		}) {
			dbMembers = append(dbMembers, database.Member{
				ID:          event.Creator.ID,
				Username:    event.Creator.Username,
				DisplayName: event.Creator.DisplayName,
				AvatarURL:   event.Creator.AvatarURL,
				ImportedAt:  time.Now(),
				RawJSON:     event.Creator.Raw,
			})
		}
		for _, rsvp := range dbRSVPs {
			if !slices.ContainsFunc(dbMembers, func(m database.Member) bool {
				return m.ID == rsvp.MemberID
			}) {
				dbMembers = append(dbMembers, database.Member{
					ID:          rsvp.MemberID,
					Username:    "",
					DisplayName: "",
					AvatarURL:   "",
					RawJSON:     []byte("{}"),
					ImportedAt:  rsvp.ImportedAt,
				})
			}
		}

		if err = s.DB.InsertMembers(ctx, dbMembers); err != nil {
			slog.ErrorContext(ctx, "Failed to insert members", slog.String("club_id", clubID), slog.String("event_id", event.ID), slog.Any("err", err))
			continue
		}

		if err = s.DB.InsertEvents(ctx, []database.Event{dbEvent}); err != nil {
			slog.ErrorContext(ctx, "Failed to insert event", slog.String("club_id", clubID), slog.String("event_id", event.ID), slog.Any("err", err))
			continue
		}

		if err = s.DB.InsertEventRSVPs(ctx, dbRSVPs); err != nil {
			slog.ErrorContext(ctx, "Failed to insert event RSVPs", slog.String("club_id", clubID), slog.String("event_id", event.ID), slog.Any("err", err))
			continue
		}
		slog.InfoContext(ctx, "Imported event",
			slog.String("club_id", clubID),
			slog.String("event_id", event.ID),
			slog.String("event_name", event.Name),
			slog.Int("rsvps", len(dbRSVPs)),
			slog.Int("members", len(dbMembers)),
		)
	}

	return nil
}
