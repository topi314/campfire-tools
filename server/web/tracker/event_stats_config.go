package tracker

// ConfiguredEvents controls the event dropdown on /tracker/event-stats.
//
// This is the list you edit by hand: add one entry per event you want to show
// in the dropdown, and list each day of that event with its Campfire live event
// ID (copy the IDs straight from the database). An event can have any number of
// days.
var ConfiguredEvents = []ConfiguredEvent{
	{
		Key:  "go-fest-2026",
		Name: "GO Fest 2026",
		Days: []ConfiguredDay{
			{Label: "Saturday", LiveEventID: "b4439a90-5630-4249-bfd2-99cae5217fc1"},
			{Label: "Sunday", LiveEventID: "0c33ab52-f475-4f74-a090-fa02d2c95b85"},
		},
	},
	{
		Key:  "go-fest-2026-road-of-legends",
		Name: "GO Fest 2026 & Road of Legends",
		Days: []ConfiguredDay{
			{Label: "Monday", LiveEventID: "a28f8b17-9a5e-4aa7-a6b4-e9180480bf99"},
			{Label: "Tuesday", LiveEventID: "b18744df-978f-4460-aad2-faab1ffcd476"},
			{Label: "Wednesday", LiveEventID: "744c56d0-02cf-4d0b-8923-5a5f3605cec1"},
			{Label: "Thursday", LiveEventID: "cddafcd0-4d81-4a48-9b1e-1a123317fabd"},
			{Label: "Friday", LiveEventID: "419a711c-79cc-4783-aac8-338edfc7d584"},
			{Label: "Saturday", LiveEventID: "b4439a90-5630-4249-bfd2-99cae5217fc1"},
			{Label: "Sunday", LiveEventID: "0c33ab52-f475-4f74-a090-fa02d2c95b85"},
		},
	},
	{
		Key:  "go-tour-2026",
		Name: "GO Tour 2026",
		Days: []ConfiguredDay{
			{Label: "Saturday", LiveEventID: "a55e08ec-ab64-42f8-a757-8132824e3a02"},
			{Label: "Sunday", LiveEventID: "98004aa9-413b-4c3e-bba0-fca2e3e6d395"},
		},
	},
	{
		Key:  "go-tour-2026-road-to-kalos",
		Name: "GO Tour 2026 & Road to Kalos",
		Days: []ConfiguredDay{
			{Label: "Monday", LiveEventID: "22724136-3cd1-4ade-9f02-9c59e2d3c14a"},
			{Label: "Tuesday", LiveEventID: "b651e53e-6b63-4891-a885-eb08701286a1"},
			{Label: "Wednesday", LiveEventID: "64e5abc2-8c54-4391-bf9f-6f0213125655"},
			{Label: "Thursday", LiveEventID: "968cf82e-f971-4c80-9881-7eb7fafb5232"},
			{Label: "Friday", LiveEventID: "aaa5e6e2-5f44-401a-afc3-fddd2324bb23"},
			{Label: "Saturday", LiveEventID: "a55e08ec-ab64-42f8-a757-8132824e3a02"},
			{Label: "Sunday", LiveEventID: "98004aa9-413b-4c3e-bba0-fca2e3e6d395"},
		},
	},
}

// ConfiguredEvent is a single selectable event in the dropdown.
type ConfiguredEvent struct {
	Key  string // stable slug used in the URL (?event=...)
	Name string // shown in the dropdown
	Days []ConfiguredDay
}

// ConfiguredDay maps a human label to one Campfire live event ID.
type ConfiguredDay struct {
	Label       string // e.g. "Day 1"; shown as the breakdown row label
	LiveEventID string // Campfire live event ID from the database
}

// LiveEventIDs returns all non-empty live event IDs configured for the event.
func (e ConfiguredEvent) LiveEventIDs() []string {
	ids := make([]string, 0, len(e.Days))
	for _, day := range e.Days {
		if day.LiveEventID == "" {
			continue
		}
		ids = append(ids, day.LiveEventID)
	}
	return ids
}

// findConfiguredEvent looks up a configured event by its key.
func findConfiguredEvent(key string) (ConfiguredEvent, bool) {
	for _, event := range ConfiguredEvents {
		if event.Key == key {
			return event, true
		}
	}
	return ConfiguredEvent{}, false
}
