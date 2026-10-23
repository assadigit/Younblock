#!/usr/bin/env bash
# release-unified.sh — the unified release lane (the one-release round,
# D111): builds EVERY version from this one source tree and ships them
# together as ONE GitHub release — the Windows apps (64/32-bit), the
# headless engines, the Linux web editions, the .deb installers, the
# AppImages, the offline pack, and the Android APK. One release, one UI
# (D110: every edition ships the same v3 screen), one version line.
#
# Usage:
#   GITHUB_TOKEN=ghp_… ./release-unified.sh <tag> [notes-file.md]
#
#   <tag>            the release tag (e.g. v0.8.0); created on HEAD if it
#                    does not exist yet — commit your work first, the
#                    lane builds the tree it stands on
#   [notes-file.md]  optional markdown prepended to the generated body
#                    (the checksum table and the edition list are always
#                    appended)
#
# The token rides the environment (or the second positional after the
# tag); it is never written into the repo. Exit codes are honest: any
# failing step stops the lane with its status.
#
# Prerequisites: the union of build.sh and build-android.sh (Go, JDK,
# the Android SDK, Node/npm; wails and the webkit stack are fetched/
# skipped by the build scripts themselves).
set -euo pipefail
cd "$(dirname "$0")"

REPO="${GITHUB_REPO:-assadigit/Younblock}"
TAG="${1:?usage: GITHUB_TOKEN=… ./release-unified.sh <tag> [notes-file.md]}"
NOTES_FILE="${2:-}"
TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ] && [ $# -ge 3 ]; then TOKEN="$3"; fi
if [ -z "$TOKEN" ]; then
  echo "no token: set GITHUB_TOKEN (or pass it as the third argument)" >&2
  exit 1
fi

VER="$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/config/defaults.go)"
if [ -z "$VER" ]; then
  echo "could not read the version from internal/config/defaults.go" >&2
  exit 1
fi
# The mobile string (e.g. 1.6.7-debug.1) names the APK asset (D103):
# v1.6.7-debug1.apk — the filename always names its build.
MOBILE_VER="$(sed -n 's/.*Version: "\([0-9][0-9.]*-debug\.[0-9][0-9]*\)".*/\1/p' mobile/mobile.go)"
if [ -z "$MOBILE_VER" ]; then
  echo "could not read the mobile version string from mobile/mobile.go" >&2
  exit 1
fi
APK_NAME="v${MOBILE_VER/-debug./-debug}.apk"

auth_header() { printf 'Authorization: token %s' "$TOKEN"; }

echo "=== Younblock unified release ($TAG, app $VER, apk $APK_NAME) ==="

# The tree must be clean no matter what: the lane releases a committed
# tree, and a tag always pins what ships.
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "the work tree has uncommitted changes — commit first, then release" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# [0] The tag: the lane stands on a committed tree; the tag pins it (the
#     offline pack archives the tag, so the pack can never drift from
#     what was released).
# ---------------------------------------------------------------------------
if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  git tag -a "$TAG" -m "Younblock $TAG (all versions, one release, one UI)"
  echo "tagged $TAG on $(git rev-parse --short HEAD)"
fi

# ---------------------------------------------------------------------------
# [1-3] The builds. SKIP_BUILDS=1 resumes with the artifacts already in
#     dist/ — allowed ONLY when HEAD is exactly the tag (the artifacts
#     must come from this very tree; anything else refuses).
# ---------------------------------------------------------------------------
if [ "${SKIP_BUILDS:-0}" = 1 ]; then
  HEAD_DESC="$(git describe --tags --exact-match 2>/dev/null || true)"
  if [ "$HEAD_DESC" != "$TAG" ]; then
    echo "SKIP_BUILDS=1 refused: HEAD is not exactly $TAG (is '$HEAD_DESC') — rebuild honestly" >&2
    exit 1
  fi
  echo "SKIP_BUILDS=1 — publishing the artifacts already built from $TAG"
else
  # The desktop half: Windows apps + engines + Linux editions + debs +
  # AppImages (+ flatpak when the tools exist). Everything lands in
  # dist/ — whatever this machine could build ships; the honest skip
  # messages name what it could not.
  ./build.sh

  # The Android half: the same frontend bundle (the same screen) rides
  # the gomobile AAR into the APK.
  ./build-android.sh

  # The offline pack (the everything-inside zip): best-effort — it
  # downloads the Windows Go/Node toolchains, so a slow or blocked
  # network may starve it. A skip is honest, not fatal.
  if ./build-offline-pack.sh; then
    echo "offline pack built"
  else
    echo "  the offline pack could not be built — shipping without it (honest skip)"
  fi
fi
mkdir -p dist
if [ -f android/app/build/outputs/apk/debug/app-debug.apk ]; then
  cp android/app/build/outputs/apk/debug/app-debug.apk "dist/$APK_NAME"
fi

# ---------------------------------------------------------------------------
# [4] Collect the release set: every regular file in dist/ ships. One
#     release, all versions.
# ---------------------------------------------------------------------------
shopt -s nullglob
ASSETS=(dist/*)
shopt -u nullglob
if [ "${#ASSETS[@]}" -eq 0 ]; then
  echo "dist/ is empty — nothing to release" >&2
  exit 1
fi

CHECKSUMS="$(mktemp)"
BODY="$(mktemp)"
{
  echo "## SHA-256 checksums"
  echo
  echo '```'
  for f in "${ASSETS[@]}"; do
    ( cd "$(dirname "$f")" && sha256sum "$(basename "$f")" )
  done
  echo '```'
} >"$CHECKSUMS"

{
  echo "# Younblock $TAG — every version, one release, one UI"
  echo
  if [ -n "$NOTES_FILE" ] && [ -f "$NOTES_FILE" ]; then
    cat "$NOTES_FILE"
    echo
  fi
  cat "$CHECKSUMS"
} >"$BODY"

# ---------------------------------------------------------------------------
# [5] Publish: push the tag, create the release (or resume the one this
#     tag already has — a timed-out run can be re-run safely), upload
#     every asset (an already-uploaded asset with the right size is
#     skipped; a wrong-size one is replaced). The verify step's
#     byte-compare is the final word either way.
# ---------------------------------------------------------------------------
git push origin "refs/tags/$TAG"

RELEASE_JSON="$(curl -s -H "$(auth_header)" \
  "https://api.github.com/repos/$REPO/releases/tags/$TAG")"
RELEASE_ID="$(printf '%s' "$RELEASE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("id",""))' 2>/dev/null || true)"
if [ -z "$RELEASE_ID" ] || [ "$RELEASE_ID" = "None" ]; then
  RELEASE_JSON="$(curl -s -X POST \
    -H "$(auth_header)" \
    -H "Accept: application/vnd.github+json" \
    -d "{\"tag_name\":\"$TAG\",\"name\":\"$TAG — all versions, one release, one UI\",\"body\":$(python3 -c 'import json,sys; print(json.dumps(open(sys.argv[1]).read()))' "$BODY"),\"draft\":false,\"prerelease\":false}" \
    "https://api.github.com/repos/$REPO/releases")"
  RELEASE_ID="$(printf '%s' "$RELEASE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("id",""))')"
  if [ -z "$RELEASE_ID" ] || [ "$RELEASE_ID" = "None" ]; then
    echo "release creation failed:" >&2
    printf '%s\n' "$RELEASE_JSON" >&2
    exit 1
  fi
  echo "release $TAG created (id $RELEASE_ID)"
else
  echo "resuming the existing release $TAG (id $RELEASE_ID)"
fi

asset_field() { # asset_field <json-file> <name> <field>
  python3 -c 'import json,sys
for a in json.load(open(sys.argv[1])):
    if a["name"]==sys.argv[2]:
        print(a[sys.argv[3]]); break' "$1" "$2" "$3"
}

UPLOAD_BASE="https://uploads.github.com/repos/$REPO/releases/$RELEASE_ID/assets"
LISTING="$(mktemp)"
trap 'rm -f "$CHECKSUMS" "$BODY" "$LISTING"' EXIT
curl -s -H "$(auth_header)" \
  "https://api.github.com/repos/$REPO/releases/$RELEASE_ID/assets" -o "$LISTING"
for f in "${ASSETS[@]}"; do
  name="$(basename "$f")"
  size="$(stat -c%s "$f")"
  have_id="$(asset_field "$LISTING" "$name" "id")"
  if [ -n "$have_id" ] && [ "$(asset_field "$LISTING" "$name" "size")" = "$size" ]; then
    echo "  $name: already uploaded (same size) — skipping"
    continue
  fi
  if [ -n "$have_id" ]; then
    echo "  $name: replacing the stale upload..."
    curl -s -X DELETE -H "$(auth_header)" \
      "https://api.github.com/repos/$REPO/releases/assets/$have_id" >/dev/null || true
  fi
  echo "  uploading $name ($(du -h "$f" | cut -f1))..."
  for attempt in 1 2 3; do
    UP="$(curl -s -X POST \
      -H "$(auth_header)" \
      -H "Content-Type: application/octet-stream" \
      --data-binary "@$f" \
      "$UPLOAD_BASE?name=$name")"
    if printf '%s' "$UP" | grep -q '"id"'; then
      break
    fi
    echo "    attempt $attempt failed, retrying..." >&2
    sleep $((attempt * 5))
  done
  if ! printf '%s' "$UP" | grep -q '"id"'; then
    echo "upload of $name failed:" >&2
    printf '%s\n' "$UP" >&2
    exit 1
  fi
done

# ---------------------------------------------------------------------------
# [6] Verify the honest way (the repo's release discipline): re-download
#     every asset and byte-compare it with what was built.
# ---------------------------------------------------------------------------
echo "verifying (re-download + byte-compare)..."
ASSET_LIST="$(mktemp)"
curl -sL -H "$(auth_header)" \
  "https://api.github.com/repos/$REPO/releases/$RELEASE_ID/assets" -o "$ASSET_LIST"
FAILED=0
for f in "${ASSETS[@]}"; do
  name="$(basename "$f")"
  browser_url="$(python3 -c 'import json,sys; [print(a["browser_download_url"]) for a in json.load(open(sys.argv[1])) if a["name"]==sys.argv[2]]' "$ASSET_LIST" "$name")"
  if [ -z "$browser_url" ]; then
    echo "  $name: NOT LISTED in the release" >&2
    FAILED=1
    continue
  fi
  curl -sL -H "$(auth_header)" -o "/tmp/verify-$name" "$browser_url"
  if cmp -s "$f" "/tmp/verify-$name"; then
    echo "  $name: byte-identical re-download ($(stat -c%s "$f") B)"
    rm -f "/tmp/verify-$name"
  else
    echo "  $name: RE-DOWNLOAD DIFFERS" >&2
    FAILED=1
  fi
done
rm -f "$ASSET_LIST"

# ---------------------------------------------------------------------------
# [7] The record: what shipped, for PROGRESS.md.
# ---------------------------------------------------------------------------
echo
echo "=== RELEASE RECORD (for PROGRESS.md) ==="
echo "tag: $TAG   release id: $RELEASE_ID   url: https://github.com/$REPO/releases/tag/$TAG"
curl -s -H "$(auth_header)" "https://api.github.com/repos/$REPO/releases/$RELEASE_ID/assets" |
  python3 -c 'import json,sys
for a in json.load(sys.stdin):
    print(f"asset: {a[\"name\"]}  {a[\"size\"]} B  id {a[\"id\"]}")'
cat "$CHECKSUMS"

if [ "$FAILED" != 0 ]; then
  echo "VERIFICATION FAILED for at least one asset — see above" >&2
  exit 1
fi
echo
echo "Done. All versions shipped in one release: https://github.com/$REPO/releases/tag/$TAG"
