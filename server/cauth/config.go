package cauth

import (
	"fmt"
	"strings"
)

type Config struct {
	PublicURL    string `toml:"public_url"`
	AuthURL      string `toml:"auth_url"`
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n PublicURL: %s\n AuthURL: %s\n ClientID: %s\n ClientSecret: %s",
		c.PublicURL,
		c.AuthURL,
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
	)
}
