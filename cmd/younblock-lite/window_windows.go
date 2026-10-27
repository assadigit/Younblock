//go:build windows

// window_windows.go — the lite desktop edition's window: one Win32
// top-level window, painted with plain GDI (double-buffered), no child
// controls, no common controls, no COM — nothing that could be missing
// on a fresh Windows box. The screen is the brand's charcoal, the Y
// mark (the D113 glyph now drawn as the header), one state word with
// an LED dot, one honest line, one primary button, an Open YouTube
// link and a version footer (DECISIONS.md D114).
//
// Threading: this file owns THE UI thread (LockOSThread). The engine
// runs on its own goroutines; state reaches the window through the
// App's channel, drained by a 150 ms timer — and the engine's quit
// callback marshals back with PostMessage, the only thread-safe door.
package main

import (
        "runtime"
        "syscall"
        "unsafe"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
        "golang.org/x/sys/windows"
)

// The one window class name (also the single-instance FindWindow key).
const liteClassName = "YounblockLiteDesktopWindow"

// Client size in logical pixels (96 dpi); everything scales with the
// window's real DPI.
const (
        liteClientW = 360
        liteClientH = 470
)

// The brand palette (the v3 screen's charcoal and red; amber for the
// transitional states, gray for off — the tray icons' vocabulary).
var (
        colBg       = rgb(0x2d, 0x2d, 0x37)
        colText     = rgb(0xF2, 0xF3, 0xF7)
        colSecond   = rgb(0xA6, 0xA9, 0xB3)
        colRed      = rgb(0xEE, 0x30, 0x29)
        colRedHot   = rgb(0xFF, 0x44, 0x3C)
        colRedDown  = rgb(0xC2, 0x26, 0x21)
        colAmber    = rgb(0xF2, 0xB8, 0x4B)
        colGrayDot  = rgb(0x6B, 0x6E, 0x78)
        colDisabled = rgb(0x3A, 0x3B, 0x45)
        colDisText  = rgb(0x8A, 0x8D, 0x97)
)

// liteWindow carries everything the UI thread owns.
type liteWindow struct {
        app  *App
        hwnd windows.Handle

        dpi  int32 // the window's current DPI (96 at 100%)
        inst windows.Handle

        // fonts (rebuilt when the DPI changes)
        fWordmark, fHeadline, fDetail, fButton, fLink, fFooter windows.Handle

        // layout rectangles in device pixels (rebuilt on resize/DPI change)
        rcY, rcWordmark, rcHeadline, rcDetail, rcButton, rcLink, rcFooter wRect

        // interaction
        focus   int // 0 = primary button, 1 = Open YouTube
        hot     int // -1 none, 0 button, 1 link
        pressed bool

        // what is currently on screen (repaint only on change)
        painted engine.State

        // the once-per-run guard for the openBrowserOnConnect setting
        openedForRun bool
        // the quit watchdog fired already
        forcedQuit bool
}

// runUI is main's one call: the whole window life.
func runUI(app *App) {
        runtime.LockOSThread()
        defer runtime.UnlockOSThread()

        makeDpiAware()

        // Single instance: a second launch just raises the first window.
        if bringUpExisting() {
                return
        }

        w := &liteWindow{app: app, hot: -1}
        if !w.create() {
                return
        }
        // The engine's quit callback arrives on its own goroutine — the
        // only safe marshaling is a posted message.
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
        // carrier while a quit settles).
        procSetTimer.Call(uintptr(w.hwnd), 1, 150, 0)

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
        procKillTimer.Call(uintptr(w.hwnd), 1)
}

