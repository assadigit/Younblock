#!/usr/bin/env bash
# build-offline-pack.sh — assembles the two extra release bundles:
#
#   dist/younblock-setup_<ver>.zip        the three setup scripts,
#                                         for downloading on a fresh PC
#   dist/Younblock-offline_<ver>.zip      the everything-inside pack:
#                                         source + vendored Go deps +
#                                         Windows node_modules + the Go
#                                         toolchain + Node.js + the
#                                         Wails CLI — zero internet
#
# Run on Linux after build.sh, with the release tag checked out (the
# source in the pack is `git archive` of the tag, so the pack can never
# drift from what was released).
set -euo pipefail
cd "$(dirname "$0")"

VER="$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/config/defaults.go)"
TAG="$(git describe --tags --abbrev=0)"
GOZIP=go1.27.2.windows-amd64.zip
NODEVER=v22.23.3
NODEZIP="node-$NODEVER-win-x64.zip"
CACHE="$HOME/.cache/younblock"
mkdir -p "$CACHE" dist

echo "=== offline pack builder (app $VER, tag $TAG) ==="

echo "[1/6] staging the source (git archive $TAG)..."
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
mkdir -p "$STAGE/Younblock"
git archive "$TAG" | tar -x -C "$STAGE/Younblock"
[ -f "$STAGE/Younblock/vendor/modules.txt" ] || {
  echo "the tag has no vendor/ directory - vendor first (go mod vendor)" >&2
  exit 1
}

echo "[2/6] installing the Windows node_modules (npm ci --os=win32)..."
(
  cd "$STAGE/Younblock/frontend"
  npm ci --os=win32 --cpu=x64 --no-audit --no-fund >/dev/null
)
# The Windows native binding must be present (vite 8 / rolldown).
[ -d "$STAGE/Younblock/frontend/node_modules/@rolldown/binding-win32-x64-msvc" ] || {
  echo "the win32 rolldown binding is missing - the pack would not build on Windows" >&2
  exit 1
}
# wails skips "npm install" when this committed marker matches package.json
# (the marker file holds the md5 hex twice; compare its first 32 chars).
MD5_NOW="$(md5sum "$STAGE/Younblock/frontend/package.json" | cut -d' ' -f1)"
MD5_OLD="$(head -c 32 "$STAGE/Younblock/frontend/package.json.md5" 2>/dev/null)"
if [ "$MD5_NOW" != "$MD5_OLD" ]; then
  echo "frontend/package.json.md5 does not match package.json ($MD5_OLD vs $MD5_NOW)" >&2
  echo "regenerate it (run any wails command once, or fix it) and re-tag" >&2
  exit 1
fi

echo "[3/6] cross-compiling the Wails CLI (windows/amd64, app's pinned version)..."
mkdir -p "$STAGE/Younblock/tools"
GOOS=windows GOARCH=amd64 go build -mod=mod -trimpath -ldflags "-s -w" \
  -o "$STAGE/Younblock/tools/wails.exe" github.com/wailsapp/wails/v2/cmd/wails

echo "[4/6] fetching the toolchain zips (cached in $CACHE)..."
fetch_cached() {
  # fetch_cached <output> <url>... — first mirror that works wins
  local out="$1"; shift
  [ -s "$out" ] && return 0
  local u
  for u in "$@"; do
    echo "  trying $u"
    if curl -fsSL --connect-timeout 20 -o "$out.part" "$u"; then
      mv "$out.part" "$out"
      return 0
    fi
    rm -f "$out.part"
  done
  echo "every mirror failed for $(basename "$out")" >&2
  return 1
}
fetch_cached "$CACHE/$GOZIP" \
  "https://mirrors.aliyun.com/golang/$GOZIP" \
  "https://go.dev/dl/$GOZIP"
fetch_cached "$CACHE/$NODEZIP" \
  "https://nodejs.org/dist/$NODEVER/$NODEZIP" \
  "https://registry.npmmirror.com/-/binary/node/$NODEVER/$NODEZIP"
cp "$CACHE/$GOZIP" "$STAGE/Younblock/tools/$GOZIP"
cp "$CACHE/$NODEZIP" "$STAGE/Younblock/tools/$NODEZIP"

echo "[5/6] zipping..."
SETUPZIP="dist/younblock-setup_${VER}.zip"
OFFZIP="dist/Younblock-offline_${VER}.zip"
rm -f "$SETUPZIP" "$OFFZIP"
python3 - "$STAGE" "$SETUPZIP" "$OFFZIP" "$VER" <<'PY'
import os, sys, zipfile, time

stage, setup_zip, off_zip, ver = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]

def add(z, path, arc):
    z.write(path, arc, compress_type=zipfile.ZIP_DEFLATED)

# The setup bundle: the three scripts + a README, straight from the tag.
with zipfile.ZipFile(setup_zip, "w", zipfile.ZIP_DEFLATED) as z:
    src = os.path.join(stage, "Younblock")
    for name in ("setup-dev.bat", "setup-build.bat", "setup.ps1"):
        add(z, os.path.join(src, name), name)
    readme = (
        "Younblock setup - quick start\n"
        "=============================\n"
        "\n"
        "1. Extract ALL of this zip's files into one folder.\n"
        "2. Double-click setup-dev.bat to run the app live (wails dev),\n"
        "   or setup-build.bat to build the exe files.\n"
        "\n"
        "Keep the three files together - the .bat files call setup.ps1.\n"
        "\n"
        "Everything is installed automatically (Git, Go, Node.js, WebView2,\n"
        "the Wails CLI), with mirrors when the usual download sites are\n"
        "unreachable. When GitHub asks you to log in: the repository is\n"
        "private, so approve the browser login (or paste a GitHub token\n"
        "when the script offers).\n"
        "\n"
        "No internet at all? Use the Younblock-offline zip from the same\n"
        "Releases page instead.\n"
    )
    z.writestr("README-FIRST.txt", readme)

# The offline pack: the whole staged tree (source + vendor + node_modules
# + tools). Compress level 6 - the toolchain zips are already compressed;
# only pass those through with store to save time.
with zipfile.ZipFile(off_zip, "w") as z:
    for root, dirs, files in os.walk(stage):
        dirs.sort()
        for f in sorted(files):
            full = os.path.join(root, f)
            arc = os.path.relpath(full, stage)
            if f.endswith(".zip") or f.endswith(".exe"):
                z.write(full, arc, compress_type=zipfile.ZIP_STORED)
            else:
                z.write(full, arc, compress_type=zipfile.ZIP_DEFLATED)
print("zips written")
PY

echo "[6/6] done:"
ls -lh "$SETUPZIP" "$OFFZIP" | awk '{print "  " $NF "  " $5}'
echo "SHA-256:"
sha256sum "$SETUPZIP" "$OFFZIP"
