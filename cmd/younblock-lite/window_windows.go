//go:build windows

// window_windows.go — the lite desktop edition's window: one Win32
// top-level window on the v3 design screen (360x640 client, resizable
// down to the 320x560 floors like the main desktop), painted through
// the screen painter (screen_windows.go) into a double buffer.
//
// The D115 round: the plain GDI screen became the full v3 main screen —
// the state gradient backgrounds, the key-cap lever with its drag
// gestures (the reference's pointerdown/move/up port: the cap follows
// the finger, releasing past 45% of the travel commits, a plain tap
// fires, a sub-threshold release springs back), the menu sheet with
// the two honest settings toggles, and a 30 fps animation clock that
// carries every one of the design's infinite loops (the breathing
// auras, the LED blink, the swipe cue, the timer's seconds).
//
// Threading: this file owns THE UI thread (LockOSThread). The engine
// runs on its own goroutines; state reaches the window through the
// App's channel, drained by a 150 ms timer — and the engine's quit
// callback marshals back with PostMessage, the only thread-safe door.
package main

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/assadigit/Younblock/internal/engine"
	"golang.org/x/sys/windows"
)

// The one window class name (also the single-instance FindWindow key).
const liteClassName = "YounblockLiteDesktopWindow"

// Client size in logical pixels (96 dpi) — the v3 design screen, the
// same 360x640 the main desktop window opens at; everything scales
// with the window's real DPI and re-flows for any client size from the
// 320x560 floors up (the same floors the Wails window enforces).
const (
	liteClientW = 360
	liteClientH = 640
	liteMinW    = 320
	liteMinH    = 560
)

// Timer identities: 1 = the state poller (150 ms), 2 = the quit
// watchdog, 3 = the animation clock (33 ms — the design's every loop
// is a >= 450 ms soft fade; 30 fps renders them smoothly).
const (
	pollTimerID  = 1
	watchdogID   = 2
	animTimerID  = 3
	pollPeriod   = 150
	animPeriod   = 33
	watchdogWait = 8000
)

// Focus / hot identities (the sheet's rows ride the 100 range).
const (
	focusLever  = 0
	focusCard   = 1
	focusMenu   = 2
	focusSheet0 = 100
)

// liteWindow carries everything the UI thread owns.
type liteWindow struct {
	app  *App
	hwnd windows.Handle
	inst windows.Handle

	dpi int32 // the window's current DPI (96 at 100%)

	// fonts (GDI: the weights the design's 400-700 type scale needs)
	fBrand, fEyebrow, fHeadline, fSub, fTimer, fStats windows.Handle
	fHint, fCap, fCardTitle, fCardSub                 windows.Handle
	fSheetTitle, fRowTitle, fRowHint                  windows.Handle

	// layout + pre-rendered texture layers (rebuilt on resize/DPI)
	lay       layout
	dotBmp    uintptr
	dotMem    fixedMem
	grainBmp  uintptr
	grainMem  fixedMem
	grainSize int32

	// the screen state + its animation anchors
	scr                Screen
	prevBg             ScreenState
	bgFadeStart        int64
	stateEntered       int64
	colorFadeStart     int64
	prevLEDColor       uint32
	prevSubColor       uint32
	ghostAlpha         float64
	ghostFrom, ghostTo float64
	ghostStart         int64
	cardShownAt        int64

	// the lever's cap (pos: 0 = top, 1 = bottom; animated or dragged)
	capPos          float64
	capFrom, capTo  float64
	capAnimStart    int64
	dragging        bool
	dragStartY      int32
	dragBase        float64
	dragMoved       int32
	pressed         bool
	pendingRevertAt int64
	lastFire        int64

	// the menu sheet
	sheetOpen      bool
	sheetAnimStart int64
	tglReadAt      int64
	tglConnect     bool
	tglOpen        bool
	tglConnectFlip int64
	tglOpenFlip    int64

	// interaction
	focus int
	hot   int

	// the paint in progress (the memory DC — the cap label's
	// per-character GDI text needs it)
	paintDC uintptr

	// bookkeeping
	lastEngine   engine.State
	openedForRun bool
	forcedQuit   bool
	animRunning  bool
	sizeDirty    bool
}

// runUI is main's one call: the whole window life.
func runUI(app *App) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	diagLog("dpi awareness: %s", makeDpiAware())

	if gdiplusStartup() {
		diagLog("gdiplus: started")
	} else {
		// Not fatal: the painter's fallback keeps the screen
		// honest (flat background, the words) without GDI+.
		diagLog("gdiplus: unavailable — the GDI-only fallback paints")
	}
	defer gdiplusShutdown()

	// Single instance: a second launch just raises the first window.
	if bringUpExisting() {
		return
	}

	w := &liteWindow{app: app, hot: -1, focus: focusLever}
	if !w.create() {
		diagLog("window creation failed — exiting")
		return
	}
	diagLog("window up: dpi=%d state=%s", w.dpi, w.scr.State)
	app.OnEngineQuit(func() {
		procPostMessageW.Call(uintptr(w.hwnd), wmAppQuit, 0, 0)
	})

	// Connect on launch — the same setting, the same behavior as the
	// main desktop and the web edition.
	if app.ConnectOnLaunch() {
		go app.Connect()
	}

	procShowWindow.Call(uintptr(w.hwnd), swShownormal)
	procUpdateWindow.Call(uintptr(w.hwnd))

	// The state poller (drains the App's channel; also the watchdog
	// carrier while a quit settles) and the animation clock.
	procSetTimer.Call(uintptr(w.hwnd), pollTimerID, pollPeriod, 0)
	w.startAnimTimer()

	// The message loop — THE UI thread.
	var m wMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	procKillTimer.Call(uintptr(w.hwnd), pollTimerID)
	w.stopAnimTimer()
	w.disposeTextures()
	w.deleteFonts()
}

