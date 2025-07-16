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
	"regexp"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	publicEndpoint = "https://niantic-social-api.nianticlabs.com/public/graphql"
	endpoint       = "https://niantic-social-api.nianticlabs.com/graphql"
)

var (
	MeetupURLRegex = regexp.MustCompile(`https://niantic-social.nianticlabs.com/public/meetup(-without-location)?/[a-zA-Z0-9-]+`)

	ErrUnsupportedMeetup = errors.New("meetup not supported")
	ErrTooManyRequests   = errors.New("too many requests, please try again later")

	//go:embed queries/public_events.graphql
	publicEventsQuery string

	//go:embed queries/event.graphql
	eventQuery string
)

func New(cfg Config, httpClient *http.Client) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		limiter:    rate.NewLimiter(rate.Every(time.Duration(cfg.Every)), cfg.Burst),
	}
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	limiter    *rate.Limiter
}

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
			return "", errors.New("event not found")
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
		slog.ErrorContext(ctx, "Failed to fetch events", slog.Int("status_code", rs.StatusCode), slog.String("response", string(data)))
		return nil, fmt.Errorf("request failed with status code: %d", rs.StatusCode)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[Events]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", slog.String("response", logBuf.String()), slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	slog.DebugContext(ctx, "Fetching full event", slog.String("event_id", eventID))
	return c.getEvent(ctx, eventID, 0)
}

func (c *Client) getEvent(ctx context.Context, eventID string, try int) (*Event, error) {
	if try >= c.cfg.MaxRetries {
		return nil, fmt.Errorf("failed to fetch full event after %d retries: %w", c.cfg.MaxRetries, ErrTooManyRequests)
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: eventQuery,
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

	if rs.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(rs.Body)
		slog.ErrorContext(ctx, "Failed to fetch full event", slog.Int("status_code", rs.StatusCode), slog.String("response", string(data)))

		if rs.StatusCode == http.StatusBadGateway {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(30 * time.Second):
			}
			return c.getEvent(ctx, eventID, try+1)
		}

		return nil, fmt.Errorf("request failed with status code: %d", rs.StatusCode)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[fullEvent]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", slog.String("response", logBuf.String()), slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	slog.DebugContext(ctx, "Fetched full event", slog.String("event_id", resp.Data.Event.ID), slog.String("response", logBuf.String()))
	if len(resp.Errors) > 0 {
		var errs []any
		for _, e := range resp.Errors {
			errs = append(errs, slog.String("message", e.String()))
		}
		slog.ErrorContext(ctx, "GraphQL errors", append([]any{slog.String("event_id", eventID)}, errs...)...)
	}

	return &resp.Data.Event, nil
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
