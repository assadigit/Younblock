package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
)

// Compile-level guarantee that Platform satisfies the engine contract.
var _ engine.Platform = (*Platform)(nil)

// fakeGSettings answers gsettings calls from an in-memory table so the
// backup/restore state machine is tested without a desktop. It models the
// REAL printed forms verified against glib in this sandbox: strings quote
// themselves ('none'), ports print as plain integers (the schema keys are
// int32, not uint32), arrays use comma + single space.
type fakeGSettings struct {
	mu      sync.Mutex
	schemas []string          // what list-schemas prints (one per line)
	values  map[string]string // "schema key" -> raw GVariant literal
	sets    []string          // every set, as "schema key value", in order
	listErr error             // when set, list-schemas fails (missing binary)
}

func newFakeGSettings(schemas ...string) *fakeGSettings {
	return &fakeGSettings{schemas: schemas, values: map[string]string{}}
}

// seed fills the default values of one schema the way a fresh desktop
// has them, so every key the adapter reads exists.
func (f *fakeGSettings) seed(schema string) {
	f.set(schema, keyMode, "'none'")
	f.set(schema, keyIgnoreHosts, "['localhost', '127.0.0.0/8', '::1']")
	f.set(schema+".http", keyHost, "''")
	f.set(schema+".http", keyPort, "0")
	f.set(schema+".https", keyHost, "''")
	f.set(schema+".https", keyPort, "0")
}

// set records a write and applies it to the table.
func (f *fakeGSettings) set(schema, key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets = append(f.sets, schema+" "+key+" "+value)
	f.values[schema+" "+key] = value
}

// run is the cmdRunner handed to NewWithRunner.
func (f *fakeGSettings) run(argv ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case len(argv) == 1 && argv[0] == "list-schemas":
		if f.listErr != nil {
			return "", f.listErr
		}
		return strings.Join(f.schemas, "\n") + "\n", nil
	case len(argv) == 3 && argv[0] == "get":
		v, ok := f.values[argv[1]+" "+argv[2]]
		if !ok {
			// Real gsettings fails loudly on an unknown schema or key,
			// and the adapter must surface that (the engine maps an
			// Attach error to the honest PROXY_SETTING_FAILED state).
			return "", fmt.Errorf("unknown schema or key: %s %s", argv[1], argv[2])
		}
		return v + "\n", nil
	case len(argv) == 4 && argv[0] == "set":
		f.sets = append(f.sets, strings.Join(argv[1:], " "))
		f.values[argv[1]+" "+argv[2]] = argv[3]
		return "", nil
	}
	return "", fmt.Errorf("unexpected gsettings call: %v", argv)
}

// setCalls returns the recorded writes.
func (f *fakeGSettings) setCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sets...)
}

// value reads the current raw value of one key.
func (f *fakeGSettings) value(schema, key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values[schema+" "+key]
}

// newTestPlatform builds a platform around a fake gsettings with all
// three supported schemas installed (like a fully-loaded desktop), each
// seeded with fresh defaults, plus a temp home for the autostart file.
func newTestPlatform(t *testing.T) (*Platform, *fakeGSettings) {
	t.Helper()
	f := newFakeGSettings(
		// Unrelated schemas around ours prove the probe needs exact
		// line matches, not substring matches.
		"org.cinnamon.desktop.background",
		"org.cinnamon.system.proxy",
		"org.gnome.desktop.interface",
		"org.gnome.system.proxy",
		"org.gnome.system.proxy.http", // a child schema, not a parent
		"org.gtk.Settings.Debug",
		"org.mate.interface",
		"org.mate.system.proxy",
	)
	f.seed("org.gnome.system.proxy")
	f.seed("org.cinnamon.system.proxy")
	f.seed("org.mate.system.proxy")
	p := NewWithRunner(f.run, t.TempDir())
	return p, f
}

// youtubeSettings returns the default (proxy-managed) mode.
func youtubeSettings() config.Settings { return config.DefaultSettings() }

// manualSettings returns the leave-me-alone mode.
func manualSettings() config.Settings {
	s := config.DefaultSettings()
	s.Mode = config.ModeManual
	return s
}