// makeDpiAware asks for per-monitor v2 (crisp text at any scaling),
// falling back through the older doors — each best-effort, because a
// blurry-but-working window still beats a failed call.
func makeDpiAware() {
        if r, _, _ := procSetProcessDpiAwarenessContx.Call(dpiAwarePerMonitorV2); r != 0 {
                if r, _, _ := procSetProcessDpiAwareness.Call(2); r != 0 {
                        procSetProcessDPIAware.Call()
                }
        }
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
        }
        // The standard arrow over the client; the hand appears over the
        // actionable things (WM_SETCURSOR).
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
        r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
        if r == 0 {
                return false
        }

        // Outer size for the wanted client size at this DPI.
        dpi := systemDpi()
        frame := wRect{0, 0, liteClientW * dpi / 96, liteClientH * dpi / 96}
        style := uintptr(wsCaption | wsSysMenu | wsMinimizeBx)
        if ok, _, _ := procAdjustWindowRectForDpi.Call(
                uintptr(unsafe.Pointer(&frame)), style, 0, uintptr(dpi)); ok == 0 {
                procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
        }
        title := mustUtf16("Younblock Lite")
        h, _, _ := procCreateWindowExW.Call(
                0,
                uintptr(unsafe.Pointer(className)),
                uintptr(unsafe.Pointer(title)),
                style,
                0x80000000, 0x80000000, // CW_USEDEFAULT position
                uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
                0, 0, uintptr(inst), 0,
        )
        if h == 0 {
                return false
        }
        w.hwnd = windows.Handle(h)
        w.dpi = w.windowDpi()
        w.makeFonts()
        w.layout()
        w.painted = w.app.Poll() // the very first paint is a real state
        return true
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

// windowDpi reads the window's own DPI (per-monitor aware).
func (w *liteWindow) windowDpi() int32 {
        if r, _, _ := procGetDpiForWindow.Call(uintptr(w.hwnd)); r != 0 {
                return int32(r)
        }
        return w.dpi
}

// px converts logical 96-dpi pixels to this window's device pixels.
func (w *liteWindow) px(v int32) int32 { return v * w.dpi / 96 }

// makeFonts builds the font set for the current DPI.
func (w *liteWindow) makeFonts() {
        w.deleteFonts()
        w.fWordmark = makeFont(w.dpi, 12, fwSemibold, false)
        w.fHeadline = makeFont(w.dpi, 20, fwSemibold, false)
        w.fDetail = makeFont(w.dpi, 10, 400, false)
        w.fButton = makeFont(w.dpi, 11, fwBold, false)
        w.fLink = makeFont(w.dpi, 10, 400, true)
        w.fFooter = makeFont(w.dpi, 9, 400, false)
}

func (w *liteWindow) deleteFonts() {
        for _, f := range []windows.Handle{w.fWordmark, w.fHeadline, w.fDetail, w.fButton, w.fLink, w.fFooter} {
                if f != 0 {
                        procDeleteObject.Call(uintptr(f))
                }
        }
}

func makeFont(dpi int32, pt int32, weight int32, underline bool) windows.Handle {
        lf := wLogFontW{
                Height:   -(pt * dpi / 72),
                Weight:   weight,
                Underline: boolByte(underline),
                CharSet:  defaultCharset,
                Quality:  clearTypeQuality,
        }
        copy(lf.FaceName[:], utf16FaceName())
        h, _, _ := procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&lf)))
        return windows.Handle(h)
}

// utf16FaceName is "Segoe UI" as UTF-16 (LOGFONTW wants the wide form).
func utf16FaceName() []uint16 {
        w, _ := windows.UTF16FromString("Segoe UI")
        return w
}

func boolByte(b bool) byte {
        if b {
                return 1
        }
        return 0
}

// layout computes every rectangle in device pixels for the current
// client size. The composition is a centered column under a header —
// one word, one line, one button, one link, one footer.
func (w *liteWindow) layout() {
        var rc wRect
        procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
        width, height := rc.Right, rc.Bottom
        m := w.px(28)

        // Header: the Y mark, then the wordmark.
        w.rcY = wRect{m, w.px(26), m + w.px(40), w.px(26) + w.px(40)}
        w.rcWordmark = wRect{m + w.px(52), w.rcY.Top, width - m, w.rcY.Bottom}

        // The centered state band (LED + word).
        w.rcHeadline = wRect{m, w.px(148), width - m, w.px(148) + w.px(36)}

        // The honest line under it.
        w.rcDetail = wRect{m, w.px(192), width - m, w.px(192) + w.px(24)}

        // The primary button: centered, 200x46 logical.
        bw, bh := w.px(200), w.px(46)
        bx := (width - bw) / 2
        by := w.px(248)
        w.rcButton = wRect{bx, by, bx + bw, by + bh}

        // The Open YouTube link.
        w.rcLink = wRect{m, w.px(318), width - m, w.px(318) + w.px(26)}

        // The footer.
        w.rcFooter = wRect{m, height - w.px(40), width - m, height - w.px(20)}
}

