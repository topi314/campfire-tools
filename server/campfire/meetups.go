package campfire

import (
	"context"
	_ "embed"
	"time"
)

const (
	eventsPerPage  = 25
	membersPerPage = 25
)

var (
	//go:embed queries/archived_events.graphql
	archivedEventsQuery string

	//go:embed queries/event_members.graphql
	eventMembersQuery string
)

func (c *Client) GetPastMeetups(ctx context.Context, clubID string) ([]Event, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	var allEvents []Event
	var cursor *string

	for {
		var club archivedFeedResp
		if err = c.Do(ctx, token, archivedEventsQuery, map[string]any{
			"clubId":       clubID,
			"first":        eventsPerPage,
			"after":        cursor,
			"membersFirst": membersPerPage,
		}, &club); err != nil {
			return nil, err
		}

		for _, edge := range club.Club.ArchivedFeed.Edges {
			if edge.Node.Members.PageInfo.HasNextPage {
				memberCursor := edge.Node.Members.PageInfo.EndCursor
				for {
					var event eventResp
					if err = c.Do(ctx, token, eventMembersQuery, map[string]any{
						"eventId": edge.Node.ID,
						"first":   membersPerPage,
						"after":   memberCursor,
					}, &event); err != nil {
						return nil, err
					}

					edge.Node.Members.Edges = append(edge.Node.Members.Edges, event.Event.Members.Edges...)

					if !event.Event.Members.PageInfo.HasNextPage {
						break
					}
					memberCursor = event.Event.Members.PageInfo.EndCursor
					time.Sleep(3 * time.Second)
				}
			}
			allEvents = append(allEvents, edge.Node)
		}

		if !club.Club.ArchivedFeed.PageInfo.HasNextPage {
			break
		}
		cursor = &club.Club.ArchivedFeed.PageInfo.EndCursor
	}

	return allEvents, nil
}
