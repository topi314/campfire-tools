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

func (s *Server) updateEvents() {
	for {
		s.doUpdateEvents()
		time.Sleep(10 * time.Second)
	}
}

func (s *Server) doUpdateEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.doUpdateNextEvent(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to update next event", slog.Any("err", err))
	}
}

func (s *Server) doUpdateNextEvent(ctx context.Context) error {
	event, err := s.DB.GetNextUpdateEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	slog.InfoContext(ctx, "Updating event", slog.String("club_id", event.ClubID), slog.String("event_id", event.ID), slog.String("event_name", event.Name))

	importErr := s.importEvent(ctx, event.ID)

	if err = s.DB.UpdateClubLastAutoEventImport(ctx, event.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to update club last auto event import", slog.String("club_id", event.ClubID), slog.Any("err", err))
	}

	return importErr
}

func (s *Server) importEvent(ctx context.Context, eventID string) error {
	event, err := s.Campfire.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}

	if event.ID == "" {
		return nil
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
		return err
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
		return err
	}

	if err = s.DB.InsertEvents(ctx, []database.Event{dbEvent}); err != nil {
		return err
	}

	if err = s.DB.InsertEventRSVPs(ctx, dbRSVPs); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Updated event",
		slog.String("club_id", event.Club.ID),
		slog.String("event_id", event.ID),
		slog.String("event_name", event.Name),
		slog.Int("rsvps", len(dbRSVPs)),
		slog.Int("members", len(dbMembers)),
	)

	return nil
}
