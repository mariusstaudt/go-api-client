# go-api-client

A flexible, configurable HTTP API client for Go with a built-in middleware chain for caching, authentication, retry logic, and more.

## Features

- **Functional Options Pattern** – Clean, extensible configuration
- **Middleware Chain** – Modular HTTP transport layers
- **Caching** – Response caching with singleflight deduplication
- **Authentication** – Multiple auth strategies with automatic token refresh
- **Retry Logic** – Automatic retries on server errors
- **Flexible Decoding** – JSON, YAML, or raw bytes

## Installation

```bash
go get github.com/mariusstaudt/go-api-client
```

## Quick Start

```go
package main

import (
    "fmt"
    api "github.com/mariusstaudt/go-api-client"
)

func main() {
    client := api.NewClient("my-api",
        api.WithBaseURL("https://api.example.com"),
        api.WithTokenProvider(api.StaticTokenProvider("my-token")),
        api.WithAuthStrategy(api.BearerStrategy),
    )

    var result map[string]any
    err := client.Get("/users/1", &result)
    if err != nil {
        panic(err)
    }
    fmt.Println(result)
}
```

## Configuration

### Client Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Sets the base URL for all requests |
| `WithTokenProvider(p)` | Sets the token provider for dynamic tokens |
| `WithAuthStrategy(s)` | Defines how the token is applied to requests |
| `WithDefaultHeaders(h)` | Adds default headers to all requests |
| `WithTransport(t)` | Overrides the default HTTP transport |
| `WithContext(ctx)` | Sets the default context for requests |
| `WithDecodeStrategy(d)` | Defines how responses are decoded |

### Authentication

#### Auth Strategies

```go
// Bearer Token (Authorization: Bearer <token>)
api.BearerStrategy

// Basic Auth (Authorization: Basic <token>)
api.BasicStrategy

// GitLab Private Token (PRIVATE-TOKEN: <token>)
api.GitLabStrategy

// No authentication
api.NoAuthStrategy
```

#### Token Provider

The token provider enables dynamic token management:

```go
// Static token
api.StaticTokenProvider("my-static-token")

// Dynamic token (e.g. OAuth)
func myTokenProvider(ctx context.Context) (string, error) {
    // Fetch token from an OAuth server, load from Vault, etc.
    return fetchTokenFromSomewhere()
}

client := api.NewClient("my-api",
    api.WithTokenProvider(myTokenProvider),
    api.WithAuthStrategy(api.BearerStrategy),
)
```

**Automatic Token Refresh:** On `401 Unauthorized` or `404 Not Found` (GitLab-specific behavior), the token provider is automatically called and the request is retried.

### Decode Strategies

```go
// JSON (default)
api.JSONDecodeStrategy

// YAML
api.YamlDecodeStrategy

// Raw Bytes
api.ByteDecodeStrategy
```

**Example with Bytes:**

```go
client := api.NewClient("file-api",
    api.WithBaseURL("https://files.example.com"),
    api.WithDecodeStrategy(api.ByteDecodeStrategy),
)

var data []byte
client.Get("/document.pdf", &data)
```

## Middleware Chain

The client uses a chained transport architecture:

```
Request → Cache → Auth → Headers → Retry → HTTP Transport → Server
```

### 1. Cache Transport

- Only caches `GET` requests
- Default TTL: 1 minute
- Only caches `200 OK` responses
- Uses singleflight to deduplicate concurrent identical requests
- Cache key is based on: Method + URL + Accept header

### 2. Auth Transport

- Sets the auth header based on the chosen strategy
- Automatic token refresh on 401/404
- Request body is preserved for retries

### 3. Header Transport

- Applies configured default headers
- Supports Host header override

### 4. Retry Transport

- Up to 2 retries on server errors (5xx)
- Exponential backoff (500ms, 1000ms, …)
- No retries on client errors (4xx)

## API

### Methods

```go
// Generic request method
func (c *Client) Do(method, path string, body any, target any) error

// GET request
func (c *Client) Get(path string, target any) error

// POST request
func (c *Client) Post(path string, body any, target any) error
```

### Examples

```go
// GET with struct
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var user User
client.Get("/users/123", &user)

// POST with body
newUser := User{Name: "Jane Doe"}
var created User
client.Post("/users", newUser, &created)

// GET without response body
client.Get("/health", nil)
```

## Custom Headers

```go
client := api.NewClient("my-api",
    api.WithBaseURL("https://api.example.com"),
    api.WithDefaultHeaders(map[string]string{
        "X-Custom-Header": "value",
        "Accept":          "application/json",
        "Host":            "custom-host.example.com", // Host header receives special handling
    }),
)
```

## Logging

The client uses [logrus](https://github.com/sirupsen/logrus) for structured logging:

- **Debug:** Request preparation, cache hits/misses, header application
- **Info:** Token refresh, retries, request completion
- **Error:** Failed requests

```go
import "github.com/sirupsen/logrus"

// Enable debug logging
logrus.SetLevel(logrus.DebugLevel)
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    httpClient                        │   │
│  │  ┌─────────────────────────────────────────────┐    │   │
│  │  │              Transport Chain                 │    │   │
│  │  │                                              │    │   │
│  │  │  cacheTransport                              │    │   │
│  │  │       ↓                                      │    │   │
│  │  │  authTransport                               │    │   │
│  │  │       ↓                                      │    │   │
│  │  │  headerTransport                             │    │   │
│  │  │       ↓                                      │    │   │
│  │  │  retryTransport                              │    │   │
│  │  │       ↓                                      │    │   │
│  │  │  http.DefaultTransport / custom Transport    │    │   │
│  │  └─────────────────────────────────────────────┘    │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Dependencies

- [github.com/goccy/go-yaml](https://github.com/goccy/go-yaml) – YAML decoding
- [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus) – Structured logging
- [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) – Singleflight for cache deduplication
