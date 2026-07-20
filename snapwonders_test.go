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

// Offline tests — the pure logic that needs no live API or key. Integration tests that hit the live
// API are run separately with a real key.
package snapwonders

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// roundTripFunc lets a function act as an http.RoundTripper so tests can serve canned responses.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}
}

// recordingTransport records request paths and returns canned status/result JSON — no network.
func recordingTransport(paths *[]string, statusBody, resultBody map[string]any) *transport {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*paths = append(*paths, r.URL.Path)
		body := statusBody
		if strings.HasSuffix(r.URL.Path, "/results") || strings.Contains(r.URL.Path, "/result/") {
			body = resultBody
		}
		return jsonResponse(200, body), nil
	})}
	return newTransport("sw_x", DefaultBaseURL, 30*time.Second, client)
}

func cannedBytesTransport(payload []byte) *transport {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})}
	return newTransport("sw_x", DefaultBaseURL, 30*time.Second, client)
}

func TestVersionPresent(t *testing.T) {
	if !strings.HasPrefix(Version, "0.") {
		t.Fatalf("unexpected version %q", Version)
	}
}

func TestBuildMetadataIncludesAllFieldsInOrder(t *testing.T) {
	meta := buildMetadata(map[string]string{
		"upload_uid": "u", "step": "1", "name": "a.jpg", "client_upload_id": "cid",
	})
	var keys []string
	pairs := map[string]string{}
	for _, part := range strings.Split(meta, ",") {
		kv := strings.SplitN(part, " ", 2)
		keys = append(keys, kv[0])
		pairs[kv[0]] = kv[1]
	}
	want := []string{"upload_uid", "step", "name", "client_upload_id"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("key order = %v, want %v", keys, want)
	}
	if got := decodeB64(t, pairs["name"]); got != "a.jpg" {
		t.Fatalf("name = %q", got)
	}
}

