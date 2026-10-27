//go:build windows

// screen_windows.go — the lite desktop edition's screen: the v3 main
// screen (docs/ui-v3/younblock-tokens.css + frontend/src/App.svelte,
// LeverButton.svelte, OpenYouTubeCard.svelte, MenuSheet.svelte),
// redrawn natively — GDI+ shapes and gradients over a GDI-text layer,
// both into one double-buffered memory DC.
//
// Every visual value below is ported from the tokens file: the state
// gradient backgrounds with their 700 ms cross-fade, the halftone dot
// texture (the 5% grain is web-only — D119: its mix-blend-mode overlay
// has no GDI+ equivalent and a plain-alpha copy reads as static), the
// glass header with the red brand tile, the status column (ghost play
// tile, eyebrow, 32 px hero word, sub line, tabular timer, arrowed byte
// totals), the Open YouTube card, and the signature key-cap lever — the
// recessed 128x272 track with the sliding 112x124 satin-red cap, its
// 9 px side wall, grip ridges, LED, power icon and letterspaced cap
// label, driven by the same gesture rules (tap, drag-follow, 45%
// commit threshold, springy 550 ms travel) and the same breathing
// auras, on the same timings. The menu sheet slides up from the
// bottom in the reference's surfaces.
//
// Text stays on plain GDI (weights 400-700, ClearType, per-character
// drawing where the design letterspaces); every shape is a
// polygon-sampled GDI+ path (see gdiplus_windows.go for why the
// float-arg entry points are avoided).
package main

import (
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/assadigit/Younblock/internal/config"
	"golang.org/x/sys/windows"
)

// ---- the design tokens (docs/ui-v3/younblock-tokens.css, ported) ----

var (
	tokText     = argb(0xff, 0xF4, 0xF0, 0xEE) // --yb-text
	tokHint     = argb(0xff, 0xD9, 0xD8, 0xE2) // --yb-hint
	tokSheetBg  = argb(0xff, 0x1D, 0x1C, 0x26) // --sheet-bg
	tokSheetRse = argb(0xff, 0x26, 0x25, 0x32) // --sheet-raised
	tokRed400   = argb(0xff, 0xE0, 0x6B, 0x6B) // --red-400 (icons)
	tokRed500   = argb(0xff, 0xFF, 0x00, 0x00) // --red-500 (toggle on)
)

// bgStops is a state's background gradient (the .bg layers, exact).
var bgStops = map[ScreenState]struct {
	colors    []uint32
	positions []float32
}{
	ScreenIdle: {
		colors:    []uint32{0xff3b3a4a, 0xff2d2d37, 0xff26262e, 0xff26262e},
		positions: []float32{0, 0.35, 0.70, 1},
	},
	// connecting rides the idle background (the CSS keeps .bg.idle
	// lit through the connecting state).
	ScreenConnecting: {
		colors:    []uint32{0xff3b3a4a, 0xff2d2d37, 0xff26262e, 0xff26262e},
		positions: []float32{0, 0.35, 0.70, 1},
	},
	ScreenConnected: {
		colors:    []uint32{0xff54756a, 0xff46665b, 0xff2d443c, 0xff253a33},
		positions: []float32{0, 0.38, 0.78, 1},
	},
	ScreenFailed: {
		colors:    []uint32{0xff7b5a3f, 0xff664a33, 0xff3a2b20, 0xff2e2219},
		positions: []float32{0, 0.38, 0.78, 1},
	},
}

// stateColors is the per-state token set (the data-state rules).
var stateColors = map[ScreenState]struct {
	LED      uint32 // --c-led
	LEDGlow  uint32 // --c-led-glow
	Sub      uint32 // --c-sub
	Aura     uint32 // --c-aura
	PillGlow uint32 // --pill-glow
}{
	ScreenIdle: {
		LED: argb(0xff, 0x6b, 0x6b, 0x78), LEDGlow: 0,
		Sub: argb(0xff, 0xC9, 0xC8, 0xD4), Aura: 0,
		PillGlow: argb(0x66, 0xFF, 0x28, 0x1E), // rgba(255,40,30,.40)
	},
	ScreenConnecting: {
		LED: argb(0xff, 0xFF, 0xFF, 0xFF), LEDGlow: argb(0xb3, 0xFF, 0xFF, 0xFF),
		Sub: argb(0xff, 0xC9, 0xC8, 0xD4), Aura: argb(0x8c, 0xFF, 0x5A, 0x46), // rgba(255,90,70,.55)
		PillGlow: argb(0x4d, 0xFF, 0x28, 0x1E),
	},
	ScreenConnected: {
		LED: argb(0xff, 0x39, 0xB9, 0x4C), LEDGlow: argb(0xd9, 0x39, 0xB9, 0x4C),
		Sub: argb(0xff, 0xF0, 0xF7, 0xF3), Aura: argb(0x8c, 0x39, 0xB9, 0x4C),
		PillGlow: argb(0x38, 0xFF, 0x28, 0x1E),
	},
	ScreenFailed: {
		LED: argb(0xff, 0xFF, 0xA7, 0x26), LEDGlow: argb(0xd9, 0xFF, 0xA7, 0x26),
		Sub: argb(0xff, 0xFF, 0xD9, 0xA0), Aura: argb(0x8c, 0xFF, 0xA7, 0x26),
		PillGlow: argb(0x4d, 0xFF, 0x28, 0x1E),
	},
}

// The slow variant of connected keeps the failed amber sub color.
var slowSub = argb(0xff, 0xFF, 0xD9, 0xA0)

// The cap face gradient (the .pill background, exact stops).
var capFaceColors = []uint32{0xffEC3D34, 0xffD62A20, 0xffC8231D}
var capFacePos = []float32{0, 0.55, 1}

// tokEdge is the cap's side wall (--edge #8f130e).
const tokEdge = 0xff8f130e

// ---- layout constants (logical px; scaled by the window's DPI) ----

const (
	lyMargin    = 16
	lyHeaderTop = 12
	lyHeaderH   = 48
	lyBrandW    = 34
	lyBrandH    = 24
	lyBrandR    = 7
	lyTrackW    = 128
	lyTrackH    = 272
	lyTrackR    = 34
	lyCapW      = 112
	lyCapH      = 124
	lyCapR      = 26
	lyPad       = 8
	lyPadBottom = 18
	lyTravel    = lyTrackH - lyCapH - lyPad - lyPadBottom // 122
	lyTouch     = 16
	lyCardH     = 64
	lyCardR     = 22
	lyGhostW    = 92
	lyGhostH    = 66
	lyGhostR    = 20
	lySheetW    = 360
	lySheetR    = 24
)

// ---- the paint ----

// layout holds the frame's rectangles in device pixels.
type layout struct {
	brandMark, menuBtn, track, hit, card, hint wRect
	sheetW, sheetRowsY, sheetH                 int32
	tall                                       bool
}

// paintScreen draws one full frame into the memory DC.
func (w *liteWindow) paintScreen(hdc uintptr, width, height int32) {
	now := time.Now().UnixMilli()
	sc := w.scr
	g := newGraphics(hdc)
	defer deleteGraphics(g)

	// The GDI+-less fallback (gdiplus.dll absent — a box from before
	// XP): flat background + the words, still honest, never blank.
	if g == 0 {
		w.fallbackPaint(hdc, width, height, sc)
		return
	}

	w.drawBackground(g, width, height, sc, now)
	w.drawTextures(g, width, height)
	w.drawLever(g, sc, now)
	w.drawHeader(g, hdc, width)
	w.drawStatus(g, hdc, width, height, sc, now)
	w.drawHint(hdc, width)
	w.drawCard(g, hdc, sc, now)
	if w.sheetPos(now) > 0.001 {
		w.drawSheet(g, hdc, width, height, now)
	}
	w.drawFocusRings(g, sc, now)
}

// fallbackPaint is the no-GDI+ screen: charcoal, the hero word, the
// cap word and the hint — usable, honest, rare.
func (w *liteWindow) fallbackPaint(hdc uintptr, width, height int32, sc Screen) {
	w.fillRectGdi(hdc, 0, 0, width, height, 0xff26262e)
	rc := wRect{w.px(lyMargin), height / 3, width - w.px(lyMargin), height/3 + w.px(40)}
	w.gdiText(hdc, w.fHeadline, sc.Headline, rc, dtCenter|dtVCenter|dtSingleLine, tokText)
	rc2 := wRect{w.px(lyMargin), height/3 + w.px(48), width - w.px(lyMargin), height/3 + w.px(72)}
	w.gdiText(hdc, w.fSub, sc.Sub, rc2, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis, tokHint)
	rc3 := wRect{w.px(lyMargin), height - w.px(60), width - w.px(lyMargin), height - w.px(36)}
	w.gdiText(hdc, w.fHint, sc.Hint, rc3, dtCenter|dtVCenter|dtSingleLine, tokHint)
}

