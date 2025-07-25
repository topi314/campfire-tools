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
	Message string   `json:"message"`
	Path    []string `json:"path"`
}

func (e Error) String() string {
	return fmt.Sprintf("Error: %s, Path: %v", e.Message, strings.Join(e.Path, "."))
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

type fullEvent struct {
	Event Event `json:"event"`
}

type Event struct {
	ID                           string    `json:"id"`
	Name                         string    `json:"name"`
	Address                      string    `json:"address"`
	CoverPhotoURL                string    `json:"coverPhotoUrl"`
	Details                      string    `json:"details"`
	EventTime                    time.Time `json:"eventTime"`
	EventEndTime                 time.Time `json:"eventEndTime"`
	RSVPStatus                   string    `json:"rsvpStatus"`
	CreatedByCommunityAmbassador bool      `json:"createdByCommunityAmbassador"`
	BadgeGrants                  []string  `json:"badgeGrants"`
	TopicID                      string    `json:"topicId"`
	CommentCount                 int       `json:"commentCount"`
	DiscordInterested            int       `json:"discordInterested"`
	Creator                      Member    `json:"creator"`
	Club                         Club      `json:"club"`
	Members                      struct {
		TotalCount int `json:"totalCount"`
		Edges      []struct {
			Node Member `json:"node"`
		} `json:"edges"`
	}
	IsPasscodeRewardEligible bool   `json:"isPasscodeRewardEligible"`
	CommentsPermissions      string `json:"commentsPermissions"`
	CommentsPreview          []any  `json:"commentsPreview"`
	IsSubscribed             bool   `json:"isSubscribed"`
	CampfireLiveEventID      string `json:"campfireLiveEventId"`
	CampfireLiveEvent        struct {
		EventName            string `json:"eventName"`
		ModalHeadingImageURL string `json:"modalHeadingImageUrl"`
		ID                   string `json:"id"`
		CheckInRadiusMeters  int    `json:"checkInRadiusMeters"`
	} `json:"campfireLiveEvent"`
	MapPreviewURL string `json:"mapPreviewUrl"`
	Location      string `json:"location"`
	Passcode      string `json:"passcode"`
	RSVPStatuses  []struct {
		UserID     string `json:"userId"`
		RSVPStatus string `json:"rsvpStatus"`
	} `json:"rsvpStatuses"`
	Game                  string `json:"game"`
	ClubID                string `json:"clubId"`
	CheckedInMembersCount int    `json:"checkedInMembersCount"`
	Visibility            string `json:"visibility"`
	Raw                   []byte `json:"-"`
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

type Member struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	AvatarURL   string  `json:"avatarUrl"`
	Badges      []Badge `json:"badges"`
	Raw         []byte  `json:"-"`
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
