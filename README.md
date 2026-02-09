# go-api-client

Ein flexibler, konfigurierbarer HTTP-API-Client für Go mit eingebauter Middleware-Chain für Caching, Authentifizierung, Retry-Logik und mehr.

## Features

- **Functional Options Pattern** - Saubere, erweiterbare Konfiguration
- **Middleware-Chain** - Modulare HTTP-Transport-Layer
- **Caching** - Intelligentes Response-Caching mit Singleflight-Deduplizierung
- **Authentifizierung** - Mehrere Auth-Strategien mit automatischem Token-Refresh
- **Retry-Logik** - Automatische Wiederholungen bei Server-Fehlern
- **Flexible Dekodierung** - JSON, YAML oder Raw-Bytes

## Installation

```bash
go get gitlab.devops.telekom.de/marius.staudt/go-api-client
```

## Schnellstart

```go
package main

import (
    "fmt"
    api "gitlab.devops.telekom.de/marius.staudt/go-api-client"
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

## Konfiguration

### Client Options

| Option | Beschreibung |
|--------|-------------|
| `WithBaseURL(url)` | Setzt die Basis-URL für alle Requests |
| `WithTokenProvider(p)` | Setzt den Token-Provider für dynamische Tokens |
| `WithAuthStrategy(s)` | Definiert wie der Token im Request gesetzt wird |
| `WithDefaultHeaders(h)` | Fügt Standard-Header zu allen Requests hinzu |
| `WithTransport(t)` | Überschreibt den Standard-HTTP-Transport |
| `WithContext(ctx)` | Setzt den Standard-Context für Requests |
| `WithDecodeStrategy(d)` | Definiert wie Responses dekodiert werden |

### Authentifizierung

#### Auth-Strategien

```go
// Bearer Token (Authorization: Bearer <token>)
api.BearerStrategy

// Basic Auth (Authorization: Basic <token>)
api.BasicStrategy

// GitLab Private Token (PRIVATE-TOKEN: <token>)
api.GitLabStrategy

// Keine Authentifizierung
api.NoAuthStrategy
```

#### Token Provider

Der Token Provider ermöglicht dynamisches Token-Management:

```go
// Statischer Token
api.StaticTokenProvider("my-static-token")

// Dynamischer Token (z.B. OAuth)
func myTokenProvider(ctx context.Context) (string, error) {
    // Token von OAuth-Server holen, aus Vault laden, etc.
    return fetchTokenFromSomewhere()
}

client := api.NewClient("my-api",
    api.WithTokenProvider(myTokenProvider),
    api.WithAuthStrategy(api.BearerStrategy),
)
```

**Automatischer Token-Refresh:** Bei `401 Unauthorized` oder `404 Not Found` (GitLab-Spezifikum) wird automatisch der Token-Provider aufgerufen und der Request wiederholt.

### Decode-Strategien

```go
// JSON (Standard)
api.JSONDecodeStrategy

// YAML
api.YamlDecodeStrategy

// Raw Bytes
api.ByteDecodeStrategy
```

**Beispiel mit Bytes:**

```go
client := api.NewClient("file-api",
    api.WithBaseURL("https://files.example.com"),
    api.WithDecodeStrategy(api.ByteDecodeStrategy),
)

var data []byte
client.Get("/document.pdf", &data)
```

## Middleware-Chain

Der Client verwendet eine verkettete Transport-Architektur:

```
Request → Cache → Auth → Headers → Retry → HTTP Transport → Server
```

### 1. Cache Transport

- Cached nur `GET`-Requests
- Standard-TTL: 1 Minute
- Cached nur `200 OK` Responses
- Verwendet Singleflight zur Deduplizierung paralleler identischer Requests
- Cache-Key basiert auf: Method + URL + Accept-Header

### 2. Auth Transport

- Setzt Auth-Header basierend auf der gewählten Strategie
- Automatischer Token-Refresh bei 401/404
- Request-Body wird für Retry gespeichert

### 3. Header Transport

- Fügt konfigurierte Default-Headers hinzu
- Unterstützt Host-Header-Überschreibung

### 4. Retry Transport

- Max. 2 Wiederholungen bei Server-Fehlern (5xx)
- Exponentielles Backoff (500ms, 1000ms, ...)
- Kein Retry bei Client-Fehlern (4xx)

## API

### Methoden

```go
// Generische Request-Methode
func (c *Client) Do(method, path string, body any, target any) error

// GET Request
func (c *Client) Get(path string, target any) error

// POST Request
func (c *Client) Post(path string, body any, target any) error
```

### Beispiele

```go
// GET mit Struct
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var user User
client.Get("/users/123", &user)

// POST mit Body
newUser := User{Name: "Max Mustermann"}
var created User
client.Post("/users", newUser, &created)

// GET ohne Response-Body
client.Get("/health", nil)
```

## Custom Headers

```go
client := api.NewClient("my-api",
    api.WithBaseURL("https://api.example.com"),
    api.WithDefaultHeaders(map[string]string{
        "X-Custom-Header": "value",
        "Accept":          "application/json",
        "Host":            "custom-host.example.com", // Host-Header wird speziell behandelt
    }),
)
```

## Logging

Der Client verwendet [logrus](https://github.com/sirupsen/logrus) für strukturiertes Logging:

- **Debug:** Request-Vorbereitung, Cache-Hits/Misses, Header-Anwendung
- **Info:** Token-Refresh, Retries, Request-Completion
- **Error:** Fehlgeschlagene Requests

```go
import "github.com/sirupsen/logrus"

// Debug-Logging aktivieren
logrus.SetLevel(logrus.DebugLevel)
```

## Architektur

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

## Abhängigkeiten

- [github.com/goccy/go-yaml](https://github.com/goccy/go-yaml) - YAML-Dekodierung
- [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus) - Strukturiertes Logging
- [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) - Singleflight für Cache-Deduplizierung
