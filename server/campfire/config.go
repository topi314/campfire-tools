package campfire

import (
	"fmt"

	"github.com/topi314/campfire-tools/internal/xtime"
)

type Config struct {
	Every           xtime.Duration `toml:"every"`
	Burst           int            `toml:"burst"`
	MaxRetries      int            `toml:"max_retries"`
	EventAutoUpdate bool           `toml:"event_auto_update"`
	EventAutoImport bool           `toml:"event_auto_import"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n Every: %s\n Burst: %d\n MaxRetries: %d\n EventAutoUpdate: %t\n EventAutoImport: %t",
		c.Every,
		c.Burst,
		c.MaxRetries,
		c.EventAutoUpdate,
		c.EventAutoImport,
	)
}
