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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	tusVersion   = "1.0.0"
	tusChunkSize = 5 * 1024 * 1024 // 5 MiB
)

// uploadFile performs a hand-rolled TUS 1.0.0 upload of one file for uploadUID at step. It creates
// the upload resource (POST /api/tus), then streams the bytes in chunks (PATCH), resuming from the
// server's current offset. Returns the base-relative TUS upload path used.
func (t *transport) uploadFile(filePath, uploadUID string, step int) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return "", &TusUploadError{Message: "Not a file: " + filePath}
	}
	total := info.Size()

	// Phase 1 — create. `name` records the original filename (feeds input_original_names /
	// reveal-side filename recovery); a stable `client_upload_id` keeps a retried create from
	// creating a duplicate upload row (TusController reads both).
	clientUploadID, err := uuid4()
	if err != nil {
		return "", &TusUploadError{Message: "cannot generate client_upload_id: " + err.Error()}
	}
	create, err := t.request("POST", "/api/tus", reqOpts{
		Headers: map[string]string{
			"Tus-Resumable": tusVersion,
			"Upload-Length": strconv.FormatInt(total, 10),
			"Upload-Metadata": buildMetadata(map[string]string{
				"upload_uid":       uploadUID,
				"step":             strconv.Itoa(step),
				"name":             filepath.Base(filePath),
				"client_upload_id": clientUploadID,
			}),
		},
		Expected: []int{200, 201},
	})
	if err != nil {
		return "", err
	}
	location := create.Header.Get("Location")
	if location == "" {
		return "", &TusUploadError{Message: "TUS create returned no Location header"}
	}
	uploadPath := toRelative(location, t.baseURL)

	// Phase 2 — stream in chunks from the current server offset (resume-safe).
	offset, err := t.serverOffset(uploadPath)
	if err != nil {
		return "", err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return "", &TusUploadError{Message: "Cannot open file: " + filePath}
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return "", &TusUploadError{Message: "Cannot seek for resume: " + err.Error()}
		}
	}
	buf := make([]byte, tusChunkSize)
	for offset < total {
		n, readErr := f.Read(buf)
		if n > 0 {
			resp, err := t.request("PATCH", uploadPath, reqOpts{
				Headers: map[string]string{
					"Tus-Resumable": tusVersion,
					"Upload-Offset": strconv.FormatInt(offset, 10),
					"Content-Type":  "application/offset+octet-stream",
				},
				Content:  buf[:n],
				Expected: []int{200, 204},
			})
			if err != nil {
				return "", err
			}
			if newOffset := resp.Header.Get("Upload-Offset"); newOffset != "" {
				if parsed, perr := strconv.ParseInt(newOffset, 10, 64); perr == nil {
					offset = parsed
				} else {
					offset += int64(n)
				}
			} else {
				offset += int64(n)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", &TusUploadError{Message: "Read error during upload: " + readErr.Error()}
		}
	}

	if offset != total {
		return "", &TusUploadError{Message: fmt.Sprintf("Upload incomplete: sent to offset %d of %d", offset, total)}
	}
	return uploadPath, nil
}

// serverOffset HEADs the upload to learn how many bytes the server already has (0 for a fresh create).
func (t *transport) serverOffset(uploadPath string) (int64, error) {
	resp, err := t.request("HEAD", uploadPath, reqOpts{
		Headers:  map[string]string{"Tus-Resumable": tusVersion},
		Expected: []int{200, 204},
	})
	if err != nil {
		return 0, err
	}
	if v := resp.Header.Get("Upload-Offset"); v != "" {
		if parsed, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			return parsed, nil
		}
	}
	return 0, nil
}

// buildMetadata encodes an Upload-Metadata header: comma-separated "key <base64(value)>" pairs.
// The key order is fixed so the header is deterministic (Go map iteration is randomised).
func buildMetadata(fields map[string]string) string {
	order := []string{"upload_uid", "step", "name", "client_upload_id"}
	var pairs []string
	for _, k := range order {
		if v, ok := fields[k]; ok {
			pairs = append(pairs, k+" "+base64.StdEncoding.EncodeToString([]byte(v)))
		}
	}
	return strings.Join(pairs, ",")
}

// toRelative normalises a create Location to a base-relative path for subsequent PATCH/HEAD.
func toRelative(location, baseURL string) string {
	if strings.HasPrefix(location, baseURL) {
		return strings.TrimPrefix(location, baseURL)
	}
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		// Absolute but different host — keep the path portion only.
		if u, err := url.Parse(location); err == nil {
			return u.Path
		}
	}
	if strings.HasPrefix(location, "/") {
		return location
	}
	return "/" + location
}

func uuid4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
