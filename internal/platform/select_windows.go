//go:build windows

// Package platform picks the Platform implementation for this build: the
// real Windows adapter on Windows, the call-recording stub elsewhere.
package platform

import (
	"fmt"
	"os"
	"sync"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
	"github.com/assadigit/Younblock/internal/platform/windows"
)

var (
	once sync.Once
	plat engine.Platform
)

// New returns the platform adapter for this build. If the Windows adapter
// cannot initialize (for example the config directory is unavailable), the
// stub is returned so the app still runs without system-proxy support;
// that fallback is recorded in DECISIONS.md.
func New() engine.Platform {
	once.Do(func() {
		p, err := windows.New(stdLogger{})
		if err != nil {
			stdLogger{}.Warn("windows platform failed to initialize; running without system proxy", "reason", err.Error())
			plat = stub.New()
			return
		}
		plat = p
	})
	return plat
}

// stdLogger writes to stderr before the engine's logger exists.
type stdLogger struct{}

func (stdLogger) Info(msg string, args ...any)  { logLine("INFO", msg, args...) }
func (stdLogger) Warn(msg string, args ...any)  { logLine("WARN", msg, args...) }
func (stdLogger) Error(msg string, args ...any) { logLine("ERROR", msg, args...) }

func logLine(level, msg string, args ...any) {
	line := level + " " + msg
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			line += fmt.Sprintf(" %s=%v", k, args[i+1])
		}
	}
	os.Stderr.WriteString("younblock: " + line + "\n")
}
