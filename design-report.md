# Visual Forensics: VPN reference → Younblock

Usage: read this to understand WHAT was measured and WHY choices were made. To build, use `agent-instructions.md`. Rules for future changes are in `design.md`.
Reference: one 640×640 JPEG showing two phone screens (connected / not connected). Mode: Replicate with changes.

## Executive summary
- **Style (confidence: medium, one small JPEG):** a tactile, hardware-like control (raised thumb in a recessed track) on dark vertical-gradient surfaces, with a halftone dot texture and dotted concentric rings. Neumorphism-adjacent, but not classic neumorphism: the thumb and track differ in tone, and shadows fall one way.
- **Defining traits:** one huge vertical power pill, centered status text, big timer, dots as the only decorative language, one calm color family per state.
- **Reference-specific (not the style):** the world map, flags, "Go Premium", location chip, IP row, gold crown.

## Observed (sampled with color_probe, JPEG-averaged)
| Item | Value |
|---|---|
| Connected surface top / mid / bottom | #6A8B80 / #517267 / #253A33 |
| Not-connected surface top / mid / bottom | #3C3B4B / #272730 / #26262F |
| Track (connected / not) | #334E45 / #1F1F29 |
| Thumb face (connected / not) | #5D857D / #565661 |
| LED green | #36BF51 body, #41E865 peak |
| Ring dots | green (connected), #F0F1F8 white (not) |
| Text | white, secondary at lower weight/opacity |
| Phone frame | 241×523 px, corner radius ≈ 40 px (ratio 0.46) |
| Pill track | ≈ 64×153 px → 96×228 at 360 px width (×1.494) |
| Thumb | ≈ 55×83 px → 82×124; travel ≈ 92 |

Not measurable from this image (Inferred): the font family (a wide geometric sans; Sora is the closest free match), hover/press states, animation timing.

## What changed from the reference
**Your three removals:** country flag (also the flag row and IP flag), Go Premium button, location chip.
**Consequences I decided:**
1. "Canada" label → "YouTube" (same slot: target, then state).
2. IP row → "Ping 42 ms" (Younblock has no remote IP).
3. World map → faint play mark, only in the not-connected state (the timer fills that space when connected).
4. Header: menu button kept; red logo + "Younblock" added in the centre to balance the empty right side.
5. Thumb is **YouTube red in both states**; LED meaning flipped (green = connected, off = not). The reference had it the other way round, which contradicts the "green = connected" rule you asked for.
6. Ring dots: red (not connected), green (connected).
7. Connected green darkened slightly: white text on the reference's top green is 3.7:1; on the new one 4.75:1.

## Red + green: it works, with rules
Red and green are opposites, so they clash when both are bright and touching. Here they stay friendly because: the green is a muted sage; a dark track always sits between red and green; each color has one job (**red = the control and brand, green = the state**). About 1 in 12 men cannot tell red from green well, so meaning is never carried by color alone: the thumb says START/STOP, the chevron shows direction, the status is written in words.

## Contrast (WCAG), measured
White on connected top 4.75:1, mid 6.96:1; white on not-connected 10.9–15:1; secondary text at its real positions 5.0–5.7:1; white on thumb 5.5:1; red-400 text on dark 4.6:1. Pure #FF0000 as text is only 3.75:1, so it is used for fills, glows and dots only.

## What NOT to copy
Flags, map, premium upsell, crown gold, the phone-shaped bezel and shadow (your window is the screen).

## Verification done
Rendered at 360×760, 411×890 and 300×620; automated tests pass for click, drag (past and short of halfway), keyboard, cancel, menu, the Wails bridge and a failing Go call; Go bridge tests pass with the race detector. See `preview/`.
