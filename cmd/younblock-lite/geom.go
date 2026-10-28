// geom.go — the screen's pure geometry and color math (the cubic-bezier
// easings, the animation phases, the polygon samplers that turn rounded
// rectangles and round-capped strokes into GDI+ paths, and the ARGB
// helpers). OS-neutral on purpose: the token math is unit-tested on
// every host beside the view mapping, and the Windows painter is a thin
// consumer of it (see gdiplus_windows.go for why the shapes are
// polygon-sampled at all).
package main

import "math"

// wPoint is a Win32 ABI point (two int32s — mouse messages and
// MINMAXINFO; NOT path geometry).
type wPoint struct{ X, Y int32 }

// gPointF is GDI+'s PointF — the PATH vertex. D122: vertices are
// float32, not int32 — the polygon samplers run at sub-pixel precision
// and GDI+'s AntiAlias then smooths genuinely smooth curves (the
// int32 vertices quantized every arc to the pixel grid before AA ever
// ran — the "low resolution" glyphs of D121's build). The type stays
// OS-neutral here so the samplers unit-test on every host.
type gPointF struct{ X, Y float32 }

// fpt is the float32 vertex constructor (float64 math in, float32 out).
func fpt(x, y float64) gPointF { return gPointF{X: float32(x), Y: float32(y)} }

// argb packs a GDI+ ARGB color.
func argb(a, r, g, b uint32) uint32 { return a<<24 | r<<16 | g<<8 | b }

func dedupPtsF(pts []gPointF) []gPointF {
	out := pts[:0]
	for _, p := range pts {
		if len(out) == 0 {
			out = append(out, p)
			continue
		}
		last := out[len(out)-1]
		if math.Abs(float64(p.X-last.X))+math.Abs(float64(p.Y-last.Y)) < 1e-4 {
			continue
		}
		out = append(out, p)
	}
	if len(out) > 1 {
		first, last := out[0], out[len(out)-1]
		if math.Abs(float64(first.X-last.X))+math.Abs(float64(first.Y-last.Y)) < 1e-4 {
			out = out[:len(out)-1]
		}
	}
	return out
}

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

// ---- path geometry (float polygon sampling; see the file headers) ----

// arcPointsF samples an elliptical arc — screen angles (degrees,
// clockwise, 0 = +x, 90 = +y) matching how the design's SVG paths read
// in a y-down coordinate system. A full 360-degree sweep never repeats
// the starting point (D119): GDI+ merges a duplicated closing point back
// into the figure, so a polygon that counts its own closing point feeds
// SetSurroundColorsWithCount a count the real Windows GDI+ rejects —
// leaving the surround colors at their OPAQUE-WHITE default (the
// white-disc bug; wine's laxer GDI+ accepts the mismatch, which is why
// the wine oracle alone could never catch it).
// D122: vertices carry sub-pixel precision — the sampler rounds nothing.
func arcPointsF(cx, cy, rx, ry float64, startDeg, sweepDeg float64, steps int) []gPointF {
	if steps < 1 {
		steps = 1
	}
	pts := make([]gPointF, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := (startDeg + sweepDeg*float64(i)/float64(steps)) * math.Pi / 180
		pts = append(pts, fpt(cx+rx*math.Cos(a), cy+ry*math.Sin(a)))
	}
	if len(pts) > 1 {
		// Full circles close without a repeated closing point. Float
		// trig leaves a residue (sin(2pi) ~ 1e-16), so near-equality
		// — not exact equality — is the duplicate test.
		dx := float64(pts[0].X - pts[len(pts)-1].X)
		dy := float64(pts[0].Y - pts[len(pts)-1].Y)
		if math.Abs(dx)+math.Abs(dy) < 1e-4 {
			pts = pts[:len(pts)-1]
		}
	}
	return pts
}

// rrectPointsF returns the clockwise outline of a rounded rectangle as
// a polygon (arcSteps points per corner; 12 is visually an arc at these
// radii — the chord error at r=26 is 0.06 px, and at float precision it
// is the ONLY error left).
func rrectPointsF(x, y, w, h, r float64, arcSteps int) []gPointF {
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
		return []gPointF{fpt(x, y), fpt(x+w, y), fpt(x+w, y+h), fpt(x, y+h)}
	}
	pts := make([]gPointF, 0, arcSteps*4+4)
	// Start at the left end of the top-left corner arc, walk
	// clockwise: TL, top edge, TR, right edge, BR, bottom edge, BL.
	pts = append(pts, arcPointsF(x+r, y+r, r, r, 180, 90, arcSteps)...)   // (x,y+r)->(x+r,y)
	pts = append(pts, arcPointsF(x+w-r, y+r, r, r, 270, 90, arcSteps)...) // (x+w-r,y)->(x+w,y+r)
	pts = append(pts, arcPointsF(x+w-r, y+h-r, r, r, 0, 90, arcSteps)...) // (x+w,y+h-r)->(x+w-r,y+h)
	pts = append(pts, arcPointsF(x+r, y+h-r, r, r, 90, 90, arcSteps)...)  // (x+r,y+h)->(x,y+h-r)
	return pts
}

// capsulePointsF returns one counter-clockwise capsule (a line segment
// with round caps, as a closed polygon) — the stroke primitive.
//
// D121 history: with INT vertices, r <= 1 (the screen's 2 px strokes)
// collapsed the round-cap arcs into runs of near-coincident points
// whose polygon self-intersected, and GDI+'s winding fill left only
// fragments — the owner's "artifacts like dots". The D121 fix was
// square caps at small radii. D122 removes the degeneracy at its root:
// FLOAT vertices keep every cap-arc point distinct at any radius, so
// the true round caps (the design's actual stroke caps) return at every
// width, and the polygon can never self-intersect.
func capsulePointsF(x1, y1, x2, y2, r float64) []gPointF {
	dx, dy := x2-x1, y2-y1
	ln := math.Hypot(dx, dy)
	if ln < 1e-6 || r <= 0 {
		// Degenerate: a disk.
		return arcPointsF(x1, y1, r, r, 0, 360, 16)
	}
	ux, uy := dx/ln, dy/ln
	nx, ny := -uy, ux // the left-hand normal (screen y-down)
	ax, ay := x1+nx*r, y1+ny*r
	bx, by := x2+nx*r, y2+ny*r
	cx, cy := x2-nx*r, y2-ny*r
	dxp, dyp := x1-nx*r, y1-ny*r
	nDeg := math.Atan2(ny, nx) * 180 / math.Pi
	pts := []gPointF{fpt(ax, ay), fpt(bx, by)}
	// End cap: from +n around the forward direction to -n (sweep -180
	// keeps the outline counter-clockwise in screen coordinates).
	pts = append(pts, arcPointsF(x2, y2, r, r, nDeg, -180, 10)...)
	pts = append(pts, fpt(cx, cy), fpt(dxp, dyp))
	// Start cap: from -n around the backward direction to +n.
	pts = append(pts, arcPointsF(x1, y1, r, r, nDeg+180, -180, 10)...)
	return dedupPtsF(pts)
}

// dedupPtsF drops consecutive vertices that land on the same float32
// point, plus a trailing vertex that closes onto the first. Two
// mathematically identical points computed by different routes — the
// side point x2+nx*r and the cap arc's first sample, the ring's close
// and its start — can round together at small radii; the zero-length
// edges they make are harmless to GDI+'s fill, but a minimal outline
// is cleaner to test, to reason about, and to keep degenerate-free
// (the D121 lesson: collapsed geometry is where winding fill begins
// to bead).
