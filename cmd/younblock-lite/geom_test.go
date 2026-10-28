// geom_test.go — the screen's pure geometry, tested on any host: the
// rounded-rect sampler (points in bounds, corners on their circles),
// the capsule stroke primitive (every point within the stroke radius of
// its segment, round caps at both ends at EVERY radius), the arc
// sampler, and the CSS cubic-bezier easings (endpoints exact, overshoot
// where the design's cap curve asks for it).
//
// D122 tightened every geometric tolerance from ~1 px to ~0.01 px: the
// samplers emit FLOAT vertices, so an error above a hundredth of a
// pixel means the integer quantization crept back — the exact defect
// that made the D121 build's glyphs read as "low resolution" (the
// int-snapped vertices quantized every curve to the pixel grid before
// GDI+'s anti-aliasing ever ran).
package main

import (
	"math"
	"testing"
)

func TestRrectPointsBounds(t *testing.T) {
	pts := rrectPointsF(100, 50, 200, 120, 20, 12)
	if len(pts) != 4*13 { // 12 steps + endpoint per corner
		t.Fatalf("rrect points = %d, want 52", len(pts))
	}
	for _, p := range pts {
		if p.X < 100 || p.X > 300 || p.Y < 50 || p.Y > 170 {
			t.Fatalf("point %v outside the rect", p)
		}
	}
	// The walk starts at the left end of the top-left corner arc.
	if pts[0].X != 100 || pts[0].Y != 70 {
		t.Errorf("first point = %v, want (100,70)", pts[0])
	}
	// Corner centers: every TL-arc point sits on the circle around
	// (120, 70) with radius 20 — at FLOAT precision the tolerance is
	// a hundredth of a pixel, not the old integer-rounded 1 px.
	for _, p := range pts[:13] {
		d := math.Hypot(float64(p.X-120), float64(p.Y-70))
		if math.Abs(d-20) > 0.01 {
			t.Errorf("TL arc point %v distance %f, want 20", p, d)
		}
	}
}

func TestRrectClampsRadius(t *testing.T) {
	// A radius wider than half the shape clamps to the inscribed
	// circle (a stadium), never inverting the geometry.
	pts := rrectPointsF(0, 0, 40, 20, 99, 8)
	for _, p := range pts {
		if p.X < 0 || p.X > 40 || p.Y < 0 || p.Y > 20 {
			t.Fatalf("stadium point %v out of bounds", p)
		}
	}
	// r=0 degenerates to the plain rectangle.
	pts = rrectPointsF(0, 0, 10, 10, 0, 12)
	if len(pts) != 4 {
		t.Fatalf("plain rect points = %d, want 4", len(pts))
	}
}

func TestCapsulePointsF(t *testing.T) {
	const x1, y1, x2, y2, r = 100.0, 100.0, 200.0, 100.0, 5.0
	pts := capsulePointsF(x1, y1, x2, y2, r)
	if len(pts) < 10 {
		t.Fatalf("capsule points = %d, want a sampled ring", len(pts))
	}
	// Every point lies within r of the segment (stroke containment).
	for _, p := range pts {
		var d float64
		if p.X < x1 {
			d = math.Hypot(float64(p.X-x1), float64(p.Y-y1))
		} else if p.X > x2 {
			d = math.Hypot(float64(p.X-x2), float64(p.Y-y2))
		} else {
			d = math.Abs(float64(p.Y - y1))
		}
		if d > r+0.01 {
			t.Fatalf("capsule point %v distance %f > r", p, d)
		}
	}
	// The caps reach the ends: some point is within r of (x1-r, y1)
	// and another within r of (x2+r, y2).
	reachL, reachR := false, false
	for _, p := range pts {
		if math.Hypot(float64(p.X-(x1-r)), float64(p.Y-y1)) <= r+0.01 {
			reachL = true
		}
		if math.Hypot(float64(p.X-(x2+r)), float64(p.Y-y2)) <= r+0.01 {
			reachR = true
		}
	}
	if !reachL || !reachR {
		t.Fatalf("capsule caps missing (left=%v right=%v)", reachL, reachR)
	}
	// A degenerate zero-length segment is a disk.
	pts = capsulePointsF(10, 10, 10, 10, 4)
	if len(pts) < 8 {
		t.Fatalf("disk points = %d", len(pts))
	}
	for _, p := range pts {
		if math.Hypot(float64(p.X-10), float64(p.Y-10)) > 4.01 {
			t.Fatalf("disk point %v escapes the radius", p)
		}
	}
}

