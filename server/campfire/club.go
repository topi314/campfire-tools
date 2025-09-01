package campfire

import (
	"context"
	_ "embed"
)

//go:embed queries/club.graphql
var clubQuery string

func (c *Client) GetClub(ctx context.Context, token string, id string) (*Club, error) {
	var club Club
	if err := c.Do(ctx, token, clubQuery, map[string]any{
		"$clubId": id,
	}, &club); err != nil {
		return nil, err
	}

	return &club, nil
}
