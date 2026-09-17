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

// Package snapwonders is the official Go client for the snapWONDERS API: steganography, forensic
// media analysis, and format conversion.
//
// It wraps the resumable TUS upload and the session → job → poll → download choreography so a whole
// job is a few lines:
//
//	client := snapwonders.NewClient("sw_...")
//	job, err := client.Stego.Hide([]string{"secret.pdf", "cover.jpg"}, "Str0ng!Pass")
//	files, err := job.Results()
//	for _, f := range files {
//		f.Download("out/")
//	}
//
// The three product namespaces — Stego (hide/reveal), Analyse (forensics), and Convert (media) — all
// share the same session → job → poll → download shape.
package snapwonders
