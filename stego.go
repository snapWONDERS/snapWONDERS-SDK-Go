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

// Stego is client.Stego — steganography (hide / reveal), the flagship flow. Session factory plus the
// one-shot Hide / Reveal helpers.
type Stego struct{ t *transport }

// StegoSession is an open hide/reveal upload session. Upload files, then start a job.
type StegoSession struct {
	t           *transport
	UploadUID   string
	SessionType string // "hide" | "reveal"
}

// StegoJob is a queued hide/reveal job; poll it with Wait, then read Results.
type StegoJob struct {
	t *transport

	// Status and results are keyed by the session's upload_uid, not the job_uid. Poll and fetch by
	// upload_uid to match the API contract.
	UploadUID       string
	JobUID          string
	JobType         string
	Status          string
	Error           string
	ProgressMessage string
}

// CreateSession opens a "hide" or "reveal" upload session.
func (s *Stego) CreateSession(sessionType string) (*StegoSession, error) {
	if sessionType != "hide" && sessionType != "reveal" {
		return nil, errors.New(`sessionType must be "hide" or "reveal"`)
	}
	resp, err := s.t.request("POST", "/api/session", reqOpts{JSON: map[string]any{"type": sessionType}})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	return &StegoSession{t: s.t, UploadUID: stringValue(data["upload_uid"]), SessionType: sessionType}, nil
}

// Hide is the one-shot: create a hide session, upload secret(s) then cover, and run to completion.
// Convention: the LAST path is the cover (step 2); all earlier paths are secrets (step 1).
func (s *Stego) Hide(files []string, password string, opts ...JobOption) (*StegoJob, error) {
	if len(files) < 2 {
		return nil, errors.New("Hide needs at least one secret and one cover (>=2 files)")
	}
	session, err := s.CreateSession("hide")
	if err != nil {
		return nil, err
	}
	for _, secret := range files[:len(files)-1] {
		if _, err := session.Upload(secret, 1); err != nil {
			return nil, err
		}
	}
	if _, err := session.Upload(files[len(files)-1], 2); err != nil {
		return nil, err
	}
	if err := session.WaitForUploads(); err != nil {
		return nil, err
	}
	job, err := session.StartJob(password, opts...)
	if err != nil {
		return nil, err
	}
	return job, job.Wait()
}

// Reveal is the one-shot: create a reveal session, upload the stego file, and run to completion.
func (s *Stego) Reveal(stegoFile, password string, opts ...JobOption) (*StegoJob, error) {
	session, err := s.CreateSession("reveal")
	if err != nil {
		return nil, err
	}
	if _, err := session.Upload(stegoFile, 1); err != nil {
		return nil, err
	}
	if err := session.WaitForUploads(); err != nil {
		return nil, err
	}
	job, err := session.StartJob(password, opts...)
	if err != nil {
		return nil, err
	}
	return job, job.Wait()
}

// Upload sends one file at step (1 = secret/stego input, 2 = cover for hide). Returns the TUS path.
func (s *StegoSession) Upload(filePath string, step int) (string, error) {
	return s.t.uploadFile(filePath, s.UploadUID, step)
}

// Files lists the session's uploaded files and their status.
func (s *StegoSession) Files() ([]map[string]any, error) {
	resp, err := s.t.request("GET", "/api/session/"+s.UploadUID+"/files", reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	var payload any
	_ = json.Unmarshal(resp.Body, &payload)
	return extractFiles(payload), nil
}

// WaitForUploads blocks until every uploaded file reports "completed".
func (s *StegoSession) WaitForUploads() error {
	_, err := s.t.waitForUploads("/api/session/"+s.UploadUID+"/files", 1*time.Second, 120*time.Second)
	return err
}

// StartJob queues the hide/reveal job. Job options (WithExpiry, WithOption, …) apply the hide-only
// encoding keys (ignored for reveal).
func (s *StegoSession) StartJob(password string, opts ...JobOption) (*StegoJob, error) {
	cfg := resolveJobConfig(opts...)
	body := map[string]any{"upload_uid": s.UploadUID, "password": password, "expiry": cfg.expiry}
	for k, v := range cfg.options {
		body[k] = v
	}
	resp, err := s.t.request("POST", "/api/job", reqOpts{JSON: body})
	if err != nil {
		return nil, err
	}
	data := resp.decodeMap()
	jobType := stringValue(data["job_type"])
	if jobType == "" {
		jobType = s.SessionType
	}
	return &StegoJob{t: s.t, UploadUID: s.UploadUID, JobUID: stringValue(data["job_uid"]), JobType: jobType}, nil
}

// Refresh fetches the latest job status without waiting.
func (j *StegoJob) Refresh() error {
	resp, err := j.t.request("GET", "/api/job/"+j.UploadUID, reqOpts{Expected: []int{200}})
	if err != nil {
		return err
	}
	data := resp.decodeMap()
	j.Status = stringValue(data["status"])
	j.Error = stringValue(data["error"])
	j.ProgressMessage = stringValue(data["progress_message"])
	return nil
}

// Wait polls until the job is terminal. Returns a *JobFailedError on "failed" (or "partial" when
// Strict()).
func (j *StegoJob) Wait(opts ...WaitOption) error {
	cfg := resolveWaitConfig(opts...)
	final, err := j.t.pollJob("/api/job/"+j.UploadUID, cfg.pollInterval, cfg.timeout)
	if err != nil {
		return err
	}
	j.Status = stringValue(final["status"])
	j.Error = stringValue(final["error"])
	j.ProgressMessage = stringValue(final["progress_message"])
	return checkTerminal(final, j.JobUID, cfg.strict)
}

// Results returns the downloadable outputs of a completed job.
func (j *StegoJob) Results() ([]*ResultFile, error) {
	resp, err := j.t.request("GET", "/api/job/"+j.UploadUID+"/results", reqOpts{Expected: []int{200}})
	if err != nil {
		return nil, err
	}
	return resultFilesFrom(resp.decodeMap()["result_files"], j.t, "/api/job/download"), nil
}
