//go:build !windows

// Package platform picks the Platform implementation for this build: the
// real Windows adapter on Windows, the call-recording stub elsewhere.
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
