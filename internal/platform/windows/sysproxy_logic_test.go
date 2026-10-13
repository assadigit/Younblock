package windows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeRegistry is an in-memory internetSettings implementation.
type fakeRegistry struct {
	mu     sync.Mutex
	strs   map[string]string
	dwords map[string]uint32
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{strs: map[string]string{}, dwords: map[string]uint32{}}
}

func (f *fakeRegistry) readString(name string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.strs[name]
	return v, ok, nil
}

func (f *fakeRegistry) readDWORD(name string) (uint32, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.dwords[name]
	return v, ok, nil
}

func (f *fakeRegistry) writeString(name, val string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.strs[name] = val
	return nil
}

func (f *fakeRegistry) writeDWORD(name string, val uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dwords[name] = val
	return nil
}

func (f *fakeRegistry) deleteValue(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.strs, name)
	delete(f.dwords, name)
	return nil
}

// testLogger collects log lines.
type testLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *testLogger) Info(msg string, args ...any)  { l.add("INFO", msg, args...) }
func (l *testLogger) Warn(msg string, args ...any)  { l.add("WARN", msg, args...) }
func (l *testLogger) Error(msg string, args ...any) { l.add("ERROR", msg, args...) }

func (l *testLogger) add(level, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	line := level + " " + msg
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			line += " " + k + "=" + sprint(args[i+1])
		}
	}
	l.lines = append(l.lines, line)
}

func (l *testLogger) has(sub string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.Contains(line, sub) {
			return true
		}
	}
	return false
}

func sprint(v any) string {
	type stringer interface{ String() string }
	if s, ok := v.(stringer); ok {
		return s.String()
	}
	return ""
}

// newTestSysproxy wires a sysproxy to fakes.
func newTestSysproxy(t *testing.T) (*sysproxy, *fakeRegistry, *testLogger, *int) {
	t.Helper()
	dir := t.TempDir()
	reg := newFakeRegistry()
	log := &testLogger{}
	refreshes := 0
	refresh := func() error { refreshes++; return nil }
	s := newSysproxy(dir, func() (internetSettings, error) { return reg, nil }, refresh, log)
	t.Cleanup(func() { s.Restore() })
	return s, reg, log, &refreshes
}

func readBackup(t *testing.T, s *sysproxy) *backupFile {
	t.Helper()
	data, err := os.ReadFile(s.backupPath())
	if err != nil {
		return nil
	}
	var b backupFile
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("backup file: %v", err)
	}
	return &b
}

func TestEnableSetsPACAndBacksUp(t *testing.T) {
	s, reg, _, refreshes := newTestSysproxy(t)
	reg.strs[valAutoConfigURL] = "http://old-proxy.example/proxy.pac" // user's old PAC
	reg.dwords[valProxyEnable] = 1

	if err := s.Enable(4525); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://127.0.0.1:4525/proxy.pac" {
		t.Errorf("AutoConfigURL = %q, want our PAC URL", got)
	}
	if *refreshes != 1 {
		t.Errorf("refresh calls = %d, want 1", *refreshes)
	}
	b := readBackup(t, s)
	if b == nil {
		t.Fatal("backup file missing")
	}
	if !b.WeSetIt {
		t.Error("backup must say weSetIt: true")
	}
	if !b.AutoConfigURL.Existed || b.AutoConfigURL.Previous != "http://old-proxy.example/proxy.pac" {
		t.Errorf("backup AutoConfigURL = %+v, want the old PAC recorded", b.AutoConfigURL)
	}
	if !b.AutoConfigURL.Changed {
		t.Error("backup must mark AutoConfigURL as changed")
	}
	if !b.ProxyEnable.Existed || b.ProxyEnable.PreviousDword != 1 {
		t.Errorf("backup ProxyEnable = %+v, want 1 recorded", b.ProxyEnable)
	}
	if b.ProxyEnable.Changed {
		t.Error("we never change ProxyEnable in PAC mode")
	}
}

func TestEnableWhenNothingExisted(t *testing.T) {
	s, reg, _, _ := newTestSysproxy(t)
	if err := s.Enable(4525); err != nil {
		t.Fatal(err)
	}
	b := readBackup(t, s)
	if b.AutoConfigURL.Existed {
		t.Error("AutoConfigURL did not exist before; backup must record that")
	}
	if err := s.Restore(); err != nil {
		t.Fatal(err)
	}
	if _, exists := reg.strs[valAutoConfigURL]; exists {
		t.Error("restore must delete a value that did not exist before")
	}
}

func TestDoubleEnableKeepsOriginalBackup(t *testing.T) {
	s, reg, _, _ := newTestSysproxy(t)
	reg.strs[valAutoConfigURL] = "http://old.example/proxy.pac"
	if err := s.Enable(4525); err != nil {
		t.Fatal(err)
	}
	// Enable again with a different port (a restart on another port).
	if err := s.Enable(4530); err != nil {
		t.Fatal(err)
	}
	b := readBackup(t, s)
	if b.AutoConfigURL.Previous != "http://old.example/proxy.pac" {
		t.Errorf("second Enable overwrote the original backup: %+v", b.AutoConfigURL)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://127.0.0.1:4530/proxy.pac" {
		t.Errorf("AutoConfigURL = %q, want the new port", got)
	}
}

func TestRestorePutsBackPreviousValue(t *testing.T) {
	s, reg, _, refreshes := newTestSysproxy(t)
	reg.strs[valAutoConfigURL] = "http://old.example/proxy.pac"
	if err := s.Enable(4525); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(); err != nil {
		t.Fatal(err)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://old.example/proxy.pac" {
		t.Errorf("AutoConfigURL = %q, want the old value back", got)
	}
	if _, err := os.Stat(s.backupPath()); !os.IsNotExist(err) {
		t.Error("backup file must be removed after restore")
	}
	if *refreshes != 2 {
		t.Errorf("refresh calls = %d, want 2 (enable + restore)", *refreshes)
	}
}

