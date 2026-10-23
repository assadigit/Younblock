# UI v3 — implementation report

The new main screen per `docs/ui-v3/UI-SPEC.md`, using the values in
`docs/ui-v3/younblock-tokens.css` — imported into the Svelte frontend by
`src/pro.ts` (`import "../../docs/ui-v3/younblock-tokens.css"`); no value
is copied by hand (verified: the built bundle carries the token CSS
verbatim — `dist/assets/pro-*.css` contains `--yb-cap-hi:#e75655` etc.).
`docs/ui-v3/reference/` does not exist in this repository; the "current
look" screenshots live in `docs/screenshots/` (classic UI only) and were
reviewed. All work runs automatically; every non-obvious choice below is
the simplest option that matches the spec.

## What changed (file list)

**Frontend (the pro variant — what the Android WebView loads):**

| File | Change |
|---|---|
| `src/App.svelte` | rewritten as the v3 screen: flex column (safe insets → header → status block centered in the free space above the button → hint → reserved card slot), fixed button zone at `--yb-button-center-y`, `data-state`/`data-health` on the root, aria-live status region, `visibilitychange` pause, per-state copy |
| `src/lib/v3.ts` | new — the state model (`v3State` folds the engine Snapshot into the four states) and the exact per-state copy table |
| `src/lib/LeverButton.svelte` | new — the lever: track + cap + chevrons + LED, the full gesture model (tap, 60%-commit swipe, spring-back, 600ms debounce, direction gating, keyboard), the failed shake |
| `src/lib/RingV3.svelte` | new — the ring decoration; the drawing code is the existing `DotRings` approach (dots computed once along a circle), recolored and re-parameterized per state |
| `src/lib/OpenYouTubeCard.svelte` | new — the connected-only card (200ms fade in, removed instantly) |
| `src/lib/haptics.ts` | new — tick/confirm/error via `navigator.vibrate`, silent no-op where unsupported |
| `src/lib/backend.ts` | `GENERIC_CONNECT_ERROR` export; the HTTP host folds the engine's `degraded` phase into `Snapshot.degraded` (slow mode — a real engine event); the preview backend gained `?demo=<state>` pins for the acceptance runs |
| `src/lib/types.ts` | `Snapshot.degraded?: boolean` |
| `src/lib/DiagnosticsView.svelte` | fixed the 5 pre-existing type errors (they fail on HEAD too; re-verified before touching anything) |
| `src/lib/icons.ts` | `ExternalLink`, `AlertTriangle` added (v3 glyphs) |
| `src/lib/Timer.svelte` | unchanged (reused: tabular digits, dimmed colons); sized from App via `--yb-fs-timer` |
| `src/lib/MenuSheet.svelte` | unchanged (contents kept, per spec §1) |
| `src/app.css` | rewritten as the v3 base (the fixed-stage rem scaling of the old screen is gone; the v3 layout is flex + px tokens) |
| `src/pro.ts` | imports the v3 token file; keeps the legacy `tokens.css` import (the menu sheet and diagnostics view still read their surfaces from it) |
| `src/lib/PowerPill.svelte`, `src/lib/DotRings.svelte` | deleted (dead: nothing references them) |

**Android shell:**

| File | Change |
|---|---|
| `android/.../MainActivity.kt` | `PageBridge.openYouTube()` bridge method (launch intent → app; not installed → `https://m.youtube.com` in the browser) |
| `android/app/src/main/AndroidManifest.xml` | the `VIBRATE` permission (see below) |

**Docs:** `docs/ui-v3/UI-SPEC.md`, `docs/ui-v3/younblock-tokens.css` (the
uploaded files, in their spec-mandated place), this report, and
`docs/ui-v3/screenshots/` (16 PNGs).

## New Android permission (listed per spec §4)

