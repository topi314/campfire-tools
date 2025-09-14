package campfire

import (
	"context"
	_ "embed"
)

const (
	eventsPerPage  = 100
	membersPerPage = 100
)

var (
	//go:embed queries/archived_events.graphql
	archivedEventsQuery string

	//go:embed queries/event_members.graphql
	eventMembersQuery string
)

func (c *Client) GetPastEvents(ctx context.Context, clubID string, initialCursor *string) ([]Event, *string, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, nil, err
	}

	var allEvents []Event
	var cursor *string
	if initialCursor != nil {
		cursor = initialCursor
	}

	for {
		var club archivedFeedResp
		if err = c.Do(ctx, token, archivedEventsQuery, map[string]any{
			"clubId": clubID,
			"first":  eventsPerPage,
			"after":  cursor,
		}, &club); err != nil {
			return allEvents, cursor, err
		}

		for _, edge := range club.Club.ArchivedFeed.Edges {
			allEvents = append(allEvents, edge.Node)
		}

		if !club.Club.ArchivedFeed.PageInfo.HasNextPage {
			break
		}
		cursor = &club.Club.ArchivedFeed.PageInfo.EndCursor
	}

	return allEvents, nil, nil
}

func (c *Client) GetEventMembers(ctx context.Context, eventID string, initialCursor *string) ([]Member, *string, error) {
	var (
		members      []Member
		memberCursor *string
	)
	if initialCursor != nil {
		memberCursor = initialCursor
	}

	for {
		var event eventResp
		if err := c.Do(ctx, "", eventMembersQuery, map[string]any{
			"eventId": eventID,
			"first":   membersPerPage,
			"after":   memberCursor,
		}, &event); err != nil {
			return members, memberCursor, err
		}

		for _, edge := range event.Event.Members.Edges {
			members = append(members, edge.Node)
		}

		if !event.Event.Members.PageInfo.HasNextPage {
			break
		}
		memberCursor = &event.Event.Members.PageInfo.EndCursor
	}

	return members, nil, nil
}
