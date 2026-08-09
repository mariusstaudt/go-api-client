package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/sync/singleflight"
)

type Client struct {
	ctx            context.Context
	name           string
	httpClient     *http.Client
	baseURL        string
	token          string
	tokenMu        sync.RWMutex
	tokenProvider  TokenProvider
	authStrategy   AuthStrategy
	defaultHeaders map[string]string
	transport      *http.Transport
	decoder        DecodeStrategy
	cacheTTL       time.Duration
	validator      *validator.Validate
	logger         *slog.Logger
}

type cachedResponse struct {
	status int
	header http.Header
	body   []byte
	exp    time.Time
}

type cacheTransport struct {
	next   http.RoundTripper
	logger *slog.Logger

	mu    sync.RWMutex
	group singleflight.Group

	TTL   time.Duration
	cache map[string]cachedResponse
}

type headerTransport struct {
	next           http.RoundTripper
	logger         *slog.Logger
	defaultHeaders map[string]string
}

type authTransport struct {
	client *Client
	next   http.RoundTripper
	logger *slog.Logger
}

type retryTransport struct {
	maxRetries int
	next       http.RoundTripper
	logger     *slog.Logger
}

func respStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

// reqAttrs returns the attributes shared by every request-scoped log record.
func reqAttrs(req *http.Request) []any {
	return []any{
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
	}
}
