# Younblock Android UI: refinement spec (v7)

## 0. Rule for this job
This is a REFINEMENT of the original look, not a redesign. The previous round replaced depth, texture and state colors with flat tokens. That was a regression. Do not invent new styling. Port the values from `younblock-ui-reference.html` (CSS in that file is the source of truth) and look at `younblock-states-preview.png` for the target.

Step 1: check `git log` for the UI files (Svelte components + CSS) before the last UI round. If the old styling exists, restore it first, then apply section 3. If it does not exist, build straight from the reference file.

## 1. What must come back (regression list)
| Lost | Restore as |
|---|---|
| Depth / 3D | The pill is a PHYSICAL KEY CAP (see item 12). Track: recessed slot with inset shadows, NO red outline. |
| Texture | Halftone dot grid fading in toward the bottom + 5% grain overlay (both in reference `.tex`, `.grain`). |
| Ugly circles | REMOVE them completely. No dot rings, no dashed rings, no circles around the button anywhere. Status glow follows the track shape instead (see section 2). |
| Green / orange states | Whole-screen background changes with state (section 2). |
| Menu button | Filled glass circle with top highlight (not an outlined ring). |

## 2. States (exact tokens are in the reference `.app[data-state=...]` rules)
The pill rests at the BOTTOM of the track (thumb zone). Pulling it UP connects. Pulling it back DOWN disconnects.
There are NO rings or circles. Each state's life comes from a soft glow that hugs the track's own pill shape (`--c-aura`, keyframes `aura` / `aurabeat`).
| State | Background | Track glow | LED | Pill | Texts |
|---|---|---|---|---|---|
| idle | slate #3b3a4a to #26262e | none | grey | BOTTOM, slow red glow breathing, chevron up | "Not connected" / last session |
| connecting | same as idle | red-white pulse, 1.1 s, in sync with pill glow + label pulse | white blink | TOP | "Connecting…" / "Setting things up" |
| connected | sage-teal #54756a to #253a33 | green glow BREATHING slowly (3.4 s), LED breathes in sync | green + glow | TOP, chevron down | "Connected" / "YouTube unblocked." + timer + data |
| failed | warm brown-orange #7b5a3f to #2e2219 | amber double-beat (3.6 s) | amber + glow | BOTTOM, glow, chevron up | "Couldn't connect" / "Could not start. Try again." in amber |

Meaning of color: green = working, orange = needs attention, neutral = idle. The red pill stays red in every state (brand). Color never carries meaning alone: every state also has its own title, sub-line and button label.
Background change = cross-fade 700 ms, never a hard cut.

## 3. Improvements over the original look
1. Bottom-to-top lever: the pill rests at the bottom, where the thumb already is. It FOLLOWS the finger while dragging; releasing past 45% of the travel commits, otherwise it springs back. A plain tap also works. Disconnect is the reverse (pull down, or tap). Reference implements this in the pointer handlers (`pointerdown/move/up`).
2. Bigger thumb target and squircle geometry: track 128x272 with radius 34, cap 112x124 with radius 26 (concentric: cap radius = track radius minus 8px padding). Not capsules.
3. No circles at all: the old dot rings are deleted (not hidden). Do not leave dead code or unused assets for them.
4. Connecting pulses (track glow + pill glow + label, all in one 1.1 s rhythm), so the user never sees a frozen START.
5. Connected feels alive: the green glow and the LED breathe together, slowly. Keep it subtle.
6. Failed state is orange with a calm double-beat glow, not a red error.
7. Hint text raised to #d9d8e2 (contrast 10:1 on idle bg). Green sub-line uses #f0f7f3 because lighter greens failed 4.5:1 on the top of the green gradient.
8. Pill slide uses a springy easing; call `navigator.vibrate(12)` on connect and on disconnect (guard with `if (navigator.vibrate)`).
9. Open YouTube card: glass style, only while connected (unchanged behavior).
11. SWIPE-UP CUE (idle and failed only): three low-opacity chevrons in the empty top part of the track light up one after another, bottom to top, 1.8 s loop, peak opacity .45, drifting 8px upward as they fade. They fade out instantly while the user is dragging (`.dragging`), and are hidden in connecting and connected. Reduced motion: static at .4. Replaces the old single static up chevron (delete it). Connected keeps its single static down chevron.
12. PHYSICAL KEY CAP (copy the `--cap-*` variables and `.pill` rules from the reference exactly):
   - Top face: satin red gradient, 1px top highlight, hairline inner edge, very soft sheen (no glossy gel).
   - Thickness: a solid darker-red side wall 9px tall (box-shadow `0 9px 0 var(--edge)`) plus a 1px dark outline and a soft cast shadow on the track. The cap rests 18px above the track floor so the wall is visible.
   - Controls on the cap: status LED slot, 3 grip ridges (engraved look), label, thin line power icon. No dark disc behind the icon.
   - PRESS: while the finger is down (`.dragging`) the cap sinks 6px (CSS `translate`) and its wall shrinks to 3px, so it feels like pressing a real key. Release = it springs back. Also call `navigator.vibrate(8)` on pointerdown (guard with `if (navigator.vibrate)`).
   - The pull travel is 122px (`--travel`). If the cap size changes, recompute it (track height - cap height - 8 - 18).
10. Hints: "Tap or swipe up to connect" / "...down to disconnect" / "...up to retry".

## 4. Behavior
Swipe direction is now REVERSED compared to the old UI (old: down to connect). Update the gesture code, the chevrons (up chevron in idle/failed, down chevron when connected, none while connecting) and all hint strings. Do not touch the Go engine, VPN service, or any non-UI code. Map the status names the UI already receives to the four states above.  All animation is pure CSS (no JS animation loops).

## 5. Accessibility
Track is a button with aria-label = current hint. Status block has aria-live="polite". `prefers-reduced-motion`: no glow animation, instant transitions (already in reference).

## 6. Keep the app's existing font. Do not add a new font file.

## 7. Done when
- All four states match the preview image side by side (background gradient, pill depth, track glow, LED color). No circles anywhere.
- Cross-fading between states works; no flash of the wrong color on app start.
- No console errors; APK/web build passes.
- If the repo has a screenshot/preview script, run it and compare. Do not ask the user to check anything.
