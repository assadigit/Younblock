//go:build windows

package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"

	"github.com/go-toast/toast"

	syswin "golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// internetSettingsKey adapts registry.Key to the internetSettings
// interface used by the (platform-neutral) logic.
type internetSettingsKey struct {
	key registry.Key
}

func (k internetSettingsKey) readString(name string) (string, bool, error) {
	v, _, err := k.key.GetStringValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (k internetSettingsKey) readDWORD(name string) (uint32, bool, error) {
	v, _, err := k.key.GetIntegerValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return 0, false, nil
		}
		return 0, false, err
	}
	return uint32(v), true, nil
}

func (k internetSettingsKey) writeString(name, val string) error {
	return k.key.SetStringValue(name, val)
}

func (k internetSettingsKey) writeDWORD(name string, val uint32) error {
	return k.key.SetDWordValue(name, val)
}

func (k internetSettingsKey) deleteValue(name string) error {
	return k.key.DeleteValue(name)
}

// internetSettingsPath is the proxy key Chrome follows.
const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func openInternetSettings() (internetSettings, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	return internetSettingsKey{key: k}, nil
}

func openRunKey() (internetSettings, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, RunKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	return internetSettingsKey{key: k}, nil
}

// wininet refresh: tell Windows the proxy settings changed (file 01 §8).
const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

var internetSetOption = newLazyInternetSetOption()

func newLazyInternetSetOption() func(option uint32) error {
	dll := syswin.NewLazySystemDLL("wininet.dll")
	proc := dll.NewProc("InternetSetOptionW")
	return func(option uint32) error {
		r1, _, lastErr := proc.Call(0, uintptr(option), 0, 0)
		if r1 == 0 {
			return fmt.Errorf("InternetSetOptionW(%d): %w", option, lastErr)
		}
		return nil
	}
}

func refreshProxySettings() error {
	if err := internetSetOption(internetOptionSettingsChanged); err != nil {
		return err
	}
	return internetSetOption(internetOptionRefresh)
}

// Platform is the real Windows adapter.
type Platform struct {
	mu      sync.Mutex
	dir     string
	exePath string
	sys     *sysproxy
	notify  func(kind engine.NoticeKind, text string)
}

// New builds the Windows platform and runs crash recovery (R16): if a
// previous run left our proxy setting behind, it is restored right now,
// before the engine starts anything.
func New(log Logger) (*Platform, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("user config dir: %w", err)
	}
	dir := filepath.Join(base, config.AppName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	p := &Platform{
		dir:     dir,
		exePath: exe,
		sys:     newSysproxy(dir, openInternetSettings, refreshProxySettings, log),
	}
	// Crash recovery at every app start (file 01, section 8).
	if err := p.sys.Restore(); err != nil {
		log.Warn("crash recovery could not restore the proxy setting", "reason", err.Error())
	}
	// Autostart path rewrite (handles a moved exe).
	p.rewriteAutostart(log)
	return p, nil
}

func (p *Platform) rewriteAutostart(log Logger) {
	if p.exePath == "" {
		return
	}
	key, err := openRunKey()
	if err != nil {
		return
	}
	rewritten, err := rewriteIfMoved(p.exePath, key)
	if err != nil {
		log.Warn("checking the autostart entry failed", "reason", err.Error())
		return
	}
	if rewritten {
		log.Info("autostart entry updated after the app moved")
	}
}

// Name reports "windows".
func (p *Platform) Name() string { return "windows" }

// Capabilities reports what Windows supports.
func (p *Platform) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		Tray:          true,
		StartWithOS:   true,
		SystemProxy:   true,
		PerAppRouting: false,
	}
}

// DialControl returns nil on desktop.
func (p *Platform) DialControl() engine.ControlFunc { return nil }

// Attach applies the system proxy per the mode (file 01, section 8).
func (p *Platform) Attach(port int, s config.Settings) error {
	switch s.Mode {
	case config.ModeManual:
		return nil // touch nothing in Windows
	default:
		return p.sys.Enable(port)
	}
}

// Detach restores the previous settings.
func (p *Platform) Detach() error { return p.sys.Restore() }

// RestoreProxy is the crash-recovery/help-button path.
func (p *Platform) RestoreProxy() error { return p.sys.Restore() }

// SetAutostart adds or removes the Run-key entry.
func (p *Platform) SetAutostart(enabled bool) error {
	key, err := openRunKey()
	if err != nil {
		return err
	}
	return setAutostart(enabled, p.exePath, key)
}

// SetNotifier wires the toast notification function (phase 4 tray work).
func (p *Platform) SetNotifier(fn func(kind engine.NoticeKind, text string)) {
	p.mu.Lock()
	p.notify = fn
	p.mu.Unlock()
}

// Notify shows a Windows toast for failure and recovery (file 01 section 9:
// "via the tray library or a toast library"). go-toast drives the Windows
// toast API through SnoreToast; if that ever fails, the tray attention icon
// and the UI notice line still carry the message.
func (p *Platform) Notify(kind engine.NoticeKind, text string) {
	p.mu.Lock()
	fn := p.notify
	p.mu.Unlock()
	if fn != nil {
		fn(kind, text)
		return
	}
	n := toast.Notification{
		AppID:   config.AppName,
		Title:   config.AppName,
		Message: text,
	}
	if err := n.Push(); err != nil {
		// Never let a notification failure break the engine.
		_ = err
	}
}

// ConfigDir returns %APPDATA%\Younblock.
func (p *Platform) ConfigDir() string { return p.dir }
