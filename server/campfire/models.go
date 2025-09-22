package campfire

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Req struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type Resp[T any] struct {
	Errors []Error `json:"errors"`
	Data   T       `json:"data"`
}

type Error struct {
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

func (e Error) String() string {
	return e.Error()
}

func (e Error) Error() string {
	msg := fmt.Sprintf("Error: %s", e.Message)
	if len(e.Path) > 0 {
		var path []string
		for _, p := range e.Path {
			path = append(path, fmt.Sprint(p))
		}
		msg += fmt.Sprintf(", Path: %v", strings.Join(path, "."))
	}
	return msg
}

type Pagination[T any] struct {
	TotalCount int       `json:"totalCount"`
	Edges      []Edge[T] `json:"edges"`
	PageInfo   PageInfo  `json:"pageInfo"`
}

type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor string `json:"cursor"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	StartCursor string `json:"startCursor"`
	EndCursor   string `json:"endCursor"`
}

type Events struct {
	PublicMapObjectsByID []struct {
		ID    string `json:"id"`
		Event struct {
			ID                       string `json:"id"`
			Name                     string `json:"name"`
			Details                  string `json:"details"`
			ClubName                 string `json:"clubName"`
			ClubID                   string `json:"clubId"`
			ClubAvatarURL            string `json:"clubAvatarUrl"`
			IsPasscodeRewardEligible bool   `json:"isPasscodeRewardEligible"`
			Place                    any    `json:"place"`
			MapObjectLocation        struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"mapObjectLocation"`
			EventTime    time.Time `json:"eventTime"`
			EventEndTime time.Time `json:"eventEndTime"`
			Address      string    `json:"address"`
		} `json:"event"`
	} `json:"publicMapObjectsById"`
}

type eventResp struct {
	Event Event `json:"event"`
}

type Event struct {
	ID                           string             `json:"id"`
	Name                         string             `json:"name"`
	Visibility                   string             `json:"visibility"`
	Address                      string             `json:"address"`
	CoverPhotoURL                string             `json:"coverPhotoUrl"`
	Details                      string             `json:"details"`
	EventTime                    time.Time          `json:"eventTime"`
	EventEndTime                 time.Time          `json:"eventEndTime"`
	RSVPStatus                   string             `json:"rsvpStatus"`
	CreatedByCommunityAmbassador bool               `json:"createdByCommunityAmbassador"`
	BadgeGrants                  []string           `json:"badgeGrants"`
	TopicID                      string             `json:"topicId"`
	CommentCount                 int                `json:"commentCount"`
	DiscordInterested            int                `json:"discordInterested"`
	Creator                      Member             `json:"creator"`
	Club                         Club               `json:"club"`
	Members                      Pagination[Member] `json:"members"`
	IsPasscodeRewardEligible     bool               `json:"isPasscodeRewardEligible"`
	CommentsPermissions          string             `json:"commentsPermissions"`
	CommentsPreview              []any              `json:"commentsPreview"`
	IsSubscribed                 bool               `json:"isSubscribed"`
	CampfireLiveEventID          string             `json:"campfireLiveEventId"`
	CampfireLiveEvent            struct {
		EventName            string `json:"eventName"`
		ModalHeadingImageURL string `json:"modalHeadingImageUrl"`
		ID                   string `json:"id"`
		CheckInRadiusMeters  int    `json:"checkInRadiusMeters"`
	} `json:"campfireLiveEvent"`
	MapPreviewURL         string       `json:"mapPreviewUrl"`
	Location              string       `json:"location"`
	Passcode              string       `json:"passcode"`
	RSVPStatuses          []RSVPStatus `json:"rsvpStatuses"`
	Game                  string       `json:"game"`
	ClubID                string       `json:"clubId"`
	CheckedInMembersCount int          `json:"checkedInMembersCount"`
	Raw                   []byte       `json:"-"`
}

func (e *Event) UnmarshalJSON(data []byte) error {
	type Alias Event
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = Event(a)
	e.Raw = data
	return nil
}

type RSVPStatus struct {
	UserID     string `json:"userId"`
	RSVPStatus string `json:"rsvpStatus"`
}

type clubResp struct {
	Club Club `json:"club"`
}

type Club struct {
	ID                           string   `json:"id"`
	Name                         string   `json:"name"`
	AvatarURL                    string   `json:"avatarUrl"`
	Visibility                   string   `json:"visibility"`
	MyPermissions                []string `json:"myPermissions"`
	BadgeGrants                  []string `json:"badgeGrants"`
	CreatedByCommunityAmbassador bool     `json:"createdByCommunityAmbassador"`
	Game                         string   `json:"game"`
	AmIMember                    bool     `json:"amIMember"`
	Creator                      Member   `json:"creator"`
	Raw                          []byte   `json:"-"`
}

func (c *Club) UnmarshalJSON(data []byte) error {
	type Alias Club
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*c = Club(a)
	c.Raw = data
	return nil
}

type ClubWithEvents struct {
	Club
	ArchivedFeed
}

type archivedFeedResp struct {
	Club ArchivedFeed `json:"club"`
}

type ArchivedFeed struct {
	ArchivedFeed Pagination[Event] `json:"archivedFeed"`
	Raw          []byte            `json:"-"`
}

func (c *ArchivedFeed) UnmarshalJSON(data []byte) error {
	type Alias ArchivedFeed
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*c = ArchivedFeed(a)
	c.Raw = data
	return nil
}

type activeFeedResp struct {
	Club ActiveFeed `json:"club"`
}

type ActiveFeed struct {
	ActiveFeed Pagination[Event] `json:"activeFeed"`
	Raw        []byte            `json:"-"`
}

func (c *ActiveFeed) UnmarshalJSON(data []byte) error {
	type Alias ActiveFeed
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*c = ActiveFeed(a)
	c.Raw = data
	return nil
}

type Member struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	AvatarURL   string     `json:"avatarUrl"`
	Badges      []Badge    `json:"badges"`
	ClubRoles   []ClubRole `json:"clubRoles"`
	ClubRank    int        `json:"clubRank"`
	Raw         []byte     `json:"-"`
}

func (m *Member) UnmarshalJSON(data []byte) error {
	type Alias Member
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = Member(a)
	m.Raw = data
	return nil
}

type ClubRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Badge struct {
	Alias     string `json:"alias"`
	BadgeType string `json:"badgeType"`
}

func FindMember(id string, event Event) (Member, bool) {
	for _, edge := range event.Members.Edges {
		if edge.Node.ID == id {
			return edge.Node, true
		}
	}
	return Member{}, false
}
