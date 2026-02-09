package api

import (
	"context"
	"net/http"
	"sync"
	"time"

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
}

type cachedResponse struct {
	status int
	header http.Header
	body   []byte
	exp    time.Time
}

type cacheTransport struct {
	next http.RoundTripper

	mu    sync.RWMutex
	group singleflight.Group

	TTL   time.Duration
	cache map[string]cachedResponse
}

type headerTransport struct {
	next           http.RoundTripper
	defaultHeaders map[string]string
}

type authTransport struct {
	client *Client
	next   http.RoundTripper
}

type retryTransport struct {
	maxRetries int
	next       http.RoundTripper
}

func respStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