- `android.permission.VIBRATE` — the haptics run through the WebView's
  `navigator.vibrate` (spec: "use the existing Android bridge for haptics
  if one exists; otherwise use navigator.vibrate"). No haptics bridge
  existed, so this is the smallest possible path. Normal permission (no
  runtime prompt). Nothing else was added.

## Engine honesty (spec §8)

- `data-state` is driven only by the engine's real status events: `idle` →
  disconnected, `connecting` → connecting, `connect error` → failed, `on` →
  connected. The one pre-existing local error signal (a refused Go call —
  e.g. a tap raced the engine's window) shows the failed look for its
  6-second life, exactly like the screen this one replaces; it never
  invents a failure a real engine event did not report, and no UI timer
  guesses anything.
- Slow mode (`data-health="slow"`) is real: the engine's health monitor
  flaps the serving run between `on` and `degraded`
  (`internal/engine/health.go`); the HTTP/SSE host (which is what the
  Android WebView talks to, ADR-0005) folds it into `Snapshot.degraded`.
  The Wails bridge has no degraded state to read, so the Wails editions
  never show slow mode — noted, not faked.
- The generic engine failure ("Could not connect. Try again.") maps to
  the spec's exact failed copy; a *more specific* engine error (the
  NO_TARGET_APPS explanation) wins over the spec line so the person gets
  the words that actually match their situation.

## VPN badge audit (spec §8, the previous build's complaint)

Full teardown trace, from code (not reproducible on a device here — no
Android runtime in this environment):

1. `Engine.Stop()` always emits `stopping` then `off`
   (`internal/engine/engine.go` — `setPhaseLocked(PhaseStopping)` →
   `emitState` → ... → `setPhaseLocked(PhaseOff)` → `emitState`). There is
   no path to "off" that skips the TUN teardown on the engine side.
2. `YounblockVpnService`'s state listener, on `off`/`error`:
   `unbindTun()` → `mobile.Engine.StopTun()` → the stack stops and
   `wire.Close()` closes the TUN fd — the interface goes away, which is
   what clears the system's key icon — then `stopSelfHonest()` runs
   `stopForeground(STOP_FOREGROUND_REMOVE)` + `stopSelf()`, so the
   foreground notification (the other thing that pins the badge) is
   removed too. `onRevoke` and `onDestroy` repeat the same unwind
   idempotently; `running` is reset in `onDestroy`.
3. No static path was found in which the engine reports off (the UI shows
   "Not connected") while the fd or the service survives.

**Finding, documented not fixed (untestable here):** the one remaining
window is the rapid disconnect→reconnect race — a new connect run that
re-binds the TUN *inside* the service's stop→destroy window can be torn
down by the just-posted `stopSelf()` (the "orphan" race: the engine is
connecting, the service is dead, `MainActivity.reconcile()` restarts the
service on resume and the engine is held at the start gate, so the app
cannot lie about being connected — but the badge can linger in the gap).
Reproducing and fixing it needs a device with adb; recorded here so the
next field round can target it.

## Deviations from the spec (each the simplest option that matches)

- **Copy dots:** the spec's "Connecting...", "Opening a route to
  YouTube...", "Slow connection, tuning..." are rendered with three
  periods exactly as written; the one carried-over line (the warm gate's
  "Getting the first video ready…", M8-an) keeps its existing ellipsis
  character. Both read identically; kept to avoid rewriting shipped copy.
- **Warming (the M8-an hold):** while the engine serves but its video lane
  is still settling, the screen keeps the connecting look (cap waiting at
  the bottom, CANCEL) — that is what the hold means and the v3 states have
  no fifth slot for it. It only applies where something waits on it
  (Android); desktops connect the old way.
- **`disconnecting`:** the engine's `stopping` window (~sub-second) maps to
  the disconnected look — the cap springs home while the engine winds
  down; a Start tapped in that window is refused by the bridge and
  ignored (never a fake failure).
- **The Open YouTube card's icon:** the repo has no YouTube bitmap asset,
  so the card uses the existing in-house play-mark glyph (the same one as
  the ghost icon and the brand) at `--yb-red`. Same for the ghost icon
  itself — "existing style" is that glyph; there is no other.
- **Sizes the token file does not carry** (chosen once, listed here): the
  LED slit 28×4px; the label "YouTube" at `--yb-fs-stats` weight 500; the
  brand row 15px/600; the ghost icon 120px at 5% opacity (96px under
  700px height per the short-phone rule); shadow alphas (`rgba(0,0,0,…)` —
  depth rendering, in the style of the code this replaces — colors
  themselves are only ever token values); the chevrons 22px in
  `--yb-text-dim` at 0.9 inside the track; the ring's dot counts (30/48/64
  + a 14-dot 70° sweep) and the sweep period (2.6s) — the tokens carry the
  *state* motion values, not these decoration densities.
- **The menu sheet and diagnostics view** are untouched (spec §1): they
  still read the legacy `tokens.css` surfaces, which stays imported for
  exactly that.
- **The classic desktop variant** (`src/classic/`) is a separate component
  tree (the Wails editions bake `VITE_UI_VARIANT=classic`), so per spec §1
  the redesign applies to the Android/web "pro" screen; classic is
  unchanged and reported here.
- **Server selection / ping / "Privacy. Freedom." / "VPN CONNECTED" /
  power icon:** all gone with the old screen (spec §2.3/2.4).
- **Root font scaling:** the old screen scaled the root font to a fixed
  stage; v3 drops that (it is flex + px tokens). The kept menu sheet
  renders at the 16px base everywhere — it was already 16px at the design
  width.

## Items left out

