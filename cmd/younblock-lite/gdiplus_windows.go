//go:build windows

// gdiplus_windows.go — the GDI+ flat-API syscall layer for the lite
// desktop edition's screen (D115). GDI+ ships inside every Windows
// since XP as a system component (gdiplus.dll) — nothing to install,
// exactly like user32 and gdi32 — and its anti-aliased paths, gradient
// brushes and per-pixel-alpha images are what lift the lite screen to
// the v3 design's look.
//
// THE ONE RULE OF THIS FILE: only entry points whose parameters are
// ALL integers, enums or pointers are bound. GDI+'s floating-point
// variants (GdipAddPathArc's REAL angles, GdipCreatePen1's REAL width,
// GdipCreateFont's REAL em size) cannot be called through Go's syscall
// layer on windows/amd64, where by-value floats travel in XMM
// registers the syscall interface never fills. Every float the screen
// needs therefore reaches GDI+ through MEMORY (PointF and blend-array
// pointers) or not at all:
//   - rounded shapes and strokes are polygon-sampled paths
//     (GdipAddPathPolygon — the PointF variant: the vertex array is a
//     POINTER, so the floats live in memory and the ABI rule holds),
//   - D122: the vertices are FLOAT, not int — GdipAddPathPolygonI's
//     int32 vertices quantized every curve to the pixel grid BEFORE
//     anti-aliasing ever ran, which was the owner's "low resolution"
//     power icon and menu button (the AA smoothed the polygon's edges,
//     but a polygon whose vertices snap to whole pixels is a crude
//     approximation of the curve it samples). Sub-pixel vertices +
//     AntiAlias = reference-quality curves,
//   - gradients take their geometry as PointF pointers and their color
//     stops as an ARGB array + REAL* positions,
//   - text stays on plain GDI (CreateFontIndirectW carries integer
//     weights 400-700, DrawTextW centers it) on the same memory DC.
package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var modGdiPlus = windows.NewLazySystemDLL("gdiplus.dll")

var (
	procGdiplusStartup         = modGdiPlus.NewProc("GdiplusStartup")
	procGdiplusShutdown        = modGdiPlus.NewProc("GdiplusShutdown")
	procGdipCreateFromHDC      = modGdiPlus.NewProc("GdipCreateFromHDC")
	procGdipDeleteGraphics     = modGdiPlus.NewProc("GdipDeleteGraphics")
	procGdipSetSmoothingMode   = modGdiPlus.NewProc("GdipSetSmoothingMode")
	procGdipCreatePath         = modGdiPlus.NewProc("GdipCreatePath")
	procGdipDeletePath         = modGdiPlus.NewProc("GdipDeletePath")
	procGdipAddPathPolygon     = modGdiPlus.NewProc("GdipAddPathPolygon")
	procGdipClosePathFigure    = modGdiPlus.NewProc("GdipClosePathFigure")
	procGdipCreateSolidFill    = modGdiPlus.NewProc("GdipCreateSolidFill")
	procGdipDeleteBrush        = modGdiPlus.NewProc("GdipDeleteBrush")
	procGdipCreateLineBrush    = modGdiPlus.NewProc("GdipCreateLineBrush")
	procGdipSetLinePresetBlend = modGdiPlus.NewProc("GdipSetLinePresetBlend")
	procGdipSetClipPath        = modGdiPlus.NewProc("GdipSetClipPath")
	procGdipResetClip          = modGdiPlus.NewProc("GdipResetClip")
	procGdipFillPath           = modGdiPlus.NewProc("GdipFillPath")
	procGdipFillEllipseI       = modGdiPlus.NewProc("GdipFillEllipseI")
	procGdipFillRectangleI     = modGdiPlus.NewProc("GdipFillRectangleI")
)

// gdiplusStartupInput is GdiplusStartup's ABI struct (version 1, no
// callbacks, no suppressed threads — the plain default).
type gdiplusStartupInput struct {
	Version                  uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}

// gPointF is GDI+'s PointF — always passed BY POINTER here, so its
// floats live in memory, never in registers. (Defined in geom.go —
// the float samplers are OS-neutral and unit-tested on every host.)

const (
	smoothingModeAntiAlias = 4
	fillModeAlternate      = 0 // even-odd: overlapping subfigures punch holes
	fillModeWinding        = 1 // non-zero: overlapping subfigures union
	wrapModeTileFlipXY     = 3 // the linear-gradient brush rejects Clamp (D118)
)

// gdiplusToken is the one startup token (runUI's lifetime).
var gdiplusToken uintptr

// gdiplusStartup brings GDI+ up once for the whole window life; a
// failure is not fatal — the screen falls back to GDI-only drawing
// (flat colors instead of gradients) rather than not appearing at all.
func gdiplusStartup() bool {
	in := gdiplusStartupInput{Version: 1}
	r, _, _ := procGdiplusStartup.Call(
		uintptr(unsafe.Pointer(&gdiplusToken)),
		uintptr(unsafe.Pointer(&in)),
		0,
	)
	return r == 0
}