// makeDpiAware asks for per-monitor v2 (crisp text at any scaling),
// falling back through the older doors — each GUARDED (D117): the
// lazy Call panics when an export is missing, and every one of these
// three doors debuted on a different Windows (1607 / 8.1 / Vista), so
// the chain must step over absent procs instead of dying on them.
// Returns the door taken, for the run log.
func makeDpiAware() string {
	if procOK(procSetProcessDpiAwarenessContx) {
		r, _, _ := procSetProcessDpiAwarenessContx.Call(dpiAwarePerMonitorV2)
		if r == 0 {
			return "per-monitor-v2"
		}
		diagLog("dpi: SetProcessDpiAwarenessContext refused (err %d) — stepping down", r)
	} else {
		diagLog("dpi: SetProcessDpiAwarenessContext absent (pre-1607 Windows) — stepping down")
	}
	if procOK(procSetProcessDpiAwareness) {
		r, _, _ := procSetProcessDpiAwareness.Call(2)
		if r == 0 {
			return "per-monitor-v1"
		}
		diagLog("dpi: SetProcessDpiAwareness refused (err %d) — stepping down", r)
	} else {
		diagLog("dpi: shcore SetProcessDpiAwareness absent — stepping down")
	}
	if procOK(procSetProcessDPIAware) {
		procSetProcessDPIAware.Call()
		return "system-dpi-aware"
	}
	return "none (unaware)"
}

// bringUpExisting returns true when another instance already owns the
// window: this launch raises it and gets out of the way. The lazy
// proc's Call reports the real GetLastError — 183 is
// ERROR_ALREADY_EXISTS for the mutex creation.
func bringUpExisting() bool {
	_, _, err := procCreateMutexW.Call(
		0, 0,
		uintptr(unsafe.Pointer(mustUtf16("Local\\YounblockLiteDesktop"))),
	)
	if errno, ok := err.(syscall.Errno); !ok || uintptr(errno) != 183 {
		return false
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(mustUtf16(liteClassName))), 0)
	if h == 0 {
		return false
	}
	procShowWindow.Call(h, swRestore)
	procSetForegroundWindow.Call(h)
	return true
}

func mustUtf16(s string) *uint16 {
	p, err := utf16(s)
	if err != nil {
		panic("lite: bad utf16 string: " + err.Error())
	}
	return p
}

// create registers the class and makes the window (positioned by
// CW_USEDEFAULT, sized for the current DPI).
func (w *liteWindow) create() bool {
	rawInst, _, _ := procGetModuleHandleW.Call(0)
	inst := windows.Handle(rawInst)
	w.inst = inst

	className := mustUtf16(liteClassName)
	wc := wWndClassEx{
		Style:     csHRedraw | csVRedraw,
		WndProc:   windows.NewCallback(w.wndProc),
		Instance:  inst,
		MenuName:  nil,
		ClassName: className,
		// Pre-paint the idle gradient's top (#3b3a4a) — no flash
		// of the wrong color before the first frame lands.
		Background: windows.Handle(makeBrush(rgb(0x3b, 0x3a, 0x4a))),
	}
	if h, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow)); h != 0 {
		wc.Cursor = windows.Handle(h)
	}
	// The exe's own icon (the owner's mark, embedded by the .syso);
	// best-effort — a missing resource only means the default icon.
	if h, _, _ := procLoadIconW.Call(uintptr(inst), 1); h != 0 {
		wc.Icon = windows.Handle(h)
		wc.IconSm = windows.Handle(h)
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		diagLog("RegisterClassExW failed: %v", err)
		return false
	}

	// Outer size for the design client size at this DPI.
	dpi := systemDpi()
	w.dpi = dpi
	frame := wRect{0, 0, px2(dpi, liteClientW), px2(dpi, liteClientH)}
	style := uintptr(wsCaption | wsSysMenu | wsMinimizeBx | wsThickenFrame | wsMaximizeBox)
	adjustFrame(&frame, style, dpi)
	title := mustUtf16("Younblock Lite")
	h, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style,
		0x80000000, 0x80000000, // CW_USEDEFAULT position
		uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
		0, 0, uintptr(inst), 0,
	)
	if h == 0 {
		diagLog("CreateWindowExW failed: %v", err)
		return false
	}
	w.hwnd = windows.Handle(h)
	w.dpi = w.windowDpi()
	w.darkenTitleBar()
	w.makeFonts()
	w.relayout()
	// The very first paint is a real state, already at rest.
	now := time.Now().UnixMilli()
	w.lastEngine = w.app.Poll()
	w.scr = screen(w.lastEngine)
	w.prevBg = w.scr.State
	w.bgFadeStart = now - 700 // fully settled
	w.stateEntered = now
	w.colorFadeStart = now - 400
	w.prevLEDColor = stateColors[w.scr.State].LED
	w.prevSubColor = stateColors[w.scr.State].Sub
	w.ghostAlpha = ghostRest(w.scr.State)
	w.ghostFrom, w.ghostTo = w.ghostAlpha, w.ghostAlpha
	w.ghostStart = now
	w.capPos = capRestPos(w.scr.State)
	w.capFrom, w.capTo = w.capPos, w.capPos
	w.capAnimStart = now - 550 // settled: no travel on the first frame
	w.cardShownAt = 0
	if w.scr.ShowCard {
		w.cardShownAt = now - 450
	}
	return true
}

