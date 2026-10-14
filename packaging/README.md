# Younblock packaging — the Linux Mint formats

All three formats wrap **the same static Linux binary** (`cmd/core` built
with the embedded UI, `--web`). There is nothing format-specific inside
the app — one change to the main version lands in every edition the next
time `./build.sh` runs (DECISIONS.md D52/D53).

| Format | Script | Best for |
|---|---|---|
| `.deb` | `packaging/deb/build-deb.sh` | Installing Younblock *properly* on Linux Mint — menu entry, icons and binary in the standard places, tracked by dpkg. |
| `.AppImage` | `packaging/appimage/build-appimage.sh` | One portable file you download, make executable (`chmod +x`) and launch. Delete it to uninstall. |
| Flatpak | `packaging/flatpak/build-flatpak.sh` | Convenient installation and updates with fewer dependency conflicts. |

## One command builds everything

`./build.sh` (repo root) builds the frontend, all six binaries and then
all Linux packages from them. The individual scripts exist for
re-cutting a single format:

```sh
packaging/deb/build-deb.sh       amd64 dist/younblock-linux-amd64 1.4.1
packaging/deb/build-deb.sh       i386  dist/younblock-linux-386  1.4.1
packaging/appimage/build-appimage.sh x86_64 dist/younblock-linux-amd64 1.4.1
packaging/appimage/build-appimage.sh i686   dist/younblock-linux-386  1.4.1
packaging/flatpak/build-flatpak.sh    dist/younblock-linux-amd64 1.4.1
```

## Shared pieces

- `desktop/younblock.desktop.in` — one menu-entry template; `@EXEC@` and
  `@ICON@` are rendered per format (absolute path for the .deb, binary
  name for the AppImage, app id for Flatpak).
- `gen-icons.py` — redraws Ali's 32 px logo at high resolution and emits
  the hicolor icon set (16–512 px) for both icon names; run with
  `--check` to compare against the original pixel map. The generated
  icons are committed, so builds do not require PIL.
- `icons/` — the generated set (`younblock.png` for .deb/AppImage,
  `io.github.assadigit.Younblock.png` for Flatpak).

## Edition behavior notes

- Start-with-OS writes `~/.config/autostart/younblock.desktop` with an
  `Exec=` line that names the **edition's launcher** (`/usr/bin/younblock`,
  the `.AppImage` file, or `flatpak run <id>`) — see D54 in DECISIONS.md.
- The Flatpak edition keeps its settings in its sandbox
  (`~/.var/app/io.github.assadigit.Younblock`); the .deb and AppImage
  share `~/.config/Younblock`. Its permissions (session bus + host
  filesystem) are what let the system-proxy and autostart integrations
  reach the real desktop — recorded as D55.
