package models

// EventStatsVars is the view model for the /tracker/event-stats page.
type EventStatsVars struct {
	User             DiscordUser
	Clubs            []ClubOption
	SelectedClubIDs  []string
	Events           []EventOption
	SelectedEventKey string
	EventName        string
	HasResults       bool
	MultiClub        bool
	ClubNames        []string
	Combined         ClubStats
	PerClub          []ClubStats
	Where            []WhereRow
	Errors           []string
}

// ClubOption is one selectable club in the club checkbox list.
type ClubOption struct {
	ID       string
	Name     string
	Selected bool
}

// EventOption is one selectable event in the event dropdown.
type EventOption struct {
	Key      string
	Name     string
	Selected bool
}

// ClubStats holds the full per-day / overall / attended-all breakdown for one
// scope (either a single club or the combined union across clubs).
type ClubStats struct {
	ClubID      string
	Name        string
	Days        []DayStat
	Overall     DayStat
	AttendedAll MetricGroup
}

// DayStat holds the check-in and accepted breakdown for a single day (one
// Campfire live event) of the selected event.
type DayStat struct {
	Label    string
	CheckIns MetricGroup
	Accepted MetricGroup
}

// MetricGroup is a single stat: a count plus the members behind it and an
// anchor ID linking to the rendered member list.
type MetricGroup struct {
	AnchorID string
	Title    string
	Count    int
	Members  []Member
}

// WhereRow is one member's cross-club check-in row for the "who checked in
// where" matrix. CheckedIn is aligned with EventStatsVars.ClubNames order.
type WhereRow struct {
	Member    Member
	CheckedIn []bool
	ClubCount int
	MultiClub bool
}
