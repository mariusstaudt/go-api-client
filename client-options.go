package api

import (
	"context"
	"net/http"
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
