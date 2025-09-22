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
	"regexp"
	"time"
)

//go:embed queries/event.graphql
var eventQuery string

var meetupURLRegex = regexp.MustCompile(`https://niantic-social.nianticlabs.com/public/meetup(-without-location)?/[a-zA-Z0-9-]+`)

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

	url := meetupURLRegex.FindString(string(html))
	if url == "" {
		return "", fmt.Errorf("no valid meetup URL found in response")
	}
	return url, nil
}

func (c *Client) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	slog.DebugContext(ctx, "Fetching full event", slog.String("event_id", eventID))
	return c.getEvent(ctx, eventID, 0)
}

func (c *Client) getEvent(ctx context.Context, eventID string, try int) (*Event, error) {
	if try >= c.cfg.MaxRetries {
		return nil, fmt.Errorf("failed to fetch full event after %d retries: %w", c.cfg.MaxRetries, ErrTooManyRequests)
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(Req{
		Query: eventQuery,
		Variables: map[string]any{
			"id":    eventID,
			"first": 100000000, // Large enough to fetch all members
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

	logBuf := new(bytes.Buffer)
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[eventResp]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", slog.String("response", logBuf.String()), slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	slog.DebugContext(ctx, "Fetched full event", slog.String("event_id", resp.Data.Event.ID), slog.String("response", logBuf.String()))

	if len(resp.Errors) == 0 {
		return &resp.Data.Event, nil
	}

	var (
		errArgs []any
		errs    error
	)
	for _, e := range resp.Errors {
		errArgs = append(errArgs, slog.String("message", e.String()))

		switch e.Message {
		case "event not found":
			err = fmt.Errorf("%w: %w", ErrEventNotFound, e)
		default:
			err = e
		}
		errs = errors.Join(errs, err)
	}
	slog.ErrorContext(ctx, "GraphQL errors", append([]any{slog.String("event_id", eventID)}, errArgs...)...)
	return nil, fmt.Errorf("graphql errors: %w", errs)
}
