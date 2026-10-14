package windows

import (
        "encoding/json"
        "os"
        "path/filepath"
        "strings"
        "sync"
        "testing"
        "time"
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

func TestEnableSetsManualProxyAndBacksUp(t *testing.T) {
        s, reg, _, refreshes := newTestSysproxy(t)
        reg.strs[valAutoConfigURL] = "http://old-proxy.example/proxy.pac" // user's old PAC
        reg.dwords[valProxyEnable] = 1
        reg.strs[valProxyServer] = "proxy.example:8080" // user's old manual proxy
        reg.strs[valProxyOverride] = "<local>"

        if err := s.Enable(4525); err != nil {
                t.Fatalf("Enable: %v", err)
        }
        // The manual values are ours; the PAC is gone (it would win over them).
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4525" {
                t.Errorf("ProxyServer = %q, want 127.0.0.1:4525", got)
        }
        if got := reg.dwords[valProxyEnable]; got != 1 {
                t.Errorf("ProxyEnable = %d, want 1", got)
        }
        if _, exists := reg.strs[valAutoConfigURL]; exists {
                t.Error("AutoConfigURL must be cleared while we are active")
        }
        if got := reg.strs[valProxyOverride]; got != proxyOverrideValue {
                t.Errorf("ProxyOverride = %q, want the bypass list", got)
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
        if b.Server != "127.0.0.1:4525" {
                t.Errorf("backup Server = %q", b.Server)
        }
        if !b.AutoConfigURL.Existed || b.AutoConfigURL.Previous != "http://old-proxy.example/proxy.pac" {
                t.Errorf("backup AutoConfigURL = %+v, want the old PAC recorded", b.AutoConfigURL)
        }
        if !b.AutoConfigURL.Changed {
                t.Error("backup must mark AutoConfigURL as changed (we cleared it)")
        }
        if !b.ProxyServer.Existed || b.ProxyServer.Previous != "proxy.example:8080" {
                t.Errorf("backup ProxyServer = %+v, want the old server recorded", b.ProxyServer)
        }
        if !b.ProxyEnable.Existed || b.ProxyEnable.PreviousDword != 1 {
                t.Errorf("backup ProxyEnable = %+v, want 1 recorded", b.ProxyEnable)
        }
}

func TestEnableWhenNothingExisted(t *testing.T) {
        s, reg, _, _ := newTestSysproxy(t)
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        b := readBackup(t, s)
        if b.AutoConfigURL.Existed || b.ProxyServer.Existed || b.ProxyEnable.Existed {
                t.Error("nothing existed before; backup must record that")
        }
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if _, exists := reg.strs[valProxyServer]; exists {
                t.Error("restore must delete a value that did not exist before")
        }
        if _, exists := reg.dwords[valProxyEnable]; exists {
                t.Error("restore must delete ProxyEnable when it did not exist before")
        }
}

func TestDoubleEnableKeepsOriginalBackup(t *testing.T) {
        s, reg, _, _ := newTestSysproxy(t)
        reg.strs[valProxyServer] = "proxy.example:8080"
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        // Enable again with a different port (a restart on another port).
        if err := s.Enable(4530); err != nil {
                t.Fatal(err)
        }
        b := readBackup(t, s)
        if b.ProxyServer.Previous != "proxy.example:8080" {
                t.Errorf("second Enable overwrote the original backup: %+v", b.ProxyServer)
        }
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4530" {
                t.Errorf("ProxyServer = %q, want the new port", got)
        }
}

func TestRestorePutsBackPreviousValues(t *testing.T) {
        s, reg, _, refreshes := newTestSysproxy(t)
        reg.strs[valAutoConfigURL] = "http://old.example/proxy.pac"
        reg.strs[valProxyServer] = "proxy.example:8080"
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valAutoConfigURL]; got != "http://old.example/proxy.pac" {
                t.Errorf("AutoConfigURL = %q, want the old value back", got)
        }
        if got := reg.strs[valProxyServer]; got != "proxy.example:8080" {
                t.Errorf("ProxyServer = %q, want the old value back", got)
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
        reg.strs[valProxyServer] = "proxy.example:8080"
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        // Simulate a crash: no Restore ran; our values remain.
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4525" {
                t.Fatalf("precondition: %q", got)
        }
        // The next start calls Restore (crash recovery): internet comes back.
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valProxyServer]; got != "proxy.example:8080" {
                t.Errorf("crash recovery failed: %q", got)
        }
}

// The upgrade path: a legacy (<= v1.2.0) backup with our PAC URL still in
// the registry must be cleaned up by the first Enable of the new version.
func TestEnableCleansUpLegacyPAC(t *testing.T) {
        s, reg, _, _ := newTestSysproxy(t)
        // What v1.2.0 left behind: our PAC set, backup with PACURL recorded,
        // and the user's original PAC noted in the backup.
        reg.strs[valAutoConfigURL] = pacURLFor(4525)
        writeBackupFile(t, s, &backupFile{
                WeSetIt: true,
                PACURL:  pacURLFor(4525),
                AutoConfigURL: backupValue{
                        Previous: "http://user-original.example/proxy.pac",
                        Existed:  true,
                        Changed:  true,
                },
                SavedAt: time.Now(),
        })
        if err := s.Enable(4531); err != nil {
                t.Fatal(err)
        }
        if _, exists := reg.strs[valAutoConfigURL]; exists {
                t.Fatal("our legacy PAC URL must be removed by the new Enable")
        }
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4531" {
                t.Fatalf("ProxyServer = %q", got)
        }
        b := readBackup(t, s)
        if b.PACURL != "" {
                t.Errorf("backup PACURL = %q, want cleared", b.PACURL)
        }
        // The user's original PAC is still the recorded previous value.
        if b.AutoConfigURL.Previous != "http://user-original.example/proxy.pac" || !b.AutoConfigURL.Existed {
                t.Errorf("backup AutoConfigURL = %+v, want the user's original kept", b.AutoConfigURL)
        }
        // And a later Restore brings it back.
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valAutoConfigURL]; got != "http://user-original.example/proxy.pac" {
                t.Errorf("Restore must return the user's original PAC, got %q", got)
        }
}