func TestCrashRecoveryRestoresLeftoverSetting(t *testing.T) {
	s, reg, _, _ := newTestSysproxy(t)
	reg.strs[valAutoConfigURL] = "http://old.example/proxy.pac"
	if err := s.Enable(4525); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash: no Restore ran; the backup file and our PAC remain.
	if got := reg.strs[valAutoConfigURL]; got != "http://127.0.0.1:4525/proxy.pac" {
		t.Fatalf("precondition: %q", got)
	}
	// The next start calls Restore (crash recovery): internet comes back.
	if err := s.Restore(); err != nil {
		t.Fatal(err)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://old.example/proxy.pac" {
		t.Errorf("crash recovery failed: %q", got)
	}
}

func TestRestoreLeavesForeignValueAlone(t *testing.T) {
	s, reg, log, _ := newTestSysproxy(t)
	if err := s.Enable(4525); err != nil {
		t.Fatal(err)
	}
	// Another program changes the setting while ours is active.
	reg.strs[valAutoConfigURL] = "http://another-program.example/proxy.pac"
	if err := s.Restore(); err != nil {
		t.Fatal(err)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://another-program.example/proxy.pac" {
		t.Errorf("restore must not touch a value that is no longer ours, got %q", got)
	}
	if !log.has("changed by another program") {
		t.Errorf("the foreign change should be logged: %v", log.lines)
	}
}

func TestRestoreWithoutBackupIsNoop(t *testing.T) {
	s, _, _, refreshes := newTestSysproxy(t)
	if err := s.Restore(); err != nil {
		t.Fatal(err)
	}
	if *refreshes != 0 {
		t.Errorf("no-op restore should not refresh, calls = %d", *refreshes)
	}
}

func TestCorruptBackupIgnoredSafely(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sysproxy-backup.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := newFakeRegistry()
	log := &testLogger{}
	s := newSysproxy(dir, func() (internetSettings, error) { return reg, nil }, func() error { return nil }, log)
	// A corrupt backup must neither crash nor block a fresh Enable.
	if err := s.Enable(4525); err != nil {
		t.Fatalf("Enable with corrupt backup: %v", err)
	}
	if got := reg.strs[valAutoConfigURL]; got != "http://127.0.0.1:4525/proxy.pac" {
		t.Errorf("AutoConfigURL = %q", got)
	}
	if !log.has("corrupt") {
		t.Errorf("corrupt backup should be logged: %v", log.lines)
	}
}

// ---- Autostart logic ----

func TestAutostartCommandFormat(t *testing.T) {
	got := autostartCommand(`C:\Program Files\Younblock\younblock.exe`)
	want := `"C:\Program Files\Younblock\younblock.exe" --minimized`
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

func TestSetAutostartEnableDisable(t *testing.T) {
	reg := newFakeRegistry()
	exe := `C:\Apps\Younblock\younblock.exe`
	if err := setAutostart(true, exe, reg); err != nil {
		t.Fatal(err)
	}
	got, exists := reg.strs[AutostartValueName]
	if !exists || got != autostartCommand(exe) {
		t.Fatalf("run value = %q exists=%v", got, exists)
	}
	if err := setAutostart(false, exe, reg); err != nil {
		t.Fatal(err)
	}
	if _, exists := reg.strs[AutostartValueName]; exists {
		t.Error("autostart value must be removed when disabled")
	}
	// Disabling again is a no-op.
	if err := setAutostart(false, exe, reg); err != nil {
		t.Fatal(err)
	}
}

func TestAutostartNeverDuplicates(t *testing.T) {
	reg := newFakeRegistry()
	// Enabling twice leaves exactly one value (map semantics in the fake;
	// the real registry has one named value too).
	for i := 0; i < 3; i++ {
		if err := setAutostart(true, `C:\Apps\Younblock\younblock.exe`, reg); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for name := range reg.strs {
		if strings.Contains(name, "Younblock") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate autostart entries: %d values mention Younblock", count)
	}
}

func TestAutostartRewriteAfterMove(t *testing.T) {
	reg := newFakeRegistry()
	old := `C:\Old\younblock.exe`
	if err := setAutostart(true, old, reg); err != nil {
		t.Fatal(err)
	}
	newPath := `D:\New Folder\younblock.exe`
	rewritten, err := rewriteIfMoved(newPath, reg)
	if err != nil || !rewritten {
		t.Fatalf("rewriteIfMoved = %v, %v; want true, nil", rewritten, err)
	}
	if got := reg.strs[AutostartValueName]; got != autostartCommand(newPath) {
		t.Errorf("run value = %q, want the new path", got)
	}
	// No rewrite when the path already matches.
	rewritten, err = rewriteIfMoved(newPath, reg)
	if err != nil || rewritten {
		t.Errorf("rewriteIfMoved = %v, %v; want false, nil", rewritten, err)
	}
}

func TestAutostartEnabledReadsBack(t *testing.T) {
	reg := newFakeRegistry()
	if ok, _ := autostartEnabled(reg); ok {
		t.Error("autostart should be off initially")
	}
	if err := setAutostart(true, `C:\x\younblock.exe`, reg); err != nil {
		t.Fatal(err)
	}
	if ok, _ := autostartEnabled(reg); !ok {
		t.Error("autostart should read back as on")
	}
}

func TestPACURLFor(t *testing.T) {
	if got := pacURLFor(4525); got != "http://127.0.0.1:4525/proxy.pac" {
		t.Errorf("pacURLFor = %q", got)
	}
}