// drawBackground paints the state gradient with the 700 ms cross-fade
// (never a hard cut — the reference's stacked .bg layers).
func (w *liteWindow) drawBackground(g uintptr, width, height int32, sc Screen, now int64) {
	from, to := bgStops[w.prevBg], bgStops[sc.State]
	// The outgoing layer at full strength.
	if br := newVGradient(0, 0, width, height, from.colors, from.positions); br != 0 {
		fillRectI(g, br, 0, 0, width, height)
		deleteBrush(br)
	}
	// The incoming layer fades over it.
	t := animPhase(now, w.bgFadeStart, 700)
	if t > 0 {
		colors := to.colors
		if t < 1 {
			colors = make([]uint32, len(to.colors))
			for i, c := range to.colors {
				colors[i] = scaleAlpha(c, t)
			}
		}
		if br := newVGradient(0, 0, width, height, colors, to.positions); br != 0 {
			fillRectI(g, br, 0, 0, width, height)
			deleteBrush(br)
		}
	}
}

// drawTextures blits the pre-rendered halftone dot layer (fading in
// toward the bottom, 7 px grid, 1 px dots). The reference's second
// texture — the 5% grain — is web-only (D119): it rides a
// mix-blend-mode overlay in the CSS, and a plain-alpha approximation
// reads as static (the owner's measured noise was 5x the reference's;
// GDI+ has no blend modes, so the honest port drops the layer and keeps
// the smooth gradient the browser shows underneath it).
func (w *liteWindow) drawTextures(g uintptr, width, height int32) {
	if w.dotBmp != 0 {
		drawImageTile(g, w.dotBmp, 0, 0, width, height, 0, 0, width, height)
	}
}

// drawHeader paints the glass header: the red brand tile with its play
// triangle and glow, the wordmark, and the "..." glass circle.
func (w *liteWindow) drawHeader(g uintptr, hdc uintptr, width int32) {
	m := w.px(lyMargin)
	mk := w.lay.brandMark
	mw, mh := mk.Right-mk.Left, mk.Bottom-mk.Top

	// The tile's soft red glow (0 0 14px rgba(255,40,30,.55)).
	w.glowRRect(g, mk.Left, mk.Top, mw, mh, w.px(lyBrandR), w.px(7), argb(0x8c, 0xFF, 0x28, 0x1E))

	// The tile: vertical red gradient (#f0392c -> #c3180f).
	if br := newVGradient(mk.Left, mk.Top, mw, mh, []uint32{0xfff0392c, 0xffc3180f}, []float32{0, 1}); br != 0 {
		w.fillRRectB(g, mk.Left, mk.Top, mw, mh, w.px(lyBrandR), br)
		deleteBrush(br)
	}
	// The inset top highlight (inset 0 1px 0 rgba(255,255,255,.35)).
	w.fillRRect(g, mk.Left+w.px(1), mk.Top+w.px(1), mw-w.px(2), w.px(2), w.px(2), argb(0x59, 0xFF, 0xFF, 0xFF))

	// The white play triangle (the CSS border triangle: 9/5/5 at
	// left 13, top 7 of the 34x24 tile).
	tri := []wPoint{
		{mk.Left + w.px(13), mk.Top + w.px(7)},
		{mk.Left + w.px(13), mk.Top + w.px(17)},
		{mk.Left + w.px(22), mk.Top + w.px(12)},
	}
	w.fillPoly(g, tri, argb(0xFF, 0xFF, 0xFF, 0xFF))

	// The wordmark (17 px / 600).
	tr := wRect{mk.Right + w.px(10), mk.Top - w.px(6), width - m - w.px(60), mk.Bottom + w.px(6)}
	w.gdiText(hdc, w.fBrand, "Younblock", tr, dtLeft|dtVCenter|dtSingleLine, tokText)

	// The "..." glass circle.
	w.drawMenuButton(g)
}

// drawMenuButton paints the filled glass circle with its top highlight,
// drop shadow and three dots.
func (w *liteWindow) drawMenuButton(g uintptr) {
	rc := w.lay.menuBtn
	d := rc.Right - rc.Left
	cx, cy := (rc.Left+rc.Right)/2, (rc.Top+rc.Bottom)/2

	// The drop shadow (0 6px 14px rgba(0,0,0,.35)) — a soft halo offset
	// down: three stacked solid ellipses whose cumulative edge weight is
	// .35 fading to .12 at the rim (D119: the layered-solid construction
	// renders identically on every GDI+ — the old single path-gradient
	// circle could diverge into a white ring on real Windows).
	sr := d / 2
	for _, s := range []struct {
		e int32
		a uint32
	}{{w.px(5), 0x1E}, {w.px(3), 0x24}, {w.px(1), 0x2B}} {
		w.fillEllipse(g, cx, cy+w.px(3), sr+s.e, argb(s.a, 0x00, 0x00, 0x00))
	}

	// The glass fill: rgba(255,255,255,.2) -> .08.
	if br := newVGradient(rc.Left, rc.Top, d, d, []uint32{0x33FFFFFF, 0x14FFFFFF}, []float32{0, 1}); br != 0 {
		w.fillEllipseB(g, cx, cy, d/2, br)
		deleteBrush(br)
	}
	// The inset top highlight (inset 0 1px 0 rgba(255,255,255,.28)):
	// a bright arc band over the top half.
	top := arcPoints(cx, cy, d/2, d/2, 190, 160, 14)
	for i := range top {
		top[i].Y += w.px(1)
	}
	w.strokePolyline(g, top, w.px(2), argb(0x47, 0xFF, 0xFF, 0xFF))

	// Three dots, 20 px type with 1 px letterspacing — dot radius
	// ~1.8, spacing 9.
	r := w.px(2)
	if r < 1 {
		r = 1
	}
	off := []int32{-w.px(9), 0, w.px(9)}
	for _, dx := range off {
		w.fillEllipse(g, cx+dx, cy, r, argb(0xFF, 0xFF, 0xFF, 0xFF))
	}
}

// drawStatus paints the centered status column: the ghost play tile
// (fading away while connected), the eyebrow, the hero word, the sub
// line, the tabular timer and the arrowed byte totals.
func (w *liteWindow) drawStatus(g uintptr, hdc uintptr, width, height int32, sc Screen, now int64) {
	// The free band between the header and the lever's touch zone.
	top := w.px(lyHeaderTop+lyHeaderH) + w.px(12)
	bottom := w.lay.hit.Top + w.px(4)
	if bottom <= top {
		bottom = top + 1
	}

	// The animated ghost height (66 -> 0 while connecting/connected).
	ghostT := w.ghostAlpha
	gapAfterGhost := w.px(6)
	if w.lay.tall {
		gapAfterGhost = w.px(12)
	}
	gh := int32(float64(w.px(lyGhostH)) * clamp01(ghostT))

	// The stack (heights in device px).
	hLabel := w.px(21)
	hHead := w.px(38)
	hSub := w.px(22)
	if w.lay.tall {
		hSub = w.px(25)
	}
	hTimer, hStats := w.px(21), w.px(22)
	stack := gh
	if gh > 0 {
		stack += gapAfterGhost
	}
	stack += hLabel + w.px(4) + hHead + hSub
	if sc.ShowTimer {
		stack += w.px(6) + hTimer
	}
	if sc.ShowStats {
		stack += w.px(6) + hStats
	}
	y := top + (bottom-top-stack)/2
	if y < top {
		y = top
	}

	// The ghost tile.
	if gh > w.px(3) && ghostT > 0.02 {
		af := clamp01(ghostT)
		gw := int32(float64(w.px(lyGhostW)) * af)
		gx := (width - gw) / 2
		// The glass tile (fill .07).
		w.fillRRect(g, gx, y, gw, gh, int32(float64(w.px(lyGhostR))*af), argb(uint32(255*af*0.07), 0xFF, 0xFF, 0xFF))
		// The inset top highlight (.10).
		w.fillRRect(g, gx+w.px(2), y+w.px(1), gw-w.px(4), w.px(2), w.px(2), argb(uint32(255*af*0.10), 0xFF, 0xFF, 0xFF))
		// The faint play triangle (.12; amber .30 while failed).
		tcol := argb(uint32(255*af*0.12), 0xFF, 0xFF, 0xFF)
		if sc.State == ScreenFailed {
			tcol = argb(uint32(255*af*0.30), 0xFF, 0xBE, 0x5A)
		}
		// The CSS triangle: left 37, top 19, 24 wide, 28 tall
		// (14/14 borders) in the 92x66 tile — scaled by the tile's
		// current collapse.
		s := float64(gh) / float64(w.px(lyGhostH))
		tx0 := gx + int32(float64(w.px(37))*s)
		ty0 := y + int32(float64(w.px(19))*s)
		tx1 := gx + int32(float64(w.px(61))*s)
		tym := y + int32(float64(w.px(33))*s)
		ty1 := y + int32(float64(w.px(47))*s)
		w.fillPoly(g, []wPoint{{tx0, ty0}, {tx0, ty1}, {tx1, tym}}, tcol)
	}
	if gh > 0 {
		y += gh + gapAfterGhost
	}

	// The eyebrow ("YouTube"), 15/500, letterspacing 0.3 — plain
	// centered at these sizes.
	subCol := w.subColor(sc, now)
	w.gdiText(hdc, w.fEyebrow, "YouTube", wRect{0, y, width, y + hLabel}, dtCenter|dtVCenter|dtSingleLine, scaleAlpha(subCol, 0.85))
	y += hLabel + w.px(4)

	// The hero word: 32/700 with the reference's soft text shadow.
	w.gdiText(hdc, w.fHeadline, sc.Headline, wRect{0, y, width, y + hHead}, dtCenter|dtVCenter|dtSingleLine, argb(0x40, 0x00, 0x00, 0x00), w.px(0), w.px(2))
	w.gdiText(hdc, w.fHeadline, sc.Headline, wRect{0, y, width, y + hHead}, dtCenter|dtVCenter|dtSingleLine, tokText)
	y += hHead

	// The sub line (amber words get the warning icon beside them —
	// the amber words are never alone).
	if sc.Sub != "" {
		col := subCol
		iconW := int32(0)
		if sc.SubAmber {
			iconW = w.px(18) + w.px(6)
		}
		var tw int32
		if m := w.measureText(hdc, w.fSub, sc.Sub); m > 0 {
			tw = m
		}
		left := (width - tw - iconW) / 2
		if sc.SubAmber {
			w.drawAlertTriangle(g, left, y+hSub/2-w.px(9), col)
			left += iconW
		}
		w.gdiText(hdc, w.fSub, sc.Sub, wRect{left - w.px(40), y, left + tw + w.px(40), y + hSub}, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis, col)
	}
	y += hSub

	// The timer (tabular boxes: digit 0.8em, colon 0.4em).
	if sc.ShowTimer {
		ms := time.Since(sc.ConnectedAt).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		w.drawTimer(hdc, formatDuration(ms), width, y+hTimer)
		y += hTimer
	}

	// The byte totals with their arrows.
	if sc.ShowStats {
		w.drawStats(hdc, g, sc, width, y+hStats)
	}
}

