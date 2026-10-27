//go:build windows

// ui_windows.go — the raw Win32 syscall layer for the lite desktop
// edition: the lazy procs, the ABI structs and the small helpers. Every
// call goes through golang.org/x/sys/windows' lazy loader (already a
// repo dependency, pure Go, no cgo), so the whole edition cross-builds
// from Linux exactly like younblock-core.exe does. The window itself
// lives in window_windows.go.
//
// Why hand-rolled Win32 at all (DECISIONS.md D114): the main desktop
// rides Wails + WebView2 — a runtime Windows must have installed. The
// lite edition's promise is "copy the exe, double-click, it works":
// GDI and user32 have shipped inside every Windows since forever, so
// there is nothing to install and nothing that can be missing.
package main

import (
        "unsafe"

        "golang.org/x/sys/windows"
)

// ---- lazy DLLs and procs (resolved once, first use) ----

var (
        modUser32   = windows.NewLazySystemDLL("user32.dll")
        modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
        modGdi32    = windows.NewLazySystemDLL("gdi32.dll")
        modShell32  = windows.NewLazySystemDLL("shell32.dll")
        modShcore   = windows.NewLazySystemDLL("shcore.dll")

        procRegisterClassExW            = modUser32.NewProc("RegisterClassExW")
        procCreateWindowExW             = modUser32.NewProc("CreateWindowExW")
        procDefWindowProcW              = modUser32.NewProc("DefWindowProcW")
        procDestroyWindow               = modUser32.NewProc("DestroyWindow")
        procGetMessageW                 = modUser32.NewProc("GetMessageW")
        procTranslateMessage            = modUser32.NewProc("TranslateMessage")
        procDispatchMessageW            = modUser32.NewProc("DispatchMessageW")
        procPostQuitMessage             = modUser32.NewProc("PostQuitMessage")
        procPostMessageW                = modUser32.NewProc("PostMessageW")
        procBeginPaint                  = modUser32.NewProc("BeginPaint")
        procEndPaint                    = modUser32.NewProc("EndPaint")
        procInvalidateRect              = modUser32.NewProc("InvalidateRect")
        procUpdateWindow                = modUser32.NewProc("UpdateWindow")
        procGetClientRect               = modUser32.NewProc("GetClientRect")
        procGetWindowRect               = modUser32.NewProc("GetWindowRect")
        procShowWindow                  = modUser32.NewProc("ShowWindow")
        procSetTimer                    = modUser32.NewProc("SetTimer")
        procKillTimer                   = modUser32.NewProc("KillTimer")
        procMessageBoxW                 = modUser32.NewProc("MessageBoxW")
        procSetForegroundWindow         = modUser32.NewProc("SetForegroundWindow")
        procIsIconic                    = modUser32.NewProc("IsIconic")
        procFindWindowW                 = modUser32.NewProc("FindWindowW")
        procLoadCursorW                 = modUser32.NewProc("LoadCursorW")
        procLoadIconW                   = modUser32.NewProc("LoadIconW")
        procSetCursor                   = modUser32.NewProc("SetCursor")
        procTrackMouseEvent             = modUser32.NewProc("TrackMouseEvent")
        procSetWindowPos                = modUser32.NewProc("SetWindowPos")
        procGetDC                       = modUser32.NewProc("GetDC")
        procReleaseDC                   = modUser32.NewProc("ReleaseDC")
        procGetDpiForWindow             = modUser32.NewProc("GetDpiForWindow")
        procAdjustWindowRectForDpi      = modUser32.NewProc("AdjustWindowRectForDpi")
        procAdjustWindowRect            = modUser32.NewProc("AdjustWindowRect")
        procSetProcessDpiAwareness      = modShcore.NewProc("SetProcessDpiAwareness")
        procSetProcessDPIAware          = modUser32.NewProc("SetProcessDPIAware")
        procSetProcessDpiAwarenessContx = modUser32.NewProc("SetProcessDpiAwarenessContext")

        procCreateMutexW = modKernel32.NewProc("CreateMutexW")
        procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")

        procCreateFontIndirectW     = modGdi32.NewProc("CreateFontIndirectW")
        procCreateSolidBrush        = modGdi32.NewProc("CreateSolidBrush")
        procCreatePen               = modGdi32.NewProc("CreatePen")
        procExtCreatePen            = modGdi32.NewProc("ExtCreatePen")
        procDeleteObject            = modGdi32.NewProc("DeleteObject")
        procSelectObject            = modGdi32.NewProc("SelectObject")
        procSetTextColor            = modGdi32.NewProc("SetTextColor")
        procSetBkMode               = modGdi32.NewProc("SetBkMode")
        procDrawTextW               = modGdi32.NewProc("DrawTextW")
        procGetTextExtentPoint32W   = modGdi32.NewProc("GetTextExtentPoint32W")
        procGetDeviceCaps           = modGdi32.NewProc("GetDeviceCaps")
        procFillRect                = modUser32.NewProc("FillRect")
        procRoundRect               = modGdi32.NewProc("RoundRect")
        procEllipse                 = modGdi32.NewProc("Ellipse")
        procMoveToEx                = modGdi32.NewProc("MoveToEx")
        procLineTo                  = modGdi32.NewProc("LineTo")
        procCreateCompatibleDC      = modGdi32.NewProc("CreateCompatibleDC")
        procCreateCompatibleBitmap  = modGdi32.NewProc("CreateCompatibleBitmap")
        procBitBlt                  = modGdi32.NewProc("BitBlt")
        procDeleteDC                = modGdi32.NewProc("DeleteDC")

        procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// ---- ABI structs (layouts verified against the Win32 headers; the
// Go declaration order matches the C member order on both amd64 and
// 386, where all handle-sized fields are uintptr) ----

type wPoint struct{ X, Y int32 }

type wRect struct{ Left, Top, Right, Bottom int32 }

type wMsg struct {
        Hwnd   windows.Handle
        Msg    uint32
        WParam uintptr
        LParam uintptr
        Time   uint32
        Pt     wPoint
}

type wWndClassEx struct {
        Size         uint32
        Style        uint32
        WndProc      uintptr
        ClsExtra     int32
        WndExtra     int32
        Instance     windows.Handle
        Icon         windows.Handle
        Cursor       windows.Handle
        Background   windows.Handle
        MenuName     *uint16
        ClassName    *uint16
        IconSm       windows.Handle
}

type wPaintStruct struct {
        Hdc         windows.Handle
        FErase      int32
        RcPaint     wRect
        FRestore    int32
        FIncUpdate  int32
        RgbReserved [32]byte
}

type wLogFontW struct {
        Height         int32
        Width          int32
        Escapement     int32
        Orientation    int32
        Weight         int32
        Italic         byte
        Underline      byte
        StrikeOut      byte
        CharSet        byte
        OutPrecision   byte
        ClipPrecision  byte
        Quality        byte
        PitchAndFamily byte
        FaceName       [32]uint16
}

type wLogBrush struct {
        Style uint32
        Color uintptr
        Hatch uintptr
}

type wTrackMouseEvent struct {
        Size       uint32
        Flags      uint32
        HwndTarget windows.Handle
        HoverTime  uint32
}

type wSize struct{ Cx, Cy int32 }

// ---- constants (WinUser / WinGDI) ----

const (
        wmCreate         = 0x0001
        wmDestroy        = 0x0002
        wmQueryEndSession = 0x0011
        wmEndSession     = 0x0016
        wmClose          = 0x0010
        wmPaint          = 0x000F
        wmEraseBkgnd     = 0x0014
        wmSetCursor      = 0x0020
        wmKeyDown        = 0x0100
        wmTimer          = 0x0113
        wmMouseMove      = 0x0200
        wmLButtonDown    = 0x0201
        wmLButtonUp      = 0x0202
        wmMouseLeave     = 0x02A3
        wmDpiChanged     = 0x02E0
        wmAppQuit        = 0x8001 // WM_APP+1: the engine's quit callback

        vkTab    = 0x09
        vkReturn = 0x0D
        vkSpace  = 0x20

        wsCaption    = 0x00C00000
        wsSysMenu    = 0x00080000
        wsMinimizeBx = 0x00020000

        swShownormal = 1
        swRestore    = 9

        mbYesNo         = 0x00000004
        mbIconQuestion  = 0x00000020
        idYes           = 6

        csHRedraw = 0x0002
        csVRedraw = 0x0001

        idcArrow = 32512
        idcHand  = 32649

        tmeLeave = 0x00000002

        swpNoZOrder    = 0x0004
        swpNoActivate  = 0x0010

        dtLeft         = 0x0000
        dtCenter       = 0x0001
        dtVCenter      = 0x0004
        dtSingleLine   = 0x0020
        dtEndEllipsis  = 0x8000

        transparentBk = 1

        psGeometric = 0x10000 // round caps/joins are the zero defaults

        psDot = 2 // cosmetic dotted pen (focus ring)

        srcCopy = 0x00CC0020

        logPixelsX = 88

        fwSemibold = 600
        fwBold     = 700

        clearTypeQuality = 5
        defaultCharset   = 1

        dpiAwarePerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
)

// rgb packs a GDI COLORREF (0x00BBGGRR).
func rgb(r, g, b uint8) uintptr {
        return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}

// utf16 returns the pointer for a Go string as UTF-16 (the callers keep
// the slice alive across the Call, which takes uintptr — the classic
// unsafe pattern the Win32 world uses).
func utf16(s string) (*uint16, error) { return windows.UTF16PtrFromString(s) }

// textExtent measures a string in the currently selected font.
func textExtent(hdc uintptr, s string) (int32, int32) {
        w, err := windows.UTF16FromString(s)
        if err != nil || len(w) == 0 {
                return 0, 0
        }
        var sz wSize
        procGetTextExtentPoint32W.Call(
                uintptr(hdc),
                uintptr(unsafe.Pointer(&w[0])),
                uintptr(len(w)-1),
                uintptr(unsafe.Pointer(&sz)),
        )
        return sz.Cx, sz.Cy
}