func TestEnsureNoopWhenValuesInPlace(t *testing.T) {
        s, _, log, refreshes := newTestSysproxy(t)
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        *refreshes = 0
        changed, err := s.ensure(4525)
        if err != nil {
                t.Fatal(err)
        }
        if changed {
                t.Error("ensure must report no change when our values are active")
        }
        if *refreshes != 0 {
                t.Errorf("refresh calls = %d, want 0 (nothing to re-assert)", *refreshes)
        }
        if log.has("re-applied") {
                t.Error("no re-apply warning should be logged")
        }
}

func TestEnsureReassertsAfterClobber(t *testing.T) {
        s, reg, log, _ := newTestSysproxy(t)
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        // Another program overwrites the server address and re-enables a PAC.
        reg.strs[valProxyServer] = "another-tool.example:9"
        reg.strs[valAutoConfigURL] = "http://another-tool.example/proxy.pac"
        changed, err := s.ensure(4525)
        if err != nil {
                t.Fatal(err)
        }
        if !changed {
                t.Fatal("ensure must report the re-assert")
        }
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4525" {
                t.Errorf("ProxyServer = %q, want ours re-applied", got)
        }
        if _, exists := reg.strs[valAutoConfigURL]; exists {
                t.Error("the foreign PAC must be cleared again")
        }
        if !log.has("re-applied") {
                t.Errorf("the re-assert should be logged: %v", log.lines)
        }
        // The original backup is kept (not a fresh capture of the clobbered values).
        b := readBackup(t, s)
        if b.ProxyServer.Changed && b.ProxyServer.Previous != "" {
                t.Errorf("backup must keep the original previous value, got %+v", b.ProxyServer)
        }
}

func TestRestoreLeavesForeignValueAlone(t *testing.T) {
        s, reg, log, _ := newTestSysproxy(t)
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        // Another program changes the setting while ours is active.
        reg.strs[valProxyServer] = "another-program.example:9"
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valProxyServer]; got != "another-program.example:9" {
                t.Errorf("restore must not touch a value that is no longer ours, got %q", got)
        }
        if !log.has("changed by another program") {
                t.Errorf("the foreign change should be logged: %v", log.lines)
        }
}

func writeBackupFile(t *testing.T, s *sysproxy, b *backupFile) {
        t.Helper()
        if err := s.saveBackup(b); err != nil {
                t.Fatal(err)
        }
}

// The tiny v1.2.0 crash window: our PAC URL is set but no backup exists.
// The startup cleanup must remove it, and a later Enable/Stop cycle must
// not resurrect it as if it were the user's own.
func TestCleanupLegacyRemovesOrphanedPACURL(t *testing.T) {
        s, reg, _, _ := newTestSysproxy(t)
        reg.strs[valAutoConfigURL] = pacURLFor(4525)
        if err := s.cleanupLegacy(); err != nil {
                t.Fatal(err)
        }
        if _, exists := reg.strs[valAutoConfigURL]; exists {
                t.Fatal("cleanupLegacy must remove our orphaned PAC URL")
        }
        // And a fresh Enable must not record the legacy PAC as the user's
        // original (Restore would otherwise bring it back).
        reg.strs[valAutoConfigURL] = pacURLFor(4526)
        if err := s.Enable(4525); err != nil {
                t.Fatal(err)
        }
        if err := s.Restore(); err != nil {
                t.Fatal(err)
        }
        if _, exists := reg.strs[valAutoConfigURL]; exists {
                t.Error("Restore resurrected the orphaned legacy PAC URL")
        }
}

func TestCleanupLegacyLeavesForeignAndBackedUpAlone(t *testing.T) {
        s, reg, _, _ := newTestSysproxy(t)
        // A foreign PAC without our backup is not ours to touch.
        reg.strs[valAutoConfigURL] = "http://user.example/proxy.pac"
        if err := s.cleanupLegacy(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valAutoConfigURL]; got != "http://user.example/proxy.pac" {
                t.Fatalf("cleanupLegacy must leave a foreign PAC alone, got %q", got)
        }
        // With our backup present the normal Restore path owns the case.
        reg.strs[valAutoConfigURL] = pacURLFor(4525)
        writeBackupFile(t, s, &backupFile{WeSetIt: true, PACURL: pacURLFor(4525), SavedAt: time.Now()})
        if err := s.cleanupLegacy(); err != nil {
                t.Fatal(err)
        }
        if got := reg.strs[valAutoConfigURL]; got != pacURLFor(4525) {
                t.Fatalf("cleanupLegacy must defer to the backup-holding Restore path, got %q", got)
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
        if got := reg.strs[valProxyServer]; got != "127.0.0.1:4525" {
                t.Errorf("ProxyServer = %q", got)
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
        if !isOurLegacyPACURL("http://127.0.0.1:4599/proxy.pac") {
                t.Error("isOurLegacyPACURL must recognize any port")
        }
        if isOurLegacyPACURL("http://old-proxy.example/proxy.pac") {
                t.Error("isOurLegacyPACURL must not match a foreign PAC")
        }
}
