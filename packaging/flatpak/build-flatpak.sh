#!/usr/bin/env bash
# build-flatpak.sh — builds the Younblock Flatpak bundle from the static
# binary (D55). Best for convenient installation and updates with fewer
# dependency conflicts: the runtime ships everything the app needs.
#
# Usage: build-flatpak.sh <binary> <version>
# Output: dist/Younblock_<version>_x86_64.flatpak
#
# Requirements (one-time, on Linux Mint):
#   sudo apt install flatpak flatpak-builder
#   flatpak remote-add --if-not-exists flathub \
#     https://dl.flathub.org/repo/flathub.flatpakrepo
# (the GNOME runtime this manifest uses is downloaded from flathub
# automatically on the first build)
#
# The build first tries flatpak-builder (the normal path on any desktop).
# On build hosts whose containers block the nested PID namespaces the
# sandbox needs (some CI runners, this project's build sandbox), it falls
# back to flatpak's low-level build commands, which produce the same
# bundle without entering a sandbox — the manifest stays the single
# source of truth either way (its own build-commands are executed).
set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <binary> <version>" >&2
  exit 2
fi
BIN="$1"
VER="$2"
[ -f "$BIN" ] || { echo "binary not found: $BIN" >&2; exit 1; }

command -v flatpak >/dev/null 2>&1 || {
  echo "flatpak not found — install it first (see the header of this script)." >&2
  exit 1
}
command -v flatpak-builder >/dev/null 2>&1 || {
  echo "flatpak-builder not found — sudo apt install flatpak-builder" >&2
  exit 1
}

APP_ID="io.github.assadigit.Younblock"

# Stage every source next to a copy of the manifest: flatpak-builder
# resolves source paths relative to the manifest's own directory.
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp "$BIN" "$STAGE/younblock-linux"
sed -e "s|@EXEC@|younblock --web|" \
    -e "s|@ICON@|$APP_ID|" \
  packaging/desktop/younblock.desktop.in \
  > "$STAGE/$APP_ID.desktop"
cp "packaging/flatpak/$APP_ID.metainfo.xml" "$STAGE/"
cp "packaging/flatpak/$APP_ID.json" "$STAGE/"
mkdir -p "$STAGE/icons"
for s in 16 24 32 48 64 96 128 256 512; do
  install -D -m 0644 "packaging/icons/hicolor/$s/apps/$APP_ID.png" \
    "$STAGE/icons/$s/apps/$APP_ID.png"
done

mkdir -p dist
OUT="dist/Younblock-${VER}-x86_64.flatpak"

# ---------------------------------------------------------------------------
# Primary path: flatpak-builder, straight from the manifest.
# ---------------------------------------------------------------------------
build_with_builder() {
  # rofiles-fuse protects the build dir during export; it needs /dev/fuse
  # (present on any real desktop, absent in some containers/builders).
  local args=(--force-clean --repo="$STAGE/repo" --default-branch=stable)
  if [ ! -e /dev/fuse ]; then
    args+=(--disable-rofiles-fuse)
  fi
  flatpak-builder "${args[@]}" "$STAGE/build" "$STAGE/$APP_ID.json" &&
    flatpak build-bundle "$STAGE/repo" "$OUT" "$APP_ID" stable
}

# ---------------------------------------------------------------------------
# Fallback path: flatpak's low-level build commands. The sandboxed build
# commands never run here, so it works on hosts where bwrap's nested PID
# namespace is blocked. Everything (app id, runtime, finish args, even the
# per-file install commands) is read from the manifest itself.
# ---------------------------------------------------------------------------
build_manual() {
  # Read the manifest into shell variables (finish args and the build
  # commands come straight from it — the manifest stays authoritative).
  eval "$(python3 - "$STAGE/$APP_ID.json" <<'PYEOF'
import json, sys
m = json.load(open(sys.argv[1]))
def sh(s):
    return "'" + s.replace("'", "'\\''") + "'"
mod = m["modules"][0]
body = "\n".join(c.replace("/app/", "$FILES/") for c in mod["build-commands"])
print(f"APP_ID={sh(m['app-id'])}")
print(f"COMMAND={sh(m['command'])}")
print(f"RUNTIME={sh(m['runtime'])}")
print(f"RUNTIME_VER={sh(m['runtime-version'])}")
print(f"SDK={sh(m['sdk'])}")
print(f"FINISH_ARGS={sh(' '.join(m['finish-args']))}")
print(f"BUILD_CMDS={sh(body)}")
PYEOF
)"
  local BUILD="$STAGE/build" REPO="$STAGE/repo" FILES
  flatpak build-init --arch=x86_64 "$BUILD" "$APP_ID" "$SDK" "$RUNTIME" "$RUNTIME_VER"
  FILES="$BUILD/files"
  # Execute the manifest's own build-commands (with /app redirected to
  # the files dir, and the staged sources as the working directory).
  ( cd "$STAGE" && eval "$BUILD_CMDS" )

  # Finalize the metadata FIRST (flatpak-builder does the same):
  # build-finish refuses a build dir that already has an export/ tree.
  flatpak build-finish --command="$COMMAND" $FINISH_ARGS "$BUILD"

  # flatpak-builder also collects the store-facing files into export/.
  mkdir -p "$BUILD/export/share/applications" "$BUILD/export/share/icons"
  cp "$FILES/share/applications/$APP_ID.desktop" \
     "$BUILD/export/share/applications/$APP_ID.desktop"
  cp -r "$FILES/share/icons/." "$BUILD/export/share/icons/"

  # AppStream metadata for software stores/search (best effort — the
  # bundle installs and runs fine without it).
  local ac=""
  if command -v appstream-compose >/dev/null 2>&1; then
    ac="$(command -v appstream-compose)"
  elif [ -x /usr/libexec/appstreamcli-compose ]; then
    ac=/usr/libexec/appstreamcli-compose
  fi
  if [ -n "$ac" ]; then
    "$ac" --prefix=/ --origin=flatpak --result-root="$BUILD/export" \
      --components="$APP_ID,$APP_ID.desktop" --no-net "$FILES" \
      || echo "note: appstream-compose reported issues (non-fatal)"
  fi

  # build-export commits the build dir into an archive repo (it does not
  # create the repo itself, and takes the branch as a positional arg).
  # Icon validation normally runs inside a bwrap sandbox; on hosts without
  # bwrap (this project's restricted build sandbox), it runs directly —
  # same check the flatpak-builder path gets from /dev/fuse above.
  local export_args=(--arch=x86_64)
  if ! command -v bwrap >/dev/null 2>&1; then
    export_args+=(--disable-sandbox)
  fi
  mkdir -p "$REPO"
  ostree init --repo="$REPO" --mode=archive-z2 2>/dev/null || true
  flatpak build-export "${export_args[@]}" "$REPO" "$BUILD" stable
  flatpak build-bundle "$REPO" "$OUT" "$APP_ID" stable
}

echo "building the Flatpak bundle (flatpak-builder path)..."
if ! build_with_builder; then
  echo "flatpak-builder failed (restricted build host?) —"
  echo "falling back to the low-level flatpak build commands (same manifest)."
  rm -rf "$STAGE/build" "$STAGE/repo"
  build_manual
fi
echo "built $OUT ($(du -h "$OUT" | cut -f1))"
echo "install it with: flatpak install --user \"$OUT\""
