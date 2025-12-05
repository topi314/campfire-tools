package cauth

import (
	"fmt"
	"strings"
)

type Config struct {
	AuthURL      string `toml:"auth_url"`
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n AuthURL: %s\n ClientID: %s\n ClientSecret: %s",
		c.AuthURL,
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
	)
}
