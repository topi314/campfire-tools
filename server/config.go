package server

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/topi314/campfire-tools/internal/xtime"
	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

func LoadConfig(cfgPath string) (Config, error) {
	file, err := os.Open(cfgPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	cfg := defaultConfig()
	if _, err = toml.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to decode config file: %w", err)
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Log: LogConfig{
			Level:     slog.LevelInfo,
			Format:    LogFormatText,
			AddSource: false,
		},
		Server: ServerConfig{
			Addr: ":8085",
		},
		Database: database.Config{
			Host:     "localhost",
			Port:     5432,
			Username: "postgres",
			Password: "password",
			Database: "campfire-tools",
		},
		Campfire: campfire.Config{
			Every:      xtime.Duration(1 * time.Second),
			Burst:      40,
			MaxRetries: 3,
		},
	}
}

type Config struct {
	Dev                        bool                `toml:"dev"`
	WarnUnknownEventCategories bool                `toml:"warn_unknown_event_categorie"`
	Log                        LogConfig           `toml:"log"`
	Server                     ServerConfig        `toml:"server"`
	Database                   database.Config     `toml:"database"`
	Campfire                   campfire.Config     `toml:"campfire"`
	Auth                       auth.Config         `toml:"auth"`
	Notifications              NotificationsConfig `toml:"notifications"`
}

func (c Config) String() string {
	return fmt.Sprintf("Dev: %t\nWarnUnknownEventCategories: %t\nLog: %s\nServer: %s\nDatabase: %s\nCampfire: %s\nAuth: %s\nNotifications: %s",
		c.Dev,
		c.WarnUnknownEventCategories,
		c.Log,
		c.Server,
		c.Database,
		c.Campfire,
		c.Auth,
		c.Notifications,
	)
}

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

type LogConfig struct {
	Level     slog.Level `toml:"level"`
	Format    LogFormat  `toml:"format"`
	AddSource bool       `toml:"add_source"`
}

func (c LogConfig) String() string {
	return fmt.Sprintf("\n Level: %s\n Format: %s\n AddSource: %t",
		c.Level,
		c.Format,
		c.AddSource,
	)
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

func (c ServerConfig) String() string {
	return fmt.Sprintf("\n Address: %s",
		c.Addr,
	)
}

type NotificationsConfig struct {
	Enabled    bool   `toml:"enabled"`
	WebhookURL string `toml:"webhook_url"`
}

func (c NotificationsConfig) String() string {
	return fmt.Sprintf("\n Enabled: %t\n WebhookURL: %s",
		c.Enabled,
		c.WebhookURL,
	)
}
