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
- optional regex rename pass;
- safe move/rename with SQLite path update and rollback attempt;
- React + TypeScript Wails UI;
- GitHub Actions CI/build/release workflows.

## Runtime dependencies

### Required: FFmpeg + ffprobe

CCML intentionally uses FFmpeg rather than implementing codecs/DSP in Go. Install a recent FFmpeg build and ensure `ffmpeg` and `ffprobe` are in `PATH`, or set:

```text
CCML_FFMPEG=/path/to/ffmpeg
CCML_FFPROBE=/path/to/ffprobe
```

The FFmpeg build should include `adeclip`, `mcompand`, `alimiter`. Pitch shift additionally needs the `rubberband` filter (`--enable-librubberband`).

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
- FFmpeg

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

The current pitch function is a high-quality semitone pitch shift through FFmpeg/libRubberBand. It is **not** automatic vocal Auto-Tune or note-by-note pitch correction. That requires a dedicated pitch detector/correction engine and is intentionally a separate future DSP module.

The multiband compressor currently uses FFmpeg's documented `mcompand` multi-band profile and is opt-in. Treat it as a starting preset, not a universal mastering profile. Production audio software should expose presets, A/B preview, headroom safeguards and regression tests against reference audio.

## Tests

```bash
go test ./...
```

Unit tests currently cover duplicate grouping, rename/template sanitization, processing defaults, FFmpeg loudness JSON extraction, and filter construction.

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
2. Metadata candidate merge/scoring and writing tags/artwork back to files.
3. Credential UI + OAuth/PKCE token lifecycle for authenticated providers.
4. Batch jobs with progress/cancel/resume and bounded worker queues.
5. Dry-run/batch organizer with collision policy and undo journal.
6. Waveform/A-B audio preview and preset management.
7. Automatic clipping statistics and per-track DSP decision engine.
8. Signed/notarized Windows/macOS installers and bundled FFmpeg licensing review.
