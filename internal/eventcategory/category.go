package eventcategory

import "strings"

const (
	Other   = "Other"
	NoEvent = "No Event"
)

var All = map[string][]string{
	"Raid Day":          {"Raid Day", "Mega Raid"},
	"Raid Hour":         {"Raid Hour"},
	"Max Monday":        {"Max Monday"},
	"Research Day":      {"Research Day"},
	"Hatch Day":         {"Hatch Day"},
	"Community Day":     {"Community Day", "Community Classic Day"},
	"Spotlight Hour":    {"Spotlight Hour"},
	"Max Battle":        {"Max Battle Weekend", "Max Battle Day", "Max Weekend", "Gigantamax", "GMAX"},
	"GO Tour":           {"GO Tour"},
	"GO Fest":           {"GO Fest"},
	"GO Wild Area":      {"GOWA", "GO Wild Area"},
	"Friendship Friday": {"Friendship Friday"},
}

var Ordered = []string{
	"GO Wild Area",
	"GO Fest",
	"GO Tour",
	"Community Day",
	"Max Battle",
	"Research Day",
	"Hatch Day",
	"Friendship Friday",
	"Raid Day",
	"Raid Hour",
	"Max Monday",
	"Spotlight Hour",
}

var DigitalCodeExcluded = []string{
	"Friendship Friday",
}

func DigitalCodeExcludePatterns() []string {
	var patterns []string
	for _, category := range DigitalCodeExcluded {
		for _, name := range All[category] {
			patterns = append(patterns, "%"+name+"%")
		}
	}
	return patterns
}

func FromName(eventName string) string {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if eventName == "" {
		return NoEvent
	}
	for _, category := range Ordered {
		for _, pattern := range All[category] {
			if strings.Contains(eventName, strings.ToLower(pattern)) {
				return category
			}
		}
	}
	return Other
}
