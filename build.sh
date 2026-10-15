#!/usr/bin/env bash
# build.sh — builds every Younblock edition from this one source tree.
# Run on Linux (the Windows exes are cross-built; only the Wails GUI parts
# need a Windows machine if you insist on building there — build.bat).
#
# Since v0.7.4 every platform ships TWO editions (D66):
#   * younblock — the redesigned connect screen (v0.7.0+)
#   * younblock-classic — the simple pre-redesign UI (v0.6.0), for the
#     small-download / familiar-look crowd
# The frontend picks its screen through VITE_UI_VARIANT at build time and
# the shell picks its window size through -X main.windowSize.
#
# Outputs in dist/:
#   younblock.exe                 Windows app, 64-bit (new screen)
#   younblock-x86.exe             Windows app, 32-bit (new screen)
#   younblock-classic.exe         Windows app, 64-bit (classic screen)
#   younblock-classic-x86.exe     Windows app, 32-bit (classic screen)
#   younblock-core.exe            headless engine, 64-bit
#   younblock-core-x86.exe        headless engine, 32-bit
#   younblock-linux-amd64         single-executable web edition, 64-bit
#   younblock-linux-386           the same, 32-bit (old 32-bit Mint)
#   younblock_<ver>_amd64.deb     Linux Mint installer — NATIVE window, 64-bit
#   younblock-classic_<ver>_amd64.deb  the same, classic screen
#   younblock_<ver>_i386.deb      web edition, 32-bit
#   Younblock-<ver>-x86_64.AppImage     portable NATIVE window edition, 64-bit
#   Younblock-Classic-<ver>-x86_64.AppImage  the same, classic screen
#   Younblock-<ver>-i686.AppImage  web edition, 32-bit
#   Younblock-<ver>-x86_64.flatpak (when flatpak tools exist)
#
# Every package wraps the SAME engine; the web (browser) editions carry
# both screens switchable with ?ui=classic (D66), the desktop editions
# bake theirs in — one source of truth, nothing kept in sync by hand
# (D50/D52/D53).
set -euo pipefail
cd "$(dirname "$0")"

VER="$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/config/defaults.go)"
if [ -z "$VER" ]; then
  echo "could not read the version from internal/config/defaults.go" >&2
  exit 1
fi

echo "=== Younblock build (all versions, v$VER) ==="

echo "[1/8] Building the frontend (both screens ship; pro is the default)..."
# The lockfile is content-synced; @sveltejs/vite-plugin-svelte just hasn't
# widened its peer range to vite 8 yet, so strict npm ci refuses — the
# legacy-peer-deps retry is the documented escape hatch.
( cd frontend && npm ci --no-audit --no-fund >/dev/null 2>&1 \
  || npm ci --no-audit --no-fund --legacy-peer-deps >/dev/null 2>&1 \
  || npm install --no-audit --no-fund --legacy-peer-deps >/dev/null 2>&1 )
( cd frontend && npm run build )
# go:embed cannot reach outside its package, so the bundle is copied in
# (placeholder.txt stays; the copied files are git-ignored). This build
# carries both screens; ?ui=classic switches the web editions.
mkdir -p internal/webui/dist
cp -r frontend/dist/. internal/webui/dist/

echo "[2/8] Building the Linux web editions (single executable)..."
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-linux-amd64 ./cmd/core
CGO_ENABLED=0 GOOS=linux GOARCH=386  go build -trimpath -ldflags "-s -w" -o dist/younblock-linux-386  ./cmd/core

echo "[3/8] Building the headless engines for Windows..."
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
GOOS=windows GOARCH=386 go build -trimpath -ldflags "-s -w" -o dist/younblock-core-x86.exe ./cmd/core

echo "[4/8] Building the Windows apps (new screen + classic)..."
if command -v wails >/dev/null 2>&1; then
  # The redesigned screen (360 x 628 client).
  wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
  cp build/bin/younblock.exe dist/younblock.exe
  wails build -platform windows/386 -trimpath -ldflags "-s -w" -o younblock-x86.exe
  cp build/bin/younblock-x86.exe dist/younblock-x86.exe
  # The classic screen (360 x 760 client, the v0.6.0 window).
  VITE_UI_VARIANT=classic wails build -platform windows/amd64 -trimpath \
    -ldflags "-s -w -X main.windowSize=360x760" -o younblock-classic.exe
  cp build/bin/younblock-classic.exe dist/younblock-classic.exe
  VITE_UI_VARIANT=classic wails build -platform windows/386 -trimpath \
    -ldflags "-s -w -X main.windowSize=360x760" -o younblock-classic-x86.exe
  cp build/bin/younblock-classic-x86.exe dist/younblock-classic-x86.exe
