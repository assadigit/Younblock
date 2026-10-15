# Younblock UI rules (design.md)

Usage: give this to any agent or person who edits the Younblock screen. Values live in `tokens.css`; never type a color by hand.

## Identity
Dark, calm, tactile. One big red control. Green only means "connected". Dots are the only decoration.

## Color rules
1. **Red = control + brand.** The thumb, the logo, the not-connected ring dots, the Close button.
2. **Green = connected state.** Background, ring dots, LED. Never on buttons.
3. Red and green never touch: a dark track or gap (≥ 12px) sits between them.
4. Red text on dark uses `--red-400`. `--red-500` (#FF0000) is for fills, glows and dots, never text.
5. Never show meaning by color alone: keep START/STOP text, chevron direction, status words.
6. Keep neutral dark behind red. Do not put red on the green surface directly.

## Layout (360×760 canvas, 1rem = 16px, scales with window)
Header (menu left, logo centre) → timer + speeds (connected only) → "YouTube" + status → ping/message → pill → hint. Status block never moves between states.

## Components
- **Pill:** track 96×228, thumb 82×124, travel 92. Down = connect, up = disconnect. Click also works. Threshold 50%.
- **Thumb labels:** START, CANCEL (while connecting), STOP, WAIT (while disconnecting).
- **LED:** green lit = connected; dim = off; white blink = busy.
- **Rings:** green ×5 arcs around the thumb (connected); red ring around the track (not). Pulse slowly; faster while busy.
- **Menu button:** 40px translucent circle, four dots. Opens the About / How it works / Help sheet.

## Type
Sora Variable, bundled (never loaded from the internet: Google Fonts may be blocked where users are). Timer 40/700, status 28/700, label 14/500, stats 12/500, hint 11/500. Timer digits sit in fixed-width boxes.

## Motion
Thumb slide 380ms, background fade 700ms, pulse 2.4s. All of it stops under `prefers-reduced-motion`.

## Do
Add new states in one place (`Phase` + `STATUS_TEXT`). Test at 360×760 and 300×620. Keep text contrast ≥ 4.5:1.
## Don't
Add flags, maps, upsells, or a second accent color. Use red for "connected". Put text on the dot textures. Hardcode hex values in components.
