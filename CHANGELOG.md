# Changelog

All notable changes to the `snapwonders` Go client are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] — 2026-08-16

### Added
- **Direct upload.** Files at or under the server's cap (95 MB by default) now go up in a single
  `POST /api/upload` instead of the TUS create/PATCH sequence — one round trip instead of three,
  which matters most over Tor and I2P.
- `UploadError` for direct-upload failures, and `UploadFailure`, an interface both it and
  `TusUploadError` satisfy. Which upload path runs is chosen by file size, so matching on one
  concrete type would mean matching one that only occurs for some file sizes:

      var uf snapwonders.UploadFailure
      if errors.As(err, &uf) { ... }

- `DefaultMaxUploadBytes`, mirroring the server's own fallback.

### Changed
- `AnalyseSession`, `ConvertSession` and `StegoSession` gain `MaxUploadBytes`, read from the
  session-create response, so the cap can move server-side without an SDK release.
- `Session.Upload` returns the server's `storage_uid` for direct uploads and the TUS path for large
  ones. Previously it was always the TUS path. Neither value is needed for the normal flow —
  uploads are confirmed with `Files`/`WaitForUploads` and jobs are keyed by `UploadUID`.

### Unchanged
- The TUS implementation is untouched. TUS remains the path for files over the cap, and is still
  what handles resumption on an unreliable connection.
- Still standard library only — zero third-party dependencies.

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
