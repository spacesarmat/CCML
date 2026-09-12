# Architecture

CCML keeps Wails bindings deliberately thin. UI calls `App`; `App` delegates to services under `internal/`.

- `store`: single-owner SQLite persistence layer.
- `audio.Probe`: one ffprobe JSON parser for all supported formats.
- `library.Scanner`: bounded parallel probing, serialized SQLite writes.
- `library.FindDuplicates`: metadata + duration duplicate strategy; designed to be replaced/augmented by fingerprints.
- `metadata.Service`: provider fan-out with non-fatal per-provider warnings.
- `audio.Processor`: no shell execution; arguments are passed directly to `exec.CommandContext`.
- `organize.Service`: safe path rendering, invalid filename sanitization, non-overwriting moves, DB rollback attempt.

External APIs and DSP tools are adapters, not domain dependencies. This keeps the SQLite/library code testable without network or codecs.