// ---- the message pump ----

func (w *liteWindow) wndProc(hwnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
        switch msg {
        case wmCreate:
                // CreateWindowExW sends this before it returns: the hwnd is
                // ours from this moment (DPI messages may arrive during
                // creation).
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
        case wmDpiChanged:
                w.onDpiChanged(wp)
                return 0
        case wmMouseMove:
                w.onMouseMove(lp)
                return 0
        case wmMouseLeave:
                w.setHot(-1)
                return 0
        case wmLButtonDown:
                w.onButtonDown(lp)
                return 0
        case wmLButtonUp:
                w.onButtonUp(lp)
                return 0
        case wmKeyDown:
                if w.onKey(wp) {
                        return 0
                }
        case wmSetCursor:
                // LOWORD(lp) == HTCLIENT: the cursor is over our client area.
                if uint32(lp)&0xFFFF == 1 {
                        cur, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))
                        if w.hot == 0 || w.hot == 1 {
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
                // The engine finished its stop-and-restore: exit now.
                procPostQuitMessage.Call(0)
                return 0
        case wmQueryEndSession:
                return 1 // yes, we can end — wmEndSession does the work
        case wmEndSession:
                if wp != 0 {
                        // The session is really ending: stop synchronously so the
                        // proxy is restored before the process goes away.
                        w.app.StopNow()
                }
                return 0
        case wmDestroy:
                procPostQuitMessage.Call(0)
                return 0
        }
        r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wp, lp)
        return r
}

// onTimer drains the state channel and repaints on change. Timer 1 is
// the poller; timer 2 is the quit watchdog — a stop that never
// settles must not hang the window forever (a leftover backup file is
// healed on the next start; the platform's crash recovery owns that).
func (w *liteWindow) onTimer(wp uintptr) {
        if wp == 2 {
                procKillTimer.Call(uintptr(w.hwnd), 2)
                if !w.forcedQuit {
                        w.forcedQuit = true
                        procPostQuitMessage.Call(0)
                }
                return
        }
        st := w.app.Poll()
        if w.stateChanged(st) {
                w.painted = st
                w.invalidate()
        }
        // The openBrowserOnConnect setting, honored once per connected run.
        if w.app.OpenYouTubeOnConnect() && st.Phase == engine.PhaseOn && !w.openedForRun {
                w.openedForRun = true
                go openYouTube(w.hwnd)
        }
        if st.Phase == engine.PhaseOff || st.Phase == engine.PhaseError {
                w.openedForRun = false
        }
}

// stateChanged compares what the screen paints.
func (w *liteWindow) stateChanged(st engine.State) bool {
        p := w.painted
        if p.Phase != st.Phase || p.Warm != st.Warm || p.Port != st.Port {
                return true
        }
        if (p.LatencyMs == nil) != (st.LatencyMs == nil) {
                return true
        }
        if p.LatencyMs != nil && st.LatencyMs != nil && *p.LatencyMs != *st.LatencyMs {
                return true
        }
        if (p.Error == nil) != (st.Error == nil) {
                return true
        }
        if p.Error != nil && st.Error != nil && p.Error.Code != st.Error.Code {
                return true
        }
        return false
}

// onDpiChanged: rebuild fonts and layout, and resize the window to
// the right outer size for the new DPI, keeping its on-screen
// position. (The message's lParam points at the system's suggested
// rectangle — system-owned memory this code deliberately never
// dereferences; computing the frame from the fixed client size and
// the current position is equivalent and keeps every pointer in this
// file flowing in the Go-owned, out-parameter direction only.)
func (w *liteWindow) onDpiChanged(wp uintptr) {
        w.dpi = int32(uint32(wp) >> 16)
        var cur wRect
        procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&cur)))
        frame := wRect{0, 0, w.px(liteClientW), w.px(liteClientH)}
        style := uintptr(wsCaption | wsSysMenu | wsMinimizeBx)
        if ok, _, _ := procAdjustWindowRectForDpi.Call(
                uintptr(unsafe.Pointer(&frame)), style, 0, uintptr(w.dpi)); ok == 0 {
                procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
        }
        procSetWindowPos.Call(
                uintptr(w.hwnd), 0,
                uintptr(cur.Left), uintptr(cur.Top),
                uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
                swpNoZOrder|swpNoActivate,
        )
        w.makeFonts()
        w.layout()
        w.invalidate()
}

