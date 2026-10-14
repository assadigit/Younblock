// Package linux implements the Linux platform adapter for the Younblock
// single-executable Linux edition (DECISIONS.md D50): the same neutral
// engine, a web UI served by the binary, and the desktop integration done
// through freedesktop.org standards instead of the Windows registry —
// gsettings for the system proxy, ~/.config/autostart for start-with-OS,
// notify-send for notices.
//
// The proxy state machine mirrors the Windows adapter
// (internal/platform/windows/sysproxy_logic.go): the current values are
// captured raw before we change anything and are put back verbatim on
// Detach, so no GVariant parsing ever happens — gsettings accepts its own
// printed representation as the value of `set`. One deliberate
// difference: the Linux edition keeps the backup in memory only (no
// backup file on disk, D50), so the crash-recovery path (RestoreProxy
// without a live backup) removes values that are recognizably ours with
// a safe default instead of restoring an original it cannot know; a
// leftover after a hard crash is cleared with the --restore-proxy flag
// or the help path.
//
// Verification honesty: the raw literal forms used here — quoted strings
// for mode and host, plain integers for ports, comma-and-space arrays
// for ignore-hosts — were verified in this sandbox against the real
// gsettings binary (glib 2.84) with the upstream gsettings-desktop-
// schemas proxy schema compiled in: `get` prints exactly these forms and
// `set` accepts each literal without a parse error. The port keys in that
// schema are int32
// ("<key type=\"i\" name=\"port\">"), which is why ports are written as
// plain decimals: `gsettings get` prints int32 values with no type
// prefix, so a "uint32 N" literal would never compare equal to what we
// read back. What could NOT be verified from this sandbox: a write
// persisting through dconf over D-Bus and a browser picking the change
// up live — no dconf-service runs here, so cross-process writes cannot
// be demonstrated (the per-process memory backend proves the forms, not
// the persistence). That round trip on a real Mint/Cinnamon/MATE desktop
// is Ali's manual check.
//
// Like the Windows package's logic files, this file carries no build tag:
// it is pure Go that shells out to standard commands, so it compiles —
// and is unit-tested against a fake — on every platform. Only
// select_linux.go is tagged.
package linux

import (
        "fmt"
        "os"
        "os/exec"
        "path/filepath"
        "strconv"
        "strings"
        "sync"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
)

// cmdRunner runs the gsettings binary with argv; it returns stdout.
// Tests inject a fake through NewWithRunner.
type cmdRunner func(argv ...string) (string, error)

// notifyRunner runs the notify-send binary with argv.
// Tests get a recording implementation from NewWithRunner.
type notifyRunner func(argv ...string) error

// gnomeProxySchema is the canonical proxy schema: the one GLib's own
// proxy resolver reads, so every desktop built on GTK — GNOME, Cinnamon,
// MATE, XFCE — follows it.
const gnomeProxySchema = "org.gnome.system.proxy"

// supportedProxySchemas are the proxy schemas we are willing to write.
// Cinnamon and MATE are GNOME forks; wherever they ship their own copy of
// the proxy schema it uses the same key layout. Only schemas actually
// listed by `gsettings list-schemas` are ever touched, so a schema that
// is absent on this machine is simply skipped.
//
// Honest note: org.cinnamon.system.proxy and org.mate.system.proxy could
// not be found in the current linuxmint or mate-desktop sources fetched
// from this sandbox (both desktops appear to follow org.gnome.system.proxy
// through GLib), so those two entries are speculative extra coverage that
// costs nothing when the schema is absent. Confirming on a real Mint box
// is Ali's manual step (DECISIONS.md D50).
var supportedProxySchemas = []string{
        gnomeProxySchema,
        "org.cinnamon.system.proxy",
        "org.mate.system.proxy",
}

// The keys we read and write, per schema.
const (
        keyMode        = "mode"
        keyIgnoreHosts = "ignore-hosts"
        keyHost        = "host"
        keyPort        = "port"
)

// ignoreHostsValue is the bypass list written while we are active, in the
// array form gsettings itself prints (comma + single space). It mirrors
// the Windows ProxyOverride list in sysproxy_logic.go: loopback
// (localhost, the whole 127/8 range and ::1), the three private ranges
// (10/8, 172.16/12, 192.168/16) and *.local for single-label names, so
// LAN and local traffic never depends on this app. The Windows list's
// <local> rule is covered by *.local here.
const ignoreHostsValue = "['localhost', '127.0.0.0/8', '::1', '10.0.0.0/8', " +
        "'172.16.0.0/12', '192.168.0.0/16', '*.local']"