// 1. Attach writes mode/host/port/ignore-hosts for every supported
// schema and captures the backup of the originals.
func TestAttachWritesManualValuesForEverySchema(t *testing.T) {
	p, f := newTestPlatform(t)
	// Give the user a previous life: an automatic proxy on GNOME, manual
	// leftovers on MATE.
	f.set("org.gnome.system.proxy", keyMode, "'auto'")
	f.set("org.mate.system.proxy.http", keyHost, "'old-proxy.example'")
	f.set("org.mate.system.proxy.http", keyPort, "3128")

	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	for _, schema := range []string{
		"org.gnome.system.proxy", "org.cinnamon.system.proxy", "org.mate.system.proxy",
	} {
		if got := f.value(schema, keyMode); got != "'manual'" {
			t.Errorf("%s mode = %q, want 'manual'", schema, got)
		}
		if got := f.value(schema, keyIgnoreHosts); got != ignoreHostsValue {
			t.Errorf("%s ignore-hosts = %q, want the bypass list", schema, got)
		}
		for _, sub := range []string{"http", "https"} {
			if got := f.value(schema+"."+sub, keyHost); got != "'127.0.0.1'" {
				t.Errorf("%s.%s host = %q, want '127.0.0.1'", schema, sub, got)
			}
			if got := f.value(schema+"."+sub, keyPort); got != "4525" {
				t.Errorf("%s.%s port = %q, want the plain integer gsettings prints", schema, sub, got)
			}
		}
	}
	// The backup captured the originals, not our values.
	if got := p.backup["org.gnome.system.proxy "+keyMode]; got != "'auto'" {
		t.Errorf("backup mode = %q, want the original 'auto'", got)
	}
	if got := p.backup["org.mate.system.proxy.http "+keyHost]; got != "'old-proxy.example'" {
		t.Errorf("backup http host = %q, want the original", got)
	}
	if got := p.backup["org.mate.system.proxy.http "+keyPort]; got != "3128" {
		t.Errorf("backup http port = %q, want the original", got)
	}
}

// 2. Attach with ModeManual touches nothing.
func TestAttachManualModeTouchesNothing(t *testing.T) {
	p, f := newTestPlatform(t)
	before := len(f.setCalls())
	if err := p.Attach(4525, manualSettings()); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if got := len(f.setCalls()); got != before {
		t.Errorf("manual mode must not touch gsettings; %d extra writes: %v", got-before, f.setCalls())
	}
	if p.backup != nil {
		t.Error("manual mode must not capture a backup")
	}
	changed, err := p.EnsureAttached(4525, manualSettings())
	if err != nil || changed {
		t.Errorf("EnsureAttached(manual) = %v, %v; want false, nil", changed, err)
	}
	if err := p.Detach(); err != nil {
		t.Fatalf("Detach(manual) = %v, want nil", err)
	}
	if got := len(f.setCalls()); got != before {
		t.Errorf("a manual session must never write gsettings; %d extra writes", got-before)
	}
}

// 3. Detach restores the exact original raw values.
func TestDetachRestoresExactOriginalRawValues(t *testing.T) {
	p, f := newTestPlatform(t)
	originals := map[string]string{
		"org.gnome.system.proxy " + keyMode:        "'auto'",
		"org.gnome.system.proxy " + keyIgnoreHosts: "['localhost', '127.0.0.0/8', '::1']",
		"org.gnome.system.proxy.http " + keyHost:   "'old-proxy.example'",
		"org.gnome.system.proxy.http " + keyPort:   "3128",
		"org.gnome.system.proxy.https " + keyHost:  "''",
		"org.gnome.system.proxy.https " + keyPort:  "0",
	}
	for k, v := range originals {
		f.values[k] = v // a user with their own automatic proxy
	}
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	if err := p.Detach(); err != nil {
		t.Fatal(err)
	}
	for k, want := range originals {
		if got := f.values[k]; got != want {
			t.Errorf("%s = %q, want the original %q back verbatim", k, got, want)
		}
	}
	if p.backup != nil {
		t.Error("Detach must clear the backup")
	}
}

