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
//   - rounded shapes are polygon-sampled paths (GdipAddPathPolygonI),
//   - strokes are filled capsule paths (round caps and joins by
//     construction),
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
	procGdiplusStartup                    = modGdiPlus.NewProc("GdiplusStartup")
	procGdiplusShutdown                   = modGdiPlus.NewProc("GdiplusShutdown")
	procGdipCreateFromHDC                 = modGdiPlus.NewProc("GdipCreateFromHDC")
	procGdipDeleteGraphics                = modGdiPlus.NewProc("GdipDeleteGraphics")
	procGdipSetSmoothingMode              = modGdiPlus.NewProc("GdipSetSmoothingMode")
	procGdipCreatePath                    = modGdiPlus.NewProc("GdipCreatePath")
	procGdipDeletePath                    = modGdiPlus.NewProc("GdipDeletePath")
	procGdipAddPathLineI                  = modGdiPlus.NewProc("GdipAddPathLineI")
	procGdipAddPathPolygonI               = modGdiPlus.NewProc("GdipAddPathPolygonI")
	procGdipAddPathEllipseI               = modGdiPlus.NewProc("GdipAddPathEllipseI")
	procGdipClosePathFigure               = modGdiPlus.NewProc("GdipClosePathFigure")
	procGdipCreateSolidFill               = modGdiPlus.NewProc("GdipCreateSolidFill")
	procGdipDeleteBrush                   = modGdiPlus.NewProc("GdipDeleteBrush")
	procGdipCreateLineBrush               = modGdiPlus.NewProc("GdipCreateLineBrush")
	procGdipSetLinePresetBlend            = modGdiPlus.NewProc("GdipSetLinePresetBlend")
	procGdipCreatePathGradientFromPath    = modGdiPlus.NewProc("GdipCreatePathGradientFromPath")
	procGdipSetPathGradientCenterColor    = modGdiPlus.NewProc("GdipSetPathGradientCenterColor")
	procGdipSetPathGradientSurroundColors = modGdiPlus.NewProc("GdipSetPathGradientSurroundColorsWithCount")
	procGdipFillPath                      = modGdiPlus.NewProc("GdipFillPath")
	procGdipFillEllipseI                  = modGdiPlus.NewProc("GdipFillEllipseI")
	procGdipFillRectangleI                = modGdiPlus.NewProc("GdipFillRectangleI")
	procGdipCreateBitmapFromScan0         = modGdiPlus.NewProc("GdipCreateBitmapFromScan0")
	procGdipDisposeImage                  = modGdiPlus.NewProc("GdipDisposeImage")
	procGdipGetImageGraphicsContext       = modGdiPlus.NewProc("GdipGetImageGraphicsContext")
	procGdipDrawImageRectRectI            = modGdiPlus.NewProc("GdipDrawImageRectRectI")
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
// floats live in memory, never in registers.
type gPointF struct {
	X, Y float32
}

