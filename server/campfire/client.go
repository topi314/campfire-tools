package campfire

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	publicEndpoint = "https://niantic-social-api.nianticlabs.com/public/graphql"
	endpoint       = "https://niantic-social-api.nianticlabs.com/graphql"
)

var (
	ErrTooManyRequests = errors.New("too many requests, please try again later")
	ErrNotFound        = errors.New("not found")
	ErrEventNotFound   = errors.New("event not found")
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

func (c *Client) Do(ctx context.Context, token string, query string, vars map[string]any, rsBody any) error {
	for range c.cfg.MaxRetries {
		if err := c.do(ctx, token, query, vars, rsBody); err != nil {
			if errors.Is(err, ErrTooManyRequests) {
				continue
			}
			return err
		}
	}

	return ErrTooManyRequests
}

func (c *Client) do(ctx context.Context, token string, query string, vars map[string]any, rsBody any) error {
	buff := new(bytes.Buffer)
	if err := json.NewEncoder(buff).Encode(Req{
		Query:     query,
		Variables: vars,
	}); err != nil {
		return err
	}

	rq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buff)
	if err != nil {
		return err
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")
	if token != "" {
		rq.Header.Set("Authorization", "Bearer "+token)
	}

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		return err
	}
	defer rs.Body.Close()

	switch rs.StatusCode {
	case http.StatusTooManyRequests:
		return ErrTooManyRequests
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusOK:
		// All good
	default:
		return fmt.Errorf("request failed with status: %s", rs.Status)
	}

	var resp Resp[json.RawMessage]
	if err = json.NewDecoder(rs.Body).Decode(&resp); err != nil {
		return err
	}

	if len(resp.Errors) > 0 {
		var errs []any
		for _, e := range resp.Errors {
			errs = append(errs, slog.String("message", e.String()))
		}
		slog.ErrorContext(ctx, "GraphQL errors", errs...)
	}

	if err = json.Unmarshal(resp.Data, rsBody); err != nil {
		return err
	}

	return nil
}