// ---- input ----

func ptFromLparam(lp uintptr) (int32, int32) {
        return int32(int16(uint32(lp) & 0xFFFF)), int32(int16(uint32(lp) >> 16))
}

func inRect(rc wRect, x, y int32) bool {
        return x >= rc.Left && x < rc.Right && y >= rc.Top && y < rc.Bottom
}

func (w *liteWindow) hit(x, y int32) int {
        if inRect(w.rcButton, x, y) && view(w.painted).ButtonEnabled {
                return 0
        }
        if inRect(w.rcLink, x, y) {
                return 1
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
        w.setHot(w.hit(x, y))
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
        if h := w.hit(x, y); h >= 0 {
                w.pressed = true
                w.focus = h
                w.invalidate()
        }
}

func (w *liteWindow) onButtonUp(lp uintptr) {
        wasPressed := w.pressed
        w.pressed = false
        x, y := ptFromLparam(lp)
        h := w.hit(x, y)
        if h == 0 && wasPressed {
                w.focus = 0
                w.activatePrimary()
        } else if h == 1 && wasPressed {
                w.focus = 1
                go openYouTube(w.hwnd)
        }
        w.invalidate()
}

// onKey: Tab moves between the two actionable things; Space/Enter
// fires the focused one.
func (w *liteWindow) onKey(wp uintptr) bool {
        switch wp {
        case vkTab:
                if w.focus == 0 {
                        w.focus = 1
                } else {
                        w.focus = 0
                }
                w.invalidate()
                return true
        case vkSpace, vkReturn:
                if w.focus == 0 {
                        w.activatePrimary()
                } else if w.focus == 1 {
                        go openYouTube(w.hwnd)
                }
                return true
        }
        return false
}

// activatePrimary runs the one primary action for the current state
// (Start or Stop — the engine's own guards make raced calls honest).
func (w *liteWindow) activatePrimary() {
        if !view(w.painted).ButtonEnabled {
                return
        }
        if primaryAction(w.painted) == "stop" {
                w.app.Disconnect()
        } else {
                w.app.Connect()
        }
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
        procSetTimer.Call(uintptr(w.hwnd), 2, 8000, 0)
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

        w.paintContent(hdcMem, width, height)

        procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), hdcMem, 0, 0, srcCopy)
}

