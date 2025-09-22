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
	"time"

	"golang.org/x/time/rate"
)

const (
	publicEndpoint = "https://niantic-social-api.nianticlabs.com/public/graphql"
	endpoint       = "https://niantic-social-api.nianticlabs.com/graphql"
)

var (
	ErrTooManyRetries  = errors.New("too many retries, please try again later")
	ErrTooManyRequests = errors.New("too many requests, please try again later")
	ErrBadGateway      = errors.New("bad gateway, please try again later")
	ErrEventNotFound   = errors.New("event not found")
)

type TokenFunc func(ctx context.Context) (string, error)

func New(cfg Config, httpClient *http.Client, token TokenFunc) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		limiter:    rate.NewLimiter(rate.Every(time.Duration(cfg.Every)), cfg.Burst),
		token:      token,
	}
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	limiter    *rate.Limiter
	token      TokenFunc
}

func (c *Client) Do(ctx context.Context, token string, query string, vars map[string]any, rsBody any) error {
	for range c.cfg.MaxRetries {
		if err := c.do(ctx, token, query, vars, rsBody); err != nil {
			if errors.Is(err, ErrTooManyRequests) || errors.Is(err, ErrBadGateway) {
				time.Sleep(time.Second)
				continue
			}
			return err
		}
		return nil
	}

	return ErrTooManyRetries
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

	slog.DebugContext(ctx, "GraphQL request", slog.String("query", query), slog.String("variables", fmt.Sprintf("%+v", vars)))

	rs, err := c.httpClient.Do(rq)
	if err != nil {
		return err
	}
	defer rs.Body.Close()

	switch rs.StatusCode {
	case http.StatusTooManyRequests:
		return ErrTooManyRequests
	case http.StatusBadGateway:
		return ErrBadGateway
	case http.StatusOK:
		// All good
	default:
		return fmt.Errorf("request failed with status: %s", rs.Status)
	}

	logBuf := new(bytes.Buffer)
	bodyReader := io.TeeReader(rs.Body, logBuf)

	var resp Resp[json.RawMessage]
	if err = json.NewDecoder(bodyReader).Decode(&resp); err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", slog.String("response", logBuf.String()), slog.Any("error", err))
		return fmt.Errorf("failed to decode response: %w", err)
	}

	slog.DebugContext(ctx, "GraphQL response", slog.String("response", logBuf.String()))

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