// darkenTitleBar asks DWM for the dark caption (and, where Windows 11
// allows, the screen's own colors) — best-effort, matching the main
// desktop's dark-theme window chrome. DWM ships since Vista, but the
// guard keeps the promise airtight (D117): a missing door is skipped,
// never fatal.
func (w *liteWindow) darkenTitleBar() {
	if !procOK(procDwmSetWindowAttribute) {
		return
	}
	hwnd := uintptr(w.hwnd)
	one := uintptr(1)
	size := uintptr(4)
	if r, _, _ := procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&one)), size); r != 0 {
		procDwmSetWindowAttribute.Call(hwnd, 19, uintptr(unsafe.Pointer(&one)), size)
	}
	// The caption in the idle background's tone, the title text in
	// the screen's text color (Windows 11 only; harmless elsewhere).
	cap := uintptr(0x004a3a3b) // COLORREF for #3b3a4a
	procDwmSetWindowAttribute.Call(hwnd, 35, uintptr(unsafe.Pointer(&cap)), size)
	txt := uintptr(0x00EEF0F4) // COLORREF for #F4F0EE
	procDwmSetWindowAttribute.Call(hwnd, 36, uintptr(unsafe.Pointer(&txt)), size)
}

// systemDpi reads the primary screen's DPI (before any window exists).
func systemDpi() int32 {
	hdc, _, _ := procGetDC.Call(0)
	if hdc != 0 {
		defer procReleaseDC.Call(0, hdc)
		if r, _, _ := procGetDeviceCaps.Call(hdc, logPixelsX); r != 0 {
			return int32(r)
		}
	}
	return 96
}

// windowDpi reads the window's own DPI (per-monitor aware); without
// the 1607 door (or when it fails) the screen's system DPI is the
// honest answer — it is exactly what an unaware window renders at.
func (w *liteWindow) windowDpi() int32 {
	if procOK(procGetDpiForWindow) {
		if r, _, _ := procGetDpiForWindow.Call(uintptr(w.hwnd)); r != 0 {
			return int32(r)
		}
		diagLog("dpi: GetDpiForWindow returned 0 — using the system DPI")
	} else {
		diagLog("dpi: GetDpiForWindow absent (pre-1607 Windows) — using the system DPI")
	}
	return systemDpi()
}

// px converts logical 96-dpi pixels to this window's device pixels.
func (w *liteWindow) px(v int32) int32 { return v * w.dpi / 96 }

// pxf is px for fractional logical values.
func (w *liteWindow) pxf(v float64) int32 { return int32(float64(w.dpi)*v/96 + 0.5) }

// px2 converts logical pixels for an explicit dpi (used before the
// window exists).
func px2(dpi int32, v int32) int32 { return v * dpi / 96 }

// makeFonts builds the font set for the current DPI — the design's type
// scale in its own weights (GDI carries 400-700 natively).
func (w *liteWindow) makeFonts() {
	w.deleteFonts()
	f := func(size int32, weight int32) windows.Handle {
		return makeFontPx(w.dpi, w.px(size), weight)
	}
	w.fBrand = f(17, fwSemibold)
	w.fEyebrow = f(15, fwMedium)
	w.fHeadline = f(32, fwBold)
	w.fSub = f(15, fwMedium)
	w.fTimer = f(16, fwRegular)
	w.fStats = f(15, fwMedium)
	w.fHint = f(14, fwMedium)
	w.fCap = f(18, fwBold)
	w.fCardTitle = f(17, fwSemibold)
	w.fCardSub = f(14, fwRegular)
	w.fSheetTitle = f(15, fwSemibold)
	w.fRowTitle = f(13, fwSemibold)
	w.fRowHint = f(12, fwMedium)
}

func (w *liteWindow) deleteFonts() {
	fs := []windows.Handle{
		w.fBrand, w.fEyebrow, w.fHeadline, w.fSub, w.fTimer, w.fStats,
		w.fHint, w.fCap, w.fCardTitle, w.fCardSub, w.fSheetTitle, w.fRowTitle, w.fRowHint,
	}
	for _, f := range fs {
		if f != 0 {
			procDeleteObject.Call(uintptr(f))
		}
	}
}

// makeFontPx builds one GDI font at an explicit pixel height (negative
// lfHeight = the em height the design's px sizes mean).
func makeFontPx(dpi int32, heightPx int32, weight int32) windows.Handle {
	if heightPx < 5 {
		heightPx = 5
	}
	lf := wLogFontW{
		Height:  -heightPx,
		Weight:  weight,
		CharSet: defaultCharset,
		Quality: clearTypeQuality,
	}
	copy(lf.FaceName[:], utf16FaceName())
	h, _, _ := procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&lf)))
	return windows.Handle(h)
}

// utf16FaceName is "Segoe UI" as UTF-16 (LOGFONTW wants the wide form)
// — the system-ui face the web screen resolves to on Windows, so both
// desktops set the same words.
func utf16FaceName() []uint16 {
	w, _ := windows.UTF16FromString("Segoe UI")
	return w
}

func makeBrush(color uintptr) uintptr {
	br, _, _ := procCreateSolidBrush.Call(color)
	return br
}