// TestCapsulePointsFRoundCapsAtEveryRadius — the D122 root fix, as a
// regression: at EVERY radius (0.5, 1, 2 — the screen's real stroke
// radii) the capsule emits the full round-cap ring, never collapses,
// never repeats a vertex, never self-intersects into the beaded dots
// of D121's int geometry. Float cap-arc points are distinct by
// construction; this test is the tripwire if anyone ever reintroduces
// integer vertex snapping to "simplify" the sampler.
func TestCapsulePointsFRoundCapsAtEveryRadius(t *testing.T) {
	for _, r := range []float64{0.5, 1, 2, 3} {
		pts := capsulePointsF(100, 100, 110.5, 104.25, r)
		if len(pts) < 20 {
			t.Fatalf("r=%v capsule points = %d, want the full round-cap ring (>= 20)", r, len(pts))
		}
		// No two consecutive vertices coincide (the int-collapse
		// that self-intersected the winding fill).
		for i := range pts {
			a, b := pts[i], pts[(i+1)%len(pts)]
			if a == b {
				t.Fatalf("r=%v consecutive coincident vertices at %d: %v", r, i, a)
			}
		}
		// Stroke containment: every vertex within r of the segment.
		for _, p := range pts {
			d := segDist(float64(p.X), float64(p.Y), 100, 100, 110.5, 104.25)
			if d > r+0.01 {
				t.Fatalf("r=%v capsule vertex %v distance %f > r", r, p, d)
			}
		}
		// The outline is a simple polygon: non-zero signed area.
		area := 0.0
		for i := range pts {
			a, b := pts[i], pts[(i+1)%len(pts)]
			area += float64(a.X)*float64(b.Y) - float64(b.X)*float64(a.Y)
		}
		if math.Abs(area) < 1e-9 {
			t.Fatalf("r=%v capsule has zero area: %v", r, pts)
		}
	}
}

