#!/usr/bin/env bash
# fetch-stack.sh — assembles a user-space GTK3 + webkit2gtk-4.1 development
# stack from Ubuntu 22.04 (jammy) packages, no root needed (D67).
#
# Why jammy: it is the oldest base still on Mint 21 (glibc 2.35), so a
# binary linked against this stack runs on every Mint since 21 — and on
# Mint 22/23, Debian 12+ and friends. Stacks from newer bases would
# silently raise the glibc floor.
#
# The script prints the exports to build against, so the caller does:
#
#   eval "$(packaging/native/fetch-stack.sh)"
#   wails build -platform linux/amd64 -tags webkit2_41 ...
#
# Everything lands in ~/.cache/younblock (debs kept for re-runs; the
# extracted stack is rebuilt only when the resolved package list changes).
set -euo pipefail
cd "$(dirname "$0")/../.."

CACHE="$HOME/.cache/younblock"
DEBS="$CACHE/jammy-debs"
INDEX="$CACHE/jammy-index"
STACK="$CACHE/webkitstack"
mkdir -p "$DEBS" "$INDEX" "$STACK"

MIRROR="${YOUNBLOCK_UBUNTU_MIRROR:-http://archive.ubuntu.com/ubuntu}"

# The roots: the dev packages whose closure (headers + link-time
# libraries) the Wails Linux frontend needs. Everything else follows from
# their Depends fields.
ROOTS="libwebkit2gtk-4.1-dev libgtk-3-dev libsoup-3.0-dev"

# Runtime libraries the AppImage bundler must be able to copy out of the
# stack (they live deeper in the NEEDED tree than the link-time closure
# reaches: glib -> pcre, gstreamer -> elfutils, manette -> evdev/gudev).
FIXUPS="libpcre3 libdw1 libelf1 libevdev2 libgudev-1.0-0 libudev1 libunistring2 bubblewrap"

# Packages that must NOT come from the stack: the C toolchain and libc
# belong to the build machine (mixing them in produces binaries that
# assume a different gcc/glibc universe), and these bookkeeping packages
# carry no files the linker ever wants.
SKIP_RE='^(libc6|libc6-dev|libc-bin|linux-libc-dev|gcc|gcc-[0-9].*|gcc-[0-9]+-base|g\+\+.*|cpp.*|binutils.*|dpkg|dpkg-dev|debconf.*|tzdata|netbase|adduser|lsb-base|install-info|ucf|sensible-utils|libgcc-[0-9]+-dev|mount|util-linux|passwd)$'

# Noise the closure would otherwise drag in: gobject-introspection data,
# gstreamer plugin metas (dlopened at runtime, never linked), desktop
# plumbing (dconf/avahi/polkit), icon themes, spell dictionaries — none
# of it is needed to compile or link, and the user's desktop already has
# whatever the runtime truly wants.
NOISE_RE='^(gir1\.2-.*|gstreamer1\.0-plugins-.*|glib-networking.*|dconf.*|libdconf.*|shared-mime-info|.*-icon-theme|.*-icon-theme.*|iso-codes.*|hunspell.*|dictionaries.*|emacsen.*|aspell.*|libaspell.*|gtk-update-icon-cache|libgdk-pixbuf2\.0-bin|libglib2\.0-bin|libglib2\.0-dev-bin|.*-data|fonts-.*|libgtk-3-common|libgtk-3-bin|dbus|dbus-x11|systemd.*|libsystemd.*|polkitd|policykit-.*|libpolkit.*|libdazzle.*)$'

# Whose Depends get followed: the -dev packages (headers/.pc/.so links
# pull each other), plus the webkit/javascriptcore runtime pair (their
# NEEDED chain is what the linker must resolve). Everything else is
# included as a leaf — its own deps are runtime concerns the linker
# never chases.
EXPAND_RE='(-dev$|^libwebkit2gtk-4\.1-0$|^libjavascriptcoregtk-4\.1-0$)'

