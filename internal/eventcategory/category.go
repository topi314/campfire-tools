package eventcategory

import (
	"slices"
	"strings"
)

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

func IsPreset(name string) bool {
	if name == Other || name == NoEvent {
		return true
	}
	_, ok := All[name]
	return ok
}

func Format(name string) string {
	if name == "" || IsPreset(name) {
		return name
	}
	return name + " (Custom)"
}

// Sort orders preset categories by Ordered, then Other, then No Event,
// then custom categories alphabetically.
func Sort(categories []string) []string {
	if len(categories) == 0 {
		return categories
	}

	index := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		index[category] = struct{}{}
	}

	sorted := make([]string, 0, len(categories))
	for _, category := range Ordered {
		if _, ok := index[category]; ok {
			sorted = append(sorted, category)
			delete(index, category)
		}
	}
	if _, ok := index[Other]; ok {
		sorted = append(sorted, Other)
		delete(index, Other)
	}
	if _, ok := index[NoEvent]; ok {
		sorted = append(sorted, NoEvent)
		delete(index, NoEvent)
	}

	customs := make([]string, 0, len(index))
	for category := range index {
		customs = append(customs, category)
	}
	slices.Sort(customs)
	return append(sorted, customs...)
}