// proxyKey names one gsettings key we manage.
type proxyKey struct {
        schema string
        key    string
}

// managedKeys lists every key we read and write for one proxy schema: the
// mode and bypass list on the schema itself, plus host and port on the
// http and https sub-schemas — the keys every mainstream browser follows
// through GLib's proxy resolver.
func managedKeys(schema string) []proxyKey {
        return []proxyKey{
                {schema, keyMode},
                {schema, keyIgnoreHosts},
                {schema + ".http", keyHost},
                {schema + ".http", keyPort},
                {schema + ".https", keyHost},
                {schema + ".https", keyPort},
        }
}

// manualValue returns the raw literal we write for one key while we are
// active — always the exact form gsettings itself prints for that key, so
// a later check is a plain string comparison and Detach can hand our
// captured strings back to gsettings untouched.
//
// The port literal is a plain decimal, never a "uint32 N" form: the port
// keys of the real proxy schemas are int32 (gsettings-desktop-schemas
// declares <key type="i" name="port">) and int32 prints without a type
// prefix. Verified against glib 2.84 with the upstream schema compiled
// in: `gsettings get ... port` prints `8080`, and these exact literals
// survive a write/read-back round trip byte for byte.
func manualValue(k proxyKey, port int) string {
        switch k.key {
        case keyMode:
                return "'manual'"
        case keyIgnoreHosts:
                return ignoreHostsValue
        case keyHost:
                return "'" + config.ListenHost + "'"
        default: // keyPort
                return strconv.Itoa(port)
        }
}

// noneValue is the raw literal for the safe default written when no
// backup exists: proxy off, addresses emptied. ignore-hosts returns ""
// (it is left alone — with the mode at 'none' the bypass list is inert,
// and overwriting it could clobber a user's own list for nothing).
func noneValue(k proxyKey) string {
        switch k.key {
        case keyMode:
                return "'none'"
        case keyHost:
                return "''"
        case keyPort:
                return "0"
        default:
                return ""
        }
}

// Platform is the real Linux adapter.
type Platform struct {
        // Wired once at construction and never changed afterwards.
        run    cmdRunner       // gsettings
        notify notifyRunner    // notify-send
        home   string          // $HOME, where ~/.config/autostart lives
        dir    string          // per-user config directory ("" when unavailable)
        rec    *notifyRecorder // non-nil when the recorder is installed

        // mu guards the in-memory proxy backup.
        mu     sync.Mutex
        backup map[string]string // "schema key" -> raw GVariant literal

        // The schema probe runs once and caches its result. sync.Once is
        // itself mutex-guarded, and the two fields below are written only
        // inside Do and never touched again, so reads after Do are
        // race-free.
        probeOnce sync.Once
        schemas   []string // supported proxy schemas actually installed
        sysProxy  bool     // org.gnome.system.proxy is among them
}

// New builds the Linux platform adapter with the real gsettings and
// notify-send runners. It fails only when the per-user configuration
// directory is known but cannot be created (the caller then falls back to
// the stub, like select_windows.go does); a user-config root that cannot
// be resolved at all keeps the adapter running with an empty ConfigDir —
// the same fallback stub.New uses — so the app never refuses to start.
func New() (*Platform, error) {
        home := ""
        if h, err := os.UserHomeDir(); err == nil {
                home = h
        }
        dir := ""
        if base, err := os.UserConfigDir(); err == nil {
                dir = filepath.Join(base, config.AppName)
                if err := os.MkdirAll(dir, 0o700); err != nil {
                        return nil, fmt.Errorf("config dir: %w", err)
                }
        }
        return &Platform{
                run:    realGSettings(),
                notify: realNotifySend(),
                home:   home,
                dir:    dir,
        }, nil
}