- **Background texture** (spec §1: deferred by the spec itself).
- **Go engine changes:** none — the audit above is read-only, per spec §1.
- **Frontend lint:** no lint script exists in `frontend/package.json`
  (dev/build/preview/check only) — nothing to run; noted instead of
  inventing one.

## Acceptance results (spec §10, all run — no manual steps)

1. **Build + checks:** `vite build` ✓; `svelte-check` 0 errors / 0
   warnings (including the 5 pre-existing DiagnosticsView errors, now
   fixed); `go build ./...` ✓; **full Go test suite green** — 21 packages
   `ok`, including `internal/webui` and `mobile` against the new bundle
   (46s) and `internal/proxy` (80s); `gofmt -l` = 115 files, the exact
   pre-existing baseline (no `.go` file was touched).
2. **Screenshots:** 15 (5 states × 360×640, 360×800, 412×915) + 1
   reduced-motion, in `docs/ui-v3/screenshots/`. Layout invariants
   asserted per shot: the button's position is byte-identical across all
   states at every viewport; header anchored at 12px; no horizontal
   scroll; status/hint/card/ghost never overlap the button or each other.
   (On 360×640 the ring's lower arc passes 14px behind the card slot —
   decoration behind content, the same layering rule as the ghost icon;
   content never overlaps content.)
3. **Contrast (computed, sRGB):** headline 13.26:1 (base) / 9.99:1
   (lift); sub/hint/stats 5.71:1 (lift) / 7.58:1 (base); amber 8.21:1
   (base) / 6.19:1 (lift); cap label white-on-red 5.07:1; menu outline
   (1.5px `--yb-text` @45%) **3.37:1** on lift / 4.47:1 on base — ≥3:1
   non-text, so the spec's "raise opacity if not" does not trigger.
   All text pairs ≥4.5:1.
4. **Gestures (mobile-emulated, hasTouch):** tap toggles connect/cancel;
   the swipe commits at the 60% crossing *while still held*; early
   release springs back with the cap following the finger during the
   drag; wrong-direction and horizontal drags are ignored; double-tap
   inside 600ms does not double-trigger; Enter and Space toggle.
5. **Reduced motion:** LED pulse, sweep rotation and the failed shake all
   compute to ~0ms; the ring's static opacity still differs per state
   (0.8 idle vs 0.35 connected).
6. **Hidden page:** `visibilitychange` → `data-hidden=true` → every
   animation's computed `animation-play-state` is `paused`.
7. **Could not run here:** the Android APK build during the *acceptance*
   pass (the toolchain was still assembling) — it is attempted in the
   release step; a device run of the badge race (above) needs adb.

## Release note

This round ships as the single commit "UI v3: new main screen". The debug
build that carries it (versionCode 33, mobile string 1.6.6-debug.34) is
released separately per the repo's release convention.

---

# The one-UI round — every version ships the same screen (2026-10-23)

The owner's next ruling, verbatim in intent: *from now on, release all
versions in the same release; update the UI of all versions — they must
be consistent, so when we change the UI all versions change the same;
currently the APK UI differs from the other versions.* Two decisions
land with it (DECISIONS.md D110/D111): the classic editions retire (one
screen everywhere) and every version rides one GitHub release (built by
the new `release-unified.sh`).

## Why the APK differed from the other versions (the diagnosis)

Three facts, no guessing:

1. **The APK and the desktop editions already share one screen source**
   (`frontend/src/App.svelte`, served by the same webui bundle — the
   desktop line through Wails, the APK through its WebView). The UI v3
   round changed that one screen, so any desktop build from the current
   tree would already match the APK.
2. **The divergence was release-shaped, not code-shaped:** the desktop
   downloads stood still at the v0.7.6 release (Oct 2026's UI v2 screen
   in `younblock.exe`/`.deb`/AppImage) while the APK line ran 34 debug
   releases — the last of which (debug.34) carries UI v3. A person
   comparing the newest APK with the newest desktop download sees two
   different screens because the desktop download is five weeks of
   rounds behind, not because the code differs.
3. **The classic editions were a standing, by-design divergence:** a
   whole second component tree (`frontend/src/classic/`) baked into
   separate released binaries (`younblock-classic.exe`, the Classic deb
   and AppImage) that no UI rework ever touches — exactly the "one
   version's UI changes and the others don't" shape the ruling ends.

## What changed (file list)

**One screen — the classic variant retires (D110):**