// 4. Detach with no backup sets mode 'none' and clears host/port — but
// only when the values are recognizably ours (a crashed run's
// leftovers carry our ignore-hosts list too).
func TestDetachWithoutBackupClearsOurValues(t *testing.T) {
	p, f := newTestPlatform(t)
	// A crashed previous run left our manual values behind; this process
	// never attached, so it has no backup.
	for _, schema := range supportedProxySchemas {
		f.set(schema, keyMode, "'manual'")
		f.set(schema, keyIgnoreHosts, ignoreHostsValue)
		f.set(schema+".http", keyHost, "'127.0.0.1'")
		f.set(schema+".http", keyPort, "4525")
		f.set(schema+".https", keyHost, "'127.0.0.1'")
		f.set(schema+".https", keyPort, "4525")
	}
	if err := p.Detach(); err != nil {
		t.Fatal(err)
	}
	for _, schema := range supportedProxySchemas {
		if got := f.value(schema, keyMode); got != "'none'" {
			t.Errorf("%s mode = %q, want 'none'", schema, got)
		}
		for _, sub := range []string{"http", "https"} {
			if got := f.value(schema+"."+sub, keyHost); got != "''" {
				t.Errorf("%s.%s host = %q, want ''", schema, sub, got)
			}
			if got := f.value(schema+"."+sub, keyPort); got != "0" {
				t.Errorf("%s.%s port = %q, want 0", schema, sub, got)
			}
		}
	}
}

// Without a backup, values that are NOT recognizably ours must be left
// alone: a manual-mode user's own proxy survives Stop (the engine calls
// Detach whatever the mode is), and so does another tool's proxy.
func TestDetachWithoutBackupLeavesForeignValuesAlone(t *testing.T) {
	p, f := newTestPlatform(t)
	// A user's hand-made manual proxy: our host, but their own port and
	// the default bypass list.
	f.set("org.gnome.system.proxy", keyMode, "'manual'")
	f.set("org.gnome.system.proxy.http", keyHost, "'127.0.0.1'")
	f.set("org.gnome.system.proxy.http", keyPort, "8080")
	// Another tool's proxy on MATE: our bypass list, but its own host.
	f.set("org.mate.system.proxy", keyMode, "'manual'")
	f.set("org.mate.system.proxy", keyIgnoreHosts, ignoreHostsValue)
	f.set("org.mate.system.proxy.http", keyHost, "'corporate-proxy.example'")
	f.set("org.mate.system.proxy.http", keyPort, "3128")

	setsBefore := len(f.setCalls())
	if err := p.Detach(); err != nil {
		t.Fatal(err)
	}
	if got := f.value("org.gnome.system.proxy", keyMode); got != "'manual'" {
		t.Errorf("a user's own manual proxy was wiped: mode = %q", got)
	}
	if got := f.value("org.gnome.system.proxy.http", keyPort); got != "8080" {
		t.Errorf("a user's own port was wiped: %q", got)
	}
	if got := f.value("org.mate.system.proxy.http", keyHost); got != "'corporate-proxy.example'" {
		t.Errorf("another tool's proxy was wiped: %q", got)
	}
	if got := len(f.setCalls()) - setsBefore; got != 0 {
		t.Errorf("nothing should have been written, %d writes: %v", got, f.setCalls())
	}
}

// 5. EnsureAttached returns false + no writes when the values are intact,
// and true + a rewrite when a value was clobbered.
func TestEnsureAttachedNoopWhenIntact(t *testing.T) {
	p, f := newTestPlatform(t)
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	setsBefore := len(f.setCalls())
	changed, err := p.EnsureAttached(4525, youtubeSettings())
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("EnsureAttached must report no change when our values are intact")
	}
	if got := len(f.setCalls()); got != setsBefore {
		t.Errorf("EnsureAttached wrote %d values; want none", got-setsBefore)
	}
}

