# Younblock UI v3 - Main Screen Spec

Audience: the coding agent. Follow this file exactly. Colors, sizes and state values live in `younblock-tokens.css` (same folder). Do not invent values that are not in these two files; if something is missing, choose the simplest option that matches this spec and list it in `REPORT.md`.

## 1. Scope

- Redesign the **main screen** of the Android app (web UI in the app: Svelte + TypeScript). If the desktop (Wails) window shares the same Svelte component, apply the redesign there too so both stay consistent; if they are separate, do Android only and say so in the report.
- Do NOT change the Go engine logic. Only touch engine-facing code to read status events (see section 8).
- Keep the existing font family, the existing logo, the existing "..." menu sheet contents (About, How it works, Help) and the existing ring/dot drawing code where possible (recolor it, do not rewrite it).
- Out of scope: the background texture (deferred), server selection (removed), anything about the menu sheet.

## 2. Locked decisions

1. No green anywhere. Palette is charcoal + red + warm off-white, with one amber used only for caution.
2. Same background in every state. State is carried by the cap position, LED, ring, glow and words. Color is never the only signal.
3. Remove: "Privacy. Freedom.", "VPN CONNECTED", Change server, ping number.
4. The mechanical lever button stays. Remove the power icon from the cap. The cap shows only a word (START, CANCEL, RETRY, STOP).
5. Disconnect AND connect both work by tap or swipe (as today). Swipe direction: down to connect, up to disconnect.
6. "Open YouTube" card is shown only while connected.
7. The button must not move between states.

## 3. Layout (reference 360 x 800, build it flexibly)

Vertical order, top to bottom. Use a column layout with flexible spacers, not fixed offsets:

1. **Safe-area top inset.**
2. **Header row** (anchored to the top, 12px below the safe area): logo + wordmark "Younblock" on the left, "..." menu button on the right. Menu button is at least 48 x 48 and must have a visible outline: border 1.5px in `--yb-text` at 45% opacity (check non-text contrast is at least 3:1 against the background; raise opacity if not). No tagline.
3. **Status block** (centered in the free space above the button): see section 5 for content per state.
4. **Button zone**: ring + track + cap. Its vertical center sits at `--yb-button-center-y` (about 67% of screen height). Fixed position, identical in every state.
5. **Hint text** under the button zone, 14px, `--yb-text-dim`.
6. **Card slot** (always reserved, height `--yb-card-h`, margin `--yb-page-margin`): holds the Open YouTube card only while connected. When empty the slot stays blank so nothing jumps.
7. **Safe-area bottom inset.**

Horizontal margin everywhere: `--yb-page-margin`. Left edges of the logo, the status block text (if left-aligned items exist) and the card must share the same margin. Status block text is centered.

Short phones (height under 700px): the track height already shrinks via `clamp()`. Never go below 220px track height. If content still does not fit, shrink the status block spacing first, never the button.

## 4. The button

Parts: **ring** (decoration), **track** (dark pill), **cap** (red, moves inside the track).

Sizes (from tokens): track 112 x clamp(220, 32vh, 260); cap 96 x 124; ring diameter min(290, screen width - 24). Travel = track height - cap height - 2 x track padding (about 120px on a tall phone).

Cap positions:
- Disconnected, connecting-start, failed: cap at the **top** of the track. Chevrons (down) sit at the bottom of the track.
- Connected: cap at the **bottom**, visibly "pressed in": cap shadow shrinks and the track rim glows faintly red. Chevrons (up) sit at the top of the track.
- Connecting: cap animates from top to bottom, then waits at the bottom. If connecting fails, it springs back to the top (see failed).

Cap look: red `--yb-red` with a single highlight edge `--yb-cap-hi` and a dark rim `--yb-cap-lo` around the track (the rim keeps the cap readable on the background; the cap alone on the background is only 2.96:1). One LED slit at the cap's top edge in `--yb-led`, opacity from `--state-led-opacity`. Label: bold, letter-spaced, `--yb-fs-cap`, white. No power icon.

Hit area: the whole track plus `--yb-touch-padding` of invisible padding on all sides. Use pointer events, `touch-action: none` on the track, `user-select: none`.

Gestures:
- **Tap**: pointer up with under 10px movement and under 400ms. Toggles state (see below).
- **Swipe**: vertical drag moves the cap with the finger, clamped to the travel. Completes at `--yb-swipe-threshold` (60%) of travel; the cap then finishes the move. Released earlier: the cap springs back (use `--yb-t-cap`, small overshoot) and plays the "tick" haptic.
- Ignore horizontal movement. Debounce all triggers for 600ms.
- Direction: when the cap is at the top only a downward swipe counts; when it is at the bottom only an upward swipe counts.

Actions by state:
- Disconnected: tap/swipe down starts connecting.
- Connecting: tap cancels and returns to disconnected.
- Failed: tap/swipe down retries (goes to connecting).
- Connected: tap/swipe up disconnects.

Haptics: `tick` = 10ms when the swipe crosses the threshold and on spring-back; `confirm` = 25ms when connect/disconnect completes; `error` = pattern 30, 60, 30 ms when entering failed. Use the existing Android bridge for haptics if one exists; otherwise use `navigator.vibrate` if available; otherwise do nothing. Do not add new Android permissions without listing them in the report.

## 5. States

Use `data-state` on the screen root (`disconnected | connecting | failed | connected`) and optional `data-health="slow"` (only with connected). All values come from the token file. Copy is exact.

