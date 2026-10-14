//go:build !windows && !linux

// Package platform picks the Platform implementation for this build: the
// real Windows adapter on Windows, the real Linux adapter on Linux, and
// the call-recording stub everywhere else (macOS and any other system).
package platform

import (
	"sync"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

var (
	once sync.Once
	plat engine.Platform
)

// New returns the platform adapter for this build.
func New() engine.Platform {
	once.Do(func() {
		plat = stub.New()
	})
	return plat
}
