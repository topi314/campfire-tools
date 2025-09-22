package campfire

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

//go:embed queries/public_events.graphql
var publicEventsQuery string

var ErrUnsupportedMeetup = errors.New("meetup not supported")

func (c *Client) ResolveEventID(ctx context.Context, meetupURL string) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", err
	}

	var campfireEventID string
	if !strings.HasPrefix(meetupURL, "https://campfire.nianticlabs.com/discover/meetup/") {
		if strings.HasPrefix(meetupURL, "https://cmpf.re/") {
			var err error
			meetupURL, err = c.ResolveShortURL(ctx, meetupURL)
			if err != nil {
				return "", fmt.Errorf("failed to resolve short URL: %w", err)
			}
		}

		if strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup-without-location/") {
			return "", ErrUnsupportedMeetup
		}

		if !strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup/") {
			return "", errors.New("invalid URL. Must start with 'https://niantic-social.nianticlabs.com/public/meetup/', 'https://cmpf.re/' or 'https://campfire.nianticlabs.com/discover/meetup/'")
		}
		eventID := path.Base(meetupURL)
		if eventID == "" {
			return "", errors.New("could not extract event ID from URL")
		}

		events, err := c.GetEvents(ctx, []string{eventID})
		if err != nil {
			return "", fmt.Errorf("failed to fetch events: %w", err)
		}

		if len(events.PublicMapObjectsByID) == 0 {
			return "", ErrEventNotFound
		}

		firstEvent := events.PublicMapObjectsByID[0]

		if firstEvent.ID != eventID {
			return "", fmt.Errorf("event ID mismatch: expected %s, got %s", campfireEventID, firstEvent.Event.ID)
		}
		campfireEventID = firstEvent.Event.ID
	} else {
		campfireEventID = path.Base(meetupURL)
	}

	if campfireEventID == "" {
		return "", fmt.Errorf("invalid URL: %s", meetupURL)
	}

	return campfireEventID, nil
}

func (c *Client) ResolveEvent(ctx context.Context, meetupURL string) (*Event, error) {
	campfireEventID, err := c.ResolveEventID(ctx, meetupURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve event ID: %w", err)
	}

	event, err := c.GetEvent(ctx, campfireEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch full event: %w", err)
	}

	return event, nil
}

func (c *Client) GetEvents(ctx context.Context, eventIDs []string) (*Events, error) {
	return c.getEvents(ctx, eventIDs, 0)
}

func (c *Client) getEvents(ctx context.Context, eventIDs []string, try int) (*Events, error) {
	if try >= c.cfg.MaxRetries {
		return nil, fmt.Errorf("failed to fetch events after %d retries: %w", c.cfg.MaxRetries, ErrTooManyRequests)
	}
	buf := new(bytes.Buffer)
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
		slog.ErrorContext(ctx, "Failed to fetch events", slog.Int("status_code", rs.StatusCode), slog.String("response", string(data)))
		return nil, fmt.Errorf("request failed with status code: %d", rs.StatusCode)
	}

	logBuf := new(bytes.Buffer)
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[Events]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", slog.String("response", logBuf.String()), slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp.Data, nil
}