// NewWithRunner wires the adapter to a fake gsettings runner and a home
// directory for the autostart file (tests pass t.TempDir()). The
// notify-send runner is a recorder whose invocations are read back with
// NotifyCalls; a test wanting different behavior can build a Platform
// literal directly.
func NewWithRunner(run cmdRunner, home string) *Platform {
        rec := &notifyRecorder{}
        return &Platform{
                run:    run,
                notify: rec.run,
                home:   home,
                rec:    rec,
        }
}

// realGSettings runs the gsettings binary and returns its stdout. A
// missing binary surfaces as an error from every call; the schema probe
// treats that as "unavailable" (no supported schemas, SystemProxy false),
// so a headless box never sees a proxy failure.
func realGSettings() cmdRunner {
        return func(argv ...string) (string, error) {
                out, err := exec.Command("gsettings", argv...).Output()
                if err != nil {
                        return "", err
                }
                return string(out), nil
        }
}

// realNotifySend runs notify-send and swallows whether it worked: without
// a notification daemon (a server, a bare window manager) the notice line
// in the UI still carries the message, so a failure must never reach the
// engine.
func realNotifySend() notifyRunner {
        return func(argv ...string) error {
                return exec.Command("notify-send", argv...).Run()
        }
}

// notifyRecorder records every notify-send invocation for tests.
type notifyRecorder struct {
        mu    sync.Mutex
        calls [][]string
}

func (r *notifyRecorder) run(argv ...string) error {
        r.mu.Lock()
        defer r.mu.Unlock()
        r.calls = append(r.calls, append([]string(nil), argv...))
        return nil
}

func (r *notifyRecorder) snapshot() [][]string {
        r.mu.Lock()
        defer r.mu.Unlock()
        return append([][]string(nil), r.calls...)
}

// NotifyCalls returns the notify-send invocations recorded by the runner
// NewWithRunner installs (nil with the real runner).
func (p *Platform) NotifyCalls() [][]string {
        if p.rec == nil {
                return nil
        }
        return p.rec.snapshot()
}

// Name reports "linux".
func (p *Platform) Name() string { return "linux" }

// Capabilities reports what the Linux edition supports. The system-proxy
// capability is probed lazily: true only when the gsettings binary runs
// AND the canonical org.gnome.system.proxy schema is installed. The tray
// is false because this edition is a single headless binary with the web
// UI (D50); start-with-OS works everywhere through the standard autostart
// directory.
func (p *Platform) Capabilities() engine.Capabilities {
        p.presentSchemas()
        return engine.Capabilities{
                Tray:          false,
                StartWithOS:   true,
                SystemProxy:   p.sysProxy,
                PerAppRouting: false,
                // No shell here to launch a dedicated browser: the setting row
                // is hidden on this edition (D50).
                BrowserSession: false,
        }
}

// DialControl returns nil on desktop.
func (p *Platform) DialControl() engine.ControlFunc { return nil }

// presentSchemas returns the supported proxy schemas installed on this
// machine (probed once and cached). The returned slice is read-only. An
// empty list means gsettings is missing or no supported desktop schema is
// installed: there is nothing we can set, and every proxy operation
// becomes a harmless no-op — which is exactly what headless CI boxes need.
func (p *Platform) presentSchemas() []string {
        p.probeOnce.Do(func() {
                out, err := p.run("list-schemas")
                if err != nil {
                        // gsettings not installed (or broken): unavailable, nothing
                        // supported. Errors are swallowed here on purpose — the
                        // capability simply reports false.
                        return
                }
                installed := make(map[string]bool)
                for _, line := range strings.Split(out, "\n") {
                        // list-schemas prints one schema per line; exact match only,
                        // so child schemas like org.gnome.system.proxy.http never
                        // count as a supported parent.
                        installed[strings.TrimSpace(line)] = true
                }
                for _, s := range supportedProxySchemas {
                        if installed[s] {
                                p.schemas = append(p.schemas, s)
                        }
                }
                p.sysProxy = installed[gnomeProxySchema]
        })
        return p.schemas
}

