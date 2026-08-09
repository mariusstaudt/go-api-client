package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func NewClient(name string, options ...ClientOption) *Client {
	c := &Client{name: name}

	for _, option := range options {
		option(c)
	}

	if c.ctx == nil {
		c.ctx = context.Background()
	}

	if c.cacheTTL == 0 {
		c.cacheTTL = time.Minute
	}

	if c.logger == nil {
		c.logger = slog.Default()
	}
	c.logger = c.logger.With(slog.String("client", c.name))

	var internalTransport http.RoundTripper = c.transport
	if c.transport == nil {
		internalTransport = http.DefaultTransport
	}

	internalTransport = &retryTransport{
		next:       internalTransport,
		logger:     c.logger,
		maxRetries: 2,
	}

	internalTransport = &headerTransport{
		next:           internalTransport,
		logger:         c.logger,
		defaultHeaders: c.defaultHeaders,
	}

	internalTransport = &authTransport{
		client: c,
		next:   internalTransport,
		logger: c.logger,
	}

	internalTransport = &cacheTransport{
		next:   internalTransport,
		logger: c.logger,
		TTL:    c.cacheTTL,
		cache:  make(map[string]cachedResponse),
	}

	c.httpClient = &http.Client{
		Transport: internalTransport,
		Timeout:   5 * time.Minute,
	}

	if c.decoder == nil {
		c.decoder = JSONDecodeStrategy
	}

	c.logger.InfoContext(c.ctx, "api client initialized",
		slog.String("baseURL", c.baseURL),
		slog.Bool("hasTokenProvider", c.tokenProvider != nil),
		slog.Bool("hasAuthStrategy", c.authStrategy != nil),
	)

	return c
}

func (c *Client) refreshToken(req *http.Request) error {
	c.tokenMu.Lock()
	newToken, err := c.tokenProvider(req.Context())
	if err != nil {
		c.tokenMu.Unlock()
		return fmt.Errorf("auth refresh failed: %w", err)
	}
	c.token = newToken
	c.tokenMu.Unlock()

	return nil
}

func (c *Client) Do(method, path string, body any, target any) error {
	startTime := time.Now()

	url := c.baseURL + path

	c.logger.DebugContext(c.ctx, "preparing request",
		slog.String("method", method),
		slog.String("url", url),
		slog.Bool("hasBody", body != nil),
	)

	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequestWithContext(c.ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("request creation failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.logger.InfoContext(c.ctx, "api returned error status",
			append(reqAttrs(req), slog.Int("status", resp.StatusCode))...,
		)

		// Log the body at debug level only: error bodies routinely carry tokens
		// or personal data, which should not end up in default-level logs.
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			c.logger.DebugContext(c.ctx, "error response body", slog.String("body", string(bodyBytes)))
			// Restore the body so it can be read again if needed
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		} else {
			c.logger.WarnContext(c.ctx, "failed to read response body", slog.Any("error", err))
		}

		return fmt.Errorf("api error [%d]: %s", resp.StatusCode, resp.Status)
	}

	if target != nil {
		if err := c.decoder(resp.Body, target); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}

		if err := c.validateTarget(target); err != nil {
			c.logger.WarnContext(c.ctx, "response validation failed", append(reqAttrs(req),
				slog.Any("error", err),
			)...)
			return fmt.Errorf("response validation failed: %w", err)
		}
	}

	c.logger.InfoContext(c.ctx, "request completed", append(reqAttrs(req),
		slog.Int("status", respStatus(resp)),
		slog.Duration("duration", time.Since(startTime)),
	)...)

	return nil
}

func (c *Client) Get(path string, target any) error {
	return c.Do("GET", path, nil, target)
}

func (c *Client) Post(path string, body any, target any) error {
	return c.Do("POST", path, body, target)
}