// gdiplusShutdown takes GDI+ down on exit (after the message loop).
func gdiplusShutdown() {
	if gdiplusToken != 0 {
		procGdiplusShutdown.Call(gdiplusToken)
		gdiplusToken = 0
	}
}

// gpOK reports a GDI+ status of Ok(0).
func gpOK(r uintptr) bool { return r == 0 }

// ---- graphics ----

// newGraphics wraps a DC (the paint's memory buffer) in a GDI+ graphics
// with anti-aliased edges; the caller deletes it before the BitBlt.
func newGraphics(hdc uintptr) uintptr {
	var g uintptr
	if r, _, _ := procGdipCreateFromHDC.Call(hdc, uintptr(unsafe.Pointer(&g))); !gpOK(r) {
		return 0
	}
	procGdipSetSmoothingMode.Call(g, smoothingModeAntiAlias)
	return g
}

func deleteGraphics(g uintptr) {
	if g != 0 {
		procGdipDeleteGraphics.Call(g)
	}
}

// ---- paths ----

func newPath(fillMode int32) uintptr {
	var p uintptr
	if r, _, _ := procGdipCreatePath.Call(uintptr(fillMode), uintptr(unsafe.Pointer(&p))); !gpOK(r) {
		return 0
	}
	return p
}

func deletePath(p uintptr) {
	if p != 0 {
		procGdipDeletePath.Call(p)
	}
}

func addPathPolygon(p uintptr, pts []gPointF) bool {
	if p == 0 || len(pts) == 0 {
		return false
	}
	r, _, _ := procGdipAddPathPolygon.Call(p, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	return gpOK(r)
}

// ---- brushes ----

func newSolidBrush(argb uint32) uintptr {
	var b uintptr
	if r, _, _ := procGdipCreateSolidFill.Call(uintptr(argb), uintptr(unsafe.Pointer(&b))); !gpOK(r) {
		return 0
	}
	return b
}

func deleteBrush(b uintptr) {
	if b != 0 {
		procGdipDeleteBrush.Call(b)
	}
}

// newVGradient builds a vertical linear-gradient brush over (x, y, w, h)
// with the given ARGB stops (len(colors) == len(positions), first 0.0,
// last 1.0). The gradient's float geometry reaches GDI+ as PointF
// pointers — memory, never registers.
func newVGradient(x, y, w, h int32, colors []uint32, positions []float32) uintptr {
	if len(colors) == 0 || len(colors) != len(positions) {
		return 0
	}
	p1 := gPointF{X: float32(x), Y: float32(y)}
	p2 := gPointF{X: float32(x), Y: float32(y + h)}
	var b uintptr
	r1, _, _ := procGdipCreateLineBrush.Call(
		uintptr(unsafe.Pointer(&p1)),
		uintptr(unsafe.Pointer(&p2)),
		uintptr(colors[0]), uintptr(colors[len(colors)-1]),
		wrapModeTileFlipXY,
		uintptr(unsafe.Pointer(&b)),
	)
	if !gpOK(r1) {
		return 0
	}
	r2, _, _ := procGdipSetLinePresetBlend.Call(
		b,
		uintptr(unsafe.Pointer(&colors[0])),
		uintptr(unsafe.Pointer(&positions[0])),
		uintptr(len(colors)),
	)
	if !gpOK(r2) {
		procGdipDeleteBrush.Call(b)
		return 0
	}
	return b
}

// ---- clipping (deterministic fills inside a silhouette) ----

// combineModeReplace is GDI+'s CombineModeReplace (the clip becomes
// exactly this path — no intersection with any prior clip).
const combineModeReplace = 0

// setClipPath confines every later fill on this graphics to the path's
// interior (D119: the cap's sheen band and top bevel run inside the
// cap's own silhouette, so nothing can poke over the rounded corners).
func setClipPath(g, path uintptr) {
	if g != 0 && path != 0 {
		procGdipSetClipPath.Call(g, path, combineModeReplace)
	}
}

// resetClip restores the graphics to unclipped fills.
func resetClip(g uintptr) {
	if g != 0 {
		procGdipResetClip.Call(g)
	}
}

// ---- fills ----

func fillPath(g, brush, path uintptr) {
	if g != 0 && brush != 0 && path != 0 {
		procGdipFillPath.Call(g, brush, path)
	}
}

func fillEllipseI(g, brush uintptr, x, y, w, h int32) {
	if g != 0 && brush != 0 {
		procGdipFillEllipseI.Call(g, brush, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
	}
}

func fillRectI(g, brush uintptr, x, y, w, h int32) {
	if g != 0 && brush != 0 {
		procGdipFillRectangleI.Call(g, brush, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
	}
}
