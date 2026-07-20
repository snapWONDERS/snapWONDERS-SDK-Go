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

// Convert is client.Convert — media conversion. Session factory plus a one-shot Run helper.
type Convert struct{ t *transport }

// ConvertSession is an open convert upload session — one or more files, all at step 1.
type ConvertSession struct {
	t         *transport
	UploadUID string
}

// ConvertJob is a queued convert job; poll with Wait, then read Results.
type ConvertJob struct {
	t *transport

	// Both status and results are keyed by the session's upload_uid, not the job_uid (which differs
	// for convert). Poll and fetch by upload_uid to match the API contract.
	UploadUID string
	JobUID    string
	Status    string
	Error     string
}

// CreateSession opens a convert upload session.
func (c *Convert) CreateSession() (*ConvertSession, error) {
	resp, err := c.t.request("POST", "/api/convert/session", reqOpts{JSON: map[string]any{}})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	return &ConvertSession{t: c.t, UploadUID: stringValue(data["upload_uid"])}, nil
}

// Run is the one-shot: create a session, upload every file, and convert to completion. Set the output
// format with WithOption, e.g. WithOption("image_format", "webp").
func (c *Convert) Run(files []string, opts ...JobOption) (*ConvertJob, error) {
	if len(files) == 0 {
		return nil, errors.New("Run needs at least one file to convert")
	}
	session, err := c.CreateSession()
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

// Upload sends one file for conversion.
func (s *ConvertSession) Upload(filePath string) (string, error) {
	return s.t.uploadFile(filePath, s.UploadUID, 1)
}

// Files lists the session's uploaded files and their status.
func (s *ConvertSession) Files() ([]map[string]any, error) {
	resp, err := s.t.request("GET", "/api/convert/session/"+s.UploadUID+"/files", reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	var payload any
	_ = json.Unmarshal(resp.Body, &payload)
	return extractFiles(payload), nil
}

// WaitForUploads blocks until every uploaded file reports "completed".
func (s *ConvertSession) WaitForUploads() error {
	_, err := s.t.waitForUploads("/api/convert/session/"+s.UploadUID+"/files", 1*time.Second, 120*time.Second)
	return err
}

// StartJob queues conversion. Options are the convert encoding keys — e.g. WithOption("image_format",
// "webp") (jpeg/png/webp/avif/heic/jxl), WithOption("video_format", …), and resize/quality controls.
func (s *ConvertSession) StartJob(opts ...JobOption) (*ConvertJob, error) {
	cfg := resolveJobConfig(opts...)
	body := map[string]any{"upload_uid": s.UploadUID, "expiry": cfg.expiry}
	for k, v := range cfg.options {
		body[k] = v
	}
	resp, err := s.t.request("POST", "/api/convert/job", reqOpts{JSON: body})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	return &ConvertJob{t: s.t, UploadUID: s.UploadUID, JobUID: stringValue(data["job_uid"])}, nil
}

// Wait polls until the job is terminal.
func (j *ConvertJob) Wait(opts ...WaitOption) error {
	cfg := resolveWaitConfig(opts...)
	final, err := j.t.pollJob("/api/convert/job/"+j.UploadUID, cfg.pollInterval, cfg.timeout)
	if err != nil {
		return err
	}
	j.Status = stringValue(final["status"])
	j.Error = stringValue(final["error"])
	return checkTerminal(final, j.JobUID, cfg.strict)
}

// Results returns the converted output files.
func (j *ConvertJob) Results() ([]*ResultFile, error) {
	resp, err := j.t.request("GET", "/api/convert/job/"+j.UploadUID+"/results", reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	items := data["result_files"]
	if items == nil {
		items = data["items"] // fallback
	}
	return resultFilesFrom(items, j.t, "/api/convert/download"), nil
}
