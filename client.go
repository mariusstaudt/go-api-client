package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
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

	var internalTransport http.RoundTripper = c.transport
	if c.transport == nil {
		internalTransport = http.DefaultTransport
	}

	internalTransport = &retryTransport{
		next:       internalTransport,
		maxRetries: 2,
	}

	internalTransport = &headerTransport{
		next:           internalTransport,
		defaultHeaders: c.defaultHeaders,
	}

	internalTransport = &authTransport{
		client: c,
		next:   internalTransport,
	}

	internalTransport = &cacheTransport{
		next:  internalTransport,
		TTL:   c.cacheTTL,
		cache: make(map[string]cachedResponse),
	}

	c.httpClient = &http.Client{
		Transport: internalTransport,
		Timeout:   5 * time.Minute,
	}

	if c.decoder == nil {
		c.decoder = JSONDecodeStrategy
	}

	logrus.WithFields(logrus.Fields{
		"name":             c.name,
		"baseURL":          c.baseURL,
		"hasTokenProvider": c.tokenProvider != nil,
		"hasAuthStrategy":  c.authStrategy != nil,
	}).Infof("api client initialized")

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

	logrus.WithFields(logrus.Fields{
		"method":  method,
		"url":     url,
		"hasBody": body != nil,
	}).Debug("preparing request")

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
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
			"status": resp.StatusCode,
		}).Info("api returned error status")

		// log out the body of the response
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			logrus.WithField("body", string(bodyBytes)).Info("response body")
			// Restore the body so it can be read again if needed
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		} else {
			logrus.WithError(err).Warn("failed to read response body")
		}

		return fmt.Errorf("api error [%d]: %s", resp.StatusCode, resp.Status)
	}

	if target != nil {
		if err := c.decoder(resp.Body, target); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}
	}

	logrus.WithFields(logrus.Fields{
		"method":   req.Method,
		"url":      req.URL.String(),
		"err":      err,
		"status":   respStatus(resp),
		"duration": time.Since(startTime),
	}).Info("request completed")

	return nil
}

func (c *Client) Get(path string, target any) error {
	return c.Do("GET", path, nil, target)
}

func (c *Client) Post(path string, body any, target any) error {
	return c.Do("POST", path, body, target)
}
