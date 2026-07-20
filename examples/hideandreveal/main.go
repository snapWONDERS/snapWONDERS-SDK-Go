// snapWONDERS API — Go SDK example
//
// Copyright (c) 2026 Kenneth Springer @ snapWONDERS. MIT Licensed — see LICENSE.
// Author: Kenneth Springer @ snapWONDERS <kenneth@snapwonders.com> (https://kennethbspringer.au)
//
// Steganography: hide a secret file inside a cover image, then reveal it back out.
// Run:  SNAPWONDERS_API_KEY=sw_... go run ./examples/hideandreveal

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
	out := filepath.Join(examples, "out")
	client := snapwonders.NewClient(key)

	// A "secret" (any media file) and a "cover" image to hide it inside. The secret and cover must be
	// two different files, and the cover's shortest side must be at least 512px.
	secret := filepath.Join(examples, "assets", "secret.png")
	cover := filepath.Join(examples, "assets", "sample.png")

	fmt.Println("Hiding — the SDK creates a session, uploads both files, runs the job, and waits …")
	job, err := client.Stego.Hide([]string{secret, cover}, "Str0ng!Pass")
	must(err)
	fmt.Printf("  status: %s\n", job.Status)

	results, err := job.Results()
	must(err)
	var stego string
	for _, r := range results {
		stego, err = r.Download(out + "/") // a trailing "/" writes into the directory
		must(err)
		fmt.Printf("  stego image → %s\n", stego)
	}

	fmt.Println("Revealing the hidden file back out …")
	revealed, err := client.Stego.Reveal(stego, "Str0ng!Pass")
	must(err)
	revealedFiles, err := revealed.Results()
	must(err)
	for _, r := range revealedFiles {
		path, err := r.Download(filepath.Join(out, "recovered") + "/")
		must(err)
		fmt.Printf("  recovered → %s\n", path)
	}

	fmt.Println("Done. See", out)
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
