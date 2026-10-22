// snapWONDERS API — Go SDK
// API version: 1.0
//
// Copyright (c) 2026 Kenneth Springer @ snapWONDERS. MIT Licensed — see LICENSE.
// The MIT licence covers this client library only; the snapWONDERS API it calls is proprietary.
//
// Author: Kenneth Springer @ snapWONDERS <kenneth@snapwonders.com> (https://kennethbspringer.au)
//
// All the snapWONDERS API services are available over the Clearnet / Web and Dark Web Tor and I2P.
// Read details: https://snapwonders.com/developers

package snapwonders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the canonical snapWONDERS API host.
const DefaultBaseURL = "https://snapwonders.com"

const (
	maxRetries        = 2 // retries after the first try — 3 attempts total
	backoffCapSeconds = 30.0
)

var retryStatuses = map[int]bool{500: true, 502: true, 503: true, 504: true}

// transport is the single place the SDK talks to the network. Deliberately thin: no endpoint
// knowledge lives here. Non-2xx responses map to typed errors in raiseForResponse.
type transport struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func newTransport(apiKey, baseURL string, timeout time.Duration, client *http.Client) *transport {
	if client == nil {
		// No total Timeout on the client — large uploads/downloads must not be cut off. Bound the
		// connect/handshake/response-header phases instead so a hung dial still fails.
		client = &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: timeout}).DialContext,
				TLSHandshakeTimeout:   timeout,
				ResponseHeaderTimeout: timeout,
			},
		}
	}
	return &transport{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// apiResponse is a thin view over one HTTP response.
type apiResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// decodeMap parses the body as a JSON object; returns an empty map on empty/non-object bodies.
func (r *apiResponse) decodeMap() map[string]any {
	var m map[string]any
	if len(r.Body) == 0 || json.Unmarshal(r.Body, &m) != nil {
		return map[string]any{}
	}
	return m
}

// reqOpts holds the optional parts of a request (Go has no keyword arguments).
type reqOpts struct {
	JSON     any
	Headers  map[string]string
	Content  []byte
	Expected []int // defaults to {200, 201} when nil
}

// request sends a request, retrying transient 5xx (not maintenance) with capped exponential backoff.
// path is relative to the base URL (e.g. "/api/status"). Returns a typed error on any non-expected
// status, and a *NetworkError when the host cannot be reached.
func (t *transport) request(method, path string, o reqOpts) (*apiResponse, error) {
	expected := o.Expected
	if expected == nil {
		expected = []int{200, 201}
	}

	headers := map[string]string{}
	for k, v := range o.Headers {
		headers[k] = v
	}
	var body []byte
	switch {
	case o.JSON != nil:
		b, err := json.Marshal(o.JSON)
		if err != nil {
			return nil, err
		}
		body = b
		headers["Content-Type"] = "application/json"
	case o.Content != nil:
		body = o.Content
	}
	if t.apiKey != "" {
		headers["X-Api-Key"] = t.apiKey
	}

	attempt := 0
	for {
		attempt++
		resp, netErr := t.send(method, t.baseURL+path, headers, body)
		if netErr != nil {
			// Connection refused / DNS / timeout — retry, then surface as a typed error (never let a
			// raw transport failure escape the SDK's own error set).
			if attempt <= maxRetries {
				sleepBackoff(attempt)
				continue
			}
			return nil, &NetworkError{Message: fmt.Sprintf("Network error contacting %s: %v", t.baseURL, netErr)}
		}
		// A maintenance 503 is deliberate, not transient — the server asks for ~300s. Burning the
		// retry budget over a few seconds cannot help, so surface it immediately.
		if retryStatuses[resp.StatusCode] && attempt <= maxRetries && !isMaintenance(resp) {
			sleepBackoff(attempt)
			continue
		}
		if !containsInt(expected, resp.StatusCode) {
			return nil, raiseForResponse(resp)
		}
		return resp, nil
	}
}

func (t *transport) send(method, url string, headers map[string]string, body []byte) (*apiResponse, error) {
	var reader io.Reader
	if body != nil && method != http.MethodHead {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return &apiResponse{StatusCode: res.StatusCode, Header: res.Header, Body: data}, nil
}

func sleepBackoff(attempt int) {
	seconds := math.Min(backoffCapSeconds, math.Pow(2, float64(attempt-1)))
	time.Sleep(time.Duration(seconds * float64(time.Second)))
}

func isMaintenance(resp *apiResponse) bool {
	if resp.StatusCode != 503 {
		return false
	}
	var m map[string]any
	if json.Unmarshal(resp.Body, &m) != nil {
		return false
	}
	s, _ := m["status"].(string)
	return s == "MAINTENANCE"
}

// raiseForResponse maps a non-2xx response to the correct typed error.
func raiseForResponse(resp *apiResponse) error {
	code := resp.StatusCode

	var body any
	if json.Unmarshal(resp.Body, &body) != nil {
		body = string(resp.Body)
	}
	message := extractMessage(body)
	if message == "" {
		message = fmt.Sprintf("HTTP %d", code)
	}

	if isMaintenance(resp) {
		retryAfter := retryAfterSeconds(resp)
		wait := " Try again shortly."
		if retryAfter > 0 {
			wait = fmt.Sprintf(" Retry after %gs.", retryAfter)
		}
		return &MaintenanceError{
			Message: "snapWONDERS is temporarily unavailable for maintenance." + wait +
				" Your request was not processed and nothing is wrong with it.",
			RetryAfter: retryAfter,
		}
	}

	switch {
	case code == 401 || code == 403:
		return &AuthError{Message: message}
	case code == 402:
		return &ProRequiredError{Message: message}
	case code == 410:
		return &SessionExpiredError{Message: message}
	case code == 429:
		return &RateLimitError{Message: message, RetryAfter: retryAfterSeconds(resp)}
	default:
		return &APIError{Message: message, StatusCode: code, Body: body}
	}
}

func extractMessage(body any) string {
	if m, ok := body.(map[string]any); ok {
		if s, ok := m["message"].(string); ok && s != "" {
			return s
		}
		if s, ok := m["error"].(string); ok && s != "" {
			return s
		}
		return ""
	}
	if s, ok := body.(string); ok {
		return s
	}
	return ""
}

func retryAfterSeconds(resp *apiResponse) float64 {
	if resp.Header == nil {
		return 0
	}
	if v := resp.Header.Get("Retry-After"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
