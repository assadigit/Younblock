#!/usr/bin/env bash
# build-deb.sh — wraps a Linux binary into a .deb package (D53).
# Best for installing Younblock properly on Linux Mint: the menu entry,
# icons and binary land in the standard places and dpkg tracks them.
#
# Two flavors:
#
#   build-deb.sh <amd64|i386> <binary> <version>
#       The web edition: the static core serves its control page in the
#       browser (D50). Used for the 32-bit line.
#
#   build-deb.sh native <amd64> <binary> <version> <name>
#       The native window edition (D67): a real GTK + webkit2gtk desktop
#       app. The system webkit is used (apt resolves it — no bloat in the
#       download), which is why the native line is 64-bit only: Mint 21
#       (the oldest supported base) ships webkit2gtk-4.1 for amd64.
#
# Depends: dpkg-deb (present everywhere on Debian-family systems).
set -euo pipefail
cd "$(dirname "$0")/../.."

MODE="web"
if [ "${1:-}" = "native" ]; then
  MODE="native"
  shift
fi

if [ "$#" -ne 3 ] && [ "$MODE" = "web" ]; then
  echo "usage: $0 <amd64|i386> <binary> <version>" >&2
  exit 2
fi
if [ "$#" -ne 4 ] && [ "$MODE" = "native" ]; then
  echo "usage: $0 native <amd64> <binary> <version> <name>" >&2
  echo "  <name> is the package/binary name (younblock or younblock-classic)" >&2
  exit 2
fi

ARCH="$1"; BIN="$2"; VER="$3"; NAME="${4:-younblock}"
[ -f "$BIN" ] || { echo "binary not found: $BIN" >&2; exit 1; }

OUT="dist/${NAME}_${VER}_${ARCH}.deb"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

# --- the payload: one binary, the menu entry, the icon set ---
install -D -m 0755 "$BIN" "$STAGE/usr/bin/$NAME"
mkdir -p "$STAGE/usr/share/applications"
EXEC="$NAME"
DISPLAY_NAME="Younblock"
if [ "$NAME" = "younblock-classic" ]; then DISPLAY_NAME="Younblock Classic"; fi
if [ "$MODE" = "web" ]; then
  EXEC="/usr/bin/$NAME --web"
else
  EXEC="/usr/bin/$NAME"
fi
sed -e "s|@EXEC@|$EXEC|" -e "s|@ICON@|$NAME|" \
  packaging/desktop/younblock.desktop.in \
  | sed -e "s|^Name=Younblock\$|Name=$DISPLAY_NAME|" > "$STAGE/usr/share/applications/$NAME.desktop"
if [ "$MODE" = "native" ]; then
  printf 'StartupWMClass=%s\n' "$NAME" >> "$STAGE/usr/share/applications/$NAME.desktop"
fi
for s in 16 24 32 48 64 96 128 256 512; do
  install -D -m 0644 "packaging/icons/hicolor/$s/apps/younblock.png" \
    "$STAGE/usr/share/icons/hicolor/$s/apps/$NAME.png"
done

# --- the paperwork dpkg wants ---
mkdir -p "$STAGE/DEBIAN" "$STAGE/usr/share/doc/$NAME"
INSTALLED_SIZE="$(du -sk "$STAGE/usr" | cut -f1)"
DESCRIPTION="Watch YouTube without interruptions
 A tiny local proxy plus secure DNS (DNS-over-HTTPS) with TLS
 fragmentation that makes YouTube load and play when a network
 blocks or throttles it."
if [ "$MODE" = "web" ]; then
  DEPENDS="libglib2.0-bin"
  DESCRIPTION="$DESCRIPTION
 This edition serves its control page in the browser; the
 settings live in ~/.config/Younblock."
else
  # The window edition: gtk3 and webkit come from the Mint/Ubuntu
  # repos, matching whatever the distro ships (apt keeps them in sync).
  DEPENDS="libwebkit2gtk-4.1-0, libgtk-3-0t64 | libgtk-3-0, libglib2.0-bin"
  DESCRIPTION="$DESCRIPTION
 This edition opens as its own desktop window."
fi
cat > "$STAGE/DEBIAN/control" <<EOF
Package: $NAME
Version: $VER
Architecture: $ARCH
Maintainer: Ali <assadigit@users.noreply.github.com>
Installed-Size: $INSTALLED_SIZE
Depends: $DEPENDS
Recommends: libnotify-bin, xdg-utils
Section: net
Priority: optional
Homepage: https://github.com/assadigit/Younblock
Description: $DESCRIPTION
EOF

# Refresh the desktop/icon caches when those helpers exist (they do on
# Mint; the guards keep the package installable on minimal systems).
for script in postinst postrm; do
  cat > "$STAGE/DEBIAN/$script" <<'EOS'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
EOS
  chmod 0755 "$STAGE/DEBIAN/$script"
done

cat > "$STAGE/usr/share/doc/$NAME/copyright" <<EOS
Younblock — https://github.com/assadigit/Younblock

Copyright 2026 Ali (assadigit). All rights reserved.
The source and binaries are distributed through the project's GitHub
releases; see the repository for terms and notices.
EOS

# Checksums of the payload (dpkg -V can verify the install later).
( cd "$STAGE" && find usr -type f -exec md5sum {} \; ) > "$STAGE/DEBIAN/md5sums"

mkdir -p dist
dpkg-deb --build --root-owner-group "$STAGE" "$OUT"
echo "built $OUT ($(du -h "$OUT" | cut -f1))"
