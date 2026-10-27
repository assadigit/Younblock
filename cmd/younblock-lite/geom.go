// geom.go — the screen's pure geometry and color math (the cubic-bezier
// easings, the animation phases, the polygon samplers that turn rounded
// rectangles and round-capped strokes into GDI+ paths, and the ARGB
// helpers). OS-neutral on purpose: the token math is unit-tested on
// every host beside the view mapping, and the Windows painter is a thin
// consumer of it (see gdiplus_windows.go for why the shapes are
// polygon-sampled at all).
package main

import "math"

// wPoint is a screen point (GDI+'s Point layout: two int32s).
type wPoint struct{ X, Y int32 }

// argb packs a GDI+ ARGB color.
func argb(a, r, g, b uint32) uint32 { return a<<24 | r<<16 | g<<8 | b }

// ---- easing and color math ----

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// bezierY solves the cubic-bezier easing curve (the CSS timing
// function) at x in [0,1]: find s with Bx(s) = x, return By(s).
func bezierY(x1, y1, x2, y2, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	// Newton first (the curves are nearly linear in s), bisect as
	// the fallback — 24 iterations is far past pixel precision.
	s := x
	for i := 0; i < 8; i++ {
		cx := 3*x1*s*(1-s)*(1-s) + 3*x2*s*s*(1-s) + s*s*s - x
		if math.Abs(cx) < 1e-6 {
			break
		}
		d := 3*x1*(1-s)*(1-4*s+3*s*s) + 3*x2*s*(2-3*s) + 3*s*s
		if math.Abs(d) < 1e-8 {
			break
		}
		s -= cx / d
		s = clamp01(s)
	}
	return 3*y1*s*(1-s)*(1-s) + 3*y2*s*s*(1-s) + s*s*s
}

// The screen's named curves.
var (
	easeCap   = func(t float64) float64 { return bezierY(0.2, 0.9, 0.3, 1.15, t) } // --yb-ease-cap
	easeSheet = func(t float64) float64 { return bezierY(0.22, 1, 0.36, 1, t) }    // --ease-out (the sheet)
	easeCss   = func(t float64) float64 { return bezierY(0.25, 0.1, 0.25, 1, t) }  // "ease"
)

// animPhase is 0..1 across [start, start+dur] ms (clamped).
func animPhase(now, start, dur int64) float64 {
	if dur <= 0 {
		return 1
	}
	return clamp01(float64(now-start) / float64(dur))
}

// animCycle is the looping phase 0..1 of a period started at start.
func animCycle(now, start, period int64) float64 {
	if period <= 0 {
		return 0
	}
	el := (now - start) % period
	if el < 0 {
		el += period
	}
	return float64(el) / float64(period)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// scaleAlpha scales a color's alpha channel by f (clamped, rounded).
func scaleAlpha(c uint32, f float64) uint32 {
	a := float64(c>>24)*clamp01(f) + 0.5
	return uint32(a)<<24 | c&0x00FFFFFF
}

// lerpColor blends two ARGB colors (premultiplied by nothing — plain
// per-channel interpolation).
func lerpColor(a, b uint32, t float64) uint32 {
	ar, ag, ab, aa := (a>>16)&0xFF, (a>>8)&0xFF, a&0xFF, a>>24
	br, bg, bb, ba := (b>>16)&0xFF, (b>>8)&0xFF, b&0xFF, b>>24
	ch := func(x, y uint32) uint32 { return uint32(lerp(float64(x), float64(y), t) + 0.5) }
	return ch(aa, ba)<<24 | ch(ar, br)<<16 | ch(ag, bg)<<8 | ch(ab, bb)
}

// gdiRGB turns an ARGB token into a GDI COLORREF (0x00BBGGRR) for
// SetTextColor — alpha is dropped (GDI text has none).
func gdiRGB(c uint32) uintptr {
	return uintptr((c&0xFF)<<16 | (c>>8&0xFF)<<8 | c>>16&0xFF)
}

// ---- path geometry (polygon sampling; see the file header) ----

// arcPoints samples an elliptical arc — screen angles (degrees,
// clockwise, 0 = +x, 90 = +y) matching how the design's SVG paths read
// in a y-down coordinate system.
func arcPoints(cx, cy, rx, ry int32, startDeg, sweepDeg float64, steps int) []wPoint {
	if steps < 1 {
		steps = 1
	}
	pts := make([]wPoint, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := (startDeg + sweepDeg*float64(i)/float64(steps)) * math.Pi / 180
		pts = append(pts, wPoint{
			X: cx + int32(math.Round(float64(rx)*math.Cos(a))),
			Y: cy + int32(math.Round(float64(ry)*math.Sin(a))),
		})
	}
	return pts
}

// rrectPoints returns the clockwise outline of a rounded rectangle as
// a polygon (arcSteps points per corner; 12 is visually an arc at these
// radii).
func rrectPoints(x, y, w, h, r int32, arcSteps int) []wPoint {
	if r < 0 {
		r = 0
	}
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	if r == 0 {
		return []wPoint{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
	}
	pts := make([]wPoint, 0, arcSteps*4+4)
	// Start at the left end of the top-left corner arc, walk
	// clockwise: TL, top edge, TR, right edge, BR, bottom edge, BL.
	pts = append(pts, arcPoints(x+r, y+r, r, r, 180, 90, arcSteps)...)   // (x,y+r)->(x+r,y)
	pts = append(pts, arcPoints(x+w-r, y+r, r, r, 270, 90, arcSteps)...) // (x+w-r,y)->(x+w,y+r)
	pts = append(pts, arcPoints(x+w-r, y+h-r, r, r, 0, 90, arcSteps)...) // (x+w,y+h-r)->(x+w-r,y+h)
	pts = append(pts, arcPoints(x+r, y+h-r, r, r, 90, 90, arcSteps)...)  // (x+r,y+h)->(x,y+h-r)
	return pts
}

// capsulePoints returns one counter-clockwise capsule (a line segment
// with round caps, as a closed polygon) — the stroke primitive.
func capsulePoints(x1, y1, x2, y2, r int32) []wPoint {
	dx, dy := float64(x2-x1), float64(y2-y1)
	len := math.Hypot(dx, dy)
	if len < 1e-6 || r <= 0 {
		// Degenerate: a disk.
		return arcPoints(x1, y1, r, r, 0, 360, 16)
	}
	ux, uy := dx/len, dy/len
	nx, ny := -uy, ux // the left-hand normal (screen y-down)
	nDeg := math.Atan2(ny, nx) * 180 / math.Pi
	ax, ay := x1+int32(nx*float64(r)), y1+int32(ny*float64(r))
	bx, by := x2+int32(nx*float64(r)), y2+int32(ny*float64(r))
	cx, cy := x2-int32(nx*float64(r)), y2-int32(ny*float64(r))
	dxp, dyp := x1-int32(nx*float64(r)), y1-int32(ny*float64(r))
	pts := []wPoint{{ax, ay}, {bx, by}}
	// End cap: from +n around the forward direction to -n (sweep -180
	// keeps the outline counter-clockwise in screen coordinates).
	pts = append(pts, arcPoints(x2, y2, r, r, nDeg, -180, 10)...)
	pts = append(pts, wPoint{cx, cy}, wPoint{dxp, dyp})
	// Start cap: from -n around the backward direction to +n.
	pts = append(pts, arcPoints(x1, y1, r, r, nDeg+180, -180, 10)...)
	return pts
}