POCKETS="jammy:main jammy:universe jammy-updates:main jammy-updates:universe jammy-security:main jammy-security:universe"

# --- 1. the package indexes (cached; refreshed when older than a day) ---
for pocket in $POCKETS; do
  dist="${pocket%%:*}"; comp="${pocket##*:}"
  gz="$INDEX/${dist}_${comp}_Packages.gz"
  if [ ! -s "$gz" ] || [ -n "$(find "$gz" -mtime +1 2>/dev/null)" ]; then
    echo "index: $dist/$comp" >&2
    curl -fsSL --retry 3 -o "$gz" \
      "$MIRROR/dists/$dist/$comp/binary-amd64/Packages.gz"
  fi
done

# --- 2. parse the indexes into name<TAB>version<TAB>filename lines,
#        keeping the highest version per name (updates beat the release) ---
MAP="$INDEX/map.tsv"
cat "$INDEX"/*_Packages.gz | zcat | awk '
  /^Package: / { name = substr($0, 10) }
  /^Version: / { ver = substr($0, 10) }
  /^Filename: / { file = substr($0, 11) }
  /^$/ { if (name != "" && file != "") { print name "\t" ver "\t" file }
         name = ""; ver = ""; file = "" }
' | sort -t "$(printf '\t')" -k1,1 -k2,2V | awk -F'\t' '
  { last[$1] = $0 }
  END { for (n in last) print last[n] }
' > "$MAP"

have() {  # have <pkg> — true when the index knows the package
  awk -F'\t' -v p="$1" '$1 == p { found = 1 } END { exit !found }' "$MAP"
}

pkgdeps() {  # pkgdeps <pkg> — the Depends line of the chosen version
  local want_ver
  want_ver="$(awk -F'\t' -v p="$1" '$1 == p { print $2; exit }' "$MAP")"
  cat "$INDEX"/*_Packages.gz | zcat | awk -v p="$1" -v v="$want_ver" '
    /^Package: / { name = substr($0, 10) }
    /^Version: / { ver = substr($0, 10) }
    /^Depends: / { dep = substr($0, 10) }
    /^$/ { if (name == p && ver == v && dep != "") { print dep; exit }
           name = ""; ver = ""; dep = "" }
  '
}

resolve() {  # resolve <root> — BFS over Depends, alternatives honored
  local -a queue=("$1")
  local -A seen=()
  while [ ${#queue[@]} -gt 0 ]; do
    local pkg="${queue[0]}"; queue=("${queue[@]:1}")
    [ -z "$pkg" ] && continue
    [ -n "${seen[$pkg]:-}" ] && continue
    seen[$pkg]=1
    if [[ "$pkg" =~ $SKIP_RE ]] || [[ "$pkg" =~ $NOISE_RE ]]; then continue; fi
    local line
    line="$(awk -F'\t' -v p="$pkg" '$1 == p { print; exit }' "$MAP")"
    if [ -z "$line" ]; then
      if [ "$pkg" = "$1" ]; then
        echo "root package $pkg not found in the indexes" >&2; exit 1
      fi
      continue  # virtual or outside these pockets
    fi
    echo "$line" >> "$LIST"
    # Only the dev packages and the webkit runtime pair get their Depends
    # followed (see EXPAND_RE).
    [[ "$pkg" =~ $EXPAND_RE ]] || continue
    local deps alts first picked
    while IFS= read -r deps; do
      [ -z "$deps" ] && continue
      picked=""
      local IFS='|'
      for alts in $deps; do
        first="$(printf '%s' "$alts" | tr -d ' ' | sed -e 's/[(].*//' -e 's/\[.*\]//' -e 's/<.*//')"
        if [ -n "$first" ] && have "$first" ]; then picked="$first"; break; fi
      done
      unset IFS
      [ -n "$picked" ] && queue+=("$picked")
    done < <(pkgdeps "$pkg" | tr ',' '\n')
  done
}

