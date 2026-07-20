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
	"net/http"
	"time"
)

// Version is the SDK version.
const Version = "0.1.0"

// Client is the authenticated entry point to the snapWONDERS API. Three product namespaces share the
// session → job → poll → download shape: Stego (hide/reveal), Analyse (forensics), Convert (media).
type Client struct {
	transport *transport

	Stego   *Stego
	Analyse *Analyse
	Convert *Convert
}

type clientConfig struct {
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*clientConfig)

// WithBaseURL overrides the API base URL (default https://snapwonders.com).
func WithBaseURL(u string) Option { return func(c *clientConfig) { c.baseURL = u } }

// WithTimeout sets the connect/handshake/response-header timeout (default 30s). It does not cap total
// transfer time, so large uploads and downloads are not cut off.
func WithTimeout(d time.Duration) Option { return func(c *clientConfig) { c.timeout = d } }

// WithHTTPClient injects a custom *http.Client (useful for tests and custom transports).
func WithHTTPClient(h *http.Client) Option { return func(c *clientConfig) { c.httpClient = h } }

// NewClient builds a client. apiKey is sent as the X-Api-Key header (blank is allowed for the
// key-less endpoints such as Status).
func NewClient(apiKey string, opts ...Option) *Client {
	cfg := clientConfig{baseURL: DefaultBaseURL, timeout: 30 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}
	t := newTransport(apiKey, cfg.baseURL, cfg.timeout, cfg.httpClient)
	return &Client{
		transport: t,
		Stego:     &Stego{t: t},
		Analyse:   &Analyse{t: t},
		Convert:   &Convert{t: t},
	}
}

// Status calls GET /api/status — no API key required.
func (c *Client) Status() (map[string]any, error) {
	resp, err := c.transport.request("GET", "/api/status", reqOpts{Expected: []int{200, 503}})
	if err != nil {
		return nil, err
	}
	return resp.decodeMap(), nil
}

// --- job options (expiry + arbitrary encoding keys) ---

type jobConfig struct {
	expiry  string
	options map[string]any
}

// JobOption configures a job when starting it (Hide/Reveal/Run/StartJob).
type JobOption func(*jobConfig)

// WithExpiry sets the result retention window (default "1d").
func WithExpiry(e string) JobOption { return func(c *jobConfig) { c.expiry = e } }

// WithOption sets a single job encoding key (e.g. WithOption("image_format", "webp")).
func WithOption(key string, value any) JobOption {
	return func(c *jobConfig) { c.options[key] = value }
}

// WithOptions merges a map of job encoding keys.
func WithOptions(m map[string]any) JobOption {
	return func(c *jobConfig) {
		for k, v := range m {
			c.options[k] = v
		}
	}
}

func resolveJobConfig(opts ...JobOption) jobConfig {
	c := jobConfig{expiry: "1d", options: map[string]any{}}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// --- wait options ---

type waitConfig struct {
	pollInterval time.Duration
	timeout      time.Duration
	strict       bool
}

// WaitOption configures a job's Wait call.
type WaitOption func(*waitConfig)

// WithPollInterval sets the initial poll gap (it backs off from there; default 1.5s).
func WithPollInterval(d time.Duration) WaitOption { return func(c *waitConfig) { c.pollInterval = d } }

// WithJobTimeout sets how long to wait for the job to finish (default 15m).
func WithJobTimeout(d time.Duration) WaitOption { return func(c *waitConfig) { c.timeout = d } }

// Strict treats a "partial" result as a failure.
func Strict() WaitOption { return func(c *waitConfig) { c.strict = true } }

func resolveWaitConfig(opts ...WaitOption) waitConfig {
	c := waitConfig{pollInterval: 1500 * time.Millisecond, timeout: 900 * time.Second}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func resultFilesFrom(v any, t *transport, prefix string) []*ResultFile {
	items, _ := v.([]any)
	out := make([]*ResultFile, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok && m["asset_id"] != nil {
			out = append(out, resultFileFromJSON(m, t, prefix))
		}
	}
	return out
}
