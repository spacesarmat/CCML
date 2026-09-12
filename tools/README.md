# Managed FFmpeg tools

Do not commit FFmpeg binaries to this repository.

For local Windows development:

```powershell
.\scripts\fetch-ffmpeg.ps1
```

For local macOS development:

```bash
./scripts/fetch-ffmpeg.sh
```

Release builds fetch a pinned, checksum-verified pair automatically and package it with CCML. Runtime updates are stored in the user's CCML application-data directory.