// subColor is the sub line's color — 400 ms cross-fade on change.
func (w *liteWindow) subColor(sc Screen, now int64) uint32 {
	to := stateColors[sc.State].Sub
	if sc.State == ScreenConnected && sc.Slow {
		to = slowSub
	}
	t := animPhase(now, w.colorFadeStart, 400)
	if t < 1 {
		return lerpColor(w.prevSubColor, to, t)
	}
	return to
}

// drawTimer paints HH:MM:SS in fixed per-character boxes (the digits
// never jitter).
func (w *liteWindow) drawTimer(hdc uintptr, s string, width, boxH int32) {
	old, _, _ := procSelectObject.Call(hdc, uintptr(w.fTimer))
	defer procSelectObject.Call(hdc, old)
	procSetBkMode.Call(hdc, transparentBk)
	digitW := w.px(13) // 0.8em at 16px
	colonW := w.px(6)  // 0.4em
	var total int32
	for _, c := range s {
		if c == ':' {
			total += colonW
		} else {
			total += digitW
		}
	}
	x := (width - total) / 2
	for _, c := range s {
		bw := digitW
		col := tokText
		if c == ':' {
			bw = colonW
			col = scaleAlpha(tokText, 0.72)
		}
		rc := wRect{x, 0, x + bw, boxH}
		procSetTextColor.Call(hdc, gdiRGB(col))
		w.drawTextHdc(hdc, string(c), rc, dtCenter|dtVCenter|dtSingleLine)
		x += bw
	}
}

// drawStats paints the two arrowed byte totals (arrows 14 px, gap 6,
// groups 18 px apart).
func (w *liteWindow) drawStats(hdc uintptr, g uintptr, sc Screen, width, boxH int32) {
	down := formatBytes(sc.DownBytes)
	up := formatBytes(sc.UpBytes)
	var dw, uw int32
	if m := w.measureText(hdc, w.fStats, down); m > 0 {
		dw = m
	}
	if m := w.measureText(hdc, w.fStats, up); m > 0 {
		uw = m
	}
	arrowW := w.px(14)
	gap := w.px(6)
	groupGap := w.px(18)
	total := (arrowW+gap)*2 + dw + uw + groupGap
	x := (width - total) / 2
	cy := boxH / 2
	col := w.subColor(sc, time.Now().UnixMilli())
	w.drawArrowDown(g, x+arrowW/2, cy, col)
	x += arrowW + gap
	w.gdiText(hdc, w.fStats, down, wRect{x - w.px(8), 0, x + dw + w.px(8), boxH}, dtLeft|dtVCenter|dtSingleLine, col)
	x += dw + groupGap
	w.drawArrowUp(g, x+arrowW/2, cy, col)
	x += arrowW + gap
	w.gdiText(hdc, w.fStats, up, wRect{x - w.px(8), 0, x + uw + w.px(8), boxH}, dtLeft|dtVCenter|dtSingleLine, col)
}

// drawHint paints the quiet line above the card slot.
func (w *liteWindow) drawHint(hdc uintptr, width int32) {
	rc := w.lay.hint
	w.gdiText(hdc, w.fHint, w.scr.Hint, wRect{w.px(lyMargin), rc.Top, width - w.px(lyMargin), rc.Bottom}, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis, scaleAlpha(tokHint, 0.9))
}

// drawCard paints the Open YouTube card (only while connected): the
// glass card, the red play tile, the two text lines — rising in with
// the reference's 450 ms fade + 14 px lift.
func (w *liteWindow) drawCard(g uintptr, hdc uintptr, sc Screen, now int64) {
	if !sc.ShowCard {
		return
	}
	t := animPhase(now, w.cardShownAt, 450)
	if t <= 0 {
		return
	}
	ease := easeCss(t)
	alpha := ease
	lift := int32(float64(w.px(14)) * (1 - ease))
	rc := w.lay.card
	x, y := rc.Left, rc.Top+lift
	cw, chh := rc.Right-rc.Left, rc.Bottom-rc.Top

	// The soft drop shadow (0 8px 18px rgba(0,0,0,.3)).
	w.glowRRect(g, x, y+w.px(3), cw, chh, w.px(lyCardR), w.px(6), scaleAlpha(argb(0x4d, 0x00, 0x00, 0x00), alpha))

	// The glass fill: rgba(255,255,255,.12) -> .05.
	if br := newVGradient(x, y, cw, chh,
		[]uint32{scaleAlpha(0x1FFFFFFF, alpha), scaleAlpha(0x0DFFFFFF, alpha)},
		[]float32{0, 1}); br != 0 {
		w.fillRRectB(g, x, y, cw, chh, w.px(lyCardR), br)
		deleteBrush(br)
	}
	// The inset top highlight (inset 0 1px 0 rgba(255,255,255,.22)).
	w.fillRRect(g, x+w.px(2), y+w.px(1), cw-w.px(4), w.px(2), w.px(2), scaleAlpha(argb(0x38, 0xFF, 0xFF, 0xFF), alpha))

	// The red play tile: 48x40, radius 11, gradient + highlight.
	iw, ih := w.px(48), w.px(40)
	ix, iy := x+w.px(14), y+(chh-ih)/2
	if br := newVGradient(ix, iy, iw, ih, []uint32{scaleAlpha(0xfff0392c, alpha), scaleAlpha(0xffc3180f, alpha)}, []float32{0, 1}); br != 0 {
		w.fillRRectB(g, ix, iy, iw, ih, w.px(11), br)
		deleteBrush(br)
	}
	w.fillRRect(g, ix+w.px(1), iy+w.px(1), iw-w.px(2), w.px(2), w.px(2), scaleAlpha(argb(0x59, 0xFF, 0xFF, 0xFF), alpha))
	// The white triangle (13/9/9 at left 19, top 11).
	w.fillPoly(g, []wPoint{
		{ix + w.px(19), iy + w.px(11)},
		{ix + w.px(19), iy + w.px(29)},
		{ix + w.px(32), iy + w.px(20)},
	}, scaleAlpha(argb(0xFF, 0xFF, 0xFF, 0xFF), alpha))

	// The text lines.
	tx := ix + iw + w.px(14)
	w.gdiText(hdc, w.fCardTitle, "Open YouTube", wRect{tx, y + w.px(10), x + cw - w.px(8), y + chh/2 + w.px(2)}, dtLeft|dtVCenter|dtSingleLine, scaleAlpha(tokText, alpha))
	w.gdiText(hdc, w.fCardSub, "Launch the YouTube app", wRect{tx, y + chh/2 - w.px(2), x + cw - w.px(8), y + chh - w.px(8)}, dtLeft|dtVCenter|dtSingleLine, scaleAlpha(stateColors[sc.State].Sub, alpha))
}

// ---- the lever (the v3 screen's key cap on its recessed track) ----

// capRestPos is the resting cap position for a state: 0 = top
// (connecting/connected), 1 = bottom (idle/failed).
func capRestPos(s ScreenState) float64 {
	switch s {
	case ScreenConnecting, ScreenConnected:
		return 0
	}
	return 1
}