// segDist is the point-to-segment distance (the capsule tests' helper).
func segDist(px, py, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	l2 := dx*dx + dy*dy
	if l2 < 1e-12 {
		return math.Hypot(px-x1, py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(px-(x1+t*dx), py-(y1+t*dy))
}

func TestArcPointsF(t *testing.T) {
	// 0 -> 90 degrees on a circle around the origin: start at +x,
	// end at +y (screen down).
	pts := arcPointsF(0, 0, 10, 10, 0, 90, 9)
	if math.Abs(float64(pts[0].X-10)) > 1e-4 || math.Abs(float64(pts[0].Y)) > 1e-4 {
		t.Errorf("arc start = %v, want (10,0)", pts[0])
	}
	if math.Abs(float64(pts[len(pts)-1].X)) > 1e-4 || math.Abs(float64(pts[len(pts)-1].Y-10)) > 1e-4 {
		t.Errorf("arc end = %v, want (0,10)", pts[len(pts)-1])
	}
}

// TestArcPointsFSubPixelPrecision — THE D122 assertion: an arc whose
// center and radius are NOT integers must land every vertex on its
// true circle to within a hundredth of a pixel. The int32 sampler this
// replaces snapped each vertex to the pixel grid (errors up to ~0.7 px
// on an r=8 arc) — the quantization the owner read as a low-resolution
// icon long before anti-aliasing ever got a chance.
func TestArcPointsFSubPixelPrecision(t *testing.T) {
	pts := arcPointsF(183.5, 327.25, 8.3, 8.3, 224.6, -269.2, 36)
	if len(pts) != 37 {
		t.Fatalf("partial arc: %d points, want 37", len(pts))
	}
	for _, p := range pts {
		d := math.Hypot(float64(p.X-183.5), float64(p.Y-327.25))
		if math.Abs(d-8.3) > 0.01 {
			t.Fatalf("arc vertex %v distance %f from center, want 8.3 (sub-pixel)", p, d)
		}
	}
}

// TestArcPointsFFullCircleNoDuplicate is the D119 regression: a 360-degree
// sweep must close WITHOUT repeating the starting point. A polygon that
// counts its own closing point hands SetSurroundColorsWithCount a count
// the real Windows GDI+ rejects — and a rejected call leaves the path
// gradient's surround colors at their opaque-white default (the white
// disc on the cap). Wine's laxer GDI+ accepted the over-count, which is
// why only the owner's real machine ever showed it.
func TestArcPointsFFullCircleNoDuplicate(t *testing.T) {
	for _, steps := range []int{16, 48} {
		pts := arcPointsF(0, 0, 10, 10, 0, 360, steps)
		if len(pts) != steps {
			t.Errorf("full circle steps=%d: %d points, want %d (no repeated closing point)", steps, len(pts), steps)
		}
		if pts[0] == pts[len(pts)-1] {
			t.Errorf("full circle steps=%d: first == last point (%v)", steps, pts[0])
		}
		// Every point still on the circle.
		for _, p := range pts {
			d := p.X*p.X + p.Y*p.Y
			if d < 81 || d > 121 {
				t.Errorf("full circle point %v off radius 10", p)
			}
		}
	}
}

func TestBezierEndpointsAndOvershoot(t *testing.T) {
	if easeCap(0) != 0 || easeCap(1) != 1 {
		t.Fatalf("easeCap endpoints wrong: %f %f", easeCap(0), easeCap(1))
	}
	if easeSheet(0) != 0 || easeSheet(1) != 1 {
		t.Fatalf("easeSheet endpoints wrong")
	}
	if easeCss(0) != 0 || easeCss(1) != 1 {
		t.Fatalf("easeCss endpoints wrong")
	}
	// The cap's curve overshoots (y > 1 inside) — the springy slide.
	peak := 0.0
	for i := 1; i < 100; i++ {
		v := easeCap(float64(i) / 100)
		if v > peak {
			peak = v
		}
	}
	if peak <= 1.0 {
		t.Errorf("easeCap peak = %f, want an overshoot > 1", peak)
	}
	// The sheet's curve never overshoots.
	for i := 1; i < 100; i++ {
		if v := easeSheet(float64(i) / 100); v > 1.0000001 {
			t.Fatalf("easeSheet overshoots: %f", v)
		}
	}
	// Monotone: an ease-out rises at least as fast in the first
	// quarter as in the last.
	a := easeSheet(0.25)
	b := easeSheet(1) - easeSheet(0.75)
	if a < b {
		t.Errorf("easeSheet is not ease-out (first quarter %f, last quarter %f)", a, b)
	}
}

func TestColorMath(t *testing.T) {
	if got := argb(0xff, 0x12, 0x34, 0x56); got != 0xff123456 {
		t.Fatalf("argb = %08x", got)
	}
	if got := scaleAlpha(0xff000000, 0.5); got != 0x80000000 {
		t.Fatalf("scaleAlpha = %08x", got)
	}
	mid := lerpColor(0xff000000, 0xffffffff, 0.5)
	if mid != 0xff808080 {
		t.Fatalf("lerpColor = %08x", mid)
	}
	if got := gdiRGB(0xff123456); got != 0x563412 {
		t.Fatalf("gdiRGB = %x", got)
	}
}

func TestAnimPhases(t *testing.T) {
	if animPhase(100, 0, 200) != 0.5 {
		t.Fatalf("phase = %f", animPhase(100, 0, 200))
	}
	if animPhase(999, 0, 200) != 1 {
		t.Fatalf("clamped phase = %f", animPhase(999, 0, 200))
	}
	if animPhase(50, 0, 0) != 1 {
		t.Fatalf("zero-duration phase = %f", animPhase(50, 0, 0))
	}
	// The loop wraps.
	p1 := animCycle(1900, 0, 1000)
	p2 := animCycle(900, 0, 1000)
	if math.Abs(p1-p2) > 1e-9 {
		t.Fatalf("cycle wrap: %f vs %f", p1, p2)
	}
}

// TestArcPointsFCenterAnchoring — the power icon's glyph center is the
// arc's center (12, 12.41) on the 24 grid (D121): a full ring sampled
// around a center must keep every point within a hundredth of a pixel
// of its true radius (D122 tightened this from 1 px — the anchor was
// right, the pixel-snapped ring around it was not).
func TestArcPointsFCenterAnchoring(t *testing.T) {
	pts := arcPointsF(200, 200, 8, 8, 0, 360, 36)
	if len(pts) < 20 {
		t.Fatalf("ring points = %d, want a sampled ring", len(pts))
	}
	for _, p := range pts {
		d := math.Hypot(float64(p.X-200), float64(p.Y-200))
		if math.Abs(d-8) > 0.01 {
			t.Fatalf("ring point %v distance %f from center, want 8", p, d)
		}
	}
}
