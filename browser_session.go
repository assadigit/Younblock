// browser_session.go — the shared state for the optional dedicated Chrome
// session (user-requested; DECISIONS.md D40). The launch/stop behavior is
// per-platform: browser_windows.go does the real work, browser_other.go is
// the development no-op.
package main

import (
	"os/exec"
	"sync"
)

// browserSession tracks the Chrome process this app started (if any).
type browserSession struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}
