# Changelog

All notable changes to the `snapwonders` Go client are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2026

Initial release.

- Official Go client for the snapWONDERS API, covering all three product areas: `client.Stego`
  (hide & reveal), `client.Analyse` (forensic media analysis), and `client.Convert` (media
  conversion).
- Resumable upload and the session → job → poll → download flow wrapped internally, so a whole job
  is a single call (e.g. `client.Stego.Hide([]string{...}, "password")`). One-shot helpers plus
  step-by-step session/job control.
- Polling backs off with jitter and honours a server-supplied poll interval, to stay light under load.
- Typed errors implementing the `SnapwondersError` marker: `AuthError`, `ProRequiredError`,
  `SessionExpiredError`, `RateLimitError`, `MaintenanceError`, `JobFailedError`, `TusUploadError`,
  `NetworkError`, `APIError`.
- Zero third-party dependencies — standard library only.