// drawLever paints the track with its aura, the cue chevrons, and the
// key cap at its current (animated or dragged) position.
func (w *liteWindow) drawLever(g uintptr, sc Screen, now int64) {
	rc := w.lay.track
	tw, th := rc.Right-rc.Left, rc.Bottom-rc.Top

	// The track's breathing aura (the aura/aurabeat keyframes),
	// drawn behind everything.
	w.drawTrackAura(g, rc, sc, now)

	// The recessed track: a vertical near-black gradient.
	if br := newVGradient(rc.Left, rc.Top, tw, th,
		[]uint32{0x61000000, 0x29000000, 0x08FFFFFF},
		[]float32{0, 0.60, 1}); br != 0 {
		w.fillRRectB(g, rc.Left, rc.Top, tw, th, w.px(lyTrackR), br)
		deleteBrush(br)
	}
	// The inset top shadow (inset 0 10px 16px rgba(0,0,0,.55)): a
	// dark band fading down inside the track's mouth.
	if br := newVGradient(rc.Left, rc.Top, tw, w.px(26),
		[]uint32{0x8C000000, 0x00000000}, []float32{0, 1}); br != 0 {
		w.fillRRectB(g, rc.Left+w.px(1), rc.Top+w.px(1), tw-w.px(2), w.px(26), w.px(lyTrackR)-w.px(1), br)
		deleteBrush(br)
	}
	// The inset bottom light (inset 0 -3px 5px rgba(255,255,255,.08)).
	w.fillRRect(g, rc.Left+w.px(20), rc.Bottom-w.px(5), tw-w.px(40), w.px(4), w.px(2), argb(0x14, 0xFF, 0xFF, 0xFF))
	// The outer bottom light line (0 1px 0 rgba(255,255,255,.1)).
	w.fillRRect(g, rc.Left+w.px(6), rc.Bottom+w.px(1), tw-w.px(12), w.px(2), w.px(1), argb(0x1A, 0xFF, 0xFF, 0xFF))
	// The outer ring (0 0 0 1px rgba(0,0,0,.28)) — a 2 px ring on the
	// boundary, half in half out.
	w.fillRRectRing(g, rc.Left, rc.Top, tw, th, w.px(lyTrackR), w.px(2), argb(0x47, 0x00, 0x00, 0x00))

	// The cue chevrons (idle/failed only; hidden while dragging).
	if (sc.State == ScreenIdle || sc.State == ScreenFailed) && !w.capDragging() {
		w.drawSwipeCue(g, rc, now)
	}
	// The connected state's single static down chevron.
	if sc.State == ScreenConnected {
		cxc := (rc.Left + rc.Right) / 2
		cyc := rc.Bottom - w.px(18) - w.px(10)
		w.drawChevronDown(g, cxc, cyc, w.px(20), argb(0xD9, 0xFF, 0xFF, 0xFF))
	}

	// The cap.
	w.drawCap(g, rc, sc, now)
}

// drawTrackAura breathes the state's aura around the track — the
// keyframes' blur/scale pairings (`0 0 14px 1px` at rest to `0 0 46px 8px`
// at the peak), rendered as a layered onion of solid rounded rects whose
// stacked alphas make a near-gaussian falloff STRONGEST AT THE TRACK'S
// EDGE. D119 replaced the old single path-gradient halo: its cone fell
// off from the track's CENTER — weak at the edge, a tail twice too wide,
// and (worst) its bright center bled green through the semi-transparent
// track fill, washing the recessed slot's interior — the reference's
// aura is an outer box-shadow and never touches the inside. The onion
// runs under a CombineModeExclude clip on the track's own silhouette,
// and solid fills render identically on every GDI+.
func (w *liteWindow) drawTrackAura(g uintptr, rc wRect, sc Screen, now int64) {
	c := stateColors[sc.State].Aura
	if c == 0 {
		return // idle: no aura
	}
	var phase float64
	switch sc.State {
	case ScreenConnecting:
		phase = animCycle(now, w.stateEntered, 1100) // aura 1.1s
	case ScreenConnected:
		phase = animCycle(now, w.stateEntered, 3400) // aura 3.4s
	case ScreenFailed:
		phase = animCycle(now, w.stateEntered, 3600) // aurabeat 3.6s
	}
	// The intensity envelope: 0 at rest, 1 at the keyframe's peak.
	env := 0.0
	switch sc.State {
	case ScreenConnecting, ScreenConnected:
		env = math.Sin(math.Pi * phase) // 0 -> 1 -> 0
	case ScreenFailed:
		// aurabeat: peaks at 12-36%, dips at 24%, rests at 48%.
		if phase < 0.12 {
			env = phase / 0.12
		} else if phase < 0.24 {
			env = 1 - (phase-0.12)/0.12*0.55
		} else if phase < 0.36 {
			env = 0.45 + (phase-0.24)/0.12*0.55
		} else if phase < 0.48 {
			env = 1 - (phase-0.36)/0.12
		}
	}
	if env <= 0.01 {
		return
	}
	tw, th := rc.Right-rc.Left, rc.Bottom-rc.Top
	// Rest 14 px, peak 46 px — the halo's reach follows the envelope.
	extent := w.px(14) + int32(float64(w.px(32))*env)
	tp := newPath(fillModeWinding)
	if tp == 0 {
		return
	}
	defer deletePath(tp)
	addPathPolygon(tp, rrectPoints(rc.Left, rc.Top, tw, th, w.px(lyTrackR), 12))
	// Everything OUTSIDE the track — the shadow's side of the boundary.
	excludeClipPath(g, tp)
	for _, s := range []struct {
		e float64 // the layer's reach, as a fraction of the extent
		a float64 // the layer's alpha share at the peak envelope
	}{{0.20, 0.42}, {0.40, 0.25}, {0.60, 0.15}, {0.80, 0.09}, {1.00, 0.05}} {
		e := int32(float64(extent) * s.e)
		if e < 1 {
			continue
		}
		w.fillRRect(g, rc.Left-e, rc.Top-e, tw+2*e, th+2*e, w.px(lyTrackR)+e, scaleAlpha(c, s.a*env))
	}
	resetClip(g)
}

// drawSwipeCue paints the three staggered chevrons in the empty top of
// the track (1.8 s loop, bottom-to-top, drifting up as they fade).
func (w *liteWindow) drawSwipeCue(g uintptr, rc wRect, now int64) {
	base := animCycle(now, w.stateEntered, 1800)
	cx := (rc.Left + rc.Right) / 2
	// The cue column: top of the track + 30 px, three 16 px chevrons
	// with 2 px gaps (28x16 icons, stroke 2.6).
	yTop := rc.Top + w.px(30)
	step := w.px(18)
	for i := 0; i < 3; i++ {
		// nth-child(1) = top chevron, delay 0.5s; (3) = bottom, 0s.
		delay := float64(2-i) * 0.25 // seconds
		p := base - delay/1.8
		if p < 0 {
			p += 1.0
		}
		if p > 1 {
			continue
		}
		// swipecue: opacity 0 -> .45 (35%) -> .45 (65%) -> 0,
		// translateY 8px -> -8px.
		op := 0.0
		if p < 0.35 {
			op = p / 0.35 * 0.45
		} else if p < 0.65 {
			op = 0.45
		} else {
			op = (1 - (p-0.65)/0.35) * 0.45
		}
		if op <= 0.01 {
			continue
		}
		drift := lerp(8, -8, p)
		y := yTop + int32(i)*step + w.pxf(drift)
		w.drawChevronUp(g, cx, y+w.px(8), argb(uint32(0xFF*op), 0xFF, 0xFF, 0xFF))
	}
}