const (
	smoothingModeAntiAlias = 4
	fillModeAlternate      = 0 // even-odd: overlapping subfigures punch holes
	fillModeWinding        = 1 // non-zero: overlapping subfigures union
	wrapModeClamp          = 4
	unitPixel              = 2
	pixelFormat32bppARGB   = 0x26200A
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

func addPathPolygon(p uintptr, pts []wPoint) bool {
	if p == 0 || len(pts) == 0 {
		return false
	}
	r, _, _ := procGdipAddPathPolygonI.Call(p, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	return gpOK(r)
}

func addPathEllipse(p uintptr, x, y, w, h int32) bool {
	if p == 0 {
		return false
	}
	r, _, _ := procGdipAddPathEllipseI.Call(p, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
	return gpOK(r)
}

func addPathLine(p uintptr, x1, y1, x2, y2 int32) bool {
	if p == 0 {
		return false
	}
	r, _, _ := procGdipAddPathLineI.Call(p, uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
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
	if r, _, _ := procGdipCreateLineBrush.Call(
		uintptr(unsafe.Pointer(&p1)),
		uintptr(unsafe.Pointer(&p2)),
		uintptr(colors[0]), uintptr(colors[len(colors)-1]),
		wrapModeClamp,
		uintptr(unsafe.Pointer(&b)),
	); !gpOK(r) {
		return 0
	}
	if r, _, _ := procGdipSetLinePresetBlend.Call(
		b,
		uintptr(unsafe.Pointer(&colors[0])),
		uintptr(unsafe.Pointer(&positions[0])),
		uintptr(len(colors)),
	); !gpOK(r) {
		procGdipDeleteBrush.Call(b)
		return 0
	}
	return b
}

// newPathGlow builds a path-gradient brush over an already-built path:
// centerColor at the path's centroid fading to the surround color at
// its boundary — the outward-glow primitive the v3 screen's soft
// shadows and auras are made of.
func newPathGlow(path uintptr, center, surround uint32) uintptr {
	if path == 0 {
		return 0
	}
	var b uintptr
	if r, _, _ := procGdipCreatePathGradientFromPath.Call(path, uintptr(unsafe.Pointer(&b))); !gpOK(r) {
		return 0
	}
	procGdipSetPathGradientCenterColor.Call(b, uintptr(center))
	n := int32(1)
	sc := surround
	procGdipSetPathGradientSurroundColors.Call(b, uintptr(unsafe.Pointer(&sc)), uintptr(unsafe.Pointer(&n)))
	return b
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

// ---- bitmap layers (per-pixel ARGB, drawn with their own alpha) ----

// fixedMem owns a GDI DIB section's pixel memory — OS-owned, never
// moved by any GC — the contract GdipCreateBitmapFromScan0's
// non-copying scan0 pointer demands. The section lives until free()
// deletes the bitmap handle (disposeTextures disposes the GDI+ image
// first).
type fixedMem struct {
	hbmp uintptr
	ptr  unsafe.Pointer
	sz   int
}

// wBitmapInfoHeader is BITMAPINFOHEADER (a top-down 32bpp BI_RGB DIB
// is all the ARGB layers need).
type wBitmapInfoHeader struct {
	Size          int32
	Width         int32
	Height        int32 // negative = top-down rows
	Planes        uint16
	BitCount      uint16
	Compression   int32 // BI_RGB
	SizeImage     int32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       int32
	ClrImportant  int32
}

// newFixedMem allocates a w*h 32bpp DIB section and returns its pixel
// memory as a byte slice (the caller keeps the fixedMem alive for as
// long as the GDI+ bitmap wraps it).
func newFixedMem(w, h int32) (fixedMem, []byte) {
	if w <= 0 || h <= 0 {
		return fixedMem{}, nil
	}
	bmi := wBitmapInfoHeader{
		Size:     int32(unsafe.Sizeof(wBitmapInfoHeader{})),
		Width:    w,
		Height:   -h, // top-down: row 0 is the top row
		Planes:   1,
		BitCount: 32,
	}
	var ptr unsafe.Pointer
	hbmp, _, _ := procCreateDIBSection.Call(
		0, uintptr(unsafe.Pointer(&bmi)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&ptr)),
		0, 0,
	)
	if hbmp == 0 || ptr == nil {
		return fixedMem{}, nil
	}
	m := fixedMem{hbmp: hbmp, ptr: ptr, sz: int(w*h) * 4}
	return m, unsafe.Slice((*byte)(ptr), m.sz)
}

func (m fixedMem) free() {
	if m.hbmp != 0 {
		procDeleteObject.Call(m.hbmp)
	}
}

// newScan0Bitmap wraps fixed DIB-section memory as a GDI+ ARGB bitmap
// (the memory stays owned by the caller for the bitmap's whole life).
func newScan0Bitmap(w, h int32, mem fixedMem) uintptr {
	if w <= 0 || h <= 0 || mem.ptr == nil {
		return 0
	}
	var bmp uintptr
	stride := int32(4) * w
	if r, _, _ := procGdipCreateBitmapFromScan0.Call(
		uintptr(w), uintptr(h), uintptr(stride),
		pixelFormat32bppARGB, uintptr(mem.ptr),
		uintptr(unsafe.Pointer(&bmp)),
	); !gpOK(r) {
		return 0
	}
	return bmp
}

func disposeImage(img uintptr) {
	if img != 0 {
		procGdipDisposeImage.Call(img)
	}
}

// imageGraphics opens a GDI+ graphics that renders INTO a bitmap (used
// once, while the pre-rendered texture layers are built).
func imageGraphics(bmp uintptr) uintptr {
	if bmp == 0 {
		return 0
	}
	var g uintptr
	if r, _, _ := procGdipGetImageGraphicsContext.Call(bmp, uintptr(unsafe.Pointer(&g))); !gpOK(r) {
		return 0
	}
	procGdipSetSmoothingMode.Call(g, smoothingModeAntiAlias)
	return g
}

// drawImageTile blits one bitmap region 1:1 into a destination rect on
// the graphics — the per-pixel alpha is honored by the default
// SourceOver compositing.
func drawImageTile(g, img uintptr, dx, dy, dw, dh, sx, sy, sw, sh int32) {
	if g == 0 || img == 0 {
		return
	}
	procGdipDrawImageRectRectI.Call(
		g, img,
		uintptr(dx), uintptr(dy), uintptr(dw), uintptr(dh),
		uintptr(sx), uintptr(sy), uintptr(sw), uintptr(sh),
		unitPixel, 0, 0, 0,
	)
}
