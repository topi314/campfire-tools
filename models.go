package main

type Req struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type Resp struct {
	Data struct {
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
	} `json:"data"`
}

type MemberEdge struct {
	Node struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
}