// paintContent draws the whole screen into hdc.
func (w *liteWindow) paintContent(hdc uintptr, width, height int32) {
        st := w.painted
        v := view(st)

        // Background.
        w.fillRect(hdc, 0, 0, width, height, colBg)

        // Header: the Y mark (the D113 glyph, brand red) + wordmark.
        w.drawY(hdc, w.rcY)
        oldF, _, _ := procSelectObject.Call(hdc, uintptr(w.fWordmark))
        w.setGdiText(hdc, colText)
        w.drawText(hdc, "Younblock Lite", w.rcWordmark, dtLeft|dtVCenter|dtSingleLine)
        procSelectObject.Call(hdc, oldF)

        // The state band: LED dot + the state word, one centered line.
        oldH, _, _ := procSelectObject.Call(hdc, uintptr(w.fHeadline))
        tw, _ := textExtent(hdc, v.Headline)
        dot := w.px(13)
        gap := w.px(10)
        total := tw + gap + dot
        left := (width - total) / 2
        cy := (w.rcHeadline.Top + w.rcHeadline.Bottom) / 2
        w.ledDot(hdc, left, cy-dot/2, left+dot, cy+dot/2, v.Dot)
        band := wRect{left + dot + gap, w.rcHeadline.Top, width - left - dot - gap, w.rcHeadline.Bottom}
        w.drawText(hdc, v.Headline, band, dtLeft|dtVCenter|dtSingleLine)
        procSelectObject.Call(hdc, oldH)

        // The honest line.
        oldD, _, _ := procSelectObject.Call(hdc, uintptr(w.fDetail))
        w.setGdiText(hdc, colSecond)
        w.drawText(hdc, v.Detail, w.rcDetail, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis)
        procSelectObject.Call(hdc, oldD)

        // The primary button.
        w.drawButton(hdc, v)

        // The Open YouTube link.
        linkColor := colSecond
        if w.hot == 1 {
                linkColor = colText
        }
        oldL, _, _ := procSelectObject.Call(hdc, uintptr(w.fLink))
        w.setGdiText(hdc, linkColor)
        w.drawText(hdc, "Open YouTube", w.rcLink, dtCenter|dtVCenter|dtSingleLine)
        procSelectObject.Call(hdc, oldL)
        if w.focus == 1 {
                w.focusRing(hdc, w.rcLink)
        }

        // The footer: version (+ the live proxy while it serves).
        footer := "Younblock Lite  ·  " + config.AppVersion
        if v.Footer != "" {
                footer += "  ·  " + v.Footer
        }
        oldFt, _, _ := procSelectObject.Call(hdc, uintptr(w.fFooter))
        w.setGdiText(hdc, colGrayDot)
        w.drawText(hdc, footer, w.rcFooter, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis)
        procSelectObject.Call(hdc, oldFt)
}

// setGdiText: transparent background, this text color.
func (w *liteWindow) setGdiText(hdc uintptr, color uintptr) {
        procSetBkMode.Call(hdc, transparentBk)
        procSetTextColor.Call(hdc, color)
}