// relayout recomputes every rectangle for the current client size and
// rebuilds the pre-rendered texture layers.
func (w *liteWindow) relayout() {
	var rc wRect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
	width, height := rc.Right, rc.Bottom
	w.lay.tall = height >= w.px(700)

	m := w.px(lyMargin)
	// The header: 12 px below the top, 48 px tall.
	hdrTop := w.px(lyHeaderTop)
	hdrH := w.px(lyHeaderH)
	mkTop := hdrTop + (hdrH-w.px(lyBrandH))/2
	w.lay.brandMark = wRect{m, mkTop, m + w.px(lyBrandW), mkTop + w.px(lyBrandH)}
	w.lay.menuBtn = wRect{width - m - w.px(48), hdrTop, width - m, hdrTop + w.px(48)}

	// The lever's track, centered at 60% of the height (67% on tall
	// screens — the reference's two lanes).
	centerFrac := 0.60
	if w.lay.tall {
		centerFrac = 0.67
	}
	bcy := int32(float64(height)*centerFrac + 0.5)
	tw, th := w.px(lyTrackW), w.px(lyTrackH)
	w.lay.track = wRect{(width - tw) / 2, bcy - th/2, (width + tw) / 2, bcy + th/2}
	pad := w.px(lyTouch)
	w.lay.hit = wRect{w.lay.track.Left - pad, w.lay.track.Top - pad, w.lay.track.Right + pad, w.lay.track.Bottom + pad}

	// The card slot + the hint line above it.
	w.lay.card = wRect{m, height - m - w.px(lyCardH), width - m, height - m}
	w.lay.hint = wRect{m, w.lay.card.Top - w.px(10) - w.px(20), width - m, w.lay.card.Top - w.px(10)}

	// The sheet.
	w.lay.sheetW = width
	if w.lay.sheetW > w.px(lySheetW) {
		w.lay.sheetW = w.px(lySheetW)
	}
	rows := w.sheetRows(0, w.lay.sheetW, 0)
	w.lay.sheetH = rows[len(rows)-1].rc.Bottom + w.px(14) + w.px(36) + w.px(16)

	w.buildTextures(width, height)
	w.invalidate()
}

// disposeTextures frees the pre-rendered layers.
func (w *liteWindow) disposeTextures() {
	if w.dotBmp != 0 {
		disposeImage(w.dotBmp)
		w.dotBmp = 0
	}
	w.dotMem.free()
	w.dotMem = fixedMem{}
	if w.grainBmp != 0 {
		disposeImage(w.grainBmp)
		w.grainBmp = 0
	}
	w.grainMem.free()
	w.grainMem = fixedMem{}
}

// buildTextures pre-renders the state-independent layers once per
// resize: the halftone dot grid (7 px pitch, 1 px dots, alpha ramping
// in below 35% of the height) and the 5% grain tile (160 px, seeded).
func (w *liteWindow) buildTextures(width, height int32) {
	w.disposeTextures()
	if width <= 0 || height <= 0 {
		return
	}

	// The halftone dots.
	if mem, _ := newFixedMem(width, height); mem.ptr != nil {
		w.dotMem = mem
		if bmp := newScan0Bitmap(width, height, mem); bmp != 0 {
			w.dotBmp = bmp
			if g := imageGraphics(bmp); g != 0 {
				step := w.px(7)
				if step < 3 {
					step = 3
				}
				r := w.px(1)
				if r < 1 {
					r = 1
				}
				d := 2 * r
				// The mask ramps 0 -> 1 from 35% height to
				// the bottom; alpha = 0.075 * mask.
				for y := int32(0); y < height; y += step {
					frac := float64(y) / float64(height)
					mask := clamp01((frac - 0.35) / 0.65)
					a := uint32(0.075 * 255 * mask)
					if a == 0 {
						continue
					}
					b := newSolidBrush(argb(a, 0xFF, 0xFF, 0xFF))
					if b == 0 {
						continue
					}
					for x := int32(0); x < width; x += step {
						procGdipFillEllipseI.Call(g, b, uintptr(x), uintptr(y), uintptr(d), uintptr(d))
					}
					deleteBrush(b)
				}
				deleteGraphics(g)
			}
		}
	}

	// The grain tile (160 px logical, 5% opacity noise).
	ts := w.px(160)
	if ts < 8 {
		ts = 8
	}
	w.grainSize = ts
	if mem, buf := newFixedMem(ts, ts); mem.ptr != nil {
		w.grainMem = mem
		// A deterministic xorshift: the grain is texture, not
		// randomness anyone can see; a fixed seed keeps every
		// frame identical (no shimmer).
		state := uint32(0x9E3779B9)
		next := func() uint32 {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			return state
		}
		for y := int32(0); y < ts; y++ {
			row := buf[y*ts*4 : (y+1)*ts*4]
			for x := int32(0); x < ts; x++ {
				v := byte(next() & 0xFF)
				row[x*4] = v    // B
				row[x*4+1] = v  // G
				row[x*4+2] = v  // R
				row[x*4+3] = 13 // A — 5%
			}
		}
		w.grainBmp = newScan0Bitmap(ts, ts, mem)
	}
}

// ---- the message pump ----

