//go:build windows

package main

// The reference design's typeface, embedded: the states-preview (the
// design source of truth) is set in Space Grotesk — a geometric sans
// the system never has. The four static weights ride INSIDE the exe
// (go:embed), register process-privately at startup
// (AddFontMemResourceEx, FR_PRIVATE — nothing is installed beside the
// exe, nothing leaks to other processes, nothing survives the exit:
// the zero-dependency contract keeps holding), and GDI's family/weight
// matching picks the right cut per CreateFontIndirectW. When the
// registration cannot happen for any reason the painter falls back to
// "Segoe UI" honestly — the words always render, the look steps down.
//
// Space Grotesk by Florian Karsten — SIL Open Font License 1.1 (the
// OFL.txt beside the TTFs in the repo; the license permits exactly
// this embedding).

import (
	_ "embed"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed fonts/SpaceGrotesk-Regular.ttf
var sgRegular []byte

//go:embed fonts/SpaceGrotesk-Medium.ttf
var sgMedium []byte

//go:embed fonts/SpaceGrotesk-SemiBold.ttf
var sgSemiBold []byte

//go:embed fonts/SpaceGrotesk-Bold.ttf
var sgBold []byte

// frPrivate: the font resource is visible to this process only
// (AddFontMemResourceEx's private mode).
const frPrivate = 0x10

var procAddFontMemResourceEx = modGdi32.NewProc("AddFontMemResourceEx")

// gdiFace is the face the painter asks GDI for: the reference's cut
// when the embedded registration succeeded, the system's honest
// fallback otherwise.
var gdiFace = "Segoe UI"

// registerEmbeddedFonts installs the four Space Grotesk cuts
// process-privately. Called once from main, after the run log exists
// (so the face it chose is recorded) and before the window's fonts
// are built.
func registerEmbeddedFonts() {
	fonts := [][]byte{sgRegular, sgMedium, sgSemiBold, sgBold}
	ok := 0
	for _, f := range fonts {
		if len(f) == 0 {
			continue
		}
		var n uint32
		h, _, _ := procAddFontMemResourceEx.Call(
			uintptr(unsafe.Pointer(&f[0])),
			uintptr(len(f)),
			0,
			uintptr(unsafe.Pointer(&n)),
		)
		if h != 0 {
			ok++
		} else {
			diagLog("font registration failed (AddFontMemResourceEx), falling back per-face")
		}
	}
	if ok > 0 {
		gdiFace = "Space Grotesk"
	}
	diagLog("embedded fonts registered: %d/4 (face=%q)", ok, gdiFace)
}

// faceNameUTF16 is the painter's face in LOGFONTW's wide form.
func faceNameUTF16() []uint16 {
	w, _ := windows.UTF16FromString(gdiFace)
	return w
}