func (w *liteWindow) drawText(hdc uintptr, s string, rc wRect, flags uint32) {
        ws, err := windows.UTF16FromString(s)
        if err != nil || len(ws) <= 1 { // the NUL-only string draws nothing
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

func (w *liteWindow) fillRect(hdc uintptr, l, t, r, b int32, color uintptr) {
        br, _, _ := procCreateSolidBrush.Call(color)
        rc := wRect{l, t, r, b}
        procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(br))
        procDeleteObject.Call(br)
}

// drawY paints the Y mark — the same glyph geometry the Android status
// bar shows since D113 (round caps and joins, brand red).
func (w *liteWindow) drawY(hdc uintptr, box wRect) {
        lb := wLogBrush{Style: 0, Color: colRed}
        penW := w.px(4)
        if penW < 2 {
                penW = 2
        }
        pen, _, _ := procExtCreatePen.Call(psGeometric, uintptr(penW), uintptr(unsafe.Pointer(&lb)), 0, 0)
        if pen == 0 {
                return
        }
        defer procDeleteObject.Call(pen)
        old, _, _ := procSelectObject.Call(hdc, pen)
        defer procSelectObject.Call(hdc, old)

        // Geometry on a 40-unit grid, scaled into the box: arms (7,8) ->
        // (20,19) <- (33,8); stem (20,19) -> (20,32) — the D113 Y.
        sx := func(u int32) int32 { return box.Left + u*(box.Right-box.Left)/40 }
        sy := func(u int32) int32 { return box.Top + u*(box.Bottom-box.Top)/40 }
        line(hdc, sx(7), sy(8), sx(20), sy(19))
        line(hdc, sx(33), sy(8), sx(20), sy(19))
        line(hdc, sx(20), sy(19), sx(20), sy(32))
}

func line(hdc uintptr, x1, y1, x2, y2 int32) {
        procMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
        procLineTo.Call(hdc, uintptr(x2), uintptr(y2))
}

func (w *liteWindow) ledDot(hdc uintptr, l, t, r, b int32, c DotColor) {
        var color uintptr
        switch c {
        case DotGray:
                color = colGrayDot
        case DotAmber:
                color = colAmber
        case DotRed:
                color = colRed
        default:
                color = colGrayDot
        }
        br, _, _ := procCreateSolidBrush.Call(color)
        pen, _, _ := procCreatePen.Call(0, 1, color)
        oldB, _, _ := procSelectObject.Call(hdc, uintptr(br))
        oldP, _, _ := procSelectObject.Call(hdc, uintptr(pen))
        procEllipse.Call(hdc, uintptr(l), uintptr(t), uintptr(r), uintptr(b))
        procSelectObject.Call(hdc, oldP)
        procSelectObject.Call(hdc, oldB)
        procDeleteObject.Call(pen)
        procDeleteObject.Call(br)
}

func (w *liteWindow) drawButton(hdc uintptr, v ViewModel) {
        rc := w.rcButton
        if !v.ButtonEnabled {
                w.fillRect(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, colDisabled)
                old, _, _ := procSelectObject.Call(hdc, uintptr(w.fButton))
                w.setGdiText(hdc, colDisText)
                w.drawText(hdc, stoppingLabel(v), rc, dtCenter|dtVCenter|dtSingleLine)
                procSelectObject.Call(hdc, old)
                return
        }
        fill := colRed
        if w.hot == 0 && !w.pressed {
                fill = colRedHot
        }
        // The pill: brush fill + same-color pen border, rounded ends.
        br, _, _ := procCreateSolidBrush.Call(fill)
        edge := fill
        if w.pressed && w.hot == 0 {
                edge = colRedDown
        }
        pen, _, _ := procExtCreatePen.Call(psGeometric, 2, uintptr(unsafe.Pointer(&wLogBrush{Color: edge})), 0, 0)
        oldB, _, _ := procSelectObject.Call(hdc, uintptr(br))
        oldP, _, _ := procSelectObject.Call(hdc, uintptr(pen))
        rad := (rc.Bottom - rc.Top) / 2
        procRoundRect.Call(hdc, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), uintptr(rad), uintptr(rad))
        procSelectObject.Call(hdc, oldP)
        procSelectObject.Call(hdc, oldB)
        procDeleteObject.Call(pen)
        procDeleteObject.Call(br)

        old, _, _ := procSelectObject.Call(hdc, uintptr(w.fButton))
        w.setGdiText(hdc, colText)
        dx := int32(0)
        if w.pressed && w.hot == 0 {
                dx = 1 // the classic pressed nudge
        }
        rc2 := wRect{rc.Left + dx, rc.Top + dx, rc.Right + dx, rc.Bottom + dx}
        w.drawText(hdc, v.ButtonLabel, rc2, dtCenter|dtVCenter|dtSingleLine)
        procSelectObject.Call(hdc, old)
        if w.focus == 0 {
                w.focusRing(hdc, rc)
        }
}

// stoppingLabel: the disabled button still says where things stand.
func stoppingLabel(v ViewModel) string {
        if v.Phase == engine.PhaseStopping {
                return "Stopping…"
        }
        return v.ButtonLabel
}

// focusRing: a dotted rectangle (a cosmetic PS_DOT pen — DrawFocusRect
// XOR pattern disappears on dark backgrounds).
func (w *liteWindow) focusRing(hdc uintptr, rc wRect) {
        pen, _, _ := procCreatePen.Call(psDot, 1, colText)
        if pen == 0 {
                return
        }
        defer procDeleteObject.Call(pen)
        old, _, _ := procSelectObject.Call(hdc, pen)
        // A dotted rectangle via four lines (Rectangle() would fill it).
        r := wRect{rc.Left - 3, rc.Top - 3, rc.Right + 3, rc.Bottom + 3}
        line(hdc, r.Left, r.Top, r.Right, r.Top)
        line(hdc, r.Right, r.Top, r.Right, r.Bottom)
        line(hdc, r.Right, r.Bottom, r.Left, r.Bottom)
        line(hdc, r.Left, r.Bottom, r.Left, r.Top)
        procSelectObject.Call(hdc, old)
}

// openYouTube opens the default browser at YouTube (through the live
// tunnel whenever the system proxy is set — the link's whole point).
func openYouTube(hwnd windows.Handle) {
        verb, _ := utf16("open")
        url, _ := utf16("https://www.youtube.com/")
        procShellExecuteW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(verb)),
                uintptr(unsafe.Pointer(url)), 0, 0, swShownormal)
}
