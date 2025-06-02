package server

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
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Members     struct {
			TotalCount int          `json:"totalCount"`
			Edges      []MemberEdge `json:"edges"`
		}
		RSVPStatuses []struct {
			UserID     string `json:"userId"`
			RSVPStatus string `json:"rsvpStatus"`
		} `json:"rsvpStatuses"`
	} `json:"event"`
}

type MemberEdge struct {
	Node struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
}
