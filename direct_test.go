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

// Which upload method runs, and why. Offline — the round-tripper is stubbed, nothing leaves the
// machine.
//
// The choice is invisible to callers, which is the point of the router but also means a wrong
// choice fails somewhere far from its cause: a large file sent direct is rejected by the server,
// and a small file sent over TUS just quietly costs three round trips instead of one. These pin
// the boundary so neither can regress unnoticed.
package snapwonders

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// capturedRequest is one request the router made, without one being performed.
type capturedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Length  int64
}

func capturingTransport(calls *[]capturedRequest, body map[string]any) *transport {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*calls = append(*calls, capturedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Length:  r.ContentLength,
		})
		return jsonResponse(200, body), nil
	})}
	return newTransport("sw_x", DefaultBaseURL, 30*time.Second, client)
}

func tempFileOfSize(t *testing.T, size int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(p, make([]byte, size), 0o600); err != nil {
		t.Fatalf("cannot write temp file: %v", err)
	}
	return p
}

func okBody() map[string]any {
	return map[string]any{"file": map[string]any{"storage_uid": "stg_abc123"}}
}

func TestSmallFileGoesDirectInOneRequest(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	got, err := tr.routeUploadFile(tempFileOfSize(t, 1024), "upl_1", 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "stg_abc123" {
		t.Errorf("storage_uid = %q, want stg_abc123", got)
	}
	if len(calls) != 1 {
		t.Fatalf("direct upload is a single request, got %d", len(calls))
	}
	if calls[0].Method != "POST" || calls[0].Path != "/api/upload" {
		t.Errorf("got %s %s, want POST /api/upload", calls[0].Method, calls[0].Path)
	}
}

// The server refuses chunked uploads with 411, because without a declared length a truncated body
// cannot be told apart from a complete one. Content-Length must therefore always be set.
func TestDirectUploadDeclaresContentLength(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	if _, err := tr.routeUploadFile(tempFileOfSize(t, 4096), "upl_1", 1, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls[0].Length != 4096 {
		t.Errorf("ContentLength = %d, want 4096 — a chunked body would be rejected with 411", calls[0].Length)
	}
}

func TestServerCapWinsOverTheBuiltInDefault(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	// 4 KB file, but the server says the cap is 1 KB — must not go direct.
	_, _ = tr.routeUploadFile(tempFileOfSize(t, 4096), "upl_1", 1, 1024)

	for _, c := range calls {
		if c.Path == "/api/upload" {
			t.Fatal("over the reported cap, direct upload must not be attempted")
		}
	}
}

func TestExactlyAtTheCapStillGoesDirect(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	if _, err := tr.routeUploadFile(tempFileOfSize(t, 2048), "upl_1", 1, 2048); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls[0].Path != "/api/upload" {
		t.Errorf("the boundary is inclusive; got %s", calls[0].Path)
	}
}

func TestDirectUploadSendsRequiredHeaders(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	if _, err := tr.routeUploadFile(tempFileOfSize(t, 16), "upl_xyz", 2, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := calls[0].Headers

	// X-Upload-Uid is required — without it the endpoint answers 404 unknown_session.
	if h.Get("X-Upload-Uid") != "upl_xyz" {
		t.Errorf("X-Upload-Uid = %q", h.Get("X-Upload-Uid"))
	}
	if h.Get("X-Upload-Step") != "2" {
		t.Errorf("X-Upload-Step = %q", h.Get("X-Upload-Step"))
	}
	if h.Get("X-Filename") != "sample.bin" {
		t.Errorf("X-Filename = %q", h.Get("X-Filename"))
	}
	// Idempotency: a retry after a lost response must not store the file twice.
	if h.Get("X-Client-Upload-Id") == "" {
		t.Error("X-Client-Upload-Id must be sent")
	}
}

func TestResponseWithoutStorageUIDIsAnError(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, map[string]any{})

	_, err := tr.routeUploadFile(tempFileOfSize(t, 16), "upl_1", 1, 0)
	var ue *UploadError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UploadError, got %#v", err)
	}
}

func TestMissingFileFailsBeforeAnyRequest(t *testing.T) {
	var calls []capturedRequest
	tr := capturingTransport(&calls, okBody())

	_, err := tr.routeUploadFile(filepath.Join(t.TempDir(), "nope.bin"), "upl_1", 1, 0)
	var ue *UploadError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UploadError, got %#v", err)
	}
	if len(calls) != 0 {
		t.Errorf("no request should have been made, got %d", len(calls))
	}
}

// Both paths are chosen for the caller by file size, so matching on one concrete type would mean
// matching one that only occurs for some file sizes.
func TestBothUploadErrorsSatisfyUploadFailure(t *testing.T) {
	var uf UploadFailure
	if !errors.As(error(&UploadError{Message: "x"}), &uf) {
		t.Error("UploadError must satisfy UploadFailure")
	}
	if !errors.As(error(&TusUploadError{Message: "x"}), &uf) {
		t.Error("TusUploadError must satisfy UploadFailure")
	}
}

func TestDefaultMaxUploadBytesMatchesServerFallback(t *testing.T) {
	if DefaultMaxUploadBytes != 99614720 {
		t.Errorf("DefaultMaxUploadBytes = %d, want 99614720", DefaultMaxUploadBytes)
	}
}
