package auth

import (
	"fmt"
	"strings"
)

type Config struct {
	ClientID     string   `toml:"client_id"`
	ClientSecret string   `toml:"client_secret"`
	GuildIDs     []string `toml:"guild_ids"`
	Whitelist    []string `toml:"whitelist"`
	Admins       []string `toml:"admins"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n ClientID: %s\n ClientSecret: %s\n GuildID: %s\n Whitelist: %s\n Admins: %s",
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
		c.GuildIDs,
		strings.Join(c.Whitelist, ", "),
		strings.Join(c.Admins, ", "),
	)
}