# --- 3. resolve the closure of the roots --------------------------------
LIST="$INDEX/closure.tsv"
: > "$LIST"
for root in $ROOTS $FIXUPS; do
  resolve "$root"
done
sort -u -t "$(printf '\t')" -k1,1 -o "$LIST" "$LIST"

# --- 4. download + extract when the closure changed ----------------------
STAMP="$STACK/.stamp"
if [ -f "$STAMP" ] && cmp -s "$STAMP" "$LIST"; then
  echo "webkit stack up to date ($(wc -l < "$LIST") packages)" >&2
else
  echo "building the stack: $(wc -l < "$LIST") packages..." >&2
  rm -rf "$STACK"
  mkdir -p "$STACK"
  while IFS=$'\t' read -r name ver file; do
    deb="$DEBS/${file##*/}"
    fetch_deb() {
      curl -fsSL --retry 3 -o "$deb" "$MIRROR/$file"
    }
    # A deb left half-written by an interrupted run must not survive the
    # -s check: verify it parses before trusting it.
    if [ ! -s "$deb" ] || ! dpkg-deb -f "$deb" Package >/dev/null 2>&1; then
      rm -f "$deb"
      fetch_deb
    fi
    dpkg -x "$deb" "$STACK"
    printf '  %s %s\r' "$name" "$ver" >&2
  done < "$LIST"
  cp "$LIST" "$STAMP"
  echo >&2

  # Jammy still ships a few libraries under the legacy /lib path — fold
  # everything into /usr/lib so one -L covers it all.
  if [ -d "$STACK/lib" ]; then
    mkdir -p "$STACK/usr/lib"
    cp -an "$STACK/lib/." "$STACK/usr/lib/" || true
  fi
  # And create the SONAME symlinks dpkg's ldconfig would have made on a
  # real install (dpkg -x only lays down the versioned files).
  ldconfig -n "$STACK/usr/lib/x86_64-linux-gnu" 2>/dev/null || true

  # --- 5. point the .pc files at the stack (they still say /usr/...) ----
  find "$STACK" -name '*.pc' -print0 | while IFS= read -r -d '' pc; do
    sed -i \
      -e "s|^prefix=/usr\$|prefix=$STACK/usr|" \
      -e "s|^exec_prefix=/usr\$|exec_prefix=$STACK/usr|" \
      -e "s|^libdir=/usr/lib\$|libdir=$STACK/usr/lib|" \
      -e "s|^libdir=/usr/lib/x86_64-linux-gnu\$|libdir=$STACK/usr/lib/x86_64-linux-gnu|" \
      -e "s|^includedir=/usr/include\$|includedir=$STACK/usr/include|" \
      -e "s|^includedir=/usr/include/x86_64-linux-gnu\$|includedir=$STACK/usr/include/x86_64-linux-gnu|" \
      "$pc"
  done
fi

# --- 6. the build environment -------------------------------------------
PCPATH=""
for d in "$STACK/usr/lib/x86_64-linux-gnu/pkgconfig" "$STACK/usr/share/pkgconfig"; do
  [ -d "$d" ] && PCPATH="${PCPATH:+$PCPATH:}$d"
done
cat <<EOF
export PKG_CONFIG_PATH="$PCPATH"
export CGO_LDFLAGS="-Wl,--allow-shlib-undefined -Wl,-rpath-link,$STACK/usr/lib/x86_64-linux-gnu"
# The webkit NEEDED tree (gst -> elfutils, manette -> evdev, …) resolves at
# RUNTIME on the target desktop: apt installs it for the .deb edition and
# the AppImage bundles it. At link time we let those be (--allow-shlib-
# undefined); -rpath-link still covers the chain ld wants to open.
export CGO_LDFLAGS_ALLOW="-Wl,.*"
export YOUNBLOCK_WEBKIT_STACK="$STACK"
EOF
echo "stack ready: $STACK" >&2
