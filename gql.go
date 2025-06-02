package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func (s *Server) FetchEvent(eventID string) (*Resp, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(Req{
		Query: graphQLQuery,
		Variables: map[string]any{
			"id":         eventID,
			"isLoggedIn": false,
			"pageSize":   10000000000, // Large enough to fetch all members
		},
	}); err != nil {
		log.Fatalf("Failed to encode request body: %s", err)
	}

	rq, err := http.NewRequest(http.MethodPost, graphQLEndpoint, buf)
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

	var resp Resp
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		log.Fatalf("Failed to decode response: %s, response: %s", err, logBuf.String())
	}

	return &resp, nil
}
