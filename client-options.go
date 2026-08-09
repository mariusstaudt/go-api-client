package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
)

type ClientOption func(*Client)

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithTokenProvider(p TokenProvider) ClientOption {
	return func(c *Client) { c.tokenProvider = p }
}

func WithAuthStrategy(s AuthStrategy) ClientOption {
	return func(c *Client) { c.authStrategy = s }
}

func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) { c.defaultHeaders = headers }
}

func WithTransport(transport *http.Transport) ClientOption {
	return func(c *Client) { c.transport = transport }
}

func WithContext(ctx context.Context) ClientOption {
	return func(c *Client) { c.ctx = ctx }
}

func WithDecodeStrategy(decoder DecodeStrategy) ClientOption {
	return func(c *Client) { c.decoder = decoder }
}

func WithCacheTTL(ttl time.Duration) ClientOption {
	return func(c *Client) { c.cacheTTL = ttl }
}

func WithValidator(v *validator.Validate) ClientOption {
	return func(c *Client) { c.validator = v }
}

// WithLogger sets the logger used by the client and its transport chain.
// Defaults to slog.Default(). Pass slog.New(slog.DiscardHandler) to silence
// the client entirely.
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) { c.logger = l }
}
