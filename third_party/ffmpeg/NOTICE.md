# FFmpeg distribution notice

CCML invokes FFmpeg and ffprobe as separate executables. Release packages may include prebuilt GPL FFmpeg binaries for the target platform.

## Windows

The pinned bootstrap build is produced by `BtbN/FFmpeg-Builds`:

- Build release: `autobuild-2026-09-10-15-31`
- Build scripts: https://github.com/BtbN/FFmpeg-Builds
- Release archive: https://github.com/BtbN/FFmpeg-Builds/releases/tag/autobuild-2026-09-10-15-31

The selected Windows GPL build includes the audio filters/codecs CCML requires, including librubberband support used by CCML's pitch-shift option.

## macOS

The pinned bootstrap build is produced by `shaka-project/static-ffmpeg-binaries`:

- Build release: `n8.1.2-1` (FFmpeg n8.1.2)
- Build scripts and dependency versions: https://github.com/shaka-project/static-ffmpeg-binaries/tree/n8.1.2-1
- Release: https://github.com/shaka-project/static-ffmpeg-binaries/releases/tag/n8.1.2-1

Native Intel and Apple Silicon binaries are supported. This build does not include librubberband; CCML therefore treats high-quality RubberBand pitch shifting as an optional runtime capability on macOS.

## License and source

- FFmpeg project: https://ffmpeg.org/
- FFmpeg source: https://github.com/FFmpeg/FFmpeg

The bundled binaries are GPL builds and are separate executables from CCML's Go/React application. The GPLv3 license text is distributed alongside the binaries as `COPYING.GPLv3`.

CCML verifies the SHA-256 digest published by the GitHub release before bundling or activating an automatically downloaded toolchain. Runtime updates are stored in the user's CCML configuration directory rather than modifying the installed application bundle.

Before public redistribution, review the corresponding-source and third-party licensing obligations for the exact FFmpeg build shipped with that CCML release.
