#!/usr/bin/env bash
# build-deb.sh — wraps the static Linux binary into a .deb package (D53).
# Best for installing Younblock properly on Linux Mint: the menu entry,
# icons and binary land in the standard places and dpkg tracks them.
#
# Usage: build-deb.sh <amd64|i386> <binary> <version>
# Output: dist/younblock_<version>_<arch>.deb
#
# Depends: dpkg-deb (present everywhere on Debian-family systems).
set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <amd64|i386> <binary> <version>" >&2
  exit 2
fi
ARCH="$1"
BIN="$2"
VER="$3"

case "$ARCH" in
  amd64|i386) ;;
  *) echo "architecture must be amd64 or i386 (got: $ARCH)" >&2; exit 2 ;;
esac
[ -f "$BIN" ] || { echo "binary not found: $BIN" >&2; exit 1; }

OUT="dist/younblock_${VER}_${ARCH}.deb"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

# --- the payload: one static binary, the menu entry, the icon set ---
install -D -m 0755 "$BIN" "$STAGE/usr/bin/younblock"
mkdir -p "$STAGE/usr/share/applications"
sed -e "s|@EXEC@|/usr/bin/younblock --web|" -e "s|@ICON@|younblock|" \
  packaging/desktop/younblock.desktop.in > "$STAGE/usr/share/applications/younblock.desktop"
for s in 16 24 32 48 64 96 128 256 512; do
  install -D -m 0644 "packaging/icons/hicolor/$s/apps/younblock.png" \
    "$STAGE/usr/share/icons/hicolor/$s/apps/younblock.png"
done

# --- the paperwork dpkg wants ---
mkdir -p "$STAGE/DEBIAN" "$STAGE/usr/share/doc/younblock"
INSTALLED_SIZE="$(du -sk "$STAGE/usr" | cut -f1)"
cat > "$STAGE/DEBIAN/control" <<EOF
Package: younblock
Version: $VER
Architecture: $ARCH
Maintainer: Ali <assadigit@users.noreply.github.com>
Installed-Size: $INSTALLED_SIZE
Depends: libglib2.0-bin
Recommends: libnotify-bin, xdg-utils
Section: net
Priority: optional
Homepage: https://github.com/assadigit/Younblock
Description: Watch YouTube without interruptions
 A tiny local proxy plus secure DNS (DNS-over-HTTPS) with TLS
 fragmentation that makes YouTube load and play when a network
 blocks or throttles it. One static binary with the whole UI
 built in; settings live in ~/.config/Younblock. Works with
 every mainstream browser through the system proxy.
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

cat > "$STAGE/usr/share/doc/younblock/copyright" <<'EOS'
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
