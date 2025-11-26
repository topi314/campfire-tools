package auth

import (
	"fmt"
	"strings"
)

type Config struct {
	ClientID       string   `toml:"client_id"`
	ClientSecret   string   `toml:"client_secret"`
	DiscordGuildID string   `toml:"discord_guild_id"`
	Whitelist      []string `toml:"whitelist"`
	Admins         []string `toml:"admins"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n ClientID: %s\n ClientSecret: %s\n DiscordGuildID: %s\n Whitelist: %s\n Admins: %s",
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
		c.DiscordGuildID,
		strings.Join(c.Whitelist, ", "),
		strings.Join(c.Admins, ", "),
	)
}
