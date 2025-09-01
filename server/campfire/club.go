package campfire

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"net/url"
)

//go:embed queries/club.graphql
var clubQuery string

func (c *Client) GetClub(ctx context.Context, id string) (*Club, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	var club clubResp
	if err = c.Do(ctx, token, clubQuery, map[string]any{
		"clubId": id,
	}, &club); err != nil {
		return nil, err
	}

	return &club.Club, nil
}

func (c *Client) ResolveClub(ctx context.Context, clubURL string) (*Club, error) {
	clubID, err := c.ResolveClubID(clubURL)
	if err != nil {
		return nil, err
	}

	return c.GetClub(ctx, clubID)
}

func (c *Client) ResolveClubID(clubURL string) (string, error) {
	u, err := url.Parse(clubURL)
	if err != nil {
		return "", err
	}

	query := u.Query()
	sub := query.Get("deep_link_sub1")
	if sub == "" {
		return "", fmt.Errorf("no 'deep_link_sub1' query parameter found in URL: %s", clubURL)
	}

	decoded, err := base64.StdEncoding.DecodeString(sub)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 string: %w", err)
	}

	values, err := url.ParseQuery(string(decoded))
	if err != nil {
		return "", fmt.Errorf("failed to parse decoded string as query parameters: %w", err)
	}

	r := values.Get("r")
	if r != "clubs" {
		return "", fmt.Errorf("unexpected 'r' parameter value in decoded param: %s", string(decoded))
	}

	clubID := values.Get("c")
	if clubID == "" {
		return "", fmt.Errorf("no 'c' parameter found in decoded param: %s", string(decoded))
	}

	return clubID, nil

}
