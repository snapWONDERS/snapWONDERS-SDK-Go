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
	"errors"
	"time"
)

// Analyse is client.Analyse — forensic media analysis. Session factory plus a one-shot Run helper.
type Analyse struct{ t *transport }

// AnalyseSession is an open analyse upload session — one or more files, all at step 1.
type AnalyseSession struct {
	t         *transport
	UploadUID string
}

// AnalyseJob is a queued analyse job; poll with Wait, then read Results.
type AnalyseJob struct {
	t *transport

	// The status endpoint is keyed by the session's upload_uid; only the /result/{jobUID} endpoint
	// is keyed by job_uid. These two IDs differ, so status is polled by upload_uid.
	UploadUID string
	JobUID    string
	Status    string
	Error     string
}

// CreateSession opens an analyse upload session.
func (a *Analyse) CreateSession() (*AnalyseSession, error) {
	resp, err := a.t.request("POST", "/api/analyse/session", reqOpts{JSON: map[string]any{}})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	return &AnalyseSession{t: a.t, UploadUID: stringValue(data["upload_uid"])}, nil
}

// Run is the one-shot: create a session, upload every file, and run analysis to completion.
func (a *Analyse) Run(files []string, opts ...JobOption) (*AnalyseJob, error) {
	if len(files) == 0 {
		return nil, errors.New("Run needs at least one file to analyse")
	}
	session, err := a.CreateSession()
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if _, err := session.Upload(file); err != nil {
			return nil, err
		}
	}
	if err := session.WaitForUploads(); err != nil {
		return nil, err
	}
	job, err := session.StartJob(opts...)
	if err != nil {
		return nil, err
	}
	return job, job.Wait()
}

// Upload sends one file for analysis.
func (s *AnalyseSession) Upload(filePath string) (string, error) {
	return s.t.uploadFile(filePath, s.UploadUID, 1)
}

// Files lists the session's uploaded files and their status.
func (s *AnalyseSession) Files() ([]map[string]any, error) {
	resp, err := s.t.request("GET", "/api/analyse/session/"+s.UploadUID+"/files", reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	var payload any
	_ = json.Unmarshal(resp.Body, &payload)
	return extractFiles(payload), nil
}

// WaitForUploads blocks until every uploaded file reports "completed".
func (s *AnalyseSession) WaitForUploads() error {
	_, err := s.t.waitForUploads("/api/analyse/session/"+s.UploadUID+"/files", 1*time.Second, 120*time.Second)
	return err
}

// StartJob queues analysis. Job options carry `face_detection`, `text_detection`, `face_sensitivity`
// (standard/thorough), `forensic_depth` (standard/deep) via WithOption.
func (s *AnalyseSession) StartJob(opts ...JobOption) (*AnalyseJob, error) {
	cfg := resolveJobConfig(opts...)
	body := map[string]any{"upload_uid": s.UploadUID, "expiry": cfg.expiry}
	for k, v := range cfg.options {
		body[k] = v
	}
	resp, err := s.t.request("POST", "/api/analyse/job", reqOpts{JSON: body})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	return &AnalyseJob{t: s.t, UploadUID: s.UploadUID, JobUID: stringValue(data["job_uid"])}, nil
}

// Wait polls until the job is terminal.
func (j *AnalyseJob) Wait(opts ...WaitOption) error {
	cfg := resolveWaitConfig(opts...)
	final, err := j.t.pollJob("/api/analyse/job/"+j.UploadUID, cfg.pollInterval, cfg.timeout)
	if err != nil {
		return err
	}
	j.Status = stringValue(final["status"])
	j.Error = stringValue(final["error"])
	return checkTerminal(final, j.JobUID, cfg.strict)
}

// Results returns the per-file forensic verdicts (grade, counts, verdicts) and downloadable overlays.
func (j *AnalyseJob) Results() ([]*AnalyseItem, error) {
	// This endpoint is keyed by job_uid, unlike the status poll above.
	resp, err := j.t.request("GET", "/api/analyse/result/"+j.JobUID, reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	var payload any
	_ = json.Unmarshal(resp.Body, &payload)
	items := extractAnalyseItems(payload)
	out := make([]*AnalyseItem, 0, len(items))
	for _, item := range items {
		out = append(out, analyseItemFromJSON(item, j.t))
	}
	return out, nil
}

func extractAnalyseItems(payload any) []map[string]any {
	switch p := payload.(type) {
	case []any:
		return toMapSlice(p)
	case map[string]any:
		for _, key := range []string{"files", "items", "results"} {
			if arr, ok := p[key].([]any); ok {
				return toMapSlice(arr)
			}
		}
	}
	return nil
}