| File | Change |
|---|---|
| `src/main.ts` | rewritten: no variant dispatcher — one boot path that loads the v3 screen for every edition; `?ui=classic` is now an inert no-op (verified: same screen, same state, either way) |
| `src/classic/` (tree) | deleted (git history keeps it). The one shared file it held, `lib/clipboard.ts` (the honest clipboard ladder), **moved** to `src/lib/clipboard.ts` — `DiagnosticsView.svelte` imports it from there now |
| `src/vite-env.d.ts` | `VITE_UI_VARIANT` dropped (nothing bakes a variant anymore) |
| `src/pro.ts`, `src/App.svelte`, `src/lib/MenuSheet.svelte`, `src/lib/DiagnosticsView.svelte`, `src/lib/types.ts` | header/import comment updates only (the variant references they carried are gone); zero rendering logic touched |

**Desktop parity — the Wails bridge gains the phone's honest signals
(engine untouched, D110's "consistent" read to its end):**

| File | Change |
|---|---|
| `ui_bridge.go` | `Snapshot` gains optional `degraded`/`warming` (omitempty, so old hosts' JSON is unchanged); a new optional `HealthSource` engine capability, polled on the existing once-a-second stats cadence — a bridge without it behaves exactly as before |
| `engine_adapter.go` | implements `Health()`: reads the engine's existing `State.Phase` (`degraded` → slow mode) and `State.Warm` (the M8-an gate) — pure observation, zero engine change |

The v3 spec §8 audit already established these are real engine events;
until now only the HTTP/SSE host (the APK's path) folded them into
snapshots, so the desktops could never show slow mode. Now every
edition's screen reads the same signals from the same engine.

**The build lanes:**

| File | Change |
|---|---|
| `build.sh` | the classic lanes are gone (no `VITE_UI_VARIANT` bakes, no `windowSize=360x760` overrides, no classic deb/AppImage); every artifact list updated |
| `build.bat` | the same retirement for the Windows-native lane |
| `main.go` | header + `windowSize` comment: one edition, the v3 design window (360×640) as the only default |
| `release-unified.sh` | **new** — the unified release lane (D111): builds EVERY version (build.sh + build-android.sh + the offline pack, best-effort), renames the APK per D103, pushes the tag, creates ONE GitHub release, uploads every artifact, re-downloads and byte-compares each one, prints the release record for PROGRESS.md. The token rides `GITHUB_TOKEN` (never the repo) |

**Versioning (the unified line):** `AppVersion` 1.6.6 → **1.6.7**,
`wails.json` 1.6.7, `versionCode` 33 → **34**, `versionName` 1.6.7,
mobile string **1.6.7-debug.1**. The base moved with the unified line,
so the debug counter restarts at .1 under the new base; the versionCode
keeps climbing monotonically (34 > 33), so the APK still installs over
debug.34 in place (the pinned D85 certificate). The round's comments in
`android/app/build.gradle.kts` and `mobile/mobile.go` record all of it.

## What was verified this round (everything runnable here)

- `svelte-check`: **0 errors / 0 warnings**.
- `vite build`: clean; the bundle carries the v3 tokens verbatim
  (`--yb-cap-hi:#e75655` in `dist/assets/*.css`), one JS entry, **zero**
  references to the classic tree.
- `go build ./...` ✓ and the **full `go test ./...` green — 21
  packages ok**, including the root package (the bridge changes), the
  engine suite (18.7 s), `internal/webui` and `mobile` (46 s) against
  the new bundle.
- `gofmt -l` = **115 files, the exact pre-existing baseline** (the three
  Go files this round touched were formatted; stash-diff-proven).
- Playwright smoke of the one boot path: all four states render at
  360×640, 360×800 and 412×915; `?ui=classic` is inert (same screen,
  same state). Fresh screenshots: `docs/ui-v3/screenshots/
  oneui-disconnected-360x800.png`, `oneui-connected-360x800.png`,
  `oneui-desktop-360x640.png` (the Windows/Linux window size).

## The release (D111's first run)

The unified release — tag **v0.8.0**, every version in it — is created
by `release-unified.sh` after this commit; the artifact record (release
id, asset ids, sizes, SHA-256 digests, the byte-identical re-download
verifications) lands in PROGRESS.md and CHANGELOG.md with the docs
follow-up commit, per the repo's convention.

## Deviations and notes

- **`?ui=classic` is left inert rather than rejected** — the simplest
  option: an old bookmark or muscle-memory URL renders the same screen
  everyone gets; no redirect, no error page.
- **The historical README "What's new" sections keep their Classic
  mentions** — they describe the releases that actually shipped them;
  every *current-guidance* section now states the one-screen policy.
- **The desktop Wails bridge polls health once a second** (the stats
  cadence it already had) rather than subscribing to engine events —
  slow-mode onset may show up to ~1 s late on desktop; the decoration
  is not worth new plumbing. The APK's SSE path stays event-driven.
- **No new tests were written** (the session's standing rule); the
  existing suite — including the bridge's own tests — runs green
  against the changes.
