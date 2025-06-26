package auth

import (
	"fmt"
	"strings"
)

type Config struct {
	PublicURL      string `toml:"public_url"`
	ClientID       string `toml:"client_id"`
	ClientSecret   string `toml:"client_secret"`
	DiscordGuildID string `toml:"discord_guild_id"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n PublicURL: %s\n ClientID: %s\n ClientSecret: %s\n DiscordGuildID: %s",
		c.PublicURL,
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
		c.DiscordGuildID,
	)
}
