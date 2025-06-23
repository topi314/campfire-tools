package campfire

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	publicEndpoint = "https://niantic-social-api.nianticlabs.com/public/graphql"
	endpoint       = "https://niantic-social-api.nianticlabs.com/graphql"
)

var (
	MeetupURLRegex = regexp.MustCompile(`https://niantic-social.nianticlabs.com/public/meetup(-without-location)?/[a-zA-Z0-9-]+`)

	ErrUnsupportedMeetup = errors.New("meetup not supported")

	//go:embed queries/public_events.graphql
	publicEventsQuery string

	//go:embed queries/full_event.graphql
	fullEventQuery string
)

func New(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
	}
}

type Client struct {
	httpClient *http.Client
}

func (c *Client) FetchEvent(ctx context.Context, meetupURL string) (*FullEvent, error) {
	var campfireEventID string
	if !strings.HasPrefix(meetupURL, "https://campfire.nianticlabs.com/discover/meetup/") {
		if strings.HasPrefix(meetupURL, "https://cmpf.re/") {
			var err error
			meetupURL, err = c.ResolveShortURL(ctx, meetupURL)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve short URL: %w", err)
			}
		}

		if strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup-without-location/") {
			return nil, ErrUnsupportedMeetup
		}

		if !strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup/") {
			return nil, errors.New("invalid URL. Must start with 'https://niantic-social.nianticlabs.com/public/meetup/', 'https://cmpf.re/' or 'https://campfire.nianticlabs.com/discover/meetup/'")
		}
		eventID := path.Base(meetupURL)
		if eventID == "" {
			return nil, errors.New("could not extract event ID from URL")
		}

		events, err := c.FetchEvents(ctx, []string{eventID})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch event: %w", err)
		}

		if len(events.PublicMapObjectsByID) == 0 {
			return nil, errors.New("event not found")
		}

		firstEvent := events.PublicMapObjectsByID[0]

		if firstEvent.ID != eventID {
			return nil, fmt.Errorf("event ID mismatch: expected %s, got %s", campfireEventID, firstEvent.Event.ID)
		}
		campfireEventID = firstEvent.Event.ID
	} else {
		campfireEventID = path.Base(meetupURL)
	}
	if campfireEventID == "" {
		return nil, fmt.Errorf("invalid URL: %s", meetupURL)
	}

	event, err := c.FetchFullEvent(ctx, campfireEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event: %w", err)
	}

	return event, nil
}

func (c *Client) FetchEvents(ctx context.Context, eventIDs []string) (*Events, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: publicEventsQuery,
		Variables: map[string]any{
			"ids": eventIDs,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	rq, err := http.NewRequestWithContext(ctx, http.MethodPost, publicEndpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(rs.Body)
		return nil, fmt.Errorf("request failed with status code: %d, response: %s", rs.StatusCode, data)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[Events]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %q: %w", logBuf.String(), err)
	}

	return &resp.Data, nil
}

func (c *Client) FetchFullEvent(ctx context.Context, eventID string) (*FullEvent, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: fullEventQuery,
		Variables: map[string]any{
			"id":         eventID,
			"isLoggedIn": false,
			"pageSize":   10000000, // Large enough to fetch all members
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	rq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode == http.StatusBadGateway {
		time.Sleep(5 * time.Second) // Retry after a short delay
		return c.FetchFullEvent(ctx, eventID)
	}

	if rs.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(rs.Body)
		return nil, fmt.Errorf("request failed with status code: %d, response: %s", rs.StatusCode, data)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[FullEvent]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %q: %w", logBuf.String(), err)
	}

	return &resp.Data, nil
}

func (c *Client) ResolveShortURL(ctx context.Context, shortURL string) (string, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, shortURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for short URL: %w", err)
	}

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		return "", fmt.Errorf("failed to resolve short URL: %w", err)
	}
	defer rs.Body.Close()
	if rs.StatusCode != http.StatusOK {
		return "", fmt.Errorf("short URL resolution failed with status: %s", rs.Status)
	}

	html, err := io.ReadAll(rs.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	url := MeetupURLRegex.FindString(string(html))
	if url == "" {
		return "", fmt.Errorf("no valid meetup URL found in response")
	}
	return url, nil
}
