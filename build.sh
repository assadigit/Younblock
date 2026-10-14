#!/usr/bin/env bash
# build.sh — builds every Younblock version from this one source tree.
# Run on Linux (the Windows exes are cross-built; only the Wails GUI parts
# need a Windows machine if you insist on building there — build.bat).
#
# Outputs in dist/:
#   younblock.exe            Windows app, 64-bit
#   younblock-x86.exe        Windows app, 32-bit
#   younblock-core.exe       headless engine, 64-bit
#   younblock-core-x86.exe   headless engine, 32-bit
#   younblock-linux-amd64    the single-executable Linux edition
#   younblock-linux-386      the same, 32-bit (old 32-bit Mint)
#
# Every target embeds the SAME frontend build, so all versions ship the
# same screen (D50: one source of truth — nothing to keep in sync by hand).
set -euo pipefail
cd "$(dirname "$0")"

echo "=== Younblock build (all versions) ==="

echo "[1/5] Building the frontend (shared by every version)..."
( cd frontend && npm ci --no-audit --no-fund >/dev/null 2>&1 || npm install --no-audit --no-fund >/dev/null 2>&1 )
( cd frontend && npm run build )
# go:embed cannot reach outside its package, so the bundle is copied in
# (placeholder.txt stays; the copied files are git-ignored).
mkdir -p internal/webui/dist
cp -r frontend/dist/. internal/webui/dist/

echo "[2/5] Building the Linux single-executables..."
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-linux-amd64 ./cmd/core
CGO_ENABLED=0 GOOS=linux GOARCH=386  go build -trimpath -ldflags "-s -w" -o dist/younblock-linux-386  ./cmd/core

echo "[3/5] Building the headless engines for Windows..."
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
GOOS=windows GOARCH=386  go build -trimpath -ldflags "-s -w" -o dist/younblock-core-x86.exe ./cmd/core

echo "[4/5] Building the Windows apps (Wails cross-build)..."
if command -v wails >/dev/null 2>&1; then
  wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
  cp build/bin/younblock.exe dist/younblock.exe
  wails build -platform windows/386 -trimpath -ldflags "-s -w" -o younblock-x86.exe
  cp build/bin/younblock-x86.exe dist/younblock-x86.exe
else
  echo "  wails CLI not found — skipped the Windows GUI apps."
  echo "  (go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0, then re-run)"
fi

echo "[5/5] Done. Output files:"
for f in dist/younblock.exe dist/younblock-x86.exe dist/younblock-core.exe dist/younblock-core-x86.exe dist/younblock-linux-amd64 dist/younblock-linux-386; do
  [ -f "$f" ] || continue
  printf '  %s  (%s)\n' "$PWD/$f" "$(sha256sum "$f" | cut -c1-16)…"
done
echo "SHA-256 checksums (full):"
for f in dist/younblock.exe dist/younblock-x86.exe dist/younblock-core.exe dist/younblock-core-x86.exe dist/younblock-linux-amd64 dist/younblock-linux-386; do
  [ -f "$f" ] || continue
  sha256sum "$f"
done