func (w *liteWindow) wndProc(hwnd windows.Handle, msg uint32, wp uintptr, lp unsafe.Pointer) uintptr {
	// The never-die guard (D117): a panic anywhere in the message
	// handling — the paint path included — is logged and survived,
	// because a dead window tells the owner nothing while a logged
	// one tells us everything.
	defer func() {
		if rec := recover(); rec != nil {
			diagLog("PANIC in wndProc (msg 0x%04x): %v", msg, rec)
			w.invalidate()
		}
	}()
	switch msg {
	case wmCreate:
		w.hwnd = hwnd
		return 0
	case wmEraseBkgnd:
		return 1 // the whole client is BitBlt'ed in wmPaint
	case wmPaint:
		w.paint()
		return 0
	case wmTimer:
		w.onTimer(wp)
		return 0
	case wmSize:
		w.onSize(wp)
		return 0
	case wmGetMinMaxInfo:
		w.onMinMaxInfo(lp)
		return 0
	case wmDpiChanged:
		w.onDpiChanged(wp)
		return 0
	case wmMouseMove:
		w.onMouseMove(uintptr(lp))
		return 0
	case wmMouseLeave:
		w.setHot(-1)
		return 0
	case wmLButtonDown:
		w.onButtonDown(uintptr(lp))
		return 0
	case wmLButtonUp:
		w.onButtonUp(uintptr(lp))
		return 0
	case wmCaptureChanged:
		w.onCaptureChanged()
		return 0
	case wmKeyDown:
		if w.onKey(wp) {
			return 0
		}
	case wmSetCursor:
		if uint32(uintptr(lp))&0xFFFF == 1 { // HTCLIENT
			cur, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))
			if w.hot >= 0 {
				if h, _, _ := procLoadCursorW.Call(0, uintptr(idcHand)); h != 0 {
					cur = h
				}
			}
			procSetCursor.Call(cur)
			return 1
		}
	case wmClose:
		w.onClose()
		return 0
	case wmAppQuit:
		procPostQuitMessage.Call(0)
		return 0
	case wmQueryEndSession:
		return 1 // yes, we can end — wmEndSession does the work
	case wmEndSession:
		if wp != 0 {
			// The session is really ending: stop synchronously so
			// the proxy is restored before the process goes away.
			w.app.StopNow()
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wp, uintptr(lp))
	return r
}

// onSize: a resized window re-flows; a minimized one parks the
// animation clock (nothing visible animates while iconic). The rebuild
// itself is coalesced onto the animation clock — a drag fires WM_SIZE
// for every pixel, and the texture layers want one rebuild per tick,
// not one per pixel.
func (w *liteWindow) onSize(wp uintptr) {
	if uint32(wp)&0xFFFF == sizeMinimized {
		w.stopAnimTimer()
		return
	}
	if !w.animRunning {
		w.startAnimTimer()
	}
	w.sizeDirty = true
}

// adjustFrame grows a client rectangle to the outer window frame at
// this DPI — the modern door when this Windows has it (1607+), the
// ever-present AdjustWindowRect otherwise. The pre-D117 shape of this
// helper — an `ok == 0` fallback after a lazy Call that PANICS when
// the export is missing — is the one line that kept the lite edition
// from starting on any Windows older than 1607 (procOK tells the
// whole story).
func adjustFrame(frame *wRect, style uintptr, dpi int32) {
	if procOK(procAdjustWindowRectForDpi) {
		if ok, _, _ := procAdjustWindowRectForDpi.Call(
			uintptr(unsafe.Pointer(frame)), style, 0, uintptr(dpi)); ok != 0 {
			return
		}
	}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(frame)), style, 0)
}

// onMinMaxInfo clamps the window to the design's floors (320x560
// client — the same floors the main desktop enforces).
func (w *liteWindow) onMinMaxInfo(lp unsafe.Pointer) {
	frame := wRect{0, 0, w.px(liteMinW), w.px(liteMinH)}
	style := uintptr(wsCaption | wsSysMenu | wsMinimizeBx | wsThickenFrame | wsMaximizeBox)
	adjustFrame(&frame, style, w.dpi)
	// MINMAXINFO is 5 POINTs; ptMinTrackSize is the fourth. The array
	// view keeps every pointer in this file flowing in the Go-owned,
	// out-parameter direction only.
	mmi := (*[5]wPoint)(lp)
	mmi[3].X = frame.Right - frame.Left
	mmi[3].Y = frame.Bottom - frame.Top
}

// onDpiChanged: rebuild fonts, textures and layout; resize the outer
// frame for the new DPI while keeping the client size the person chose.
// (The message's lParam points at the system's suggested rectangle —
// system-owned memory this code deliberately never dereferences;
// computing the frame from the current client size and position is
// equivalent and keeps every pointer in this file flowing in the
// Go-owned, out-parameter direction only.)
func (w *liteWindow) onDpiChanged(wp uintptr) {
	newDpi := int32(uint32(wp) >> 16)
	var cur wRect
	procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&cur)))
	var client wRect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&client)))
	cw := int32(float64(client.Right) * float64(newDpi) / float64(w.dpi))
	ch := int32(float64(client.Bottom) * float64(newDpi) / float64(w.dpi))
	if cw < px2(newDpi, liteMinW) {
		cw = px2(newDpi, liteMinW)
	}
	if ch < px2(newDpi, liteMinH) {
		ch = px2(newDpi, liteMinH)
	}
	frame := wRect{0, 0, cw, ch}
	style := uintptr(wsCaption | wsSysMenu | wsMinimizeBx | wsThickenFrame | wsMaximizeBox)
	adjustFrame(&frame, style, newDpi)
	w.dpi = newDpi
	procSetWindowPos.Call(
		uintptr(w.hwnd), 0,
		uintptr(cur.Left), uintptr(cur.Top),
		uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
		swpNoZOrder|swpNoActivate,
	)
	w.makeFonts()
	w.relayout()
}

