# CCML — Cross-platform Music Library

Wails + React/TypeScript desktop application for Windows and macOS. The backend is idiomatic Go and keeps a local SQLite media library.

## Current MVP

- recursive scanning of MP3, FLAC, M4A/AAC, WAV, AIFF, OGG;
- technical metadata and tags through `ffprobe`;
- SQLite media library (pure-Go `modernc.org/sqlite` driver);
- probable duplicate detection by normalized artist/title + duration tolerance;
- metadata lookup through MusicBrainz, TheAudioDB and Deezer; optional authenticated adapters for Discogs, Spotify, Apple Music, YouTube Data API and SoundCloud;
- EBU R128 / LUFS analysis;
- ReplayGain-style metadata writing without audio re-encoding;
- rendering/mastering chain: pre-gain → clipping repair → optional multiband compression → optional pitch shift → two-pass loudness normalization → limiter;
- optional BPM/Key analysis via Essentia;
- mp3tag-style `%artist%/%album%/%track% - %title%` rename templates;
- direct tag editing for MP3, FLAC, M4A/AAC, WAV, AIFF and OGG, including batch partial edits and embedded cover art;
- metadata change history with Undo and provider-candidate application;
- optional regex rename pass;
- safe move/rename with SQLite path update and rollback attempt;
- React + TypeScript Wails UI;
- GitHub Actions CI/build/release workflows.

## Runtime dependencies

### Managed FFmpeg + ffprobe

CCML uses FFmpeg for decoding, probing and DSP, but end users do **not** need to install FFmpeg separately. Windows/macOS release packages bundle `ffmpeg` and `ffprobe`. CCML discovers tools in this order:

1. explicit `CCML_FFMPEG` + `CCML_FFPROBE` overrides;
2. a SHA-256 verified CCML-managed update in the user's configuration directory;
3. the binaries bundled with the application;
4. system `PATH` as a development fallback.

When no usable pair exists CCML downloads a supported build automatically. When tools are already available, the application checks for updates at most once every seven days. The UI also has an **Update FFmpeg** action. A candidate update is activated only after its published SHA-256 digest is verified and CCML confirms required filters/encoders are present. A failed update does not replace a working toolchain.

Windows uses GPL builds from `BtbN/FFmpeg-Builds` (x64/ARM64). macOS uses native Intel/Apple Silicon static builds from `shaka-project/static-ffmpeg-binaries`. Distribution notices and GPLv3 text live in `third_party/ffmpeg/` and are copied next to bundled binaries.

For local/offline development you can prefetch the pinned bootstrap binaries:

```powershell
# Windows, from the project root
.\scripts\fetch-ffmpeg.ps1
```

```bash
# macOS, from the project root
./scripts/fetch-ffmpeg.sh
```

Environment overrides remain available for debugging/custom builds:

```text
CCML_FFMPEG=/path/to/ffmpeg
CCML_FFPROBE=/path/to/ffprobe
```

The core mastering chain requires `loudnorm`, `adeclip`, `mcompand` and `alimiter`. RubberBand pitch shifting is available when the selected FFmpeg build exposes the `rubberband` filter. The bundled Windows GPL build includes it; the current native macOS bootstrap build does not, so pitch shifting is treated as an optional capability there.

### Optional: Essentia

For BPM and Key analysis install `essentia_streaming_extractor_music`, or set:

```text
CCML_ESSENTIA=/path/to/essentia_streaming_extractor_music
```

## Development prerequisites

- Go 1.27+
- Node.js 22+
- React 19.3 / TypeScript 7 / Vite 8 (pinned in `frontend/package.json`)
- Wails v2.15.x (stable v2 line used by this project)
- FFmpeg is downloaded/bundled by CCML (or run the platform fetch script for local development)

Install Wails:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor
```

Install frontend dependencies and run:

```bash
cd frontend
npm install
cd ..
wails dev
```

Production build:

```bash
wails build -clean
```

### TypeScript / CSS imports

`frontend/src/vite-env.d.ts` includes `vite/client` types. Keep this file in the project: Vite uses those declarations for CSS and other static asset imports, including `import './styles.css'`.

## Go module / GitHub repository

This project is configured for the repository `github.com/spacesarmat/CCML`. The same module path is used in `go.mod`, internal imports and the MusicBrainz `User-Agent`.

If the project directory is not yet a Git checkout, initialise and publish it with:

```bash
git init
git add .
git commit -m "Initial CCML Wails application"
git branch -M main
git remote add origin https://github.com/spacesarmat/CCML.git
git push -u origin main
```

If `origin` already exists, verify it with `git remote -v`; change it with `git remote set-url origin https://github.com/spacesarmat/CCML.git`.

