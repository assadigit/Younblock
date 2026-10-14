// Package windows implements the Windows platform adapter (file 01,
// section 8). The logic files in this package carry no build tag so they
// compile — and are unit-tested — on every platform against in-memory
// fakes; only the files with a //go:build windows tag touch the real
// registry and wininet.
package windows

import (
        "encoding/json"
        "fmt"
        "os"
        "path/filepath"
        "strings"
        "sync"
        "time"

        "github.com/assadigit/Younblock/internal/config"
)

// internetSettings abstracts the registry key
// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings.
type internetSettings interface {
        readString(name string) (val string, exists bool, err error)
        readDWORD(name string) (val uint32, exists bool, err error)
        writeString(name, val string) error
        writeDWORD(name string, val uint32) error
        deleteValue(name string) error
}

// Logger is the narrow logging interface used by this package.
type Logger interface {
        Info(msg string, args ...any)
        Warn(msg string, args ...any)
        Error(msg string, args ...any)
}

// managed values and their names in Internet Settings.
const (
        valAutoConfigURL = "AutoConfigURL"
        valProxyEnable   = "ProxyEnable"
        valProxyServer   = "ProxyServer"
        valProxyOverride = "ProxyOverride"
)

// backupValue records one registry value before we changed it.
type backupValue struct {
        Previous      string `json:"previous"`
        PreviousDword uint32 `json:"previousDword"`
        Existed       bool   `json:"existed"`
        IsDword       bool   `json:"isDword"`
        Changed       bool   `json:"changed"`
}

// backupFile is sysproxy-backup.json (file 01, section 8).
type backupFile struct {
        WeSetIt       bool        `json:"weSetIt"`
        PACURL        string      `json:"pacUrl"` // legacy (<= v1.2.0) PAC address, kept for cleanup
        Server        string      `json:"server"` // the manual ProxyServer value we last wrote
        AutoConfigURL backupValue `json:"autoConfigURL"`
        ProxyEnable   backupValue `json:"proxyEnable"`
        ProxyServer   backupValue `json:"proxyServer"`
        ProxyOverride backupValue `json:"proxyOverride"`
        SavedAt       time.Time   `json:"savedAt"`
}

// sysproxy switches the Windows proxy setting through the manual proxy
// values (ProxyEnable/ProxyServer/ProxyOverride) — the same registry
// values every mainstream browser follows and the same mechanism tools
// like v2rayN and Clash use — backing up previous values first and
// restoring only what is still ours. Earlier versions pointed Windows at
// our PAC script (AutoConfigURL); v1.3.0 switched to the manual values
// after field reports that some browsers never picked the PAC up
// (DECISIONS.md D46): a PAC left in the registry would take precedence
// over the manual values in WinINet, so any AutoConfigURL — ours from an
// older version, or the user's own — is cleared while we are active and
// put back on Restore.
type sysproxy struct {
        mu      sync.Mutex
        dir     string // config directory (backup file location)
        open    func() (internetSettings, error)
        refresh func() error // InternetSetOption SETTINGS_CHANGED + REFRESH + WM_SETTINGCHANGE
        log     Logger
}

func newSysproxy(dir string, open func() (internetSettings, error), refresh func() error, log Logger) *sysproxy {
        return &sysproxy{dir: dir, open: open, refresh: refresh, log: log}
}

func (s *sysproxy) backupPath() string {
        return filepath.Join(s.dir, "sysproxy-backup.json")
}

// pacURLFor builds the PAC address a legacy version (<= v1.2.0) wrote. It
// is only used to recognize and clean up such leftover values.
func pacURLFor(port int) string {
        return fmt.Sprintf("http://%s:%d/proxy.pac", config.ListenHost, port)
}

// isOurLegacyPACURL reports whether an AutoConfigURL value is one this app
// (any version, any port) wrote.
func isOurLegacyPACURL(url string) bool {
        return strings.HasPrefix(url, "http://"+config.ListenHost+":") && strings.HasSuffix(url, "/proxy.pac")
}

// proxyServerFor builds the ProxyServer value for a port.
func proxyServerFor(port int) string {
        return fmt.Sprintf("%s:%d", config.ListenHost, port)
}