// onTimer: timer 1 polls the engine and folds changes into the screen
// state's animation anchors; timer 2 is the quit watchdog; timer 3 is
// the animation clock (just invalidate — the painter reads the wall
// clock).
func (w *liteWindow) onTimer(wp uintptr) {
	now := time.Now().UnixMilli()
	switch wp {
	case watchdogID:
		procKillTimer.Call(uintptr(w.hwnd), watchdogID)
		if !w.forcedQuit {
			w.forcedQuit = true
			procPostQuitMessage.Call(0)
		}
		return
	case animTimerID:
		if w.sizeDirty {
			w.sizeDirty = false
			w.relayout()
		}
		w.invalidate()
		return
	}

	st := w.app.Poll()
	w.lastEngine = st
	newScr := screen(st)
	if screenChanged(w.scr, newScr) {
		w.applyScreen(newScr, now)
	}
	// The openBrowserOnConnect setting, honored once per connected
	// run.
	if w.app.OpenYouTubeOnConnect() && st.Phase == engine.PhaseOn && !w.openedForRun {
		w.openedForRun = true
		go openYouTube(w.hwnd)
	}
	if st.Phase == engine.PhaseOff || st.Phase == engine.PhaseError {
		w.openedForRun = false
	}
	// The lever's pending head start: a fired action whose state
	// never landed reverts to the honest resting end (500 ms).
	if w.pendingRevertAt != 0 && now > w.pendingRevertAt {
		w.pendingRevertAt = 0
		if !w.dragging {
			w.capAnimateTo(capRestPos(w.scr.State), now)
		}
	}
}

// applyScreen lands a new screen state: every transition anchor the
// painter animates from is set here, in one place.
func (w *liteWindow) applyScreen(newScr Screen, now int64) {
	if newScr.State != w.scr.State {
		w.prevBg = w.scr.State
		w.bgFadeStart = now
		w.prevLEDColor = w.ledColor(w.scr, now)
		w.prevSubColor = w.subColor(w.scr, now)
		w.colorFadeStart = now
		w.stateEntered = now
		w.pendingRevertAt = 0

		// The cap travels to its new resting end (unless the
		// finger is mid-gesture — the state change ends those).
		if w.dragging {
			w.dragging = false
			w.pressed = false
			procReleaseCapture.Call()
		}
		w.capAnimateTo(capRestPos(newScr.State), now)

		// The ghost fades out while connected/connecting.
		gTo := ghostRest(newScr.State)
		if gTo != w.ghostAlpha {
			w.ghostFrom, w.ghostTo, w.ghostStart = w.ghostAlpha, gTo, now
		}
		// The card rises in when it first appears.
		if newScr.ShowCard && !w.scr.ShowCard {
			w.cardShownAt = now
		}
	} else if newScr.ShowCard && !w.scr.ShowCard {
		w.cardShownAt = now
	}
	w.scr = newScr
	w.invalidate()
}

// ghostRest is the ghost tile's resting opacity for a state: the ghost
// fades away ONLY while connected (the CSS keeps it through idle,
// connecting and failed).
func ghostRest(s ScreenState) float64 {
	if s == ScreenConnected {
		return 0
	}
	return 1
}

// ---- the lever's gesture + animation ----

// capAnimateTo starts the 550 ms springy travel to a resting end.
func (w *liteWindow) capAnimateTo(to float64, now int64) {
	w.capFrom = w.capPosNow(now)
	w.capTo = to
	w.capAnimStart = now
}

// capPosNow is the cap's current position: the drag's live follow, or
// the animated travel, or the resting end.
func (w *liteWindow) capPosNow(now int64) float64 {
	if w.dragging {
		return clamp01(w.capPos)
	}
	t := animPhase(now, w.capAnimStart, 550)
	if t <= 0 {
		return clamp01(w.capFrom)
	}
	if t >= 1 {
		w.capPos = w.capTo
		return clamp01(w.capTo)
	}
	// The springy curve, evaluated once per frame.
	e := easeCap(t)
	return clamp01(lerp(w.capFrom, w.capTo, e))
}

// capDragging reports whether a gesture holds the cap.
func (w *liteWindow) capDragging() bool { return w.dragging }

// fireLever runs the lever's committed action with the visual head
// start: the cap finishes the move in the gesture's direction while
// the engine's confirming event is in flight (500 ms to land, else the
// honest resting end returns).
func (w *liteWindow) fireLever(a LeverAction, now int64) {
	if a == LeverNone {
		return
	}
	if now-w.lastFire < 600 { // debounce every trigger
		return
	}
	w.lastFire = now
	switch a {
	case LeverConnect, LeverRetry:
		w.app.Connect()
		w.capAnimateTo(0, now)
	case LeverDisconnect:
		w.app.Disconnect()
		w.capAnimateTo(1, now)
	}
	w.pendingRevertAt = now + 500
	w.invalidate()
}

// ---- input ----

func ptFromLparam(lp uintptr) (int32, int32) {
	return int32(int16(uint32(lp) & 0xFFFF)), int32(int16(uint32(lp) >> 16))
}

func inRect(rc wRect, x, y int32) bool {
	return x >= rc.Left && x < rc.Right && y >= rc.Top && y < rc.Bottom
}

// sheetGeometry returns the sheet's on-screen rectangles at its
// current slide position (fully open for hit-testing).
func (w *liteWindow) sheetGeometry(width, height int32) (x, y int32, rows []sheetRow, xrect wRect) {
	sw := w.lay.sheetW
	sh := w.lay.sheetH
	x = (width - sw) / 2
	y = height - sh
	rows = w.sheetRows(x, sw, y+w.px(24)+w.px(44)+w.px(6))
	xw := w.px(44)
	xrect = wRect{x + sw - w.px(22) - xw, y + w.px(24), x + sw - w.px(22), y + w.px(24) + xw}
	return
}

