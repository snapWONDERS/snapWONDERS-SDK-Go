// snapWONDERS API — Go SDK example
//
// Copyright (c) 2026 Kenneth Springer @ snapWONDERS. MIT Licensed — see LICENSE.
// Author: Kenneth Springer @ snapWONDERS <kenneth@snapwonders.com> (https://kennethbspringer.au)
//
// Media conversion: re-encode an image to another format.
// Run:  SNAPWONDERS_API_KEY=sw_... go run ./examples/convert [path/to/image] [format]

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
	out := filepath.Join(examples, "out", "convert")
	image := filepath.Join(examples, "assets", "sample.png")
	if len(os.Args) > 1 {
		image = os.Args[1]
	}
	format := "webp" // jpeg | png | webp | avif | heic | jxl
	if len(os.Args) > 2 {
		format = os.Args[2]
	}
	client := snapwonders.NewClient(key)

	fmt.Printf("Converting %s → %s …\n", image, format)
	job, err := client.Convert.Run([]string{image}, snapwonders.WithOption("image_format", format))
	must(err)

	results, err := job.Results()
	must(err)
	for _, r := range results {
		path, err := r.Download(out + "/")
		must(err)
		fmt.Printf("  converted → %s\n", path)
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