func TestToRelative(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://snapwonders.com/api/tus/abc", "/api/tus/abc"},
		{"/api/tus/abc", "/api/tus/abc"},
		{"api/tus/abc", "/api/tus/abc"},
		{"https://other.host/api/tus/abc", "/api/tus/abc"},
	}
	for _, c := range cases {
		if got := toRelative(c.in, "https://snapwonders.com"); got != c.want {
			t.Errorf("toRelative(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClientDefaultsToCanonicalBase(t *testing.T) {
	c := NewClient("sw_test")
	if c.transport.baseURL != "https://snapwonders.com" {
		t.Fatalf("baseURL = %q", c.transport.baseURL)
	}
}

func TestHideRequiresCoverAndSecret(t *testing.T) {
	c := NewClient("sw_test")
	if _, err := c.Stego.Hide([]string{"only-one-file.jpg"}, "Str0ng!Pass"); err == nil {
		t.Fatal("expected an error for a single file")
	}
}

func TestRunRequiresFiles(t *testing.T) {
	c := NewClient("sw_test")
	if _, err := c.Analyse.Run(nil); err == nil {
		t.Fatal("analyse: expected error for no files")
	}
	if _, err := c.Convert.Run(nil); err == nil {
		t.Fatal("convert: expected error for no files")
	}
}

func TestCreateSessionRejectsBadType(t *testing.T) {
	c := NewClient("sw_test")
	if _, err := c.Stego.CreateSession("nonsense"); err == nil {
		t.Fatal("expected error for bad session type")
	}
}

func TestExtractFilesNormalises(t *testing.T) {
	if got := extractFiles(map[string]any{"files": []any{map[string]any{"status": "completed"}}}); len(got) != 1 {
		t.Fatalf("object form: got %d", len(got))
	}
	if got := extractFiles([]any{map[string]any{"status": "x"}}); len(got) != 1 {
		t.Fatalf("array form: got %d", len(got))
	}
	if got := extractFiles("garbage"); got != nil {
		t.Fatalf("garbage: got %v", got)
	}
}

func TestConvertResultPrefersOutputName(t *testing.T) {
	rf := resultFileFromJSON(
		map[string]any{"asset_id": "a1", "name": "photo.jpg", "output_name": "photo.webp", "size_bytes": float64(12)},
		nil, "/api/convert/download",
	)
	if rf.Name != "photo.webp" || rf.FileSize != 12 {
		t.Fatalf("got name=%q size=%d", rf.Name, rf.FileSize)
	}
}

func TestAnalyseAssetUsesCategory(t *testing.T) {
	item := analyseItemFromJSON(map[string]any{
		"name": "p.jpg", "grade": "B",
		"assets": []any{map[string]any{"asset_id": "x1", "category": "ela_map", "mime_type": "image/png", "file_size": float64(5)}},
	}, nil)
	if len(item.Assets) != 1 {
		t.Fatalf("assets = %d", len(item.Assets))
	}
	a := item.Assets[0]
	if a.Name != "ela_map" || a.MimeType != "image/png" || a.FileSize != 5 {
		t.Fatalf("asset = %+v", a)
	}
}

func TestCheckTerminalSurfacesProgressMessage(t *testing.T) {
	err := checkTerminal(map[string]any{"status": "failed", "progress_message": "This job requires a Pro account.", "error": nil}, "x", false)
	var jfe *JobFailedError
	if !errors.As(err, &jfe) {
		t.Fatalf("want *JobFailedError, got %T", err)
	}
	if !strings.Contains(jfe.Message, "Pro account") || jfe.Reason != "This job requires a Pro account." {
		t.Fatalf("jfe = %+v", jfe)
	}
}

func TestCheckTerminalErrorFieldWins(t *testing.T) {
	err := checkTerminal(map[string]any{"status": "failed", "error": "sanitised detail", "progress_message": "generic"}, "x", false)
	var jfe *JobFailedError
	if !errors.As(err, &jfe) || jfe.Reason != "sanitised detail" {
		t.Fatalf("jfe = %+v", jfe)
	}
}

func TestNextWaitRespectsServerHint(t *testing.T) {
	sleep, next := nextWait(2*time.Second, map[string]any{"status": "processing", "retry_after": float64(30)})
	if sleep != 30*time.Second || next != 30*time.Second {
		t.Fatalf("sleep=%v next=%v", sleep, next)
	}
	sleep2, _ := nextWait(2*time.Second, map[string]any{"status": "processing", "poll_after": "12"})
	if sleep2 != 12*time.Second {
		t.Fatalf("sleep2=%v", sleep2)
	}
}

func TestNextWaitBacksOffAndCaps(t *testing.T) {
	interval := 1500 * time.Millisecond
	var seen []time.Duration
	for i := 0; i < 12; i++ {
		var sleep time.Duration
		sleep, interval = nextWait(interval, map[string]any{"status": "processing"})
		seen = append(seen, sleep)
	}
	if seen[len(seen)-1] <= seen[0] {
		t.Fatal("expected the interval to grow")
	}
	if max := pollMaxInterval + time.Duration(float64(pollMaxInterval)*pollJitter); seen[len(seen)-1] > max {
		t.Fatalf("interval exceeded cap: %v > %v", seen[len(seen)-1], max)
	}
}

func TestDownloadToNonexistentDirectoryCreatesDirNotFile(t *testing.T) {
	rf := &ResultFile{
		AssetID: "abc", Name: "cover-share.avif", MimeType: "image/avif", FileSize: 11,
		t: cannedBytesTransport([]byte("\x89PNG\r\n\x1a\n-fake")), downloadPath: "/api/job/download/abc",
	}
	dir := filepath.Join(t.TempDir(), "out") // does not exist yet
	written, err := rf.Download(dir + "/")
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatal("out/ must become a directory, not a file")
	}
	if want := filepath.Join(dir, "cover-share.avif"); written != want {
		t.Fatalf("written = %q, want %q", written, want)
	}
	if b, _ := os.ReadFile(written); string(b) != "\x89PNG\r\n\x1a\n-fake" {
		t.Fatal("bytes mismatch")
	}
}

func TestStegoPollsStatusByUploadUidNotJobUid(t *testing.T) {
	var paths []string
	tr := recordingTransport(&paths, map[string]any{"status": "completed"}, map[string]any{"result_files": []any{}})
	job := &StegoJob{t: tr, UploadUID: "UPLOAD-111", JobUID: "JOB-999", JobType: "hide"}
	if err := job.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := job.Results(); err != nil {
		t.Fatal(err)
	}
	assertContains(t, paths, "/api/job/UPLOAD-111")
	for _, p := range paths {
		if strings.Contains(p, "JOB-999") {
			t.Fatalf("job_uid leaked into path %q", p)
		}
	}
}

func TestAnalysePollsByUploadUidButResultsByJobUid(t *testing.T) {
	var paths []string
	tr := recordingTransport(&paths, map[string]any{"status": "completed"}, map[string]any{"files": []any{}})
	job := &AnalyseJob{t: tr, UploadUID: "UPLOAD-222", JobUID: "JOB-888"}
	if err := job.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := job.Results(); err != nil {
		t.Fatal(err)
	}
	assertContains(t, paths, "/api/analyse/job/UPLOAD-222") // status → upload_uid
	assertContains(t, paths, "/api/analyse/result/JOB-888") // result → job_uid
	for _, p := range paths {
		if p == "/api/analyse/job/JOB-888" {
			t.Fatal("must never poll status by job_uid")
		}
	}
}

func TestConvertPollsAndFetchesByUploadUid(t *testing.T) {
	var paths []string
	tr := recordingTransport(&paths, map[string]any{"status": "completed"}, map[string]any{"result_files": []any{}})
	job := &ConvertJob{t: tr, UploadUID: "UPLOAD-333", JobUID: "JOB-777"}
	if err := job.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := job.Results(); err != nil {
		t.Fatal(err)
	}
	assertContains(t, paths, "/api/convert/job/UPLOAD-333")
	assertContains(t, paths, "/api/convert/job/UPLOAD-333/results")
	for _, p := range paths {
		if strings.Contains(p, "JOB-777") {
			t.Fatalf("job_uid leaked into path %q", p)
		}
	}
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, h := range haystack {
		if h == needle {
			return
		}
	}
	t.Fatalf("%q not found in %v", needle, haystack)
}

func decodeB64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("bad base64 %q: %v", s, err)
	}
	return string(b)
}