| | Disconnected | Connecting | Failed | Connected |
|---|---|---|---|---|
| Status block | Ghost YouTube icon (existing style, about 120px, very low opacity), label "YouTube", headline, small last-session line | Ghost icon, label, headline, sub line | Ghost icon, label, headline, sub line with icon | Label "YouTube", headline, sub line, timer, stats row |
| Headline | Not connected | Connecting... | Couldn't connect | Connected |
| Sub line | Last session: 21 min, 13.6 MB (only if the last session lasted over 1 minute; otherwise show nothing) | Opening a route to YouTube... | Warning icon + "Check your internet, then retry" (amber) | YouTube unblocked. In slow mode: caution icon + "Slow connection, tuning..." (amber) |
| Cap label | START | CANCEL | RETRY | STOP |
| Hint text | Tap or swipe down to connect | Tap to cancel | Tap or swipe down to retry | Tap or swipe up to disconnect |
| LED | Dim | Pulses 0.3 to 1 (`--yb-led-pulse`) | Two quick blinks, then dim | Steady, lit |
| Ring | Sparse dashed ring, `--yb-ring-idle` | Warm white dots with a rotating sweep | Dashed ring in amber | Dense dots, breathing 0.10 to 0.35 (`--yb-breathe`) |
| Glow behind cap | None | Faint red 12% | None | Soft red ember 28% |
| Extra motion | None | Sweep rotates | Shake 300ms (+/- 6px, 3 cycles) + `error` haptic | Slow breathing |
| Open YouTube card | Hidden | Hidden | Hidden | Shown |

Rules:
- Headline: `--yb-fs-headline`, bold, `--yb-text`. Sub line: `--yb-fs-sub`, color `--state-sub`.
- Any amber text must have an icon and words next to it (never amber alone).
- Timer (connected only): `--yb-fs-timer`, `font-variant-numeric: tabular-nums` so digits do not jitter. Keep the dimmed colons. Smaller than the headline.
- Stats row (connected only): down-arrow + downloaded, up-arrow + uploaded, `--yb-fs-stats`, `--yb-text-dim`, labeled with the arrows only (as today). Total "used" figure is removed; show the two numbers.
- Transitions between states: colors and glow use `--yb-t-color`; cap uses `--yb-t-cap`.

## 6. Open YouTube card (connected only)

- Full width minus margins, height `--yb-card-h`, radius matching the track's roundness, background `--yb-base-lift` at 60% with a 1px border in `--yb-text` at 20%.
- Left: YouTube app icon (existing asset). Middle: title "Open YouTube", second line "Launch the YouTube app" in `--yb-text-dim`. Right: one external-link icon only (no chevron).
- Tap: launch the YouTube Android app (package `com.google.android.youtube`) through the existing Kotlin wrapper/bridge. If the app is not installed, open `https://m.youtube.com` in the browser. If no bridge exists, add the smallest possible bridge method and mention it in the report.
- Appears with a 200ms fade when state becomes connected; disappears immediately when leaving connected.

## 7. Accessibility

- The track/cap is a button: `role="button"`, `aria-label` by state: "Connect to YouTube", "Cancel connecting", "Retry connecting", "Disconnect". Enter and Space toggle it. Visible focus ring (3px, `--yb-text`).
- The headline + sub line sit in an `aria-live="polite"` region so state changes are announced.
- Tap is an alternative to swipe everywhere, so no gesture is required.
- `prefers-reduced-motion`: no shake, no breathing, no sweep rotation, no LED pulse; show static versions of each state (the ring opacity and glow still differ per state).
- All text must pass 4.5:1 against its background. The token file lists the verified pairs. Do not use `--yb-red` for text; use `--yb-red-text`.

## 8. Engine hooks and honesty

- Drive `data-state` only from the engine's real status events: idle, connecting, connected, connect error. Do not use UI timers to guess failure. If the engine has no connect-error event, report that and implement failed from whatever error signal already exists.
- Slow mode (`data-health="slow"`): implement only if the engine already exposes a degraded or error-rate signal; otherwise leave it out and list it in the report. Never show a made-up number.
- **VPN badge check**: in the previous build the Android status bar still showed the VPN key icon while the app said "Not connected". Audit the disconnect path (VpnService/TUN teardown, foreground notification) and fix it so the badge disappears right after disconnect. If it cannot be reproduced from code, say what you checked.

## 9. Performance and battery

- No canvas or per-frame JavaScript animation. Use CSS/SVG animations only.
- Pause all animations when the page is hidden (`visibilitychange`) or the screen is off.
- Static background, no animated gradients, no blur/backdrop-filter.

## 10. Acceptance checklist (run it yourself, no manual steps for the user)

1. Build the Svelte frontend and run type-check/lint/tests that exist. Fix everything you broke.
2. Render each of the four states plus connected-slow at viewport sizes 360x640, 360x800 and 412x915. Take screenshots into `docs/ui-v3/screenshots/`. Confirm: button never moves between states, nothing overlaps, header is anchored at the top, no horizontal scroll.
3. Check computed contrast for headline, sub line, hint, stats, amber line, cap label and menu button outline against the tokens; fix any pair under its threshold.
4. Confirm gestures in a mobile-emulated browser: tap toggles, swipe past 60% completes, early release springs back, double-tap does not double-trigger.
5. Confirm reduced-motion mode shows the static versions.
6. Build the Android app if the toolchain is available; otherwise say exactly what you could not run.
7. Write `docs/ui-v3/REPORT.md`: what changed (file list), every deviation from this spec with the reason, items left out (for example slow mode), and the VPN badge finding.