// proxyOverrideValue is the bypass list written while we are active:
// loopback, the private ranges and single-label hostnames stay direct so
// LAN and local traffic never depends on this app.
const proxyOverrideValue = "localhost;127.*;192.168.*;10.*;" +
        "172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;" +
        "172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;" +
        "172.30.*;172.31.*;<local>"

// Enable points Windows at the local proxy with the manual proxy values.
// The previous values are backed up before anything changes; a second
// Enable while ours is active only re-asserts the address (the original
// backup is kept).
func (s *sysproxy) Enable(port int) error {
        s.mu.Lock()
        defer s.mu.Unlock()
        key, err := s.open()
        if err != nil {
                return fmt.Errorf("open internet settings: %w", err)
        }
        backup, err := s.loadBackup()
        if err != nil {
                return err
        }
        if backup == nil || !backup.WeSetIt {
                if backup, err = s.capturePrevious(key); err != nil {
                        return err
                }
        }
        if err := s.applyManualLocked(key, backup, port); err != nil {
                return err
        }
        if err := s.saveBackup(backup); err != nil {
                return err
        }
        if err := s.refresh(); err != nil {
                s.log.Warn("windows did not accept the proxy-settings refresh", "reason", err.Error())
        }
        s.log.Info("windows proxy setting enabled", "server", "set")
        return nil
}

// ensure re-asserts the manual proxy values when another program has
// overwritten them (the engine's watchdog calls this periodically). It
// returns true when it had to rewrite something.
func (s *sysproxy) ensure(port int) (bool, error) {
        s.mu.Lock()
        defer s.mu.Unlock()
        key, err := s.open()
        if err != nil {
                return false, fmt.Errorf("open internet settings: %w", err)
        }
        if s.manualValuesInPlace(key, port) {
                return false, nil
        }
        backup, err := s.loadBackup()
        if err != nil {
                return false, err
        }
        if backup == nil || !backup.WeSetIt {
                if backup, err = s.capturePrevious(key); err != nil {
                        return false, err
                }
        }
        if err := s.applyManualLocked(key, backup, port); err != nil {
                return false, err
        }
        if err := s.saveBackup(backup); err != nil {
                return false, err
        }
        if err := s.refresh(); err != nil {
                s.log.Warn("windows did not accept the proxy-settings refresh", "reason", err.Error())
        }
        s.log.Warn("windows proxy setting was changed by another program; re-applied")
        return true, nil
}

// capturePrevious records the current value of all four managed values.
// A leftover PAC URL from a crashed older version is ours, not the user's,
// so it is recorded as absent (it must not be resurrected at Restore).
func (s *sysproxy) capturePrevious(key internetSettings) (*backupFile, error) {
        backup := &backupFile{WeSetIt: true, SavedAt: time.Now()}
        cur, exists, err := key.readString(valAutoConfigURL)
        if err != nil {
                return nil, err
        }
        if exists && isOurLegacyPACURL(cur) {
                backup.AutoConfigURL = backupValue{IsDword: false} // treat as absent
        } else if err := s.recordPrevious(key, &backup.AutoConfigURL, valAutoConfigURL, false); err != nil {
                return nil, err
        }
        if err := s.recordPrevious(key, &backup.ProxyEnable, valProxyEnable, true); err != nil {
                return nil, err
        }
        if err := s.recordPrevious(key, &backup.ProxyServer, valProxyServer, false); err != nil {
                return nil, err
        }
        if err := s.recordPrevious(key, &backup.ProxyOverride, valProxyOverride, false); err != nil {
                return nil, err
        }
        return backup, nil
}

