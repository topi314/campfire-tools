package server

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

const (
	publicEndpoint = "https://niantic-social-api.nianticlabs.com/public/graphql"
	endpoint       = "https://niantic-social-api.nianticlabs.com/graphql"
)

var (
	//go:embed queries/public_events.graphql
	publicEventsQuery string

	//go:embed queries/full_event.graphql
	fullEventQuery string
)

func (s *Server) FetchEvents(eventID string) (*Events, error) {
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

	rs, err := s.Client.Do(rq)
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

func (s *Server) FetchFullEvent(eventID string) (*FullEvent, error) {
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

	rs, err := s.Client.Do(rq)
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
