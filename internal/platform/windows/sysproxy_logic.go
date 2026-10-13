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
	PACURL        string      `json:"pacUrl"`
	AutoConfigURL backupValue `json:"autoConfigURL"`
	ProxyEnable   backupValue `json:"proxyEnable"`
	ProxyServer   backupValue `json:"proxyServer"`
	ProxyOverride backupValue `json:"proxyOverride"`
	SavedAt       time.Time   `json:"savedAt"`
}

// sysproxy switches the Windows proxy setting through a PAC URL, backing
// up previous values first and restoring only what is still ours.
type sysproxy struct {
	mu      sync.Mutex
	dir     string // config directory (backup file location)
	open    func() (internetSettings, error)
	refresh func() error // InternetSetOption SETTINGS_CHANGED + REFRESH
	log     Logger
}

func newSysproxy(dir string, open func() (internetSettings, error), refresh func() error, log Logger) *sysproxy {
	return &sysproxy{dir: dir, open: open, refresh: refresh, log: log}
}

func (s *sysproxy) backupPath() string {
	return filepath.Join(s.dir, "sysproxy-backup.json")
}

// pacURL builds the PAC address for a port.
func pacURLFor(port int) string {
	return fmt.Sprintf("http://%s:%d/proxy.pac", config.ListenHost, port)
}

// Enable points Windows at our PAC script. The previous values are backed
// up before anything changes; a second Enable while ours is active only
// re-asserts the URL (the original backup is kept).
func (s *sysproxy) Enable(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, err := s.open()
	if err != nil {
		return fmt.Errorf("open internet settings: %w", err)
	}
	pacURL := pacURLFor(port)

	backup, err := s.loadBackup()
	if err != nil {
		return err
	}
	ours := backup != nil && backup.WeSetIt
	if !ours {
		backup = &backupFile{WeSetIt: true, SavedAt: time.Now()}
		if err := s.recordPrevious(key, &backup.AutoConfigURL, valAutoConfigURL, false); err != nil {
			return err
		}
		if err := s.recordPrevious(key, &backup.ProxyEnable, valProxyEnable, true); err != nil {
			return err
		}
		if err := s.recordPrevious(key, &backup.ProxyServer, valProxyServer, false); err != nil {
			return err
		}
		if err := s.recordPrevious(key, &backup.ProxyOverride, valProxyOverride, false); err != nil {
			return err
		}
	}

	current, _, err := key.readString(valAutoConfigURL)
	if err != nil {
		return err
	}
	backup.PACURL = pacURL
	if current != pacURL {
		if err := key.writeString(valAutoConfigURL, pacURL); err != nil {
			return err
		}
		backup.AutoConfigURL.Changed = true
	}
	if err := s.saveBackup(backup); err != nil {
		return err
	}
	if err := s.refresh(); err != nil {
		s.log.Warn("windows did not accept the proxy-settings refresh", "reason", err.Error())
	}
	s.log.Info("windows proxy setting enabled", "pac", "set")
	return nil
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
	if err := s.restoreValue(key, &backup.ProxyEnable, valProxyEnable, ""); err != nil {
		return err
	}
	if err := s.restoreValue(key, &backup.ProxyServer, valProxyServer, ""); err != nil {
		return err
	}
	if err := s.restoreValue(key, &backup.ProxyOverride, valProxyOverride, ""); err != nil {
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
		// A dword we own would equal ours. In PAC mode this branch never
		// runs (we never change ProxyEnable); it exists so a future
		// static-proxy fallback already has correct undo behavior.
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
			return key.deleteValue(name)
		}
		return key.writeDWORD(name, bv.PreviousDword)
	}
	current, exists, err := key.readString(name)
	if err != nil {
		return err
	}
	if !exists || current != ours {
		if exists && current != ours {
			s.log.Warn("proxy setting was changed by another program; leaving it alone", "value", name)
		}
		return nil
	}
	if !bv.Existed {
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