// cleanupLegacy removes a PAC URL an older version (<= v1.2.0) left behind
// without a backup (a crash in the tiny window between writing the URL and
// saving the backup file). The app calls it once at startup, before the
// engine starts anything.
func (s *sysproxy) cleanupLegacy() error {
        s.mu.Lock()
        defer s.mu.Unlock()
        backup, err := s.loadBackup()
        if err != nil {
                return err
        }
        if backup != nil && backup.WeSetIt {
                return nil // the normal crash-recovery Restore owns this case
        }
        key, err := s.open()
        if err != nil {
                return fmt.Errorf("open internet settings: %w", err)
        }
        cur, exists, err := key.readString(valAutoConfigURL)
        if err != nil {
                return err
        }
        if !exists || !isOurLegacyPACURL(cur) {
                return nil
        }
        if err := key.deleteValue(valAutoConfigURL); err != nil {
                return err
        }
        if err := s.refresh(); err != nil {
                s.log.Warn("windows did not accept the proxy-settings refresh", "reason", err.Error())
        }
        s.log.Info("removed a leftover proxy script from an earlier version")
        return nil
}

// applyManualLocked writes the manual proxy values and clears any
// AutoConfigURL (a PAC takes precedence over ProxyServer in WinINet, so
// one left behind — ours from a legacy version, or another program's —
// would send browsers somewhere else). backup must already hold the
// previous values (capturePrevious ran for a fresh takeover); it is never
// re-captured here, so the user's true original is what Restore returns.
func (s *sysproxy) applyManualLocked(key internetSettings, backup *backupFile, port int) error {
        server := proxyServerFor(port)

        // AutoConfigURL: remove while we are active.
        current, exists, err := key.readString(valAutoConfigURL)
        if err != nil {
                return err
        }
        if exists && current != "" {
                if err := key.deleteValue(valAutoConfigURL); err != nil {
                        return err
                }
                backup.AutoConfigURL.Changed = true
        }
        // (When AutoConfigURL is already absent there is nothing to clear; a
        // legacy backup's AutoConfigURL entry keeps saying what it said, so
        // Restore still puts the user's original script back if we removed it.)
        backup.PACURL = "" // the PAC era is over
        backup.Server = server

        // ProxyEnable = 1.
        enableCur, enableExists, err := key.readDWORD(valProxyEnable)
        if err != nil {
                return err
        }
        if !enableExists || enableCur != 1 {
                if err := key.writeDWORD(valProxyEnable, 1); err != nil {
                        return err
                }
                backup.ProxyEnable.Changed = true
        }

        // ProxyServer = 127.0.0.1:port.
        serverCur, serverExists, err := key.readString(valProxyServer)
        if err != nil {
                return err
        }
        if !serverExists || serverCur != server {
                if err := key.writeString(valProxyServer, server); err != nil {
                        return err
                }
                backup.ProxyServer.Changed = true
        }

        // ProxyOverride = our bypass list.
        overCur, overExists, err := key.readString(valProxyOverride)
        if err != nil {
                return err
        }
        if !overExists || overCur != proxyOverrideValue {
                if err := key.writeString(valProxyOverride, proxyOverrideValue); err != nil {
                        return err
                }
                backup.ProxyOverride.Changed = true
        }
        return nil
}

// manualValuesInPlace reports whether our manual proxy values (and no
// foreign AutoConfigURL) are currently active.
func (s *sysproxy) manualValuesInPlace(key internetSettings, port int) bool {
        if v, exists, err := key.readDWORD(valProxyEnable); err != nil || !exists || v != 1 {
                return false
        }
        if v, exists, err := key.readString(valProxyServer); err != nil || !exists || v != proxyServerFor(port) {
                return false
        }
        if v, exists, err := key.readString(valAutoConfigURL); err != nil || (exists && v != "") {
                return false
        }
        return true
}

// recordPrevious captures the current value of one registry value.
func (s *sysproxy) recordPrevious(key internetSettings, bv *backupValue, name string, dword bool) error {
        if dword {
                v, exists, err := key.readDWORD(name)
                if err != nil {
                        return err
                }
                bv.PreviousDword = v
                bv.Existed = exists
                bv.IsDword = true
                return nil
        }
        v, exists, err := key.readString(name)
        if err != nil {
                return err
        }
        bv.Previous = v
        bv.Existed = exists
        return nil
}