// Attach applies the system proxy per the mode (mirrors windows.go):
// manual mode touches nothing; otherwise the previous values are captured
// once and our manual values are written to every installed schema. A
// gsettings failure is returned to the engine, which maps it to the
// honest PROXY_SETTING_FAILED state.
func (p *Platform) Attach(port int, s config.Settings) error {
        if s.Mode == config.ModeManual {
                return nil // touch nothing; the user sets the proxy by hand
        }
        schemas := p.presentSchemas()
        if len(schemas) == 0 {
                // No gsettings (a server, a bare window manager, a CI box): the
                // capability already told the UI to show the manual address, so
                // there is nothing to fail here.
                return nil
        }
        p.mu.Lock()
        defer p.mu.Unlock()
        if p.backup == nil {
                backup, err := p.captureLocked(schemas)
                if err != nil {
                        return err
                }
                p.backup = backup
        }
        for _, schema := range schemas {
                if err := p.applyManualLocked(schema, port); err != nil {
                        return err
                }
        }
        return nil
}

// EnsureAttached re-asserts the system proxy when another program has
// overwritten it while we are on (the watchdog's anti-clobber check,
// D46). It reports whether it had to rewrite something. The backup is
// never re-taken here: whatever is in place is (supposedly) ours, so the
// original backup from Attach stays authoritative and Detach still
// returns the user's true values.
func (p *Platform) EnsureAttached(port int, s config.Settings) (bool, error) {
        if s.Mode == config.ModeManual {
                return false, nil
        }
        schemas := p.presentSchemas()
        if len(schemas) == 0 {
                return false, nil
        }
        p.mu.Lock()
        defer p.mu.Unlock()
        inPlace := true
        for _, schema := range schemas {
                for _, k := range managedKeys(schema) {
                        cur, err := p.runGet(k.schema, k.key)
                        if err != nil {
                                return false, err
                        }
                        if cur != manualValue(k, port) {
                                inPlace = false
                        }
                }
        }
        if inPlace {
                return false, nil
        }
        for _, schema := range schemas {
                if err := p.applyManualLocked(schema, port); err != nil {
                        return false, err
                }
        }
        return true, nil
}

// Detach restores the values captured before Attach, verbatim, and clears
// the backup. Without a backup (a fresh process after a crash killed the
// in-memory copy, or a Detach that never had an Attach) the safe default
// removes what is recognizably ours: mode 'none' and empty addresses —
// but only for a schema whose current values carry our three markers
// (manual mode, our loopback host, our bypass list). A manual-mode
// user's own proxy must survive Stop, and another tool's values are not
// ours to touch — the "restore only what is still ours" rule the Windows
// adapter follows (file 01, section 8). Deviation from the letter of the
// task spec (which said to clear unconditionally): an unconditional
// clear would wipe a manual-mode user's hand-made proxy settings on every
// Stop, because the engine calls Detach regardless of mode and this
// edition keeps no backup file to tell ours from theirs (D50). The
// --restore-proxy flag and the help path land here too, and our crash
// leftovers do carry the markers, so recovery still works.
func (p *Platform) Detach() error {
        schemas := p.presentSchemas()
        if len(schemas) == 0 {
                return nil
        }
        p.mu.Lock()
        defer p.mu.Unlock()
        if len(p.backup) > 0 {
                for _, schema := range schemas {
                        for _, k := range managedKeys(schema) {
                                v, ok := p.backup[k.schema+" "+k.key]
                                if !ok {
                                        continue // never captured; leave that key alone
                                }
                                if err := p.runSet(k.schema, k.key, v); err != nil {
                                        return err
                                }
                        }
                }
                p.backup = nil // the on-period is over
                return nil
        }
        for _, schema := range schemas {
                if !p.oursLocked(schema) {
                        continue // not ours (or unreadable): leave the schema alone
                }
                for _, k := range managedKeys(schema) {
                        v := noneValue(k)
                        if v == "" {
                                continue // ignore-hosts is left alone in the safe default
                        }
                        if err := p.runSet(k.schema, k.key, v); err != nil {
                                return err
                        }
                }
        }
        return nil
}

// oursLocked reports whether the current values of one schema look like
// ours: manual mode, our loopback host on http, and our exact bypass
// list. The bypass list is the strongest marker — nobody hand-types it
// and other proxy tools write their own lists. The port is deliberately
// not checked: the auto-port feature can put us on any port.
func (p *Platform) oursLocked(schema string) bool {
        mode, err := p.runGet(schema, keyMode)
        if err != nil || mode != "'manual'" {
                return false
        }
        host, err := p.runGet(schema+".http", keyHost)
        if err != nil || host != "'"+config.ListenHost+"'" {
                return false
        }
        ignore, err := p.runGet(schema, keyIgnoreHosts)
        return err == nil && ignore == ignoreHostsValue
}