// drawCap paints the key cap at its current position: the state glow,
// the cast shadow, the side wall (9 px, 3 px pressed), the satin face
// with its sheen and bevel, the LED, grip ridges, the letterspaced
// label and the power icon.
func (w *liteWindow) drawCap(g uintptr, track wRect, sc Screen, now int64) {
	// The cap's current travel offset (0 = top, 122 = bottom).
	pos := w.capPosNow(now)
	pressed := w.pressed && w.capDragging()
	sink := int32(0)
	wall := w.px(9)
	if pressed {
		sink = w.px(6)
		wall = w.px(3)
	}
	capX := track.Left + w.px(lyPad)
	travelPx := float64(w.px(lyTravel))
	capY := track.Top + w.px(lyPad) + sink + int32(pos*travelPx)
	capW, capH := w.px(lyCapW), w.px(lyCapH)

	// The state's cap glow (pulsepill while connecting; the slow
	// breathing glow while idle/failed; connected's steady soft
	// glow).
	w.drawCapGlow(g, capX, capY, capW, capH, sc, now)

	// The cast shadow (cap-rest's 0 20px 22px rgba(0,0,0,.50) —
	// pressed sinks it to 0 8px 10px).
	shadow := argb(0x66, 0x00, 0x00, 0x00)
	soff := w.px(12)
	if pressed {
		shadow = argb(0x4d, 0x00, 0x00, 0x00)
		soff = w.px(4)
	}
	w.glowRRect(g, capX, capY+soff, capW, capH, w.px(lyCapR), w.px(10), shadow)

	// The side wall: the edge color showing under the face.
	w.fillRRect(g, capX, capY, capW, capH+wall, w.px(lyCapR), tokEdge)
	// The wall's own dark seam (0 10px 0 rgba(0,0,0,.38)).
	w.fillRRect(g, capX, capY+capH+wall-w.px(2), capW, w.px(2), w.px(2), argb(0x61, 0x00, 0x00, 0x00))

	// The satin face.
	if br := newVGradient(capX, capY, capW, capH, capFaceColors, capFacePos); br != 0 {
		w.fillRRectB(g, capX, capY, capW, capH, w.px(lyCapR), br)
		deleteBrush(br)
	}
	// The sheen (radial 130%x40% at 50% 0%: rgba(255,255,255,.2) ->
	// transparent 70%). Measured on the reference render, the visible
	// gloss is a top-edge wash: ~28-31% white at the very top (right
	// under the 45% bevel line), nearly full cap width, fading to zero
	// by ~28 px down. Ported as a vertical gradient band confined to the
	// cap's own silhouette by a GDI+ clip (D119: no path gradients here
	// — the old circle brush rendered as an OPAQUE WHITE DISC on real
	// Windows while wine showed it soft, so the class is banned from the
	// cap; linear gradients and clips are deterministic everywhere).
	if cp := newPath(fillModeWinding); cp != 0 {
		addPathPolygon(cp, rrectPoints(capX, capY, capW, capH, w.px(lyCapR), 12))
		setClipPath(g, cp)
		shH := w.px(28)
		if br := newVGradient(capX, capY, capW, shH,
			[]uint32{0x4DFFFFFF, 0x47FFFFFF, 0x00FFFFFF},
			[]float32{0, 0.35, 1}); br != 0 {
			fillRectI(g, br, capX, capY, capW, shH)
			deleteBrush(br)
		}
		// The top bevel (inset 0 1px 0 rgba(255,255,255,.45)) — inside
		// the same clip so it can never poke over the rounded corners.
		w.fillRRect(g, capX+w.px(2), capY+w.px(1), capW-w.px(4), w.px(2), w.px(2), argb(0x73, 0xFF, 0xFF, 0xFF))
		resetClip(g)
		deletePath(cp)
	}
	// The bottom inner shade (inset 0 -2px 3px rgba(80,0,0,.30)).
	if br := newVGradient(capX, capY+capH-w.px(8), capW, w.px(8),
		[]uint32{0x00500000, 0x4D500000}, []float32{0, 1}); br != 0 {
		w.fillRRectB(g, capX+w.px(2), capY+capH-w.px(8), capW-w.px(4), w.px(8)-w.px(2), w.px(8), br)
		deleteBrush(br)
	}

	// The controls column: LED (13 px from the top), grip, label,
	// power icon (12 px from the bottom).
	// The controls column, the space-between rhythm computed once:
	// LED 13, grip 30, label 53, power icon centered at 100 (of the
	// 124-tall face, 13/12 padding).
	cx := capX + capW/2
	w.drawLed(g, cx, capY+w.px(13), sc, now)
	w.drawGrip(g, cx, capY+w.px(30))
	w.drawCapLabel(cx, capY+w.px(53), capW, sc)
	w.drawPowerIcon(g, cx, capY+w.px(100), w.px(24))
}

// drawCapGlow is the cap's animated halo (pulsepill / glow keyframes).
func (w *liteWindow) drawCapGlow(g uintptr, x, y, cw, chh int32, sc Screen, now int64) {
	var expand int32
	var alpha float64
	switch sc.State {
	case ScreenConnecting:
		// pulsepill 1.1s: 0/100% 22px .25; 45% 80px .85.
		p := animCycle(now, w.stateEntered, 1100)
		env := math.Sin(math.Pi * p)
		// The keyframe peaks at 45%, close enough to a sine's 50%.
		expand = w.px(22) + int32(float64(w.px(58))*env)
		alpha = 0.25 + 0.60*env
	case ScreenConnected:
		expand, alpha = w.px(26), 0.28 // glow's resting arm
	default:
		// glow 4s: 0/100% 26px .28; 50% 52px .5.
		p := animCycle(now, w.stateEntered, 4000)
		env := math.Sin(math.Pi * p)
		expand = w.px(26) + int32(float64(w.px(26))*env)
		alpha = 0.28 + 0.22*env
	}
	c := stateColors[sc.State].PillGlow
	base := float64(c>>24) / 255.0
	if base <= 0 {
		base = 0.30
	}
	c = scaleAlpha(c, alpha/base)
	w.glowRRect(g, x, y, cw, chh, w.px(lyCapR), expand, c)
}

// drawLed paints the 24x4 LED bar with its inset shadow and animated
// glow (blink while connecting, breath while connected).
func (w *liteWindow) drawLed(g uintptr, cx, y int32, sc Screen, now int64) {
	led := w.ledColor(sc, now)
	lw, lh := w.px(24), w.px(4)
	x := cx - lw/2
	// The animated glow.
	glow := stateColors[sc.State].LEDGlow
	if glow != 0 {
		var env float64
		switch sc.State {
		case ScreenConnecting: // blink: full <-> faint, 1 s
			p := animCycle(now, w.stateEntered, 1000)
			env = 0.25 + 0.75*math.Cos(math.Pi*p)*math.Cos(math.Pi*p)
			if p > 0.5 {
				env = 0.25 + 0.75*(2*(p-0.5))*(2*(p-0.5))
			}
		case ScreenConnected: // ledbreath 3.4s: 6px <-> 16px 2px
			env = 0.5 + 0.5*math.Sin(math.Pi*animCycle(now, w.stateEntered, 3400))
		default:
			env = 0.5
		}
		expand := w.px(4) + int32(float64(w.px(7))*env)
		w.glowRRect(g, x, y, lw, lh, lh/2, expand, scaleAlpha(glow, env))
	}
	// The bar with its inset top shadow.
	w.fillRRect(g, x, y, lw, lh, lh/2, led)
	w.fillRRect(g, x+w.px(2), y, lw-w.px(4), w.px(1), 1, argb(0x66, 0x00, 0x00, 0x00))
}

// ledColor is the LED's color — the 400 ms cross-fade on change.
func (w *liteWindow) ledColor(sc Screen, now int64) uint32 {
	to := stateColors[sc.State].LED
	t := animPhase(now, w.colorFadeStart, 400)
	if t < 1 {
		return lerpColor(w.prevLEDColor, to, t)
	}
	return to
}

// drawGrip paints the three engraved ridges.
func (w *liteWindow) drawGrip(g uintptr, cx, y int32) {
	gw := w.px(34)
	x := cx - gw/2
	for i := int32(0); i < 3; i++ {
		ry := y + i*w.px(6)
		// The groove (rgba(70,0,0,.4)).
		w.fillRRect(g, x, ry, gw, w.px(2), w.px(1), argb(0x66, 0x46, 0x00, 0x00))
		// The light line under it (0 1px 0 rgba(255,255,255,.18)).
		w.fillRRect(g, x, ry+w.px(2), gw, w.px(1), w.px(1), argb(0x2E, 0xFF, 0xFF, 0xFF))
	}
}

// drawCapLabel paints the cap word — 18/700 uppercase (the CSS's
// text-transform: uppercase — the design's cap always reads
// START/WAIT/STOP/RETRY), 2.2 px letterspacing, engraved into the red
// face.
func (w *liteWindow) drawCapLabel(cx, y, capW int32, sc Screen) {
	hdc := w.paintDC
	old, _, _ := procSelectObject.Call(hdc, uintptr(w.fCap))
	defer procSelectObject.Call(hdc, old)
	procSetBkMode.Call(hdc, transparentBk)
	label := strings.ToUpper(sc.CapLabel)
	spacing := w.pxf(2.2)
	if spacing < 1 {
		spacing = 1
	}
	// Measure the run.
	var widths []int32
	var total int32
	for _, c := range label {
		wd, _ := textExtent(hdc, string(c))
		if wd <= 0 {
			wd = w.px(10)
		}
		widths = append(widths, wd)
		total += wd + spacing
	}
	total -= spacing
	x := cx - total/2
	h := w.px(24)
	shadow := gdiRGB(argb(0xFF, 0x8F, 0x1A, 0x16)) // the face blended toward rgba(90,0,0,.45)
	for i, c := range label {
		rc := wRect{x, y, x + widths[i], y + h}
		// The engraved shadow pass (0 1px 1px rgba(90,0,0,.45)).
		procSetTextColor.Call(hdc, shadow)
		rcS := wRect{x, y + w.px(1), x + widths[i], y + h}
		w.drawTextHdc(hdc, string(c), rcS, dtCenter|dtVCenter|dtSingleLine)
		procSetTextColor.Call(hdc, gdiRGB(tokText))
		w.drawTextHdc(hdc, string(c), rc, dtCenter|dtVCenter|dtSingleLine)
		x += widths[i] + spacing
	}
}