// Restore puts back what we changed, but only values that are still ours
// (file 01, section 8: "restore only values we changed and only if the
// current value is still ours"). It is also the crash-recovery path
// (R16): it runs at every app start and behind --restore-proxy.
func (s *sysproxy) Restore() error {
        s.mu.Lock()
        defer s.mu.Unlock()
        backup, err := s.loadBackup()
        if err != nil {
                return err
        }
        if backup == nil || !backup.WeSetIt {
                return nil // nothing of ours to restore
        }
        key, err := s.open()
        if err != nil {
                return fmt.Errorf("open internet settings: %w", err)
        }
        if err := s.restoreValue(key, &backup.AutoConfigURL, valAutoConfigURL, backup.PACURL); err != nil {
                return err
        }
        if err := s.restoreValue(key, &backup.ProxyEnable, valProxyEnable, "1"); err != nil {
                return err
        }
        if err := s.restoreValue(key, &backup.ProxyServer, valProxyServer, backup.Server); err != nil {
                return err
        }
        if err := s.restoreValue(key, &backup.ProxyOverride, valProxyOverride, proxyOverrideValue); err != nil {
                return err
        }
        if err := os.Remove(s.backupPath()); err != nil && !os.IsNotExist(err) {
                s.log.Warn("removing the proxy backup file failed", "reason", err.Error())
        }
        if err := s.refresh(); err != nil {
                s.log.Warn("windows did not accept the proxy-settings refresh", "reason", err.Error())
        }
        s.log.Info("windows proxy setting restored")
        return nil
}

// restoreValue restores one value when it is still ours.
func (s *sysproxy) restoreValue(key internetSettings, bv *backupValue, name, ours string) error {
        if !bv.Changed {
                return nil
        }
        if bv.IsDword {
                current, exists, err := key.readDWORD(name)
                if err != nil {
                        return err
                }
                oursDword := uint32(0)
                if n, ok := parseDword(ours); ok {
                        oursDword = uint32(n)
                }
                if !exists || current != oursDword {
                        if exists {
                                s.log.Warn("proxy setting was changed by another program; leaving it alone", "value", name)
                        }
                        return nil
                }
                if !bv.Existed {
                        if !exists {
                                return nil // already absent
                        }
                        return key.deleteValue(name)
                }
                return key.writeDWORD(name, bv.PreviousDword)
        }
        current, exists, err := key.readString(name)
        if err != nil {
                return err
        }
        // A foreign value must be left alone; an absent value is ours in the
        // manual-proxy era (we cleared it), so the original comes back.
        if exists && current != ours {
                s.log.Warn("proxy setting was changed by another program; leaving it alone", "value", name)
                return nil
        }
        if !bv.Existed {
                if !exists {
                        return nil // already absent
                }
                if err := key.deleteValue(name); err != nil {
                        return err
                }
                return nil
        }
        return key.writeString(name, bv.Previous)
}

// loadBackup reads sysproxy-backup.json (nil when absent).
func (s *sysproxy) loadBackup() (*backupFile, error) {
        data, err := os.ReadFile(s.backupPath())
        if err != nil {
                if os.IsNotExist(err) {
                        return nil, nil
                }
                return nil, err
        }
        var b backupFile
        if err := json.Unmarshal(data, &b); err != nil {
                // A corrupt backup must not block the internet: treat as absent.
                s.log.Warn("proxy backup file was corrupt; ignoring it", "reason", err.Error())
                return nil, nil
        }
        return &b, nil
}

// saveBackup writes the backup atomically.
func (s *sysproxy) saveBackup(b *backupFile) error {
        if err := os.MkdirAll(s.dir, 0o700); err != nil {
                return err
        }
        data, err := json.MarshalIndent(b, "", "  ")
        if err != nil {
                return err
        }
        tmp := filepath.Join(s.dir, ".sysproxy-backup.json.tmp")
        if err := os.WriteFile(tmp, data, 0o600); err != nil {
                return err
        }
        return os.Rename(tmp, s.backupPath())
}

// parseDword reads a decimal dword from a string.
func parseDword(s string) (int, bool) {
        if s == "" {
                return 0, false
        }
        n := 0
        for i := 0; i < len(s); i++ {
                if s[i] < '0' || s[i] > '9' {
                        return 0, false
                }
                n = n*10 + int(s[i]-'0')
                if n > 1<<31 {
                        return 0, false
                }
        }
        return n, true
}
