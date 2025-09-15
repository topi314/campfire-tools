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

var ErrContinueLater = errors.New("continue later")

func (s *Server) importClubs() {
	for {
		s.doImportClubs()
		time.Sleep(5 * time.Second)
	}
}

func (s *Server) doImportClubs() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.doImportNextClub(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to import next club", slog.Any("err", err))
	}
}

func (s *Server) doImportNextClub(ctx context.Context) error {
	job, err := s.DB.GetNextPendingClubImportJob(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	slog.InfoContext(ctx, "Importing club", slog.Int("job_id", job.ID), slog.String("club_id", job.ClubID))

	job.LastTriedAt = time.Now()
	state, err := s.importClubEvents(ctx, *job, job.State.V)
	if err != nil {
		if errors.Is(err, ErrContinueLater) {
			job.Status = database.ClubImportJobStatusPending
		} else {
			job.Status = database.ClubImportJobStatusFailed
			job.Error = err.Error()
		}
	} else {
		job.Status = database.ClubImportJobStatusCompleted
		job.CompletedAt = time.Now()
	}
	job.State.V = state

	if updateErr := s.DB.UpdateClubImportJob(context.WithoutCancel(ctx), *job); updateErr != nil {
		slog.ErrorContext(ctx, "Failed to update club import job after failure", slog.Any("err", updateErr))
	}

	return err
}

func (s *Server) importClubEvents(ctx context.Context, job database.ClubImportJob, state database.ClubImportJobState) (database.ClubImportJobState, error) {
	if len(state.Events) == 0 || state.EventCursor != nil {
		pastEvents, cursor, err := s.Campfire.GetPastEvents(ctx, job.ClubID, state.EventCursor)
		state.EventCursor = cursor
		for _, event := range pastEvents {
			var rsvps []database.EventRSVP
			for _, rsvpStatus := range event.RSVPStatuses {
				rsvps = append(rsvps, database.EventRSVP{
					EventID:    event.ID,
					MemberID:   rsvpStatus.UserID,
					Status:     rsvpStatus.RSVPStatus,
					ImportedAt: time.Now(),
				})
			}

			state.Events = append(state.Events, database.EventState{
				Event: database.Event{
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
					ImportedAt:                   time.Now(),
					RawJSON:                      event.Raw,
				},
				Creator: database.Member{
					ID:          event.Creator.ID,
					Username:    event.Creator.Username,
					DisplayName: event.Creator.DisplayName,
					AvatarURL:   event.Creator.AvatarURL,
					ImportedAt:  time.Now(),
					RawJSON:     event.Creator.Raw,
				},
				RSVPs: rsvps,
			})
		}

		if err != nil {
			return state, err
		}

		if len(pastEvents) == 0 {
			slog.InfoContext(ctx, "No past events found for club", slog.String("club_id", job.ClubID))
			return state, nil
		}
	}

	for {
		if len(state.Events) == 0 {
			break
		}

		event := state.Events[0]

		members, cursor, err := s.Campfire.GetEventMembers(ctx, event.Event.ID, state.MemberCursor)
		state.MemberCursor = cursor
		for _, member := range members {
			state.Members = append(state.Members, database.Member{
				ID:          member.ID,
				Username:    member.Username,
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
				ImportedAt:  time.Now(),
				RawJSON:     member.Raw,
			})
		}
		if err != nil {
			return state, ErrContinueLater
		}

		if !slices.ContainsFunc(state.Members, func(m database.Member) bool {
			return m.ID == event.Creator.ID
		}) {
			state.Members = append(state.Members, event.Creator)
		}
		for _, rsvp := range event.RSVPs {
			if !slices.ContainsFunc(state.Members, func(m database.Member) bool {
				return m.ID == rsvp.MemberID
			}) {
				state.Members = append(state.Members, database.Member{
					ID:          rsvp.MemberID,
					Username:    "",
					DisplayName: "",
					AvatarURL:   "",
					RawJSON:     []byte("{}"),
					ImportedAt:  rsvp.ImportedAt,
				})
			}
		}

		if err = s.DB.InsertMembers(ctx, state.Members); err != nil {
			return state, err
		}

		if err = s.DB.InsertEvents(ctx, []database.Event{event.Event}); err != nil {
			return state, err
		}

		if err = s.DB.InsertEventRSVPs(ctx, event.RSVPs); err != nil {
			return state, err
		}

		state.Events = state.Events[1:]
		state.Members = nil
		state.MemberCursor = nil

		slog.InfoContext(ctx, "Imported event", slog.String("event_id", event.Event.ID), slog.String("event_name", event.Event.Name), slog.Int("remaining_events", len(state.Events)))
	}

	return state, nil
}
