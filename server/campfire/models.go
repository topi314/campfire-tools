package campfire

import (
	"time"
)

type Req struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type Resp[T any] struct {
	Data T `json:"data"`
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

type FullEvent struct {
	Event struct {
		ID                           string    `json:"id"`
		Name                         string    `json:"name"`
		Address                      string    `json:"address"`
		CoverPhotoURL                string    `json:"coverPhotoUrl"`
		Details                      string    `json:"details"`
		EventTime                    time.Time `json:"eventTime"`
		EventEndTime                 time.Time `json:"eventEndTime"`
		RsvpStatus                   string    `json:"rsvpStatus"`
		CreatedByCommunityAmbassador bool      `json:"createdByCommunityAmbassador"`
		BadgeGrants                  []string  `json:"badgeGrants"`
		TopicID                      string    `json:"topicId"`
		CommentCount                 int       `json:"commentCount"`
		DiscordInterested            int       `json:"discordInterested"`
		Creator                      struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			AvatarURL   string `json:"avatarUrl"`
			Badges      []struct {
				BadgeType string `json:"badgeType"`
				Alias     string `json:"alias"`
			} `json:"badges"`
			Username string `json:"username"`
		} `json:"creator"`
		Club struct {
			ID                           string   `json:"id"`
			Name                         string   `json:"name"`
			AvatarURL                    string   `json:"avatarUrl"`
			Visibility                   string   `json:"visibility"`
			MyPermissions                []string `json:"myPermissions"`
			BadgeGrants                  []string `json:"badgeGrants"`
			CreatedByCommunityAmbassador bool     `json:"createdByCommunityAmbassador"`
			Game                         string   `json:"game"`
			AmIMember                    bool     `json:"amIMember"`
			Creator                      struct {
				ID string `json:"id"`
			} `json:"creator"`
		} `json:"club"`
		Members struct {
			TotalCount int          `json:"totalCount"`
			Edges      []MemberEdge `json:"edges"`
		}
		IsPasscodeRewardEligible bool          `json:"isPasscodeRewardEligible"`
		CommentsPermissions      string        `json:"commentsPermissions"`
		CommentsPreview          []interface{} `json:"commentsPreview"`
		IsSubscribed             bool          `json:"isSubscribed"`
		CampfireLiveEventID      string        `json:"campfireLiveEventId"`
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
	} `json:"event"`
}

type MemberEdge struct {
	Node struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
}

func FindMemberName(id string, event FullEvent) (string, bool) {
	for _, edge := range event.Event.Members.Edges {
		if edge.Node.ID == id {
			return edge.Node.DisplayName, true
		}
	}
	return "", false
}
