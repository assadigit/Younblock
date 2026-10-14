//go:build windows

// clientsize_windows.go — D48 root fix: Wails' initial SetSize applies the
// logical 360x560 to the OUTER window rectangle (title bar and frame
// included, after DPI scaling), so the WebView2 client area — the CSS
// viewport — always comes up short (Ali's screenshots measured 336x522
// instead of 360x560). This file finds our window, measures its real
// frame, and resizes the outer window so the CLIENT area is exactly the
// design size at the current DPI.
package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type rect struct {
	left, top, right, bottom int32
}

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW        = user32.NewProc("FindWindowW")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowRect      = user32.NewProc("GetWindowRect")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procMoveWindow         = user32.NewProc("MoveWindow")
	procGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
	procGetClassNameW      = user32.NewProc("GetClassNameW")
)

// ensureDesignClientSize grows the outer window so the client area is
// designW x designH logical pixels at the window's current DPI. It is
// deliberately conservative: any failure leaves the window exactly as
// Wails made it (the CSS layout also carries its own responsive ladder
// and scroll safety net, D48).
func ensureDesignClientSize(designW, designH int32) error {
	hwnd, err := findOurWindow()
	if err != nil {
		return err
	}

	// The window's effective DPI (96 = 100%). Windows 10 Creators+; on
	// older systems the call is missing and we keep the Wails size.
	if err := procGetDpiForWindow.Find(); err != nil {
		return fmt.Errorf("GetDpiForWindow unavailable: %w", err)
	}
	dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	scale := float64(dpi) / 96.0

	var outer, client rect
	if r1, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&outer))); r1 == 0 {
		return fmt.Errorf("GetWindowRect failed")
	}
	if r1, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client))); r1 == 0 {
		return fmt.Errorf("GetClientRect failed")
	}

	frameW := (outer.right - outer.left) - client.right  // client.left is 0
	frameH := (outer.bottom - outer.top) - client.bottom // client.top is 0

	wantClientW := int32(float64(designW)*scale + 0.5)
	wantClientH := int32(float64(designH)*scale + 0.5)

	outerW := wantClientW + frameW
	outerH := wantClientH + frameH

	// Keep the visual center stable: shift the top-left by half the growth.
	growW := outerW - (outer.right - outer.left)
	growH := outerH - (outer.bottom - outer.top)
	x := outer.left - growW/2
	y := outer.top - growH/2

	if r1, _, _ := procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(outerW), uintptr(outerH), 1); r1 == 0 {
		return fmt.Errorf("MoveWindow failed")
	}
	return nil
}

// findOurWindow returns the wails main window: a visible top-level
// "wailsWindow"-class window owned by this process.
func findOurWindow() (uintptr, error) {
	var found uintptr
	pid := uint32(windows.GetCurrentProcessId())

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var winPid uint32
		procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&winPid)))
		if winPid != pid {
			return 1 // keep enumerating
		}
		var buf [64]uint16
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return 1
		}
		if windows.UTF16ToString(buf[:n]) != "wailsWindow" {
			return 1
		}
		found = hwnd
		return 0 // stop
	})
	_, _, _ = procEnumWindows.Call(cb, 0)
	if found == 0 {
		// Fallback: FindWindow by the wails class name (single-instance app).
		hwnd, _, _ := procFindWindowW.Call(
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("wailsWindow"))), 0)
		if hwnd == 0 {
			return 0, fmt.Errorf("wails window not found")
		}
		return hwnd, nil
	}
	return found, nil
}
