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
	"os"
	"path/filepath"
	"strconv"
)

// DefaultMaxUploadBytes mirrors the server's own fallback (upload.direct.max_bytes). It is only
// used when a session response did not carry max_upload_bytes — prefer the server's figure, which
// can move without an SDK release.
const DefaultMaxUploadBytes int64 = 99614720 // 95 MiB

// directUploadFile uploads one file for uploadUID at step in a single request, returning the
// storage_uid the server assigned.
//
// The alternative to uploadFile (TUS). TUS costs a create, one or more chunked PATCHes and
// sometimes a HEAD; this is a single POST with the file as the request body. For the common case —
// one photo, well under the cap — that is one round trip instead of three, which matters most on
// the high-latency links this API is often used over (Tor and I2P).
//
// Callers do not choose between the two; routeUploadFile picks, using the max_upload_bytes the
// server reported when the session was created.
func (t *transport) directUploadFile(filePath, uploadUID string, step int) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", &UploadError{Message: "Cannot read file: " + filePath}
	}
	if info.IsDir() {
		return "", &UploadError{Message: "Not a file: " + filePath}
	}

	// Read the whole file into memory rather than streaming it.
	//
	// reqOpts.Content is a []byte, which send() wraps in a *bytes.Reader — and http.NewRequest
	// special-cases that type to set ContentLength. Handing it an io.Reader instead would drop to
	// Transfer-Encoding: chunked, and the server refuses chunked uploads with 411, because without
	// a declared length a body truncated in transit cannot be told apart from a complete one.
	// Analysing a truncated file would return confident findings about a file the user never sent,
	// which is why that rule exists.
	//
	// The memory cost is bounded by the size check the router already made — this path only runs
	// for files under max_upload_bytes.
	body, err := os.ReadFile(filePath)
	if err != nil {
		return "", &UploadError{Message: "Cannot read file: " + filePath}
	}

	clientUploadID, err := uuid4()
	if err != nil {
		return "", &UploadError{Message: "Cannot generate client upload id: " + err.Error()}
	}

	res, err := t.request("POST", "/api/upload", reqOpts{
		Headers: map[string]string{
			"X-Upload-Uid":  uploadUID,
			"X-Upload-Step": strconv.Itoa(step),
			"X-Filename":    filepath.Base(filePath),
			"Content-Type":  "application/octet-stream",
			// Same idempotency contract as the TUS path: a retry after a lost response returns the
			// original result instead of storing the file twice.
			"X-Client-Upload-Id": clientUploadID,
		},
		Content:  body,
		Expected: []int{200, 201},
	})
	if err != nil {
		return "", err
	}

	file, _ := res.decodeMap()["file"].(map[string]any)
	storageUID, _ := file["storage_uid"].(string)
	if storageUID == "" {
		return "", &UploadError{Message: "Direct upload returned no storage_uid"}
	}
	return storageUID, nil
}

// routeUploadFile uploads one file by whichever method fits, returning an identifier for it.
//
// The API offers a single-request direct upload for files under a server-declared cap, and the
// resumable TUS protocol for everything else. Which one is right is a mechanical decision — file
// size against max_upload_bytes — and not something a caller of this SDK should ever have to think
// about, so it is made here and only here.
//
// maxUploadBytes should be the value the session-create response reported; 0 falls back to
// DefaultMaxUploadBytes, the figure this SDK was built against.
//
// ⚠️ The returned string means different things by method. Direct upload returns the server's
// storage_uid; TUS returns the upload path it used. That difference predates this router — it is
// what each underlying function has always returned — but before direct upload existed,
// Session.Upload always returned the TUS path, so anyone relying on that value for a small file now
// gets a storage_uid instead. Neither value is needed for the normal flow: uploads are confirmed
// with Files/WaitForUploads and jobs are keyed by uploadUID.
func (t *transport) routeUploadFile(filePath, uploadUID string, step int, maxUploadBytes int64) (string, error) {
	limit := maxUploadBytes
	if limit <= 0 {
		limit = DefaultMaxUploadBytes
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return "", &UploadError{Message: "Cannot read file: " + filePath}
	}

	if info.Size() <= limit {
		return t.directUploadFile(filePath, uploadUID, step)
	}

	// Over the cap, or the caller wants resume behaviour: the protocol earns its cost here.
	return t.uploadFile(filePath, uploadUID, step)
}
