#!/usr/bin/env bash
# build-appimage.sh — wraps the static Linux binary into an AppImage (D53).
# Best when you want one portable file you download, make executable and
# launch — nothing is installed, deleting the file uninstalls it.
#
# Usage: build-appimage.sh <x86_64|i686> <binary> <version>
# Output: dist/Younblock_<version>_<arch>.AppImage
#
# Depends: curl (first run only — appimagetool is fetched into
# ~/.cache/younblock and reused; the build itself is FUSE-free via
# --appimage-extract-and-run).
set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <x86_64|i686> <binary> <version>" >&2
  exit 2
fi
ARCH="$1"
BIN="$2"
VER="$3"

case "$ARCH" in
  x86_64|i686) ;;
  *) echo "architecture must be x86_64 or i686 (got: $ARCH)" >&2; exit 2 ;;
esac
[ -f "$BIN" ] || { echo "binary not found: $BIN" >&2; exit 1; }

CACHE="$HOME/.cache/younblock"
mkdir -p "$CACHE"
TOOL="$CACHE/appimagetool-x86_64.AppImage"
if [ ! -x "$TOOL" ]; then
  echo "fetching appimagetool (one-time, ~1 MB)..."
  curl -fsSL -o "$TOOL" \
    https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
  chmod +x "$TOOL"
fi

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
APPDIR="$STAGE/Younblock.AppDir"

# --- the payload: the binary, the menu entry, the icon set, AppRun ---
install -D -m 0755 "$BIN" "$APPDIR/usr/bin/younblock"
sed -e "s|@EXEC@|younblock --web|" -e "s|@ICON@|younblock|" \
  packaging/desktop/younblock.desktop.in > "$APPDIR/younblock.desktop"
install -D -m 0644 packaging/icons/younblock.png "$APPDIR/younblock.png"
for s in 16 24 32 48 64 96 128 256 512; do
  install -D -m 0644 "packaging/icons/hicolor/$s/apps/younblock.png" \
    "$APPDIR/usr/share/icons/hicolor/$s/apps/younblock.png"
done
install -D -m 0755 packaging/appimage/AppRun "$APPDIR/AppRun"

mkdir -p dist
OUT="dist/Younblock-${VER}-${ARCH}.AppImage"
# appimagetool detects the architecture from the ELF binary itself; the
# 32-bit runtime is not embedded in the tool, so pass it explicitly.
ARGS=(--appimage-extract-and-run)
if [ "$ARCH" = i686 ]; then
  RT="$CACHE/runtime-i686"
  if [ ! -f "$RT" ]; then
    echo "fetching the i686 AppImage runtime (one-time)..."
    curl -fsSL -o "$RT" \
      https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-i686
  fi
  ARGS+=(--runtime-file "$RT")
fi
ARGS+=("$APPDIR" "$OUT")
"$TOOL" "${ARGS[@]}" 2>&1 | grep -v "Failed to remove directory" || true
[ -f "$OUT" ] || { echo "appimagetool did not produce $OUT" >&2; exit 1; }
echo "built $OUT ($(du -h "$OUT" | cut -f1))"
