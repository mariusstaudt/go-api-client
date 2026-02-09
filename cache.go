package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func (t *cacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only cache GET
	if req.Method != http.MethodGet {
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Debug("skipping cache for non-GET request")
		return t.Next().RoundTrip(req)
	}

	key := cacheKey(req)
	now := time.Now()

	// Fast path: cache hit
	t.mu.RLock()
	if e, ok := t.cache[key]; ok && now.Before(e.exp) {
		t.mu.RUnlock()

		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Debug("cache hit")

		return responseFromEntry(req, e), nil
	}
	t.mu.RUnlock()

	// Dedupe concurrent identical requests
	logrus.WithFields(logrus.Fields{
		"method": req.Method,
		"url":    req.URL.String(),
	}).Debug("cache miss, fetching from upstream")

	v, err, _ := t.group.Do(key, func() (any, error) {
		// Re-check inside singleflight
		now2 := time.Now()
		t.mu.RLock()
		if e, ok := t.cache[key]; ok && now2.Before(e.exp) {
			t.mu.RUnlock()
			logrus.WithFields(logrus.Fields{
				"method": req.Method,
				"url":    req.URL.String(),
			}).Debug("cache hit inside singleflight")
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

			logrus.WithFields(logrus.Fields{
				"method":   req.Method,
				"url":      req.URL.String(),
				"ttl":      t.TTL.String(),
				"bodySize": len(body),
			}).Debug("response cached")
		} else {
			logrus.WithFields(logrus.Fields{
				"method": req.Method,
				"url":    req.URL.String(),
				"status": resp.StatusCode,
			}).Debug("response not cached due to status code")
		}

		return cRes, nil
	})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
			"error":  err.Error(),
		}).Error("cache transport request failed")
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
