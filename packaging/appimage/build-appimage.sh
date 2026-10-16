#!/usr/bin/env bash
# build-appimage.sh — wraps a Linux binary into an AppImage (D53).
#
# Two flavors:
#
#   build-appimage.sh <x86_64|i686> <binary> <version>
#       The web edition: the static core serves its control page in the
#       browser (D50). Used for the 32-bit line.
#
#   build-appimage.sh native <binary> <version> <name>
#       The native window edition (D67): a real GTK + webkit2gtk desktop
#       app. The webkit runtime (and its whole NEEDED closure) is bundled
#       so the AppImage opens as an app on any glibc >= 2.35 desktop —
#       the thing users asked for after the browser-tab editions.
#
# Depends: curl (first run only — appimagetool is fetched into
# ~/.cache/younblock and reused; the build itself is FUSE-free via
# --appimage-extract-and-run). The native flavor also needs the jammy
# webkit stack, which packaging/native/fetch-stack.sh assembles on
# first use (no root).
set -euo pipefail
cd "$(dirname "$0")/../.."

MODE="web"
if [ "${1:-}" = "native" ]; then
  MODE="native"
  shift
fi

# Web mode takes 3 args, or 4 when the name is passed explicitly
# (build.sh does; it defaults to "younblock").
if { [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; } && [ "$MODE" = "web" ]; then
  echo "usage: $0 <x86_64|i686> <binary> <version> [name]" >&2
  exit 2
fi
if [ "$#" -ne 4 ] && [ "$MODE" = "native" ]; then
  echo "usage: $0 native <binary> <version> <name>" >&2
  echo "  <name> is the binary/WM_CLASS name (younblock or younblock-classic)" >&2
  exit 2
fi

ARCH="$1"; BIN="$2"; VER="$3"; NAME="${4:-younblock}"
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
install -D -m 0755 "$BIN" "$APPDIR/usr/bin/$NAME"

EXEC="$NAME"
if [ "$MODE" = "web" ]; then EXEC="$NAME --web"; fi
# The classic edition gets its own menu name so the two can live side by
# side; the window/taskbar identity comes from StartupWMClass either way.
DISPLAY_NAME="Younblock"
if [ "$NAME" = "younblock-classic" ]; then DISPLAY_NAME="Younblock Classic"; fi
sed -e "s|@EXEC@|$EXEC|" -e "s|@ICON@|$NAME|" \
  packaging/desktop/younblock.desktop.in \
  | sed -e "s|^Name=Younblock\$|Name=$DISPLAY_NAME|" > "$APPDIR/$NAME.desktop"
# Group the taskbar entry under the real window (native edition).
if [ "$MODE" = "native" ]; then
  printf 'StartupWMClass=%s\n' "$NAME" >> "$APPDIR/$NAME.desktop"
fi
install -D -m 0644 packaging/icons/$NAME.png "$APPDIR/$NAME.png" 2>/dev/null \
  || install -D -m 0644 packaging/icons/younblock.png "$APPDIR/$NAME.png"
for s in 16 24 32 48 64 96 128 256 512; do
  install -D -m 0644 "packaging/icons/hicolor/$s/apps/$NAME.png" \
    "$APPDIR/usr/share/icons/hicolor/$s/apps/$NAME.png" 2>/dev/null \
    || install -D -m 0644 "packaging/icons/hicolor/$s/apps/younblock.png" \
      "$APPDIR/usr/share/icons/hicolor/$s/apps/$NAME.png"
done

if [ "$MODE" = "native" ]; then
  # ------------------------------------------------------------------
  # The native edition: bundle the webkit runtime (D67).
  # ------------------------------------------------------------------
  if [ -z "${YOUNBLOCK_WEBKIT_STACK:-}" ]; then
    eval "$(packaging/native/fetch-stack.sh 2>/dev/null)"
  fi
  LIBDIR="$YOUNBLOCK_WEBKIT_STACK/usr/lib/x86_64-linux-gnu"
  DEST="$APPDIR/usr/lib/x86_64-linux-gnu"
  mkdir -p "$DEST"

  # Libraries the target desktop is guaranteed to have (any GTK desktop,
  # the X/GL stack, libc and friends) — bundling those would only fight
  # the system copies. Everything ELSE in the webkit NEEDED closure ships
  # inside the AppImage.
  EXCLUDE_RE='^(ld-linux.*|libc\.so\..*|libm\.so\..*|libpthread\.so\..*|libdl\.so\..*|librt\.so\..*|libresolv\.so\..*|libutil\.so\..*|libanl\.so\..*|libgcc_s\.so\..*|libz\.so\..*|libzstd\.so\..*|liblzma\.so\..*|libbz2\.so\..*|libpng16\.so\..*|libexpat\.so\..*|libbrotli.*|libfreetype\.so\..*|libfontconfig\.so\..*|libglib-2\.0\.so\..*|libgobject-2\.0\.so\..*|libgio-2\.0\.so\..*|libgmodule-2\.0\.so\..*|libgthread-2\.0\.so\..*|libgtk-3\.so\..*|libgdk-3\.so\..*|libgdk_pixbuf-2\.0\.so\..*|libpango.*\.so\..*|libcairo.*\.so\..*|libatk.*\.so\..*|libatspi.*\.so\..*|libharfbuzz.*\.so\..*|libX.*\.so\..*|libxcb.*\.so\..*|libxkbcommon.*\.so\..*|libwayland.*\.so\..*|libICE\.so\..*|libSM\.so\..*|libGL.*\.so\..*|libEGL\.so\..*|libGLES.*\.so\..*|libglapi\.so\..*|libgbm\.so\..*|libdrm\.so\..*|libdbus-1\.so\..*|libudev\.so\..*|libsystemd\.so\..*|libselinux\.so\..*|libseccomp\.so\..*|libmount\.so\..*|libblkid\.so\..*|libffi\.so\..*|libpcre\.so\..*|libpcre2-.*\.so\..*|libcap\.so\..*|libxml2\.so\..*|libedit\.so\..*|libtinfo\.so\..*|libbsd\.so\..*|libmd\.so\..*|libgmp\.so\..*|libhogweed\.so\..*|libnettle\.so\..*|libtasn1\.so\..*|libidn2?\.so\..*|libkeyutils\.so\..*|liblz4\.so\..*)$'

  needed() { objdump -p "$1" 2>/dev/null | awk '/NEEDED/ { print $2 }'; }

  # BFS over the NEEDED closure, starting from the app binary and the
  # webkit pair; whatever is not excluded and exists in the stack ships.
  declare -A seen=()
  queue=("$BIN" "$LIBDIR/libwebkit2gtk-4.1.so.0" "$LIBDIR/libjavascriptcoregtk-4.1.so.0")
  keep=()
  while [ ${#queue[@]} -gt 0 ]; do
    cur="${queue[0]}"; queue=("${queue[@]:1}")
    for soname in $(needed "$cur"); do
      [ -n "${seen[$soname]:-}" ] && continue
      seen[$soname]=1
      [[ "$soname" =~ $EXCLUDE_RE ]] && continue
      src="$LIBDIR/$soname"
      [ -e "$src" ] || continue          # not ours (system-only) — skip
      real="$(readlink -f "$src")"
      keep+=("$soname|$real")
      queue+=("$real")
    done
  done

  declare -A copied=()
  for entry in "${keep[@]}"; do
    soname="${entry%%|*}"; real="${entry#*|}"
    base="$(basename "$real")"
    # Some sonames ARE the file name (libwoff2dec.so.1.0.2) — copying the
    # file and then linking the soname onto itself would replace the real
    # file with a self-loop, so the link only happens when they differ.
    if [ -z "${copied[$base]:-}" ]; then
      cp -L "$real" "$DEST/$base"
      copied[$base]=1
    fi
    if [ "$soname" != "$base" ]; then
      ln -sf "$base" "$DEST/$soname"
    fi
  done
  echo "  bundled $(printf '%s\n' "${keep[@]}" | wc -l) webkit-closure libraries"

  # WebKit's helper processes (WebKitWebProcess & friends) live at a
  # compile-time /usr/lib/... path. The bundled libwebkit is patched here
  # to look at fixed /tmp names instead, and AppRun points those at this
  # extraction — the only portable way to relocate them.
  mkdir -p "$DEST/webkit2gtk-4.1"
  for f in WebKitNetworkProcess WebKitWebProcess WebKitGPUProcess; do
    install -m 0755 "$LIBDIR/webkit2gtk-4.1/$f" "$DEST/webkit2gtk-4.1/$f"
  done
  cp -a "$LIBDIR/webkit2gtk-4.1/injected-bundle" "$DEST/webkit2gtk-4.1/"
  python3 - "$DEST/libwebkit2gtk-4.1.so.0" <<'PYEOF'
import sys
path = sys.argv[1]
data = bytearray(open(path, 'rb').read())
def patch(old, new):
    global data
    slot, repl = old.encode() + b'\x00', new.encode() + b'\x00'
    n = data.count(slot)
    assert n == 1, f"expected one occurrence of {old!r}, found {n} — webkit layout changed?"
    assert len(repl) <= len(slot), "replacement does not fit the slot"
    i = data.index(slot)
    data[i:i+len(slot)] = repl + b'\x00' * (len(slot) - len(repl))
patch('/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1/injected-bundle/', '/tmp/.younblock-wk412-ib/')
patch('/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1', '/tmp/.younblock-wk412')
open(path, 'wb').write(bytes(data))
print('  patched the bundled webkit helper paths')
PYEOF

  # The webkit sandbox helpers (bwrap + xdg-dbus-proxy) must be on PATH.
  for tool in bwrap xdg-dbus-proxy; do
    if [ -x "$YOUNBLOCK_WEBKIT_STACK/usr/bin/$tool" ]; then
      install -m 0755 "$YOUNBLOCK_WEBKIT_STACK/usr/bin/$tool" "$APPDIR/usr/bin/$tool"
    fi
  done

  cat > "$APPDIR/AppRun" <<APPEOF
#!/bin/sh
# Younblock native AppImage launcher (D67): the GTK window runs straight
# from this file — no browser tab. The bundled webkit looks for its
# helper processes at two fixed /tmp names (the bundled library is
# patched at build time to use them); point them at this extraction.
HERE="\$(dirname "\$(readlink -f "\$0")")"
L="\$HERE/usr/lib/x86_64-linux-gnu"
ln -sfn "\$L/webkit2gtk-4.1" /tmp/.younblock-wk412
ln -sfn "\$L/webkit2gtk-4.1/injected-bundle" /tmp/.younblock-wk412-ib
export LD_LIBRARY_PATH="\$L\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}"
export PATH="\$HERE/usr/bin:\$PATH"
# VMs and quirky drivers render more reliably without the DMABUF path;
# the UI is a 360x628 window — it does not need the speed.
export WEBKIT_DISABLE_DMABUF_RENDERER=1
exec "\$HERE/usr/bin/$NAME" "\$@"
APPEOF
  chmod 0755 "$APPDIR/AppRun"
else
  install -D -m 0755 packaging/appimage/AppRun "$APPDIR/AppRun"
fi

mkdir -p dist
PRETTY="Younblock"
[ "$NAME" = "younblock-classic" ] && PRETTY="Younblock-Classic"
OUT="dist/$PRETTY-$VER-$ARCH.AppImage"
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
