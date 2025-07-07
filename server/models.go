package server

type Club struct {
	ClubID        string
	ClubName      string
	ClubAvatarURL string
}

type Event struct {
	ID            string
	Name          string
	URL           string
	CoverPhotoURL string
}

type TopMembers struct {
	Count   int
	Open    bool
	Members []TopMember
}

type TopEvents struct {
	Count         int
	Open          bool
	Events        []TopEvent
	TotalCheckIns int
	TotalAccepted int
}

type EventCategories struct {
	Open       bool
	Categories []EventCategory
}

type EventCategory struct {
	Name     string
	CheckIns int
	Accepted int
}

type Member struct {
	ID          string
	Username    string
	DisplayName string
	AvatarURL   string
	URL         string
}

type TopMember struct {
	Member
	CheckIns int
}

type TopEvent struct {
	Event
	Accepted int
	CheckIns int
}