func TestEnsureAttachedRewritesAfterClobber(t *testing.T) {
	p, f := newTestPlatform(t)
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	// Another program points the GNOME http proxy elsewhere.
	f.set("org.gnome.system.proxy.http", keyPort, "8080")
	setsBefore := len(f.setCalls())
	changed, err := p.EnsureAttached(4525, youtubeSettings())
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("EnsureAttached must report the rewrite after a clobber")
	}
	if got := f.value("org.gnome.system.proxy.http", keyPort); got != "4525" {
		t.Errorf("clobbered port = %q, want ours re-applied", got)
	}
	for _, schema := range supportedProxySchemas {
		if got := f.value(schema, keyMode); got != "'manual'" {
			t.Errorf("%s mode = %q, want 'manual' everywhere again", schema, got)
		}
	}
	if got := len(f.setCalls()) - setsBefore; got == 0 {
		t.Error("the rewrite must actually write")
	}
	// The backup was NOT re-taken: Detach still restores the original
	// (seeded 'none'), not the clobbered or manual state.
	if err := p.Detach(); err != nil {
		t.Fatal(err)
	}
	if got := f.value("org.gnome.system.proxy", keyMode); got != "'none'" {
		t.Errorf("Detach after clobber = %q, want the pre-Attach original 'none'", got)
	}
}

// 6. RestoreProxy behaves like Detach.
func TestRestoreProxyBehavesLikeDetach(t *testing.T) {
	p, f := newTestPlatform(t)
	f.set("org.gnome.system.proxy", keyMode, "'auto'")
	f.set("org.gnome.system.proxy.http", keyPort, "3128")
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	if err := p.RestoreProxy(); err != nil {
		t.Fatal(err)
	}
	if got := f.value("org.gnome.system.proxy", keyMode); got != "'auto'" {
		t.Errorf("mode = %q, want the original 'auto'", got)
	}
	if got := f.value("org.gnome.system.proxy.http", keyPort); got != "3128" {
		t.Errorf("port = %q, want the original 3128", got)
	}
	if p.backup != nil {
		t.Error("RestoreProxy must clear the backup like Detach")
	}
}

// 7. SetAutostart true writes the .desktop file; false removes it.
func TestSetAutostartWritesAndRemovesDesktopFile(t *testing.T) {
	f := newFakeGSettings("org.gnome.system.proxy")
	home := t.TempDir()
	p := NewWithRunner(f.run, home)

	if err := p.SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "autostart", "younblock.desktop")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("desktop file: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Younblock",
		"Comment=Unblock YouTube",
		"--web",
		"Terminal=false",
		"X-GNOME-Autostart-enabled=true",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("desktop file missing %q:\n%s", want, content)
		}
	}
	// The entry names this very executable (here: the test binary).
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Exec="+exe+" --web") {
		t.Errorf("Exec line does not name the current executable:\n%s", content)
	}
	if info, err := os.Stat(path); err == nil && info.Mode().Perm() != 0o644 {
		t.Errorf("desktop file mode = %v, want 0644", info.Mode().Perm())
	}
	// Enabling again overwrites instead of appending.
	if err := p.SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	if strings.Count(string(data2), "[Desktop Entry]") != 1 {
		t.Error("enabling twice must overwrite, not append")
	}
	// Disabling removes the file; disabling again is a harmless no-op.
	if err := p.SetAutostart(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("disabling must remove the desktop file")
	}
	if err := p.SetAutostart(false); err != nil {
		t.Fatalf("second disable: %v", err)
	}
}

// 8. Capabilities: with schemas present SystemProxy is true; with the
// gsettings binary missing it is false while StartWithOS stays true; with
// no supported schema at all Attach is a no-op success.
func TestCapabilitiesWithSchemas(t *testing.T) {
	p, _ := newTestPlatform(t)
	caps := p.Capabilities()
	if !caps.SystemProxy {
		t.Error("SystemProxy must be true when gsettings lists org.gnome.system.proxy")
	}
	if !caps.StartWithOS {
		t.Error("StartWithOS must be true (the autostart entry needs no gsettings)")
	}
	if caps.Tray {
		t.Error("the Linux edition runs headless with the web UI; no tray")
	}
	if caps.PerAppRouting {
		t.Error("PerAppRouting is not supported")
	}
}

func TestCapabilitiesWithoutGSettings(t *testing.T) {
	f := newFakeGSettings()
	f.listErr = errors.New(`exec: "gsettings": executable file not found in $PATH`)
	p := NewWithRunner(f.run, t.TempDir())
	caps := p.Capabilities()
	if caps.SystemProxy {
		t.Error("SystemProxy must be false when gsettings is not installed")
	}
	if !caps.StartWithOS {
		t.Error("StartWithOS must stay true; only the system proxy is gone")
	}
}

