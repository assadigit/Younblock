// geom_test.go — the screen's pure geometry, tested on any host: the
// rounded-rect sampler (points in bounds, corners on their circles),
// the capsule stroke primitive (every point within the stroke radius of
// its segment, caps at both ends), the arc sampler, and the CSS
// cubic-bezier easings (endpoints exact, overshoot where the design's
// cap curve asks for it).
package main

import (
	"math"
	"testing"
)

func TestRrectPointsBounds(t *testing.T) {
	pts := rrectPoints(100, 50, 200, 120, 20, 12)
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
	// (120, 70) with radius 20 (1px tolerance for the rounding).
	for _, p := range pts[:13] {
		d := math.Hypot(float64(p.X-120), float64(p.Y-70))
		if math.Abs(d-20) > 1.01 {
			t.Errorf("TL arc point %v distance %f, want 20", p, d)
		}
	}
}

func TestRrectClampsRadius(t *testing.T) {
	// A radius wider than half the shape clamps to the inscribed
	// circle (a stadium), never inverting the geometry.
	pts := rrectPoints(0, 0, 40, 20, 99, 8)
	for _, p := range pts {
		if p.X < 0 || p.X > 40 || p.Y < 0 || p.Y > 20 {
			t.Fatalf("stadium point %v out of bounds", p)
		}
	}
	// r=0 degenerates to the plain rectangle.
	pts = rrectPoints(0, 0, 10, 10, 0, 12)
	if len(pts) != 4 {
		t.Fatalf("plain rect points = %d, want 4", len(pts))
	}
}

func TestCapsulePoints(t *testing.T) {
	var x1, y1, x2, y2, r int32 = 100, 100, 200, 100, 5
	pts := capsulePoints(x1, y1, x2, y2, r)
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
		if d > float64(r)+1.01 {
			t.Fatalf("capsule point %v distance %f > r", p, d)
		}
	}
	// The caps reach the ends: some point is within r of (x1-r, y1)
	// and another within r of (x2+r, y2).
	reachL, reachR := false, false
	for _, p := range pts {
		if math.Hypot(float64(p.X-(x1-r)), float64(p.Y-y1)) <= float64(r)+1.01 {
			reachL = true
		}
		if math.Hypot(float64(p.X-(x2+r)), float64(p.Y-y2)) <= float64(r)+1.01 {
			reachR = true
		}
	}
	if !reachL || !reachR {
		t.Fatalf("capsule caps missing (left=%v right=%v)", reachL, reachR)
	}
	// A degenerate zero-length segment is a disk.
	pts = capsulePoints(10, 10, 10, 10, 4)
	if len(pts) < 8 {
		t.Fatalf("disk points = %d", len(pts))
	}
	for _, p := range pts {
		if math.Hypot(float64(p.X-10), float64(p.Y-10)) > 5.01 {
			t.Fatalf("disk point %v escapes the radius", p)
		}
	}
}

func TestArcPoints(t *testing.T) {
	// 0 -> 90 degrees on a circle around the origin: start at +x,
	// end at +y (screen down).
	pts := arcPoints(0, 0, 10, 10, 0, 90, 9)
	if pts[0].X != 10 || pts[0].Y != 0 {
		t.Errorf("arc start = %v, want (10,0)", pts[0])
	}
	if pts[len(pts)-1].X != 0 || pts[len(pts)-1].Y != 10 {
		t.Errorf("arc end = %v, want (0,10)", pts[len(pts)-1])
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
