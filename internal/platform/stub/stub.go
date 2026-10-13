// Package stub is the no-op platform adapter used on Linux and in tests:
// it records every call so everything else is testable (file 01, section 8).
package stub

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
)

// AttachCall records one Attach invocation.
type AttachCall struct {
	Port int
	Mode config.Mode
}

// NoticeCall records one Notify invocation.
type NoticeCall struct {
	Kind engine.NoticeKind
	Text string
}

// Platform implements engine.Platform doing nothing but recording calls.
type Platform struct {
	mu          sync.Mutex
	dir         string
	attachCalls []AttachCall
	detachCalls int
	autostarts  []bool
	notices     []NoticeCall
	restores    int
}

// New returns a stub platform using the default config directory.
func New() *Platform {
	dir := ""
	if base, err := os.UserConfigDir(); err == nil {
		dir = filepath.Join(base, config.AppName)
	}
	return &Platform{dir: dir}
}

// NewWithDir returns a stub platform with an explicit config directory
// (tests pass a temp folder).
func NewWithDir(dir string) *Platform {
	return &Platform{dir: dir}
}

// Name reports "stub".
func (p *Platform) Name() string { return "stub" }

// Capabilities reports nothing supported.
func (p *Platform) Capabilities() engine.Capabilities {
	return engine.Capabilities{}
}

// DialControl returns nil (desktop; no socket protection needed).
func (p *Platform) DialControl() engine.ControlFunc { return nil }

// Attach records the call.
func (p *Platform) Attach(port int, s config.Settings) error {
	p.mu.Lock()
	p.attachCalls = append(p.attachCalls, AttachCall{Port: port, Mode: s.Mode})
	p.mu.Unlock()
	return nil
}

// Detach records the call.
func (p *Platform) Detach() error {
	p.mu.Lock()
	p.detachCalls++
	p.mu.Unlock()
	return nil
}

// SetAutostart records the call.
func (p *Platform) SetAutostart(enabled bool) error {
	p.mu.Lock()
	p.autostarts = append(p.autostarts, enabled)
	p.mu.Unlock()
	return nil
}

// Notify records the call.
func (p *Platform) Notify(kind engine.NoticeKind, text string) {
	p.mu.Lock()
	p.notices = append(p.notices, NoticeCall{Kind: kind, Text: text})
	p.mu.Unlock()
}

// ConfigDir returns the configured directory.
func (p *Platform) ConfigDir() string { return p.dir }

// RestoreProxy records the call and succeeds.
func (p *Platform) RestoreProxy() error {
	p.mu.Lock()
	p.restores++
	p.mu.Unlock()
	return nil
}

// AttachCalls returns the recorded Attach calls.
func (p *Platform) AttachCalls() []AttachCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]AttachCall(nil), p.attachCalls...)
}

// DetachCalls returns how many times Detach ran.
func (p *Platform) DetachCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.detachCalls
}

// Autostarts returns the recorded autostart changes.
func (p *Platform) Autostarts() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.autostarts...)
}

// Notices returns the recorded notifications.
func (p *Platform) Notices() []NoticeCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]NoticeCall(nil), p.notices...)
}

// Restores returns how many times RestoreProxy ran.
func (p *Platform) Restores() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.restores
}
