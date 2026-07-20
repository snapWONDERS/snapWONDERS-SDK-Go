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

// SnapwondersError is implemented by every error this SDK returns, so callers can match "any SDK
// error" — for example with errors.As(err, &target) against one of the concrete types below.
type SnapwondersError interface {
	error
	snapwonders()
}

// AuthError is returned for a missing, malformed, unknown, or revoked API key (HTTP 401/403).
type AuthError struct{ Message string }

func (e *AuthError) Error() string { return e.Message }
func (*AuthError) snapwonders()    {}

// SessionExpiredError is returned when the 24-hour upload session window has passed (HTTP 410).
type SessionExpiredError struct{ Message string }

func (e *SessionExpiredError) Error() string { return e.Message }
func (*SessionExpiredError) snapwonders()    {}

// ProRequiredError is returned when a Pro-only option was used on a free account (HTTP 402).
type ProRequiredError struct{ Message string }

func (e *ProRequiredError) Error() string { return e.Message }
func (*ProRequiredError) snapwonders()    {}

// MaintenanceError is returned when snapWONDERS is temporarily unavailable for maintenance (HTTP 503
// with `status: MAINTENANCE`). Distinct from a transient 5xx: the service is deliberately
// unavailable and there is nothing wrong with the request. RetryAfter is seconds when supplied.
type MaintenanceError struct {
	Message    string
	RetryAfter float64
}

func (e *MaintenanceError) Error() string { return e.Message }
func (*MaintenanceError) snapwonders()    {}

// RateLimitError is returned when rate limited (HTTP 429). RetryAfter is seconds when supplied.
type RateLimitError struct {
	Message    string
	RetryAfter float64
}

func (e *RateLimitError) Error() string { return e.Message }
func (*RateLimitError) snapwonders()    {}

// JobFailedError is returned when a hide/reveal/analyse/convert job finished as "failed" (or
// "partial" when strict). Reason carries the safe, server-sanitised description.
type JobFailedError struct {
	Message string
	Reason  string
	Status  string
}

func (e *JobFailedError) Error() string { return e.Message }
func (*JobFailedError) snapwonders()    {}

// TusUploadError is returned when a TUS create/PATCH/resume step fails.
type TusUploadError struct{ Message string }

func (e *TusUploadError) Error() string { return e.Message }
func (*TusUploadError) snapwonders()    {}

// NetworkError is returned for a transport-level failure (connection refused, DNS, timeout) after
// retries were exhausted.
type NetworkError struct{ Message string }

func (e *NetworkError) Error() string { return e.Message }
func (*NetworkError) snapwonders()    {}

// APIError is returned for any other non-2xx API response. StatusCode and Body are attached for
// inspection.
type APIError struct {
	Message    string
	StatusCode int
	Body       any
}

func (e *APIError) Error() string { return e.Message }
func (*APIError) snapwonders()    {}
