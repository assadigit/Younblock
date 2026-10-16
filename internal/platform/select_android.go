//go:build android

// Package platform picks the Platform implementation for this build: the
// Android adapter on Android. The engine in the real app is created by the
// mobile/ facade, which owns the config dir and the two doors (protect
// hook, notice callback) handed over from Kotlin; this generic selector
// serves the other consumers (cmd/core compiled for android, the compile
// matrix of doc 06 §A) with the same adapter in its zero no-op form.
package platform

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/android"
)

var (
	once sync.Once
	plat engine.Platform
)

// New returns the platform adapter for this build.
func New() engine.Platform {
	once.Do(func() {
		dir := ""
		if base, err := os.UserConfigDir(); err == nil {
			dir = filepath.Join(base, config.AppName)
		}
		plat = android.New(android.Options{ConfigDir: dir})
	})
	return plat
}
