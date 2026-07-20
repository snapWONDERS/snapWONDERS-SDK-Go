<p align="center">
    <a href="https://www.snapwonders.com/" target="_blank">
        <img src="https://raw.githubusercontent.com/snapWONDERS/snapWONDERS-SDK-Go/main/.github/social-preview.jpg" alt="snapWONDERS Go SDK" width="640" />
    </a>
</p>

snapWONDERS — Expose what's hidden. Hide what's yours.


# snapWONDERS Go SDK — client for the snapWONDERS API

The official Go client for the snapWONDERS API: steganography, forensic media analysis, and format
conversion. The client wraps the resumable TUS upload and the session → job → poll → download
choreography so that a whole job is a few lines.

All the snapWONDERS API services are available over the Clearnet / **Web** and Dark Web **Tor** and
**I2P**.

> **Status.** Initial release (`0.1.0`). Standard library only — zero third-party dependencies.


# See it in action

Hide a secret image inside a cover — the stego output looks pixel-identical, but carries the hidden
file, recoverable only with the password:

| Secret (hidden inside) | Cover (input) | Stego output (carries the secret) |
|---|---|---|
| ![secret](https://raw.githubusercontent.com/snapWONDERS/snapWONDERS-SDK-Go/main/examples/assets/secret.png) | ![cover](https://raw.githubusercontent.com/snapWONDERS/snapWONDERS-SDK-Go/main/examples/sample-output/input.png) | ![stego](https://raw.githubusercontent.com/snapWONDERS/snapWONDERS-SDK-Go/main/examples/sample-output/stego-output.webp) |

The full walkthrough — steganography, forensic analysis (with a real graded result), and conversion —
is in **[`examples/WALKTHROUGH.md`](https://github.com/snapWONDERS/snapWONDERS-SDK-Go/blob/main/examples/WALKTHROUGH.md)**.


# Installation and setup

## snapWONDERS API key
You will need a snapWONDERS API key before you can get started:

* Sign up and create an account at [snapWONDERS sign-up](https://snapwonders.com/sign-up). If you
  wish to create an account via Tor or I2P then you can do so by accessing snapWONDERS through the
  Tor or I2P portals. For the dark web links visit
  [browsing safely](https://snapwonders.com/browsing-safely).
* Under your account settings, generate an API key. New keys start with `sw_`. It is sent as the
  `X-Api-Key` header on every request — keep it secret.

## Install the package

```bash
go get github.com/snapWONDERS/snapWONDERS-SDK-Go
```


# Quickstart

```go
package main

import (
	"fmt"

	snapwonders "github.com/snapWONDERS/snapWONDERS-SDK-Go"
)

func main() {
	client := snapwonders.NewClient("sw_your_key_here") // base URL defaults to https://snapwonders.com

	status, _ := client.Status() // no key needed for status
	fmt.Println(status)

	// Hide a secret inside a cover image (the last file is the cover).
	job, err := client.Stego.Hide([]string{"secret.png", "cover.jpg"}, "Str0ng!Pass")
	if err != nil {
		panic(err)
	}
	files, _ := job.Results()
	for _, f := range files {
		f.Download("out/") // "out/" is a directory — the server filename is kept
	}

	// Reveal it again.
	revealed, _ := client.Stego.Reveal("out/cover-share.avif", "Str0ng!Pass")
	rf, _ := revealed.Results()
	rf[0].Download("recovered/")
}
```

`client.Analyse` (forensic analysis) and `client.Convert` (media conversion) follow the same shape —
one call in, results out:

```go
// Forensic analysis — grade each file A–F, collect the overlay assets it produced.
job, _ := client.Analyse.Run([]string{"photo.jpg"}, snapwonders.WithOption("face_detection", true))
items, _ := job.Results()
for _, item := range items {
	fmt.Println(item.Filename, item.Grade, item.FaceCount)
	for _, asset := range item.Assets { // ELA map, face overlay, …
		asset.Download("out/")
	}
}

// Convert media — set the output format with WithOption.
job, _ = client.Convert.Run([]string{"photo.png"}, snapwonders.WithOption("image_format", "webp"))
out, _ := job.Results()
out[0].Download("out/")
```

Prefer to drive each stage yourself? The step-by-step surface is public too:
`CreateSession()` → `Upload(path, step)` → `WaitForUploads()` → `StartJob(...)` → `Wait()` →
`Results()`.


# Examples

Runnable end-to-end examples live in [`examples/`](https://github.com/snapWONDERS/snapWONDERS-SDK-Go/tree/main/examples) — steganography, forensic analysis,
and conversion, each a self-contained program with a bundled sample image:

```bash
export SNAPWONDERS_API_KEY=sw_your_key_here
go run ./examples/hideandreveal     # hide a file in an image, then reveal it
go run ./examples/analyse           # grade an image A–F + download overlay assets
go run ./examples/convert           # PNG → WebP
```


# Errors

Every failure the SDK returns is a typed error implementing the `SnapwondersError` marker interface,
so you can match on the kind of failure with `errors.As` rather than inspecting HTTP status codes:

```go
files, err := job.Results()
var authErr *snapwonders.AuthError
if errors.As(err, &authErr) {
	// handle a bad/expired API key
}
```

| Error | Returned when |
|-------|---------------|
| `*AuthError` | Missing, malformed, unknown, or revoked API key |
| `*ProRequiredError` | A Pro-only option was used on a free account |
| `*SessionExpiredError` | The 24-hour upload session window has passed |
| `*RateLimitError` | Rate limited — carries `RetryAfter` when the server supplies it |
| `*MaintenanceError` | snapWONDERS is temporarily unavailable for maintenance — carries `RetryAfter` |
| `*JobFailedError` | A job finished as `failed` — carries the server's `Reason` |
| `*TusUploadError` | A resumable-upload step failed |
| `*NetworkError` | The API could not be reached |
| `*APIError` | Any other non-2xx response — carries `StatusCode` and `Body` |


# Running the tests

```bash
go test ./...               # offline unit tests — no API key required
```


# Documentation

Useful documentation can be found at:

* The interactive Swagger UI and full endpoint reference:
  [snapWONDERS API](https://snapwonders.com/api)
* A guided, step-by-step integration walkthrough:
  [snapWONDERS Developers](https://snapwonders.com/developers)


# Contact

## For security concerns
If you have spotted any security concerns then please reach out via
[contacting snapWONDERS](https://snapwonders.com/contact) and set the subject to
**"SECURITY CONCERNS"** and provide the information about your concerns. If you wish to contact via
Tor or I2P then you can do so by accessing snapWONDERS through the Tor or I2P portals. For the dark
web links visit [browsing safely](https://snapwonders.com/browsing-safely).

## For FAQ and questions
It may be that your question is already answered in the [FAQ](https://snapwonders.com/faq). Be sure
to check the FAQ content first. Otherwise you may reach out via
[contacting snapWONDERS](https://snapwonders.com/contact).

## For contacting the author
Use this link to contact the author [Kenneth Springer](https://kennethbspringer.au/).


# Licence

MIT — Copyright (c) 2026 Kenneth Springer @ snapWONDERS. See [LICENSE](https://github.com/snapWONDERS/snapWONDERS-SDK-Go/blob/main/LICENSE).

**Scope.** The MIT licence covers **this client library only**. It grants no rights in the
snapWONDERS API, service, data, models, or algorithms it communicates with — these are **proprietary**
and remain the property of Kenneth Springer @ snapWONDERS. This library only sends HTTP requests to
the API; it contains none of its implementation. Using the API requires a valid API key and is
governed by the [snapWONDERS Terms of Service](https://snapwonders.com/terms).