## GoLand

Open the **project root** (the directory containing `go.mod`, `wails.json`, `app.go` and `frontend/`) in GoLand. Do not open only the `frontend` directory.

1. In **Settings | Go | GOROOT**, select your installed Go SDK (Go 1.22+; the project currently declares Go 1.27).
2. Open GoLand's built-in Terminal in the project root and run `go mod tidy`.
3. Install Wails once with `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`. Make sure your Go bin directory is present in `PATH`.
4. In `frontend/`, run `npm install`, then `npm run build`. The file `frontend/src/vite-env.d.ts` must remain in the project so TypeScript recognises CSS imports.
5. Return to the project root and run `wails doctor`, then `wails dev`.

For a production build use `wails build -clean`. Generated executables are written under `build/bin/`.

GoLand should detect `go.mod` automatically. `.idea/` remains ignored intentionally; personal IDE metadata should not be committed to GitHub.

## External metadata providers

Enabled without extra credentials:

- MusicBrainz — one request/second and an identifying `User-Agent`;
- TheAudioDB — `THEAUDIODB_API_KEY`, falling back to the documented public key;
- Deezer catalog search (subject to the developer terms/account applicable to your deployment).

Adapters included and enabled automatically when their environment variable is present:

- Discogs — `DISCOGS_TOKEN`;
- Spotify — `SPOTIFY_ACCESS_TOKEN`, optional `SPOTIFY_MARKET`; for a desktop product obtain tokens with OAuth Authorization Code + PKCE instead of embedding a client secret;
- Apple Music — `APPLE_MUSIC_DEVELOPER_TOKEN`, optional `APPLE_MUSIC_STOREFRONT`;
- YouTube Data API — `YOUTUBE_API_KEY` (used as the supported public approximation for YouTube Music discovery);
- SoundCloud — `SOUNDCLOUD_ACCESS_TOKEN`.

The backend uses a `metadata.Provider` interface, so provider-specific auth can later move into the UI without changing the library domain. Yandex Music, Beatport and Traxsource are intentionally **not scraped** in this MVP: add them only through an allowed/stable API or partner access route.

## Audio processing notes

The processing output is a copy by default (`CCML Processed/<filename>`). If an output path is explicitly set to the original path, CCML renders to a temporary file and replaces only after FFmpeg succeeds. `KeepOriginal` creates a `.ccml-backup` before replacement.

The current pitch function is a semitone pitch shift through FFmpeg/libRubberBand when that optional filter is available. It is **not** automatic vocal Auto-Tune or note-by-note pitch correction. That requires a dedicated pitch detector/correction engine and is intentionally a separate future DSP module.

The multiband compressor currently uses FFmpeg's documented `mcompand` multi-band profile and is opt-in. Treat it as a starting preset, not a universal mastering profile. Production audio software should expose presets, A/B preview, headroom safeguards and regression tests against reference audio.

## Tests

```bash
go test ./...
```

Unit tests cover duplicate grouping, rename/template sanitization, processing defaults, FFmpeg loudness JSON extraction/filter construction, updater platform selection, SHA-256 verification, Windows archive extraction, and partial tag-edit validation.

## Project layout

```text
.
├── app.go
├── main.go
├── internal/
│   ├── audio/       # ffprobe, loudness/DSP pipeline, Essentia adapter
│   ├── library/     # scanning and duplicate detection
│   ├── metadata/    # provider aggregation + MusicBrainz/Discogs/Spotify/etc.
│   ├── model/       # domain/Wails DTOs
│   ├── organize/    # templates, regex rename, safe move
│   └── store/       # SQLite schema/repository
├── frontend/        # React + TypeScript + Vite
└── .github/workflows/
```

## Next production milestones

