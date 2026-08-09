package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func (t *cacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only cache GET
	if req.Method != http.MethodGet {
		t.logger.DebugContext(req.Context(), "skipping cache for non-GET request", reqAttrs(req)...)
		return t.Next().RoundTrip(req)
	}

	key := cacheKey(req)
	now := time.Now()

	// Fast path: cache hit
	t.mu.RLock()
	if e, ok := t.cache[key]; ok && now.Before(e.exp) {
		t.mu.RUnlock()

		t.logger.DebugContext(req.Context(), "cache hit", reqAttrs(req)...)

		return responseFromEntry(req, e), nil
	}
	t.mu.RUnlock()

	// Dedupe concurrent identical requests
	t.logger.DebugContext(req.Context(), "cache miss, fetching from upstream", reqAttrs(req)...)

	v, err, _ := t.group.Do(key, func() (any, error) {
		// Re-check inside singleflight
		now2 := time.Now()
		t.mu.RLock()
		if e, ok := t.cache[key]; ok && now2.Before(e.exp) {
			t.mu.RUnlock()
			t.logger.DebugContext(req.Context(), "cache hit inside singleflight", reqAttrs(req)...)
			return e, nil
		}
		t.mu.RUnlock()

		resp, err := t.Next().RoundTrip(req)
		if err != nil {
			return cachedResponse{}, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return cachedResponse{}, err
		}

		cRes := cachedResponse{
			status: resp.StatusCode,
			header: cloneHeader(resp.Header),
			body:   body,
			exp:    time.Now().Add(t.TTL),
		}

		// Cache policy: cache only 200 OK by default.
		// (Optional: cache 404 for short time, etc.)
		if resp.StatusCode == http.StatusOK {
			t.mu.Lock()
			if t.cache == nil {
				t.cache = make(map[string]cachedResponse)
			}
			t.cache[key] = cRes
			t.mu.Unlock()

			t.logger.DebugContext(req.Context(), "response cached", append(reqAttrs(req),
				slog.Duration("ttl", t.TTL),
				slog.Int("bodySize", len(body)),
			)...)
		} else {
			t.logger.DebugContext(req.Context(), "response not cached due to status code",
				append(reqAttrs(req), slog.Int("status", resp.StatusCode))...,
			)
		}

		return cRes, nil
	})
	if err != nil {
		t.logger.ErrorContext(req.Context(), "cache transport request failed",
			append(reqAttrs(req), slog.Any("error", err))...,
		)
		return nil, err
	}

	return responseFromEntry(req, v.(cachedResponse)), nil
}

func (t *cacheTransport) Next() http.RoundTripper {
	if t.next != nil {
		return t.next
	}
	return http.DefaultTransport
}

func cacheKey(req *http.Request) string {
	// If you ever vary by language/format, include those headers here.
	// Authorization omitted because you always use the same token/user.
	h := sha256.New()
	io.WriteString(h, req.Method)
	io.WriteString(h, " ")
	io.WriteString(h, req.URL.String())
	io.WriteString(h, " accept=")
	io.WriteString(h, req.Header.Get("Accept"))
	return hex.EncodeToString(h.Sum(nil))
}

func cloneHeader(h http.Header) http.Header {
	cp := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		cp[k] = vv2
	}
	return cp
}

func responseFromEntry(req *http.Request, cRes cachedResponse) *http.Response {
	// Important: return a fresh Body reader each time
	return &http.Response{
		StatusCode: cRes.status,
		Header:     cloneHeader(cRes.header),
		Body:       io.NopCloser(bytes.NewReader(cRes.body)),
		Request:    req,
	}
}