// hitSheet identifies what a point touches while the sheet is open:
// 100-103 the rows, 104 the X, -2 the backdrop, -1 nothing.
func (w *liteWindow) hitSheet(x, y int32) int {
	var rc wRect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
	_, _, rows, xrect := w.sheetGeometry(rc.Right, rc.Bottom)
	if inRect(xrect, x, y) {
		return 104
	}
	for _, row := range rows {
		if inRect(row.rc, x, y) {
			return 100 + row.idx
		}
	}
	// The sheet panel itself (not the backdrop) swallows clicks.
	if x >= (rc.Right-w.lay.sheetW)/2 && x < (rc.Right+w.lay.sheetW)/2 && y >= rc.Bottom-w.lay.sheetH {
		return -1
	}
	return -2 // the backdrop closes
}

// hitMain identifies what a point touches on the main screen: 0 the
// lever's hit area, 1 the card, 2 the menu button, -1 nothing.
func (w *liteWindow) hitMain(x, y int32) int {
	if inRect(w.lay.hit, x, y) {
		return 0
	}
	if w.scr.ShowCard && inRect(w.lay.card, x, y) {
		return 1
	}
	if inRect(w.lay.menuBtn, x, y) {
		return 2
	}
	return -1
}

func (w *liteWindow) onMouseMove(lp uintptr) {
	x, y := ptFromLparam(lp)
	// Ask for a leave notification once per entry.
	tme := wTrackMouseEvent{
		Size:       uint32(unsafe.Sizeof(wTrackMouseEvent{})),
		Flags:      tmeLeave,
		HwndTarget: w.hwnd,
	}
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))

	// A live gesture: the cap follows the finger, clamped to the
	// track, and the cue chevrons hide.
	if w.dragging {
		dy := y - w.dragStartY
		a := dy
		if a < 0 {
			a = -a
		}
		if a > w.dragMoved {
			w.dragMoved = a
		}
		travel := float64(w.px(lyTravel))
		if travel <= 0 {
			travel = 1
		}
		w.capPos = clamp01(w.dragBase + float64(dy)/travel)
		w.invalidate()
		return
	}

	h := -1
	if w.sheetOpen {
		h = w.hitSheet(x, y)
		if h == -2 || h == -1 {
			h = -1
		}
	} else {
		h = w.hitMain(x, y)
	}
	w.setHot(h)
}

func (w *liteWindow) setHot(h int) {
	if w.hot == h {
		return
	}
	w.hot = h
	w.invalidate()
}

func (w *liteWindow) onButtonDown(lp uintptr) {
	x, y := ptFromLparam(lp)
	now := time.Now().UnixMilli()
	if w.sheetOpen {
		if h := w.hitSheet(x, y); h >= 0 {
			w.focus = h
			w.pressed = true
			w.invalidate()
		}
		return
	}
	h := w.hitMain(x, y)
	switch h {
	case 0:
		// The lever: gestures are inert while connecting (the
		// reference ignores them there).
		if w.scr.Action != LeverNone {
			w.focus = focusLever
			w.pressed = true
			w.dragging = true
			w.dragStartY = y
			w.dragBase = w.capPosNow(now)
			w.dragMoved = 0
			procSetCapture.Call(uintptr(w.hwnd))
			w.invalidate()
		}
	case 1, 2:
		w.focus = h
		w.pressed = true
		w.invalidate()
	}
}

func (w *liteWindow) onButtonUp(lp uintptr) {
	x, y := ptFromLparam(lp)
	now := time.Now().UnixMilli()
	wasPressed := w.pressed
	w.pressed = false

	if w.dragging {
		// The gesture's release: past 45% of the travel in the
		// counted direction commits; a plain tap (< 10 px) fires;
		// anything else springs back.
		w.dragging = false
		procReleaseCapture.Call()
		dy := y - w.dragStartY
		if w.dragMoved < w.px(10) {
			w.fireLever(w.scr.Action, now)
		} else {
			travel := float64(w.px(lyTravel))
			fromTop := w.scr.State == ScreenConnected
			commit := false
			if fromTop {
				commit = float64(dy) > travel*0.45
			} else {
				commit = float64(dy) < -travel*0.45
			}
			if commit {
				w.fireLever(w.scr.Action, now)
			} else {
				w.capAnimateTo(capRestPos(w.scr.State), now)
			}
		}
		w.invalidate()
		return
	}

	if !wasPressed {
		return
	}
	if w.sheetOpen {
		if h := w.hitSheet(x, y); h >= 0 && w.focus == h {
			w.activateSheet(h, now)
		}
		w.invalidate()
		return
	}
	h := w.hitMain(x, y)
	if h < 0 || w.focus != h {
		w.invalidate()
		return
	}
	switch h {
	case 1:
		go openYouTube(w.hwnd)
	case 2:
		w.openSheet(now)
	}
	w.invalidate()
}

// onCaptureChanged: a gesture lost its capture (another window took
// it) — release without committing, spring back.
func (w *liteWindow) onCaptureChanged() {
	if w.dragging {
		w.dragging = false
		w.pressed = false
		w.capAnimateTo(capRestPos(w.scr.State), time.Now().UnixMilli())
		w.invalidate()
	}
	w.pressed = false
}

// activateSheet runs one sheet row's action.
func (w *liteWindow) activateSheet(h int, now int64) {
	switch h {
	case 100: // Open YouTube
		go openYouTube(w.hwnd)
	case 101: // Connect on launch
		w.flipToggle(101, now)
	case 102: // Open YouTube when connected
		w.flipToggle(102, now)
	case 103: // Exit Younblock
		w.closeSheet(now)
		w.onClose()
	case 104: // the X
		w.closeSheet(now)
	}
}