1. Chromaprint/AcoustID fingerprint duplicates and identification.
2. Metadata candidate merge/scoring, field-by-field candidate comparison, and richer release/label/catalog-number fields.
3. Credential UI + OAuth/PKCE token lifecycle for authenticated providers.
4. Batch jobs with progress/cancel/resume and bounded worker queues.
5. Dry-run/batch organizer with collision policy and undo journal.
6. Waveform/A-B audio preview and preset management.
7. Automatic clipping statistics and per-track DSP decision engine.
8. Signed/notarized Windows/macOS installers; sign/notarize after FFmpeg is placed in the final application bundle.

## Stage 2: incremental library scanning

Stage 2 makes the local media library safe to rescan repeatedly:

- unchanged files are detected by absolute path, file size and modification time and skip `ffprobe`;
- new and changed files are probed and upserted into SQLite;
- missing files are removed from the database only after a complete, non-cancelled scan;
- an active scan can be cancelled from the Wails UI;
- managed library roots and their last successful scan time are stored in SQLite;
- the UI shows live scan counters plus track/artist/album/duration/size statistics;
- `schema_version`, `library_roots` and `scan_entries` are created automatically when an existing database is opened.

No audio file is deleted by the Stage 2 cleanup. Removing a library root with database cleanup deletes only CCML database rows.

After updating an existing checkout, run:

```powershell
go mod tidy
cd frontend
npm install
cd ..
wails dev
```

`wails dev` regenerates `frontend/wailsjs` bindings for the new `CancelScan`, `ListLibraryRoots`, `RemoveLibraryRoot` and `LibraryStatistics` methods.


## Managed FFmpeg update design

Release builds pin a known bootstrap build for reproducibility. Runtime update checks use the platform provider's current supported release and verify GitHub's published SHA-256 digest before installing. Downloaded tools are validated with `ffprobe -version`, `ffmpeg -filters` and `ffmpeg -encoders` before `current.json` is atomically switched to the new version.

Managed updates are stored under the user's CCML configuration directory (`.../CCML/tools/ffmpeg/`) instead of modifying the installed `.exe` or macOS `.app`. This keeps application signing/notarization boundaries intact and makes rollback possible by retaining versioned tool directories.

For `.ogg` processing CCML encodes Opus in the OGG container (`libopus`) so the same processing path works with both selected Windows and macOS bootstrap builds. Existing OGG/Vorbis files remain readable and scannable.

## Interface languages

CCML supports English and Russian UI languages without an external i18n dependency.

- The first launch follows the OS/WebView language (`ru-*` selects Russian; other locales select English).
- Use the `RU / EN` switch in the top bar to change the language immediately.
- The selected language is saved locally in `localStorage` under `ccml.language`.
- Dates and numeric grouping are formatted using the selected locale.
- Technical error output returned by FFmpeg, ffprobe, Essentia, operating-system APIs, or remote metadata services is intentionally kept verbatim for diagnostics.

Translations live in `frontend/src/i18n.ts`. Add new user-facing strings there instead of hard-coding them in React components.


## Stage 3: tag editor and Undo

Stage 3 adds an Mp3tag-style metadata editor backed by `github.com/tommyo123/mtag` v1.0.2. The dependency is MIT licensed; its notice is retained in `third_party/mtag/`.

- select one or many tracks directly in the library table;
- edit Title, Artist, Album, Album Artist, Genre, Composer, Comment, Year, Track/Total and Disc/Total;
- batch editing is partial: only explicitly enabled fields are changed, so mixed values on other selected tracks are preserved;
- preview before writing shows before/after values;
- changes are written to the audio file first and then synchronized to SQLite;
- JPEG/PNG front cover replacement and removal are supported; unrelated embedded pictures are preserved when removing the front cover;
- MusicBrainz/Discogs/Deezer/etc. candidates can be applied to a selected file, optionally including provider artwork;
- every successful edit creates a reversible change set in SQLite; cover bytes are backed up only when the cover actually changes;
- Undo restores text tags and the previous front cover;
- tag history is stored under the CCML user configuration directory, not beside the music files.

After installing this update, run:

```powershell
go mod tidy
go test ./...
cd frontend
npm run build
cd ..
wails dev
```

`wails dev` regenerates Wails bindings for `ReadTrackTags`, `PreviewTagEdits`, `ApplyTagEdits`, cover-art actions, metadata application and Undo/history methods.