// drawPowerIcon paints the thin-line power icon (24-grid: the stem
// M12 3v8 and the 269-degree arc) with a soft dark drop shadow.
func (w *liteWindow) drawPowerIcon(g uintptr, cx, cy, size int32) {
	s := float64(size) / 24.0
	px := func(v float64) int32 { return int32(math.Round(v * s)) }
	// The drop shadow (drop-shadow(0 1px 0 rgba(90,0,0,.45))).
	sh := argb(0x73, 0x5A, 0x00, 0x00)
	sw := w.strokeW(1.9)
	// Stem: (12,3) -> (12,11).
	w.strokePolyline(g, []wPoint{{cx + px(12), cy + px(4)}, {cx + px(12), cy + px(12)}}, sw, sh)
	// Arc: center (12, 12.41), r 8, from 224.6 degrees, -269.2 sweep
	// (counter-clockwise: the gap stays at the top).
	arc := arcPoints(cx+px(12), cy+px(12.41), px(8), px(8), 224.6, -269.2, 36)
	w.strokePolyline(g, arc, sw, sh)
	// The white icon over it.
	w.strokePolyline(g, []wPoint{{cx + px(12), cy + px(3)}, {cx + px(12), cy + px(11)}}, sw, argb(0xFF, 0xFF, 0xFF, 0xFF))
	w.strokePolyline(g, arc, sw, argb(0xFF, 0xFF, 0xFF, 0xFF))
}

// ---- icons ----

// drawChevronUp paints one "^" cue chevron — the 28x16 icon, stroke
// 2.6, exact geometry (M4 13 L14 4 L24 13).
func (w *liteWindow) drawChevronUp(g uintptr, cx, cy int32, col uint32) {
	sw := w.strokeW(2.6)
	x0, y0 := cx-w.px(14), cy-w.px(8)
	pts := []wPoint{
		{x0 + w.px(4), y0 + w.px(13)},
		{x0 + w.px(14), y0 + w.px(4)},
		{x0 + w.px(24), y0 + w.px(13)},
	}
	w.strokePolyline(g, pts, sw, col)
}

// drawChevronDown paints the connected state's double down-chevron
// (20x20, stroke 2): M5 4l5 5 5-5 M5 10l5 5 5-5.
func (w *liteWindow) drawChevronDown(g uintptr, cx, cy, size int32, col uint32) {
	sw := w.strokeW(2)
	s := float64(size) / 20.0
	x0, y0 := cx-size/2, cy-size/2
	pt := func(u, v float64) wPoint { return wPoint{x0 + int32(u*s), y0 + int32(v*s)} }
	w.strokePolyline(g, []wPoint{pt(5, 4), pt(10, 9), pt(15, 4)}, sw, col)
	w.strokePolyline(g, []wPoint{pt(5, 10), pt(10, 15), pt(15, 10)}, sw, col)
}

// drawChevronRight paints the sheet rows' ">" (20-grid, stroke 2).
func (w *liteWindow) drawChevronRight(g uintptr, cx, cy, size int32, col uint32) {
	sw := w.strokeW(2)
	s := float64(size) / 24.0
	x0, y0 := cx-size/2, cy-size/2
	pt := func(u, v float64) wPoint { return wPoint{x0 + int32(u*s), y0 + int32(v*s)} }
	w.strokePolyline(g, []wPoint{pt(9, 6), pt(15, 12), pt(9, 18)}, sw, col)
}

// drawAlertTriangle paints the amber sub-line's warning icon (18 px,
// stroke 2, lucide's triangle with the bang).
func (w *liteWindow) drawAlertTriangle(g uintptr, x, y int32, col uint32) {
	s := float64(w.px(18)) / 24.0
	sw := w.strokeW(2)
	pt := func(u, v float64) wPoint { return wPoint{x + int32(u*s), y + int32(v*s)} }
	// M10.29 3.86 L1.82 18 a2 2 0 0 0 1.71 3 h16.94 a2 2 0 0 0 1.71-3 L13.71 3.86 a2 2 0 0 0-3.42 0 z
	outline := []wPoint{pt(10.29, 3.86), pt(1.82, 18), pt(3.53, 20.73), pt(20.47, 20.73), pt(22.18, 18), pt(13.71, 3.86)}
	w.strokePolyline(g, outline, sw, col)
	// The bang: M12 9 v4 and the dot at 16.5.
	w.strokePolyline(g, []wPoint{pt(12, 9), pt(12, 13)}, sw, col)
	w.fillEllipse(g, pt(12, 16.5).X+sw/2, pt(12, 16.5).Y+sw/2, sw/2, col)
}

// drawArrowDown / drawArrowUp paint the stats' 14 px lucide arrows.
func (w *liteWindow) drawArrowDown(g uintptr, cx, cy int32, col uint32) {
	sw := w.strokeW(2)
	s := float64(w.px(14)) / 24.0
	pt := func(u, v float64) wPoint { return wPoint{cx + int32(u*s) - w.px(7), cy + int32(v*s) - w.px(7)} }
	w.strokePolyline(g, []wPoint{pt(12, 5), pt(12, 19)}, sw, col)
	w.strokePolyline(g, []wPoint{pt(19, 12), pt(12, 19), pt(5, 12)}, sw, col)
}

func (w *liteWindow) drawArrowUp(g uintptr, cx, cy int32, col uint32) {
	sw := w.strokeW(2)
	s := float64(w.px(14)) / 24.0
	pt := func(u, v float64) wPoint { return wPoint{cx + int32(u*s) - w.px(7), cy + int32(v*s) - w.px(7)} }
	w.strokePolyline(g, []wPoint{pt(12, 19), pt(12, 5)}, sw, col)
	w.strokePolyline(g, []wPoint{pt(5, 12), pt(12, 5), pt(19, 12)}, sw, col)
}

// drawXIcon paints the sheet close "X" (18 px, stroke 2).
func (w *liteWindow) drawXIcon(g uintptr, cx, cy, size int32, col uint32) {
	sw := w.strokeW(2)
	s := float64(size) / 24.0
	x0, y0 := cx-size/2, cy-size/2
	pt := func(u, v float64) wPoint { return wPoint{x0 + int32(u*s), y0 + int32(v*s)} }
	w.strokePolyline(g, []wPoint{pt(6, 6), pt(18, 18)}, sw, col)
	w.strokePolyline(g, []wPoint{pt(18, 6), pt(6, 18)}, sw, col)
}

// ---- the menu sheet ----

// sheetPos is the sheet's 0..1 slide (0 closed, 1 open) at now.
func (w *liteWindow) sheetPos(now int64) float64 {
	t := animPhase(now, w.sheetAnimStart, 380)
	if !w.sheetOpen {
		return 1 - easeSheet(t)
	}
	return easeSheet(t)
}

// sheetRow is one row's rect + identity.
type sheetRow struct {
	rc   wRect
	kind int // 0 action, 1 toggle
	idx  int
}

// drawSheet paints the backdrop, the sliding panel and its rows.
func (w *liteWindow) drawSheet(g uintptr, hdc uintptr, width, height int32, now int64) {
	pos := w.sheetPos(now)
	if pos <= 0.001 {
		return
	}
	// The backdrop (rgba(0,0,0,.5), 300 ms fade).
	bAlpha := uint32(float64(0x80) * clamp01(pos*1.27)) // fade finishes before the slide
	if bAlpha > 0x80 {
		bAlpha = 0x80
	}
	if b := newSolidBrush(argb(bAlpha, 0x00, 0x00, 0x00)); b != 0 {
		fillRectI(g, b, 0, 0, width, height)
		deleteBrush(b)
	}

	// The panel geometry.
	sw := w.lay.sheetW
	sh := w.lay.sheetH
	x := (width - sw) / 2
	y := height - int32(float64(sh)*pos)

	// The panel: #1d1c26 with the top border and top-only radii.
	w.fillRRect(g, x, y, sw, sh+w.px(30), w.px(lySheetR), tokSheetBg) // +30 bleeds below the edge
	// The top border (1px rgba(255,255,255,.07)).
	w.fillRRect(g, x+w.px(2), y+w.px(1), sw-w.px(4), w.px(2), w.px(2), argb(0x12, 0xFF, 0xFF, 0xFF))
	// A soft shadow above the sheet's top edge.
	w.glowRRect(g, x, y-w.px(6), sw, w.px(12), w.px(lySheetR), w.px(8), argb(0x40, 0x00, 0x00, 0x00))

	// The grabber (40x4, rgba(255,255,255,.18)).
	w.fillRRect(g, x+(sw-w.px(40))/2, y+w.px(10), w.px(40), w.px(4), w.px(2), argb(0x2E, 0xFF, 0xFF, 0xFF))

	// The head: title + the X circle.
	hx := x + w.px(22)
	hy := y + w.px(24)
	w.gdiText(hdc, w.fSheetTitle, "Younblock", wRect{hx, hy, hx + w.px(200), hy + w.px(44)}, dtLeft|dtVCenter|dtSingleLine, tokText)
	xw := w.px(44)
	xrect := wRect{x + sw - w.px(22) - xw, hy, x + sw - w.px(22), hy + xw}
	w.drawSheetXButton(g, xrect)

	// The rows.
	ry := hy + w.px(44) + w.px(6)
	rows := w.sheetRows(x, sw, ry)
	for _, row := range rows {
		switch row.idx {
		case 0:
			w.drawSheetActionRow(g, hdc, row.rc, "Open YouTube", "Open the site in your browser", w.hot == row.idx+100, w.focus == row.idx+100)
		case 1:
			w.drawSheetToggleRow(g, hdc, row.rc, "Connect on launch", "Start connecting when this window opens", w.tglConnect, now, row.idx+100)
		case 2:
			w.drawSheetToggleRow(g, hdc, row.rc, "Open YouTube when connected", "Open the browser as soon as the tunnel is up", w.tglOpen, now, row.idx+100)
		case 3:
			w.drawSheetExitRow(g, hdc, row.rc, w.hot == row.idx+100, w.focus == row.idx+100)
		}
	}

	// The footer: version + credit.
	fy := rows[len(rows)-1].rc.Bottom + w.px(14)
	w.gdiText(hdc, w.fRowHint, "Younblock Lite  ·  "+config.AppVersion, wRect{x + w.px(22), fy, x + sw - w.px(22), fy + w.px(18)}, dtLeft|dtVCenter|dtSingleLine, argb(0xC7, 0xFF, 0xFF, 0xFF))
	w.gdiText(hdc, w.fRowHint, "Created by ChitaCodes", wRect{x + w.px(22), fy + w.px(18), x + sw - w.px(22), fy + w.px(36)}, dtLeft|dtVCenter|dtSingleLine, argb(0x9E, 0xFF, 0xFF, 0xFF))
}