// flipToggle flips one settings row and persists it through the same
// read-modify-write every edition's sheet performs.
func (w *liteWindow) flipToggle(id int, now int64) {
	key := "connectOnLaunch"
	want := !w.tglConnect
	if id == 102 {
		key = "openBrowserOnConnect"
		want = !w.tglOpen
	}
	if !w.app.SetSetting(key, want) {
		return // a refused save changes nothing on screen
	}
	if id == 101 {
		w.tglConnect = want
		w.tglConnectFlip = now
	} else {
		w.tglOpen = want
		w.tglOpenFlip = now
	}
	w.invalidate()
}

// openSheet slides the sheet in (and reads the toggles fresh — a quick
// settings read, done synchronously so the rows never flash stale).
func (w *liteWindow) openSheet(now int64) {
	w.sheetOpen = true
	w.sheetAnimStart = now
	w.tglConnect = w.app.ConnectOnLaunch()
	w.tglOpen = w.app.OpenYouTubeOnConnect()
	w.focus = focusSheet0
	w.invalidate()
}

// closeSheet slides the sheet out; focus returns to the menu button
// (the Svelte sheet returns focus to its opener).
func (w *liteWindow) closeSheet(now int64) {
	w.sheetOpen = false
	w.sheetAnimStart = now
	w.focus = focusMenu
	w.hot = -1
	w.invalidate()
}

// onKey: Tab walks the actionable things; Space/Enter fires the
// focused one; Escape closes the sheet.
func (w *liteWindow) onKey(wp uintptr) bool {
	now := time.Now().UnixMilli()
	switch wp {
	case vkEscape:
		if w.sheetOpen {
			w.closeSheet(now)
			return true
		}
	case vkTab:
		if w.sheetOpen {
			// 100..104, wrapping.
			if w.focus < focusSheet0 || w.focus > 104 {
				w.focus = focusSheet0
			} else if w.focus >= 104 {
				w.focus = focusSheet0
			} else {
				w.focus++
			}
		} else {
			// The lever, the card (while it exists), the menu.
			if w.focus == focusLever {
				if w.scr.ShowCard {
					w.focus = focusCard
				} else {
					w.focus = focusMenu
				}
			} else {
				w.focus = focusLever
			}
		}
		w.invalidate()
		return true
	case vkSpace, vkReturn:
		if w.sheetOpen {
			if w.focus >= focusSheet0 && w.focus <= 104 {
				w.activateSheet(w.focus, now)
			}
			return true
		}
		switch w.focus {
		case focusLever:
			w.fireLever(w.scr.Action, now)
			return true
		case focusCard:
			if w.scr.ShowCard {
				go openYouTube(w.hwnd)
				return true
			}
		case focusMenu:
			w.openSheet(now)
			return true
		}
	}
	return false
}

// onClose: closing while the tunnel runs asks first — the one
// destructive confirmation the lite edition needs (its close is a
// REAL shutdown otherwise, the D93 default).
func (w *liteWindow) onClose() {
	if w.app.IsQuitting() {
		return // already exiting; the quit path closes the window
	}
	st := w.app.State()
	switch st.Phase {
	case engine.PhaseOn, engine.PhaseDegraded, engine.PhaseConnecting:
		text := mustUtf16("The tunnel is running. Stop it and exit Younblock Lite?")
		caption := mustUtf16("Younblock Lite")
		r, _, _ := procMessageBoxW.Call(
			uintptr(w.hwnd),
			uintptr(unsafe.Pointer(text)),
			uintptr(unsafe.Pointer(caption)),
			mbYesNo|mbIconQuestion,
		)
		if r != idYes {
			return
		}
		w.beginQuit()
	default:
		w.beginQuit()
	}
}

// beginQuit asks the engine to stop-and-restore; the window lives
// until the engine's quit callback (or the watchdog) says go.
func (w *liteWindow) beginQuit() {
	w.app.Quit()
	procSetTimer.Call(uintptr(w.hwnd), watchdogID, watchdogWait, 0)
}

// ---- the animation clock ----

func (w *liteWindow) startAnimTimer() {
	if !w.animRunning {
		procSetTimer.Call(uintptr(w.hwnd), animTimerID, animPeriod, 0)
		w.animRunning = true
	}
}

func (w *liteWindow) stopAnimTimer() {
	if w.animRunning {
		procKillTimer.Call(uintptr(w.hwnd), animTimerID)
		w.animRunning = false
	}
}

// ---- the paint ----

func (w *liteWindow) invalidate() {
	procInvalidateRect.Call(uintptr(w.hwnd), 0, 0)
}

func (w *liteWindow) paint() {
	var ps wPaintStruct
	hdc, _, _ := procBeginPaint.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc wRect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
	width, height := rc.Right, rc.Bottom
	if width <= 0 || height <= 0 {
		return
	}

	// Double buffer: everything draws into a memory bitmap, one BitBlt
	// lands it — zero flicker at any repaint rate.
	hdcMem, _, _ := procCreateCompatibleDC.Call(hdc)
	hbm, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	oldBmp, _, _ := procSelectObject.Call(hdcMem, uintptr(hbm))
	defer func() {
		procSelectObject.Call(hdcMem, oldBmp)
		procDeleteObject.Call(uintptr(hbm))
		procDeleteDC.Call(hdcMem)
	}()

	w.paintDC = hdcMem
	w.paintScreen(hdcMem, width, height)
	w.paintDC = 0

	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), hdcMem, 0, 0, srcCopy)
}

// openYouTube opens the default browser at YouTube (through the live
// tunnel whenever the system proxy is set — the card's whole point).
func openYouTube(hwnd windows.Handle) {
	verb, _ := utf16("open")
	url, _ := utf16("https://www.youtube.com/")
	procShellExecuteW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(url)), 0, 0, swShownormal)
}
