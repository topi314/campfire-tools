package campfire

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	MeetupURLRegex = regexp.MustCompile(`https://niantic-social.nianticlabs.com/public/meetup/[a-zA-Z0-9-]+`)

	//go:embed queries/public_events.graphql
	publicEventsQuery string

	//go:embed queries/full_event.graphql
	fullEventQuery string
)

func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type Client struct {
	httpClient *http.Client
}

func (c *Client) FetchEvent(meetupURL string) (*FullEvent, error) {
	var campfireEventID string
	if !strings.HasPrefix(meetupURL, "https://campfire.nianticlabs.com/discover/meetup/") {
		if strings.HasPrefix(meetupURL, "https://cmpf.re/") {
			var err error
			meetupURL, err = c.ResolveShortURL(meetupURL)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve short URL: %w", err)
			}
		}

		if !strings.HasPrefix(meetupURL, "https://niantic-social.nianticlabs.com/public/meetup/") {
			return nil, errors.New("invalid URL. Must start with 'https://niantic-social.nianticlabs.com/public/meetup/' or 'https://cmpf.re/' or 'https://campfire.nianticlabs.com/discover/meetup/'")
		}
		eventID := path.Base(meetupURL)
		if eventID == "" {
			return nil, errors.New("could not extract event ID from URL")
		}

		events, err := c.FetchEvents(eventID)
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

	event, err := c.FetchFullEvent(campfireEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event: %w", err)
	}

	return event, nil
}

func (c *Client) FetchEvents(eventID string) (*Events, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: publicEventsQuery,
		Variables: map[string]any{
			"ids": []string{eventID},
		},
	}); err != nil {
		log.Fatalf("Failed to encode request body: %s", err)
	}

	rq, err := http.NewRequest(http.MethodPost, publicEndpoint, buf)
	if err != nil {
		log.Fatalf("Failed to create request: %s", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		log.Fatalf("Failed to send request: %s", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(rs.Body)
		log.Fatalf("Request failed with status code: %d, response: %s", rs.StatusCode, data)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[Events]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		log.Fatalf("Failed to decode response: %s, response: %s", err, logBuf.String())
	}

	return &resp.Data, nil
}

func (c *Client) FetchFullEvent(eventID string) (*FullEvent, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: fullEventQuery,
		Variables: map[string]any{
			"id":         eventID,
			"isLoggedIn": false,
			"pageSize":   10000000, // Large enough to fetch all members
		},
	}); err != nil {
		log.Fatalf("Failed to encode request body: %s", err)
	}

	rq, err := http.NewRequest(http.MethodPost, endpoint, buf)
	if err != nil {
		log.Fatalf("Failed to create request: %s", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		log.Fatalf("Failed to send request: %s", err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(rs.Body)
		log.Fatalf("Request failed with status code: %d, response: %s", rs.StatusCode, data)
	}

	logBuf := &bytes.Buffer{}
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[FullEvent]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		log.Fatalf("Failed to decode response: %s, response: %s", err, logBuf.String())
	}

	return &resp.Data, nil
}

func (c *Client) ResolveShortURL(shortURL string) (string, error) {
	rs, err := c.httpClient.Get(shortURL)
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
