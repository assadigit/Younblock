//go:build linux && !android

// Package platform picks the Platform implementation for this build: the
// real Windows adapter on Windows, the real Linux adapter on Linux (but
// not on Android — GOOS=android implies the linux tag in Go's build
// rules, so the !android term routes Android to select_android.go; the
// audit doc 01 §5 recorded the previous belief that android fell through
// to select_other.go/stub, which was wrong — corrected this session,
// DECISIONS.md D71), and the call-recording stub everywhere else.
package platform

import (
	"fmt"
	"os"
	"sync"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/linux"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

var (
	once sync.Once
	plat engine.Platform
)

// New returns the platform adapter for this build. If the Linux adapter
// cannot initialize (for example the config directory is unavailable),
// the stub is returned so the app still runs without system-proxy
// support — the same fallback select_windows.go records in DECISIONS.md.
func New() engine.Platform {
	once.Do(func() {
		p, err := linux.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "younblock: linux platform failed to initialize; running without system proxy: %v\n", err)
			plat = stub.New()
			return
		}
		plat = p
	})
	return plat
}
