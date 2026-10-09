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
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

var terminalStates = map[string]bool{"completed": true, "partial": true, "failed": true}

// Polling cadence. Fixed-interval polling is a scaling hazard: N clients that start together poll in
// lockstep, so the server sees a synchronised burst every interval (a thundering herd). Instead we
// back off (fast first checks catch quick jobs; long jobs poll progressively less) and jitter each
// wait so concurrent clients desynchronise. The server can override the cadence centrally by
// returning `retry_after`/`poll_after` (seconds) in the status body.
const (
	pollBackoff     = 1.6
	pollMaxInterval = 15 * time.Second
	pollJitter      = 0.25
)

// nextWait returns (sleep, nextBaseInterval) for one poll cycle. A server-supplied
// retry_after/poll_after (seconds) wins; otherwise back off from current with ± jitter.
func nextWait(current time.Duration, statusBody map[string]any) (time.Duration, time.Duration) {
	if hint, ok := numericHint(statusBody); ok {
		d := time.Duration(hint * float64(time.Second))
		return d, d // honour the server; do not keep growing while it dictates
	}
	factor := 1.0 + (rand.Float64()*2-1)*pollJitter
	sleep := time.Duration(float64(current) * factor)
	next := time.Duration(math.Min(float64(current)*pollBackoff, float64(pollMaxInterval)))
	return sleep, next
}

func numericHint(statusBody map[string]any) (float64, bool) {
	for _, key := range []string{"retry_after", "poll_after"} {
		switch v := statusBody[key].(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		case string:
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// extractFiles normalises a session/{uid}/files response to a slice of file maps.
func extractFiles(payload any) []map[string]any {
	switch p := payload.(type) {
	case map[string]any:
		if files, ok := p["files"].([]any); ok {
			return toMapSlice(files)
		}
	case []any:
		return toMapSlice(p)
	}
	return nil
}

func toMapSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// waitForUploads polls filesPath until every uploaded file reports "completed".
func (t *transport) waitForUploads(filesPath string, pollInterval, timeout time.Duration) ([]map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := t.request("GET", filesPath, reqOpts{Expected: []int{200}})
		if err != nil {
			return nil, err
		}
		var payload any
		_ = json.Unmarshal(resp.Body, &payload)
		files := extractFiles(payload)
		if len(files) > 0 && allCompleted(files) {
			return files, nil
		}
		if time.Now().After(deadline) {
			return nil, &jobTimeout{fmt.Sprintf("Uploads at %s not complete within %.0fs", filesPath, timeout.Seconds())}
		}
		time.Sleep(pollInterval)
	}
}

// pollJob polls statusPath until the job reaches a terminal state; returns the final status body.
// pollInterval is the initial gap; it backs off (× ~1.6, capped at 15s) with ± jitter. A server-sent
// retry_after/poll_after overrides the cadence.
func (t *transport) pollJob(statusPath string, pollInterval, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	interval := pollInterval
	for {
		resp, err := t.request("GET", statusPath, reqOpts{Expected: []int{200}})
		if err != nil {
			return nil, err
		}
		data := resp.decodeMap()
		if status, _ := data["status"].(string); terminalStates[status] {
			return data, nil
		}
		if time.Now().After(deadline) {
			return nil, &jobTimeout{fmt.Sprintf("Job at %s did not finish within %.0fs", statusPath, timeout.Seconds())}
		}
		var sleep time.Duration
		sleep, interval = nextWait(interval, data)
		if remaining := time.Until(deadline); sleep > remaining {
			sleep = remaining
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

// checkTerminal returns a *JobFailedError on "failed" (and on "partial" when strict). The reason
// lives in progress_message, not error (which may be null); prefer error, fall back to
// progress_message.
func checkTerminal(statusBody map[string]any, uid string, strict bool) error {
	status, _ := statusBody["status"].(string)
	if status == "failed" || (strict && status == "partial") {
		reason := stringValue(statusBody["error"])
		if reason == "" {
			reason = stringValue(statusBody["progress_message"])
		}
		message := fmt.Sprintf("Job %s ended as %s", uid, status)
		if reason != "" {
			message += ": " + reason
		}
		return &JobFailedError{Message: message, Reason: reason, Status: status}
	}
	return nil
}

func allCompleted(files []map[string]any) bool {
	for _, f := range files {
		if s, _ := f["status"].(string); s != "completed" {
			return false
		}
	}
	return true
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

// jobTimeout is an internal SDK error for a wait/poll that exceeded its deadline.
type jobTimeout struct{ msg string }

func (e *jobTimeout) Error() string { return e.msg }
func (*jobTimeout) snapwonders()    {}