else
  echo "  wails CLI not found — skipped the Windows GUI apps."
  echo "  (go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0, then re-run)"
fi

echo "[5/8] Building the native Linux apps (real windows, D67)..."
NATIVE_OK=1
if command -v wails >/dev/null 2>&1; then
  STACKENV="$(mktemp)"
  if packaging/native/fetch-stack.sh >"$STACKENV" 2>/dev/null; then
    # shellcheck disable=SC1090
    . "$STACKENV"
    rm -f "$STACKENV"
    # webkit2gtk-4.1 from the jammy stack: glibc 2.35 floor — every Mint
    # since 21 runs the result.
    wails build -platform linux/amd64 -tags webkit2_41 -trimpath -ldflags "-s -w"
    cp build/bin/younblock dist/younblock-linux-gui-amd64
    VITE_UI_VARIANT=classic wails build -platform linux/amd64 -tags webkit2_41 \
      -trimpath -ldflags "-s -w -X main.windowSize=360x760" -o younblock-linux-gui-classic-amd64
    cp build/bin/younblock-linux-gui-classic-amd64 dist/younblock-linux-gui-classic-amd64
  else
    rm -f "$STACKENV"
    echo "  the webkit stack could not be assembled — skipped the native apps."
    NATIVE_OK=0
  fi
else
  echo "  wails CLI not found — skipped the native Linux apps."
  NATIVE_OK=0
fi

echo "[6/8] Packaging the .deb installers (Linux Mint)..."
if [ ! -f packaging/icons/younblock.png ]; then
  python3 packaging/gen-icons.py
fi
if [ "$NATIVE_OK" = 1 ]; then
  packaging/deb/build-deb.sh native amd64 dist/younblock-linux-gui-amd64 "$VER" younblock
  packaging/deb/build-deb.sh native amd64 dist/younblock-linux-gui-classic-amd64 "$VER" younblock-classic
else
  packaging/deb/build-deb.sh amd64 dist/younblock-linux-amd64 "$VER" younblock
fi
packaging/deb/build-deb.sh i386 dist/younblock-linux-386 "$VER" younblock

echo "[7/8] Packaging the AppImages (portable single file)..."
if [ "$NATIVE_OK" = 1 ]; then
  packaging/appimage/build-appimage.sh native x86_64 dist/younblock-linux-gui-amd64 "$VER" younblock
  packaging/appimage/build-appimage.sh native x86_64 dist/younblock-linux-gui-classic-amd64 "$VER" younblock-classic
else
  packaging/appimage/build-appimage.sh x86_64 dist/younblock-linux-amd64 "$VER" younblock
fi
packaging/appimage/build-appimage.sh i686 dist/younblock-linux-386 "$VER" younblock

echo "[8/8] Packaging the Flatpak bundle..."
if command -v flatpak >/dev/null 2>&1 && command -v flatpak-builder >/dev/null 2>&1; then
  packaging/flatpak/build-flatpak.sh dist/younblock-linux-amd64 "$VER"
else
  echo "  flatpak/flatpak-builder not found — skipped (see packaging/flatpak/)."
fi

echo "Done. Output files:"
for f in dist/younblock.exe dist/younblock-x86.exe \
         dist/younblock-classic.exe dist/younblock-classic-x86.exe \
         dist/younblock-core.exe dist/younblock-core-x86.exe \
         dist/younblock-linux-amd64 dist/younblock-linux-386 \
         dist/younblock_"$VER"_amd64.deb dist/younblock-classic_"$VER"_amd64.deb \
         dist/younblock_"$VER"_i386.deb \
         dist/Younblock-"$VER"-x86_64.AppImage dist/Younblock-Classic-"$VER"-x86_64.AppImage \
         dist/Younblock-"$VER"-i686.AppImage \
         dist/Younblock-"$VER"-x86_64.flatpak; do
  [ -f "$f" ] || continue
  printf '  %s  (%s)\n' "$PWD/$f" "$(sha256sum "$f" | cut -c1-16)…"
done
echo "SHA-256 checksums (full):"
for f in dist/younblock.exe dist/younblock-x86.exe \
         dist/younblock-classic.exe dist/younblock-classic-x86.exe \
         dist/younblock-core.exe dist/younblock-core-x86.exe \
         dist/younblock-linux-amd64 dist/younblock-linux-386 \
         dist/younblock_"$VER"_amd64.deb dist/younblock-classic_"$VER"_amd64.deb \
         dist/younblock_"$VER"_i386.deb \
         dist/Younblock-"$VER"-x86_64.AppImage dist/Younblock-Classic-"$VER"-x86_64.AppImage \
         dist/Younblock-"$VER"-i686.AppImage \
         dist/Younblock-"$VER"-x86_64.flatpak; do
  [ -f "$f" ] || continue
  sha256sum "$f"
done