// sheetRows computes the four rows' rects (fixed heights: action 56,
// toggles 52, exit 44, gaps 8).
func (w *liteWindow) sheetRows(x, sw, y int32) []sheetRow {
	var rows []sheetRow
	ry := y
	add := func(h int32, kind, idx int) {
		rows = append(rows, sheetRow{rc: wRect{x + w.px(22), ry, x + sw - w.px(22), ry + h}, kind: kind, idx: idx})
		ry += h + w.px(8)
	}
	add(w.px(56), 0, 0) // Open YouTube
	add(w.px(52), 1, 1) // Connect on launch
	add(w.px(52), 1, 2) // Open YouTube when connected
	add(w.px(44), 0, 3) // Exit Younblock
	return rows
}

// drawSheetXButton paints the neutral 44 px X circle.
func (w *liteWindow) drawSheetXButton(g uintptr, rc wRect) {
	d := rc.Right - rc.Left
	cx, cy := (rc.Left+rc.Right)/2, (rc.Top+rc.Bottom)/2
	hot := w.hot == 104
	bg := argb(0x14, 0xFF, 0xFF, 0xFF)
	if hot {
		bg = argb(0x24, 0xFF, 0xFF, 0xFF)
	}
	w.fillEllipse(g, cx, cy, d/2, bg)
	w.fillRRectRing(g, rc.Left, rc.Top, d, d, d/2, w.px(2), argb(0x24, 0xFF, 0xFF, 0xFF))
	w.drawXIcon(g, cx, cy, w.px(18), argb(0xC7, 0xFF, 0xFF, 0xFF))
}

// drawSheetActionRow paints a raised action row (the diag-entry look):
// rounded 12, #262532, icon chip, title + hint, right chevron.
func (w *liteWindow) drawSheetActionRow(g uintptr, hdc uintptr, rc wRect, title, hint string, hot, focused bool) {
	bg := tokSheetRse
	if hot {
		bg = argb(0xff, 0x33, 0x32, 0x40) // rgba(255,255,255,.14) over raised
	}
	w.fillRRect(g, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, w.px(12), bg)
	w.fillRRectRing(g, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, w.px(12), w.px(2), argb(0x24, 0xFF, 0xFF, 0xFF))

	// The icon chip (36x36, radius 10, red tint).
	ic := w.px(36)
	icx, icy := rc.Left+w.px(14), rc.Top+(rc.Bottom-rc.Top-ic)/2
	w.fillRRect(g, icx, icy, ic, ic, w.px(10), argb(0x24, 0xFF, 0x00, 0x00))
	w.fillRRectRing(g, icx, icy, ic, ic, w.px(10), w.px(2), argb(0x40, 0xFF, 0x00, 0x00))
	// The play triangle inside (red-400).
	c := icx + ic/2
	cy := icy + ic/2
	s := float64(ic) / 36.0
	w.fillPoly(g, []wPoint{
		{c - int32(5*s), cy - int32(7*s)},
		{c - int32(5*s), cy + int32(7*s)},
		{c + int32(8*s), cy},
	}, tokRed400)

	// The texts.
	tx := icx + ic + w.px(12)
	w.gdiText(hdc, w.fRowTitle, title, wRect{tx, rc.Top + w.px(8), rc.Right - w.px(36), rc.Top + (rc.Bottom-rc.Top)/2}, dtLeft|dtVCenter|dtSingleLine, tokText)
	w.gdiText(hdc, w.fRowHint, hint, wRect{tx, rc.Top + (rc.Bottom-rc.Top)/2, rc.Right - w.px(36), rc.Bottom - w.px(6)}, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis, argb(0x9e, 0xFF, 0xFF, 0xFF))

	// The right chevron.
	w.drawChevronRight(g, rc.Right-w.px(18), (rc.Top+rc.Bottom)/2, w.px(20), argb(0x62, 0xFF, 0xFF, 0xFF))
	if focused {
		w.focusRing(g, rc, w.px(12))
	}
}

// drawSheetToggleRow paints a flat settings row: title + hint on the
// left, the 44x26 toggle on the right.
func (w *liteWindow) drawSheetToggleRow(g uintptr, hdc uintptr, rc wRect, title, hint string, on bool, now int64, id int) {
	// The row's own hover brightens the toggle's track only.
	hot := w.hot == id
	// The texts.
	w.gdiText(hdc, w.fRowTitle, title, wRect{rc.Left + w.px(4), rc.Top + w.px(4), rc.Right - w.px(60), rc.Top + (rc.Bottom-rc.Top)/2}, dtLeft|dtVCenter|dtSingleLine, argb(0xC7, 0xFF, 0xFF, 0xFF))
	w.gdiText(hdc, w.fRowHint, hint, wRect{rc.Left + w.px(4), rc.Top + (rc.Bottom-rc.Top)/2, rc.Right - w.px(60), rc.Bottom - w.px(4)}, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis, argb(0x9E, 0xFF, 0xFF, 0xFF))

	// The toggle (200 ms thumb slide + track color).
	tw, th := w.px(44), w.px(26)
	tx := rc.Right - tw - w.px(4)
	ty := rc.Top + (rc.Bottom-rc.Top-th)/2
	// The animated thumb position for THIS row (the flip moment).
	t := 1.0
	if id == 101 && w.tglConnectFlip != 0 {
		t = animPhase(now, w.tglConnectFlip, 200)
	}
	if id == 102 && w.tglOpenFlip != 0 {
		t = animPhase(now, w.tglOpenFlip, 200)
	}
	pos := float64(0)
	if on {
		pos = easeCss(t)
	} else {
		pos = 1 - easeCss(t)
	}
	// The track.
	trackCol := lerpColor(argb(0x2E, 0xFF, 0xFF, 0xFF), tokRed500, pos)
	if hot {
		trackCol = lerpColor(argb(0x47, 0xFF, 0xFF, 0xFF), tokRed500, pos)
	}
	w.fillRRect(g, tx, ty, tw, th, th/2, trackCol)
	// The thumb (20 px, #f4f1ee, sliding 18 px).
	kw := w.px(20)
	kx := tx + w.px(3) + int32(float64(w.px(18))*pos)
	ky := ty + (th-kw)/2
	// The thumb's tiny resting shadow (D119: a solid ellipse — the old
	// path-gradient halo could render as a white ring on real Windows).
	w.fillEllipse(g, kx+kw/2, ky+kw/2+w.px(1), kw/2+w.px(1), argb(0x33, 0x00, 0x00, 0x00))
	w.fillEllipse(g, kx+kw/2, ky+kw/2, kw/2, argb(0xFF, 0xF4, 0xF1, 0xEE))
	// The focus ring.
	if w.focus == id {
		w.focusRing(g, wRect{rc.Left, rc.Top, rc.Right, rc.Bottom}, w.px(12))
	}
}

// drawSheetExitRow paints the quiet flat exit row (tertiary words, the
// power icon in a neutral chip — the Android sheet's Quit row look).
func (w *liteWindow) drawSheetExitRow(g uintptr, hdc uintptr, rc wRect, hot, focused bool) {
	if hot {
		w.fillRRect(g, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, w.px(12), argb(0x0F, 0xFF, 0xFF, 0xFF))
	}
	ic := w.px(36)
	icx, icy := rc.Left+w.px(14), rc.Top+(rc.Bottom-rc.Top-ic)/2
	w.fillRRect(g, icx, icy, ic, ic, w.px(10), argb(0x14, 0xFF, 0xFF, 0xFF))
	w.drawPowerIcon(g, icx+ic/2, icy+ic/2+1, w.px(20))
	tx := icx + ic + w.px(12)
	w.gdiText(hdc, w.fRowTitle, "Exit Younblock", wRect{tx, rc.Top, rc.Right - w.px(12), rc.Bottom}, dtLeft|dtVCenter|dtSingleLine, argb(0x9e, 0xFF, 0xFF, 0xFF))
	if focused {
		w.focusRing(g, rc, w.px(12))
	}
}

// ---- focus ----

