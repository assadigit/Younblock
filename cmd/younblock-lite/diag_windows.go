//go:build windows

// diag_windows.go — the Windows half of the run log: the true Windows
// version (RtlGetVersion, which never lies about compatibility) and
// the failure dialog that guarantees a startup problem is SEEN, not
// silent.
package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNtdll          = windows.NewLazySystemDLL("ntdll.dll")
	procRtlGetVersion = modNtdll.NewProc("RtlGetVersion")
)

// rtlOsVersionInfoW is RTL_OSVERSIONINFOW (the RtlGetVersion ABI).
type rtlOsVersionInfoW struct {
	Size         uint32
	MajorVersion uint32
	MinorVersion uint32
	BuildNumber  uint32
	PlatformId   uint32
	CSDVersion   [128]uint16
}

// windowsVersionString reads the REAL Windows version — the manifest
// lies (compat shims) don't apply to RtlGetVersion — plus the machine
// shape, so a log line places the run on the owner's actual system.
func windowsVersionString() string {
	arch := ""
	switch runtime.GOARCH {
	case "amd64":
		arch = " x64"
	case "386":
		arch = " x86"
	case "arm64":
		arch = " arm64"
	}
	if !procOK(procRtlGetVersion) {
		return "Windows" + arch + " (version unavailable)"
	}
	var vi rtlOsVersionInfoW
	vi.Size = uint32(unsafe.Sizeof(vi))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&vi)))
	if r != 0 || vi.MajorVersion == 0 {
		return "Windows" + arch + " (version unknown)"
	}
	name := ""
	switch {
	case vi.MajorVersion >= 11:
		name = " 11+"
	case vi.MajorVersion == 10 && vi.BuildNumber >= 22000:
		name = " 11"
	case vi.MajorVersion == 10 && vi.BuildNumber >= 19042:
		name = " 10 20H2+"
	case vi.MajorVersion == 10 && vi.BuildNumber >= 1607:
		name = " 10 1607+"
	case vi.MajorVersion == 10:
		name = " 10 (pre-1607)"
	case vi.MajorVersion == 6 && vi.MinorVersion == 3:
		name = " 8.1"
	case vi.MajorVersion == 6 && vi.MinorVersion == 2:
		name = " 8"
	case vi.MajorVersion == 6:
		name = " Vista/7"
	}
	return fmt.Sprintf("Windows %d.%d.%d%s%s", vi.MajorVersion, vi.MinorVersion, vi.BuildNumber, name, arch)
}

// diagFatal is the last resort: the app could not start, so SAY SO —
// a MessageBox with the failure and where the details live. This is
// the dialog the owner of a machine where v1.6.12 silently died
// should have seen the first time.
func diagFatal(err any) {
	msg := "Younblock Lite could not start.\n\n"
	if err != nil {
		msg += fmt.Sprintf("%v\n\n", err)
	}
	if p := diagWhere(); p != "" {
		msg += "Details were written to:\n" + p
	} else {
		msg += "No log file could be written."
	}
	title, terr := utf16("Younblock Lite")
	if terr != nil {
		return
	}
	text, terr2 := utf16(msg)
	if terr2 != nil {
		return
	}
	if !procOK(procMessageBoxW) {
		return
	}
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		mbIconError,
	)
}
