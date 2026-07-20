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
	"strings"
)

// ResultFile is one downloadable output (a stego image, recovered secret, converted file, analyse
// asset…). A thin view over the JSON the API returns, with a convenience Download method.
type ResultFile struct {
	AssetID  string
	Name     string
	MimeType string
	FileSize int64

	t            *transport
	downloadPath string
}

// Download streams this asset to dest and returns the written path.
//
// dest may be a file path ("out/photo.avif") or a directory ("out/", or "out" when it already
// exists), in which case the server-supplied Name is appended. A trailing separator means
// "directory" even when it does not exist yet — os.Stat is an error for a path yet to be created, so
// a trailing "/" must be honoured explicitly or the asset is written to a file literally named "out".
func (f *ResultFile) Download(dest string) (string, error) {
	isDir := strings.HasSuffix(dest, "/") || strings.HasSuffix(dest, "\\")
	if !isDir {
		if info, err := os.Stat(dest); err == nil && info.IsDir() {
			isDir = true
		}
	}
	target := dest
	if isDir {
		target = filepath.Join(strings.TrimRight(dest, "/\\"), f.Name)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	resp, err := f.t.request("GET", f.downloadPath, reqOpts{Expected: []int{200}})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, resp.Body, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func resultFileFromJSON(data map[string]any, t *transport, downloadPrefix string) *ResultFile {
	assetID := stringValue(data["asset_id"])
	// Convert results carry the converted filename in `output_name` and the original in `name` —
	// prefer the converted name. Stego uses `name`; analyse assets inject a name.
	name := firstString(data, "output_name", "name", "filename")
	if name == "" {
		name = assetID
	}
	return &ResultFile{
		AssetID:      assetID,
		Name:         name,
		MimeType:     stringValue(data["mime_type"]),
		FileSize:     firstInt(data, "file_size", "size_bytes"),
		t:            t,
		downloadPath: downloadPrefix + "/" + assetID,
	}
}

// AnalyseItem is the forensic verdict for one analysed file, plus its downloadable overlay assets.
// Field names are read leniently (overall_grade/grade, etc.); confirm against the live
// /api/analyse/result/{job_uid} shape as exact field names can vary.
type AnalyseItem struct {
	Filename               string
	Grade                  string
	FaceCount              int64
	TextRegionCount        int64
	WatermarkFlagged       bool
	SteganographySuspected bool

	// Verdicts holds the forensic verdicts when the API includes them: ai_generation, c2pa,
	// camera_fingerprint, findings. A plain map — the keys grow over time; read what you need.
	Verdicts map[string]any
	Assets   []*ResultFile
	Raw      map[string]any
}

func analyseItemFromJSON(data map[string]any, t *transport) *AnalyseItem {
	var assets []*ResultFile
	if rawAssets, ok := data["assets"].([]any); ok {
		for _, a := range rawAssets {
			asset, ok := a.(map[string]any)
			if !ok || asset["asset_id"] == nil {
				continue
			}
			// Analyse assets are keyed by `category` (e.g. "ela_map") — inject a display name.
			if asset["name"] == nil {
				asset["name"] = firstNonNil(asset, "category", "type", "asset_id")
			}
			assets = append(assets, resultFileFromJSON(asset, t, "/api/analyse/asset"))
		}
	}

	grade := stringValue(data["overall_grade"])
	if grade == "" {
		grade = stringValue(data["grade"])
	}
	filename := stringValue(data["filename"])
	if filename == "" {
		filename = stringValue(data["name"])
	}
	verdicts, _ := data["verdicts"].(map[string]any)
	if verdicts == nil {
		verdicts = map[string]any{}
	}

	return &AnalyseItem{
		Filename:               filename,
		Grade:                  grade,
		FaceCount:              intValue(data["face_count"]),
		TextRegionCount:        intValue(data["text_region_count"]),
		WatermarkFlagged:       boolValue(data["watermark_flagged"]),
		SteganographySuspected: boolValue(data["steganography_suspected"]),
		Verdicts:               verdicts,
		Assets:                 assets,
		Raw:                    data,
	}
}

// --- lenient JSON scalar helpers (encoding/json decodes numbers as float64) ---

func intValue(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringValue(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return intValue(v)
		}
	}
	return 0
}

func firstNonNil(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return ""
}