func TestAttachWithoutSupportedSchemasIsNoopSuccess(t *testing.T) {
	// gsettings runs, but no supported proxy schema is installed (a bare
	// window manager, or a headless CI box): Attach must still succeed.
	f := newFakeGSettings("org.gtk.Settings.Debug", "org.xfce.desktop.background")
	p := NewWithRunner(f.run, t.TempDir())
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatalf("Attach without supported schemas: %v", err)
	}
	if got := f.setCalls(); len(got) != 0 {
		t.Errorf("Attach must not write anything, wrote %v", got)
	}
	if err := p.Detach(); err != nil {
		t.Fatalf("Detach without supported schemas: %v", err)
	}
	if p.Capabilities().SystemProxy {
		t.Error("SystemProxy must be false without the proxy schema")
	}
}

// 9. Notify: the notify runner receives the text (recorded and asserted).
func TestNotifyPassesTextToNotifySend(t *testing.T) {
	f := newFakeGSettings("org.gnome.system.proxy")
	p := NewWithRunner(f.run, t.TempDir())
	p.Notify(engine.NoticeFailure, "the connection failed")
	p.Notify(engine.NoticeRecovery, "it is back")

	calls := p.NotifyCalls()
	if len(calls) != 2 {
		t.Fatalf("notify calls = %d, want 2", len(calls))
	}
	// -a names the app, -t the duration; failures are marked critical so
	// they survive a busy notification queue, recoveries are not.
	wantFailure := []string{"-a", config.AppName, "-t", "10000", "-u", "critical", "the connection failed"}
	if !sameArgv(calls[0], wantFailure) {
		t.Errorf("failure call = %v, want %v", calls[0], wantFailure)
	}
	wantRecovery := []string{"-a", config.AppName, "-t", "10000", "it is back"}
	if !sameArgv(calls[1], wantRecovery) {
		t.Errorf("recovery call = %v, want %v", calls[1], wantRecovery)
	}
}

func sameArgv(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// A gsettings failure during Attach reaches the caller (the engine maps
// it to the honest PROXY_SETTING_FAILED state).
func TestAttachErrorPropagates(t *testing.T) {
	// The schema is listed but no values exist, so the backup capture's
	// first read fails.
	f := newFakeGSettings("org.gnome.system.proxy")
	p := NewWithRunner(f.run, t.TempDir())
	if err := p.Attach(4525, youtubeSettings()); err == nil {
		t.Fatal("Attach must fail when gsettings cannot read the current values")
	}
	if p.backup != nil {
		t.Error("a failed Attach must not leave a half backup behind")
	}
}

// A restart on another port keeps the ORIGINAL backup (mirrors the
// Windows double-Enable rule).
func TestDoubleAttachKeepsOriginalBackup(t *testing.T) {
	p, f := newTestPlatform(t)
	f.set("org.gnome.system.proxy", keyMode, "'auto'")
	if err := p.Attach(4525, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	if err := p.Attach(4530, youtubeSettings()); err != nil {
		t.Fatal(err)
	}
	if got := p.backup["org.gnome.system.proxy "+keyMode]; got != "'auto'" {
		t.Errorf("backup mode = %q, want the original 'auto' kept", got)
	}
	if got := f.value("org.gnome.system.proxy.http", keyPort); got != "4530" {
		t.Errorf("port = %q, want the new port", got)
	}
	if err := p.Detach(); err != nil {
		t.Fatal(err)
	}
	if got := f.value("org.gnome.system.proxy", keyMode); got != "'auto'" {
		t.Errorf("mode = %q, want the original back after Detach", got)
	}
}

// The route-all mode goes through the same system-proxy path.
func TestAttachModeAllAlsoSetsProxy(t *testing.T) {
	p, f := newTestPlatform(t)
	s := config.DefaultSettings()
	s.Mode = config.ModeAll
	if err := p.Attach(4599, s); err != nil {
		t.Fatal(err)
	}
	if got := f.value("org.gnome.system.proxy.http", keyPort); got != "4599" {
		t.Errorf("port = %q, want 4599", got)
	}
}