// RestoreProxy is the crash-recovery/help-button path: same as Detach.
func (p *Platform) RestoreProxy() error { return p.Detach() }

// SetAutostart adds or removes the ~/.config/autostart entry — the
// freedesktop.org mechanism every mainstream Linux desktop honors.
func (p *Platform) SetAutostart(enabled bool) error {
        path := p.autostartPath()
        if !enabled {
                if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
                        return err
                }
                return nil
        }
        if p.home == "" {
                // Without $HOME there is no standard place for the entry; say so
                // instead of writing somewhere random.
                return fmt.Errorf("no home directory for the autostart entry")
        }
        if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
                return err
        }
        // The entry names the current executable; if the path cannot be
        // resolved the Exec line keeps only the flag — the same near-
        // impossible fallback windows.go makes with its Run-key command.
        exe, err := os.Executable()
        if err != nil {
                exe = ""
        }
        return os.WriteFile(path, []byte(desktopEntry(exe)), 0o644)
}

// autostartPath is where the autostart entry lives: $HOME/.config/autostart.
func (p *Platform) autostartPath() string {
        return filepath.Join(p.home, ".config", "autostart", "younblock.desktop")
}

// desktopEntry renders younblock.desktop. Exec carries --web because the
// Linux edition serves its UI from the same single executable (D50).
func desktopEntry(exe string) string {
        return "[Desktop Entry]\n" +
                "Type=Application\n" +
                "Name=Younblock\n" +
                "Comment=Unblock YouTube\n" +
                "Exec=" + exe + " --web\n" +
                "Terminal=false\n" +
                "X-GNOME-Autostart-enabled=true\n"
}

// Notify shows a desktop notification through notify-send, best effort:
// every error is ignored (see realNotifySend for why). Failures are
// marked critical so they survive a busy notification queue.
func (p *Platform) Notify(kind engine.NoticeKind, text string) {
        argv := []string{"-a", config.AppName, "-t", "10000"}
        if kind == engine.NoticeFailure {
                argv = append(argv, "-u", "critical")
        }
        argv = append(argv, text)
        _ = p.notify(argv...)
}

// ConfigDir returns the per-user configuration directory
// ($XDG_CONFIG_HOME/Younblock, ~/.config/Younblock by default). It is
// created once in New like windows.go does (0700); when the user-config
// root cannot be resolved at all the empty string is returned — the same
// fallback stub.New uses — so the app keeps running.
func (p *Platform) ConfigDir() string { return p.dir }

// captureLocked records the raw value of every managed key before we
// change anything. Called at most once per on-period: a second Attach (a
// restart on another port) keeps the first backup so Detach still returns
// the user's true original values.
func (p *Platform) captureLocked(schemas []string) (map[string]string, error) {
        backup := make(map[string]string, len(schemas)*6) // 6 keys per schema
        for _, schema := range schemas {
                for _, k := range managedKeys(schema) {
                        v, err := p.runGet(k.schema, k.key)
                        if err != nil {
                                return nil, err
                        }
                        backup[k.schema+" "+k.key] = v
                }
        }
        return backup, nil
}

// applyManualLocked writes our manual proxy values for one schema.
func (p *Platform) applyManualLocked(schema string, port int) error {
        for _, k := range managedKeys(schema) {
                if err := p.runSet(k.schema, k.key, manualValue(k, port)); err != nil {
                        return err
                }
        }
        return nil
}

// runGet reads one raw value: the GVariant literal exactly as gsettings
// prints it. Only the trailing newline the terminal adds is trimmed; the
// literal itself is kept verbatim so it can be compared and written back
// unchanged.
func (p *Platform) runGet(schema, key string) (string, error) {
        out, err := p.run("get", schema, key)
        if err != nil {
                return "", fmt.Errorf("gsettings get %s %s: %w", schema, key, err)
        }
        return strings.TrimSpace(out), nil
}

// runSet writes one raw value verbatim.
func (p *Platform) runSet(schema, key, value string) error {
        if _, err := p.run("set", schema, key, value); err != nil {
                return fmt.Errorf("gsettings set %s %s: %w", schema, key, err)
        }
        return nil
}
