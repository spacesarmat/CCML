#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-tools}"
RELEASE_TAG="${2:-n8.1.2-1}"
REPO="shaka-project/static-ffmpeg-binaries"
if [[ "$RELEASE_TAG" == "latest" ]]; then
  API="https://api.github.com/repos/${REPO}/releases/latest"
else
  API="https://api.github.com/repos/${REPO}/releases/tags/${RELEASE_TAG}"
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script is for macOS. Use scripts/fetch-ffmpeg.ps1 on Windows." >&2
  exit 1
fi
case "$(uname -m)" in
  arm64) suffix="osx-arm64" ;;
  x86_64) suffix="osx-x64" ;;
  *) echo "Unsupported macOS architecture: $(uname -m)" >&2; exit 1 ;;
esac

mkdir -p "$OUT_DIR"
release_json="$(mktemp)"
trap 'rm -f "$release_json"' EXIT
curl --fail --location --silent --show-error \
  -H 'Accept: application/vnd.github+json' \
  -H 'User-Agent: CCML-build/0.2' \
  "$API" > "$release_json"

python3 - "$release_json" "$OUT_DIR" "$suffix" <<'PY'
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys
import urllib.request

release_path, out_dir, suffix = sys.argv[1:]
with open(release_path, 'r', encoding='utf-8') as f:
    release = json.load(f)
assets = {item['name']: item for item in release.get('assets', [])}
digests = {}
for tool in ('ffmpeg', 'ffprobe'):
    name = f'{tool}-{suffix}'
    asset = assets.get(name)
    if not asset:
        raise SystemExit(f'release {release.get("tag_name")} does not contain {name}')
    digest = asset.get('digest') or ''
    if not digest.startswith('sha256:'):
        raise SystemExit(f'asset {name} has no SHA-256 digest')
    req = urllib.request.Request(asset['browser_download_url'], headers={'User-Agent': 'CCML-build/0.2'})
    target = pathlib.Path(out_dir) / tool
    temp = target.with_suffix('.download')
    h = hashlib.sha256()
    try:
        with urllib.request.urlopen(req, timeout=600) as response, open(temp, 'wb') as out:
            while True:
                chunk = response.read(1024 * 1024)
                if not chunk:
                    break
                h.update(chunk)
                out.write(chunk)
        actual = h.hexdigest()
        expected = digest.removeprefix('sha256:').lower()
        if actual != expected:
            raise SystemExit(f'SHA-256 mismatch for {name}: got {actual} expected {expected}')
        os.chmod(temp, 0o755)
        os.replace(temp, target)
        digests[tool] = digest
    finally:
        if temp.exists():
            temp.unlink()

first_line = subprocess.check_output([str(pathlib.Path(out_dir) / 'ffmpeg'), '-version'], text=True).splitlines()[0]
match = re.match(r'^ffmpeg version\s+(\S+)', first_line)
version = match.group(1) if match else release['tag_name']
manifest = {
    'version': version,
    'updateId': f'shaka-project/static-ffmpeg-binaries:{release["tag_name"]}:{digests["ffmpeg"]}:{digests["ffprobe"]}',
    'dir': '',
}
(pathlib.Path(out_dir) / 'CCML-FFMPEG-MANIFEST.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
print(version)
PY

cp third_party/ffmpeg/NOTICE.md "$OUT_DIR/FFMPEG-NOTICE.md"
cp third_party/ffmpeg/COPYING.GPLv3 "$OUT_DIR/COPYING.GPLv3"
echo "FFmpeg prepared in $OUT_DIR"
