package api

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	logrus.WithFields(logrus.Fields{
		"method": req.Method,
		"url":    req.URL.String(),
	}).Debug("applying default headers")

	for key, value := range t.defaultHeaders {
		req.Header.Set(key, value)

		if key == "Host" {
			req.Host = value
		}
	}

	return t.next.RoundTrip(req)
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Save body for potential retry (body can only be read once)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	t.client.tokenMu.RLock()
	token := t.client.token
	strategy := t.client.authStrategy
	t.client.tokenMu.RUnlock()

	// 1. set auth header
	if token != "" && strategy != nil {
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Debug("applying auth strategy")
		strategy(req, token)
	} else {
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Debug("skipping auth header (missing token or strategy)")
	}

	// 2. execute request
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 3. if 401 or 404 (GitLab) -> refresh token and retry once
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound) && t.client.tokenProvider != nil {
		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Info("unauthorized, attempting token refresh")
		resp.Body.Close() // close old response body

		err := t.client.refreshToken(req)
		if err != nil {
			return nil, err
		}

		logrus.WithFields(logrus.Fields{
			"method": req.Method,
			"url":    req.URL.String(),
		}).Info("retrying request with refreshed token")

		// clone request and resend with new token
		newReq := req.Clone(req.Context())
		// Restore body for retry
		if bodyBytes != nil {
			newReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			newReq.ContentLength = int64(len(bodyBytes))
		}
		strategy(newReq, t.client.token)
		return t.next.RoundTrip(newReq)
	}

	return resp, nil
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= t.maxRetries; i++ {
		logrus.WithFields(logrus.Fields{
			"attempt": i + 1,
			"method":  req.Method,
			"url":     req.URL.String(),
		}).Debug("issuing request")

		resp, err = t.next.RoundTrip(req)

		// success or client error -> no retry
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if i < t.maxRetries {
			logrus.WithFields(logrus.Fields{
				"attempt": i + 1,
				"method":  req.Method,
				"url":     req.URL.String(),
				"err":     err,
				"status":  respStatus(resp),
			}).Info("retrying after failure")
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return resp, err
}
