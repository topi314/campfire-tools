package campfire

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed queries/create_event_link.graphql
var createEventLinkQuery string

type CreateLinkInput struct {
	EventID string `json:"eventId"`
}

type CreateLinkVars struct {
	Input CreateLinkInput `json:"input"`
}

type createLinkResp struct {
	CreateLink struct {
		Link struct {
			URL string `json:"url"`
		} `json:"link"`
	} `json:"createLink"`
}

func (c *Client) CreateEventLink(ctx context.Context, eventID string) (string, error) {
	token, err := c.token(ctx)
	if err != nil {
		return "", err
	}

	var resp createLinkResp
	if err = c.Do(ctx, token, createEventLinkQuery, CreateLinkVars{
		Input: CreateLinkInput{EventID: eventID},
	}, &resp); err != nil {
		return "", err
	}

	if resp.CreateLink.Link.URL == "" {
		return "", fmt.Errorf("createLink returned empty url")
	}

	return resp.CreateLink.Link.URL, nil
}
