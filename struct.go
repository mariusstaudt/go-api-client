package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"golang.org/x/sync/singleflight"
)

type ContextKey struct{}

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

type TokenProvider func(ctx context.Context) (string, error)

var (
	StaticTokenProvider = func(static string) TokenProvider {
		return func(_ context.Context) (string, error) {
			return static, nil
		}
	}
)

type AuthStrategy func(req *http.Request, token string)

var (
	BearerStrategy AuthStrategy = func(r *http.Request, t string) { r.Header.Set("Authorization", "Bearer "+t) }
	BasicStrategy  AuthStrategy = func(r *http.Request, t string) { r.Header.Set("Authorization", "Basic "+t) }
	GitLabStrategy AuthStrategy = func(r *http.Request, t string) { r.Header.Set("PRIVATE-TOKEN", t) }
	NoAuthStrategy AuthStrategy = func(r *http.Request, t string) {}
)

type DecodeStrategy func(data io.ReadCloser, v interface{}) error

var (
	JSONDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		return json.NewDecoder(data).Decode(v)
	}

	YamlDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		return yaml.NewDecoder(data).Decode(v)
	}

	ByteDecodeStrategy DecodeStrategy = func(data io.ReadCloser, v interface{}) error {
		bytes, err := io.ReadAll(data)
		if err != nil {
			return err
		}
		ptr, ok := v.(*[]byte)
		if !ok {
			return nil
		}
		*ptr = bytes
		return nil
	}
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
