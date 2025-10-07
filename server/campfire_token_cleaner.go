package server

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/disgoorg/disgo/discord"
)

func (s *Server) cleanup() {
	for {
		s.doNotifyExpiringCampfireTokens()
		s.doCleanupCampfireTokens()
		time.Sleep(5 * time.Minute)
	}
}

func (s *Server) doCleanupCampfireTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.DB.DeleteExpiredCampfireTokens(ctx)
	if err != nil {
		slog.Error("failed to cleanup expired campfire tokens", slog.Any("err", err))
	}

	if rows > 0 {
		s.SendNotification(ctx, fmt.Sprintf("Cleaned up `%d` expired campfire tokens", rows))
	}
}

func (s *Server) doNotifyExpiringCampfireTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	tokens, err := s.DB.GetCampfireTokensExpiringSoon(ctx, 24*time.Hour)
	if err != nil {
		slog.Error("failed to fetch expiring campfire tokens", slog.Any("err", err))
		return
	}

	for _, token := range tokens {
		if slices.Contains(s.SentTokenNotifications, token.ID) {
			continue
		}

		s.SendNotification(ctx, fmt.Sprintf("Campfire token for `%s` is expiring at: %s", token.Email, discord.NewTimestamp(discord.TimestampStyleShortDateTime, token.ExpiresAt).String()))
		s.SentTokenNotifications = append(s.SentTokenNotifications, token.ID)
	}
}
