//go:build windows

package windows

import (
        "fmt"
        "os"
        "path/filepath"
        "sync"
        "syscall"
        "unsafe"

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

// WM_SETTINGCHANGE("Internet Settings") broadcast: the message the
// Windows settings UI sends so running applications — including browsers
// that do not watch the registry themselves — re-read their proxy
// configuration immediately instead of after a restart (D46).
const (
        wmSettingChange = 0x001A
        smtoAbortIfHung = 0x0002
        hwndBroadcast   = 0xFFFF
)

var sendMessageTimeoutW = syswin.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

func broadcastSettingChange() {
        param, err := syscall.UTF16PtrFromString("Internet Settings")
        if err != nil {
                return
        }
        // Best effort; a hung window must not block us (SMTO_ABORTIFHUNG, 1 s).
        sendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0,
                uintptr(unsafe.Pointer(param)), smtoAbortIfHung, 1000, 0)
}

func refreshProxySettings() error {
        if err := internetSetOption(internetOptionSettingsChanged); err != nil {
                return err
        }
        if err := internetSetOption(internetOptionRefresh); err != nil {
                return err
        }
        broadcastSettingChange()
        return nil
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
        // A PAC URL a pre-1.3.0 version left behind without a backup is ours:
        // remove it so browsers follow the (restored) settings again.
        if err := p.sys.cleanupLegacy(); err != nil {
                log.Warn("legacy proxy-script cleanup failed", "reason", err.Error())
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
                Tray:           true,
                StartWithOS:    true,
                SystemProxy:    true,
                PerAppRouting:  false,
                BrowserSession: true,
        }
}

// DialControl returns nil on desktop.
func (p *Platform) DialControl() engine.ControlFunc { return nil }

// Attach applies the system proxy per the routing target and mode (file
// 01, section 8; D123). RoutingTargetChrome (the shipped default) routes
// only a dedicated Chrome instance the shell launches, so the system-wide
// proxy is NEVER touched — no other browser, no other Chrome profile and
// no system component sees this app's relay. RoutingTargetSystem applies
// the manual proxy values per Mode; ModeManual still touches nothing
// because the user chose to point a browser at the proxy by hand. The
// single decision lives in config.Settings.SystemProxyActive so the
// watchdog and the engine's live re-evaluate stay in lockstep.
func (p *Platform) Attach(port int, s config.Settings) error {
        if !s.SystemProxyActive() {
                return nil // chrome target, or manual mode: touch nothing in Windows
        }
        return p.sys.Enable(port)
}

// EnsureAttached re-asserts the system proxy when another program has
// overwritten it while we are on (the watchdog's anti-clobber check). It
// returns whether it had to rewrite something. A chrome-target run never
// set the proxy, so there is nothing to re-assert (D123); the watchdog's
// tick is a quiet no-op.
func (p *Platform) EnsureAttached(port int, s config.Settings) (bool, error) {
        if !s.SystemProxyActive() {
                return false, nil
        }
        return p.sys.ensure(port)
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
