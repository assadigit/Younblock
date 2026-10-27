#!/usr/bin/env bash
# build-lite-desktop.sh — the lite desktop edition's lane
# (younblock-lite-desktop, DECISIONS.md D114): the dependency-free
# Windows shell — the same engine as every edition behind a plain
# Win32 window instead of the WebView2 screen.
#
# The whole lane needs ONLY Go: no Node, no npm, no WebView2, no wails
# CLI, no webkit stack — the lite edition embeds no web bundle at all
# (its UI is GDI). That is the point: the exe that ships is one file
# with no runtime beside it, and the lane that builds it has one
# prerequisite. Cross-built from Linux exactly like the headless
# engines in build.sh; on a Windows machine the same build is one
# `go build` (build-lite-desktop.bat).
#
# The exe icon and version info live in the committed
# rsrc_windows_{amd64,386}.syso — go build links them automatically;
# scripts/gen_lite_resources.py regenerates them when the version
# moves (or the icon changes).
#
# Outputs in dist/:
#   younblock-lite-desktop.exe       64-bit
#   younblock-lite-desktop-x86.exe   32-bit (the old-32-bit-Windows box)
set -euo pipefail
cd "$(dirname "$0")"

VER="$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/config/defaults.go)"
if [ -z "$VER" ]; then
  echo "could not read the version from internal/config/defaults.go" >&2
  exit 1
fi

echo "=== Younblock lite desktop build (v$VER) ==="

# -H windowsgui: a GUI app (the console subsystem would flash a
# console window on double-click); -s -w -trimpath: the same release
# shape every other lane ships.
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -H windowsgui" \
  -o dist/younblock-lite-desktop.exe ./cmd/younblock-lite
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -H windowsgui" \
  -o dist/younblock-lite-desktop-x86.exe ./cmd/younblock-lite

echo "Done. Output files:"
for f in dist/younblock-lite-desktop.exe dist/younblock-lite-desktop-x86.exe; do
  [ -f "$f" ] || continue
  printf '  %s  (%s)\n' "$PWD/$f" "$(sha256sum "$f" | cut -c1-16)…"
done
sha256sum dist/younblock-lite-desktop.exe dist/younblock-lite-desktop-x86.exe
