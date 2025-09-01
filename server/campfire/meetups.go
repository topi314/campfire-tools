package campfire

import (
	"context"
	_ "embed"
)

//go:embed queries/archived_meetups.graphql
var archivedMeetupsQuery string

func (c *Client) GetPastMeetups(ctx context.Context, token string, clubID string) ([]Event, error) {
	var allEvents []Event
	var cursor *string

	for {
		var club ArchivedFeed
		if err := c.Do(ctx, token, archivedMeetupsQuery, map[string]any{
			"first":        100,
			"after":        cursor,
			"membersFirst": 1000000000000000000, // Large enough to fetch all members
			"clubId":       clubID,
		}, &club); err != nil {
			return nil, err
		}

		for _, edge := range club.ArchivedFeed.Edges {
			allEvents = append(allEvents, edge.Node)
		}

		if !club.ArchivedFeed.PageInfo.HasNextPage {
			break
		}
		cursor = &club.ArchivedFeed.PageInfo.EndCursor
	}

	return allEvents, nil
}