// drawFocusRings paints the focused control's outline (3 px, offset 2 —
// the reference's focus-visible).
func (w *liteWindow) drawFocusRings(g uintptr, sc Screen, now int64) {
	if w.sheetPos(now) > 0.5 {
		return // the sheet draws its own
	}
	if !w.kbdNav {
		return // rings are for KEYBOARD navigation only (D118): the
		// window is born with no ring, pointer use keeps them hidden,
		// the first Tab brings them in — the reference's behavior
	}
	switch w.focus {
	case 0:
		// The lever: the track's outline + touch padding.
		rc := w.lay.hit
		w.focusRing(g, wRect{rc.Left + w.px(2), rc.Top + w.px(2), rc.Right - w.px(2), rc.Bottom - w.px(2)}, w.px(lyTrackR)+w.px(2))
	case 1:
		if sc.ShowCard {
			w.focusRing(g, w.lay.card, w.px(lyCardR))
		}
	case 2:
		rc := w.lay.menuBtn
		w.focusRing(g, wRect{rc.Left - w.px(2), rc.Top - w.px(2), rc.Right + w.px(2), rc.Bottom + w.px(2)}, (rc.Bottom-rc.Top)/2+w.px(2))
	}
}

// ---- the shape primitives on this window's DPI ----

// strokeW scales a logical stroke width, never below 1 device px.
func (w *liteWindow) strokeW(logical float64) int32 {
	v := int32(float64(w.dpi) * logical / 96)
	if v < 1 {
		v = 1
	}
	return v
}

// fillRRect fills a rounded rectangle with a solid color. Brushes are
// created and deleted per call on purpose: the screen animates alpha
// every frame, and a color-keyed cache would grow without bound (the
// one leak this painter must never have). GDI+ solid fills are cheap
// objects; the call rate here is a few dozen per frame.
func (w *liteWindow) fillRRect(g uintptr, x, y, wid, hei, r int32, c uint32) {
	b := newSolidBrush(c)
	if b == 0 {
		return
	}
	defer deleteBrush(b)
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	addPathPolygon(p, rrectPoints(x, y, wid, hei, r, 12))
	fillPath(g, b, p)
}

// fillRRectB fills a rounded rectangle with an existing brush.
func (w *liteWindow) fillRRectB(g uintptr, x, y, wid, hei, r int32, brush uintptr) {
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	addPathPolygon(p, rrectPoints(x, y, wid, hei, r, 12))
	fillPath(g, brush, p)
}

// fillRRectRing paints a stroke ring INSIDE a rounded rect's boundary
// (the even-odd hole between the outer and inner outlines).
func (w *liteWindow) fillRRectRing(g uintptr, x, y, wid, hei, r, thickness int32, c uint32) {
	b := newSolidBrush(c)
	if b == 0 {
		return
	}
	defer deleteBrush(b)
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	outer := rrectPoints(x, y, wid, hei, r, 12)
	inner := rrectPoints(x+thickness, y+thickness, wid-2*thickness, hei-2*thickness, maxI32(r-thickness, 0), 12)
	// Reverse the inner outline so the winding rule carves the hole.
	for i, j := 0, len(inner)-1; i < j; i, j = i+1, j-1 {
		inner[i], inner[j] = inner[j], inner[i]
	}
	addPathPolygon(p, outer)
	addPathPolygon(p, inner)
	fillPath(g, b, p)
}

// fillEllipse fills a circle at (cx, cy) with a solid color.
func (w *liteWindow) fillEllipse(g uintptr, cx, cy, r int32, c uint32) {
	if b := newSolidBrush(c); b != 0 {
		fillEllipseI(g, b, cx-r, cy-r, 2*r, 2*r)
		deleteBrush(b)
	}
}

// fillEllipseB fills a circle with an existing brush.
func (w *liteWindow) fillEllipseB(g uintptr, cx, cy, r int32, brush uintptr) {
	fillEllipseI(g, brush, cx-r, cy-r, 2*r, 2*r)
}

// fillPoly fills a closed polygon (a color arg).
func (w *liteWindow) fillPoly(g uintptr, pts []wPoint, c uint32) {
	b := newSolidBrush(c)
	if b == 0 {
		return
	}
	defer deleteBrush(b)
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	if addPathPolygon(p, pts) {
		fillPath(g, b, p)
	}
}

// strokePolyline fills a round-capped, round-joined stroke along the
// points with the given width — the capsule union.
func (w *liteWindow) strokePolyline(g uintptr, pts []wPoint, width int32, c uint32) {
	if len(pts) < 2 || width <= 0 {
		return
	}
	b := newSolidBrush(c)
	if b == 0 {
		return
	}
	defer deleteBrush(b)
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	r := width / 2
	if r < 1 {
		r = 1
	}
	for i := 0; i+1 < len(pts); i++ {
		addPathPolygon(p, capsulePoints(pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y, r))
	}
	fillPath(g, b, p)
}

// glowRRect paints an outward soft halo around a rounded rect: the
// path-gradient glow over an enlarged copy of the shape.
func (w *liteWindow) glowRRect(g uintptr, x, y, wid, hei, r, expand int32, c uint32) {
	if expand <= 0 || c>>24 == 0 {
		return
	}
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	rr := r + expand
	if rr > (wid+2*expand)/2 {
		rr = (wid + 2*expand) / 2
	}
	if rr > (hei+2*expand)/2 {
		rr = (hei + 2*expand) / 2
	}
	pts := rrectPoints(x-expand, y-expand, wid+2*expand, hei+2*expand, rr, 12)
	addPathPolygon(p, pts)
	br := newPathGlow(p, c, 0, int32(len(pts)))
	if br == 0 {
		return
	}
	defer deleteBrush(br)
	fillPath(g, br, p)
}

// (D119: glowEllipse and glowEllipseIn — soft halos built as full-circle
// path gradients — are GONE. Their 360-degree polygons carried a repeated
// closing point, so SetSurroundColorsWithCount was rejected by the real
// Windows GDI+ and the fill collapsed to the default opaque-white
// surround: a solid white disc where the cap's sheen belonged and a
// white-ringed blob under the menu button, while wine rendered both
// correctly the whole time. Every remaining glow rides rrectPoints —
// 52 distinct boundary points, count always accepted — and the sheen is
// a clipped linear-gradient band. No full-circle path gradients remain.)

// focusRing paints the 3 px focus outline around a rounded rect.
func (w *liteWindow) focusRing(g uintptr, rc wRect, r int32) {
	p := newPath(fillModeWinding)
	if p == 0 {
		return
	}
	defer deletePath(p)
	outer := rrectPoints(rc.Left-w.px(3), rc.Top-w.px(3), (rc.Right-rc.Left)+2*w.px(3), (rc.Bottom-rc.Top)+2*w.px(3), r+w.px(3), 12)
	inner := rrectPoints(rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, r, 12)
	for i, j := 0, len(inner)-1; i < j; i, j = i+1, j-1 {
		inner[i], inner[j] = inner[j], inner[i]
	}
	addPathPolygon(p, outer)
	addPathPolygon(p, inner)
	if b := newSolidBrush(argb(0xE6, 0xF4, 0xF0, 0xEE)); b != 0 {
		fillPath(g, b, p)
		deleteBrush(b)
	}
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// ---- the GDI text helpers (weights + ClearType, into the same DC) ----

// gdiText draws one string with one font and color (flags: DrawText
// format), optionally offset (the hero word's soft shadow pass).
func (w *liteWindow) gdiText(hdc uintptr, f windows.Handle, s string, rc wRect, flags uint32, color uint32, dx ...int32) {
	if s == "" {
		return
	}
	old, _, _ := procSelectObject.Call(hdc, uintptr(f))
	defer procSelectObject.Call(hdc, old)
	procSetBkMode.Call(hdc, transparentBk)
	procSetTextColor.Call(hdc, gdiRGB(color))
	r := rc
	if len(dx) == 2 {
		r = wRect{rc.Left + dx[0], rc.Top + dx[1], rc.Right + dx[0], rc.Bottom + dx[1]}
	}
	w.drawTextHdc(hdc, s, r, flags)
}

// drawTextHdc is the raw DrawTextW call (color already set).
func (w *liteWindow) drawTextHdc(hdc uintptr, s string, rc wRect, flags uint32) {
	ws, err := windows.UTF16FromString(s)
	if err != nil || len(ws) <= 1 {
		return
	}
	procDrawTextW.Call(
		hdc,
		uintptr(unsafe.Pointer(&ws[0])),
		uintptr(len(ws)-1),
		uintptr(unsafe.Pointer(&rc)),
		uintptr(flags),
	)
}

// measureText measures a string under a font (selects it temporarily).
func (w *liteWindow) measureText(hdc uintptr, f windows.Handle, s string) int32 {
	old, _, _ := procSelectObject.Call(hdc, uintptr(f))
	defer procSelectObject.Call(hdc, old)
	cx, _ := textExtent(hdc, s)
	return cx
}

// fillRectGdi is the plain-GDI flat fill (the fallback path).
func (w *liteWindow) fillRectGdi(hdc uintptr, l, t, r, b int32, color uint32) {
	br, _, _ := procCreateSolidBrush.Call(gdiRGB(color))
	rc := wRect{l, t, r, b}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), br)
	procDeleteObject.Call(br)
}
