package campfire

import (
	"context"
	_ "embed"
)

//go:embed queries/archived_meetups.graphql
var archivedMeetupsQuery string

func (c *Client) GetPastMeetups(ctx context.Context, clubID string) ([]Event, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	var allEvents []Event
	var cursor *string

	for {
		var club archivedFeedResp
		if err := c.Do(ctx, token, archivedMeetupsQuery, map[string]any{
			"first":        100,
			"after":        cursor,
			"membersFirst": 100000000, // Large enough to fetch all members
			"clubId":       clubID,
		}, &club); err != nil {
			return nil, err
		}

		for _, edge := range club.Club.ArchivedFeed.Edges {
			allEvents = append(allEvents, edge.Node)
		}

		if !club.Club.ArchivedFeed.PageInfo.HasNextPage {
			break
		}
		cursor = &club.Club.ArchivedFeed.PageInfo.EndCursor
	}

	return allEvents, nil
}
