// snapWONDERS API — Go SDK example
//
// Copyright (c) 2026 Kenneth Springer @ snapWONDERS. MIT Licensed — see LICENSE.
// Author: Kenneth Springer @ snapWONDERS <kenneth@snapwonders.com> (https://kennethbspringer.au)
//
// Forensic analysis: grade an image A–F and download the overlay assets it produces.
// Run:  SNAPWONDERS_API_KEY=sw_... go run ./examples/analyse [path/to/image.jpg]

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	snapwonders "github.com/snapWONDERS/snapWONDERS-SDK-Go"
)

func main() {
	key := os.Getenv("SNAPWONDERS_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "Set SNAPWONDERS_API_KEY (get one at https://snapwonders.com/sign-up)")
		os.Exit(1)
	}
	examples := exampleDir()
	out := filepath.Join(examples, "out", "analyse")
	image := filepath.Join(examples, "assets", "sample.png")
	if len(os.Args) > 1 {
		image = os.Args[1]
	}
	client := snapwonders.NewClient(key)

	fmt.Printf("Analysing %s …\n", image)
	job, err := client.Analyse.Run([]string{image}, snapwonders.WithOption("face_detection", true))
	must(err)
	fmt.Printf("  status: %s\n", job.Status)

	items, err := job.Results()
	must(err)
	for _, item := range items {
		fmt.Printf("\n  %s\n", item.Filename)
		fmt.Printf("    grade        : %s\n", item.Grade)
		fmt.Printf("    faces        : %d\n", item.FaceCount)
		fmt.Printf("    text regions : %d\n", item.TextRegionCount)
		fmt.Printf("    watermark    : %v\n", item.WatermarkFlagged)
		if len(item.Verdicts) > 0 {
			fmt.Printf("    AI generation: %s\n", verdictField(item.Verdicts, "ai_generation", "verdict"))
			fmt.Printf("    C2PA         : %s\n", verdictField(item.Verdicts, "c2pa", "verdict"))
			fmt.Printf("    camera match : %s\n", verdictField(item.Verdicts, "camera_fingerprint", "encoder_name"))
			if findings, ok := item.Verdicts["findings"].([]any); ok {
				for _, f := range findings {
					if m, ok := f.(map[string]any); ok {
						fmt.Printf("    finding      : %v (%v)\n", m["label"], m["severity"])
					}
				}
			}
			// forensic / audio_splice / video_tamper carry the WHY behind the grade.
			if forensic, ok := item.Verdicts["forensic"].(map[string]any); ok {
				if ela, ok := forensic["ela"].(map[string]any); ok {
					fmt.Printf("    ELA          : %v high-residual, %v anomaly tile(s)\n", ela["high_fraction"], ela["anomaly_tile_count"])
				}
				if noise, ok := forensic["noise_inconsistency"].(map[string]any); ok {
					fmt.Printf("    noise map    : %v anomaly tile(s)\n", noise["anomaly_tile_count"])
				}
			}
			if splice, ok := item.Verdicts["audio_splice"].(map[string]any); ok {
				fmt.Printf("    audio splice : %v (%v spike(s), noise CoV %v)\n", splice["verdict"], splice["spike_count"], splice["noise_cov"])
			}
			if tamper, ok := item.Verdicts["video_tamper"].(map[string]any); ok {
				if inter, ok := tamper["inter_frame_tamper"].(map[string]any); ok {
					fmt.Printf("    inter-frame  : %v (%v spike(s) / %v frames checked)\n", inter["verdict"], inter["spike_count"], inter["frame_count_checked"])
				}
				if dup, ok := tamper["frame_duplicate"].(map[string]any); ok {
					fmt.Printf("    dup frames   : %v (%v duplicate(s))\n", dup["dup_verdict"], dup["dup_count"])
				}
				if gps, ok := tamper["gps_triangle"].(map[string]any); ok && gps["applicable"] == true {
					fmt.Printf("    GPS triangle : %v (Δ%vs, %v)\n", gps["verdict"], gps["delta_secs"], gps["timezone"])
				}
			}
		}
		for _, asset := range item.Assets { // ELA map, face overlay, …
			path, err := asset.Download(out + "/")
			must(err)
			fmt.Printf("    asset        : %s → %s\n", asset.Name, path)
		}
	}

	fmt.Println("\nDone. See", out)
}

func verdictField(verdicts map[string]any, block, key string) string {
	if b, ok := verdicts[block].(map[string]any); ok {
		if s, ok := b[key].(string); ok {
			return s
		}
	}
	return ""
}

func exampleDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
