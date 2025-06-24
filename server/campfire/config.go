package campfire

import (
	"fmt"

	"github.com/topi314/campfire-tools/internal/xtime"
)

type Config struct {
	Every      xtime.Duration `toml:"every"`
	Burst      int            `toml:"burst"`
	MaxRetries int            `toml:"max_retries"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n Every: %s\n Burst: %d\n MaxRetries: %d",
		c.Every,
		c.Burst,
		c.MaxRetries,
	)
}
