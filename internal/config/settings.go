package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// settingsVersion is the settings file schema version.
const settingsVersion = 1

// Advanced holds troubleshooting-only settings, collapsed in the UI.
// Zero values mean "use the preset default".
type Advanced struct {
	// PieceSizeMin / PieceSizeMax override the active preset's random piece-size range.
	PieceSizeMin int `json:"pieceSizeMin"`
	PieceSizeMax int `json:"pieceSizeMax"`
	// PauseMs overrides the pause between pieces.
	PauseMs int `json:"pauseMs"`
	// DoHServers is the secure-DNS server list, tried in order.
	DoHServers []DoHServer `json:"dohServers"`
	// Domains is the YouTube domain list used by the PAC script.
	Domains []string `json:"domains"`
	// LogDomains keeps website names in the log (default off).
	LogDomains bool `json:"logDomains"`
	// IdleTimeoutSec overrides the idle connection timeout.
	IdleTimeoutSec int `json:"idleTimeoutSec"`
}

// Settings is everything persisted in settings.json (file 01, section 4).
type Settings struct {
	Version          int      `json:"version"`
	Mode             Mode     `json:"mode"`
	Method           Method   `json:"method"`
	Port             int      `json:"port"`
	AutoPort         bool     `json:"autoPort"`
	StartWithWindows bool     `json:"startWithWindows"`
	ConnectOnLaunch  bool     `json:"connectOnLaunch"`
	FirstRunDone     bool     `json:"firstRunDone"`
	CloseNoticeShown bool     `json:"closeNoticeShown"`
	Advanced         Advanced `json:"advanced"`
}

// DefaultSettings returns the built-in settings.
func DefaultSettings() Settings {
	return Settings{
		Version:  settingsVersion,
		Mode:     DefaultMode,
		Method:   DefaultMethod,
		Port:     ListenPort,
		AutoPort: AutoPort,
		Advanced: Advanced{
			DoHServers: Defaults(),
			Domains:    DefaultDomains(),
		},
	}
}

// SettingsPath returns the settings file path inside the app's config directory.
func SettingsPath(dir string) string {
	return filepath.Join(dir, "settings.json")
}

// Normalize validates and clamps settings in place, filling anything missing
// or out of range with safe values. It returns the normalized copy.
func Normalize(s Settings) Settings {
	if s.Version < settingsVersion {
		s.Version = settingsVersion
	}
	if !s.Mode.Valid() {
		s.Mode = DefaultMode
	}
	if !s.Method.Valid() {
		s.Method = DefaultMethod
	}
	if s.Port < 1 || s.Port > 65535 {
		s.Port = ListenPort
	}
	if s.Advanced.PieceSizeMin < 0 || s.Advanced.PieceSizeMin > 65535 {
		s.Advanced.PieceSizeMin = 0
	}
	if s.Advanced.PieceSizeMax < 0 || s.Advanced.PieceSizeMax > 65535 {
		s.Advanced.PieceSizeMax = 0
	}
	if s.Advanced.PieceSizeMin > s.Advanced.PieceSizeMax && s.Advanced.PieceSizeMax > 0 {
		s.Advanced.PieceSizeMin, s.Advanced.PieceSizeMax = s.Advanced.PieceSizeMax, s.Advanced.PieceSizeMin
	}
	if s.Advanced.PauseMs < 0 || s.Advanced.PauseMs > 1000 {
		s.Advanced.PauseMs = 0
	}
	if s.Advanced.IdleTimeoutSec < 0 || s.Advanced.IdleTimeoutSec > 3600 {
		s.Advanced.IdleTimeoutSec = 0
	}
	s.Advanced.DoHServers = normalizeDoH(s.Advanced.DoHServers)
	s.Advanced.Domains = normalizeDomains(s.Advanced.Domains)
	return s
}

func normalizeDoH(list []DoHServer) []DoHServer {
	if len(list) == 0 {
		return Defaults()
	}
	out := make([]DoHServer, 0, len(list))
	for _, s := range list {
		s.Name = strings.TrimSpace(s.Name)
		s.URL = strings.TrimSpace(s.URL)
		if s.Name == "" || s.URL == "" {
			continue
		}
		u, err := url.Parse(s.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.Contains(s.URL, "{dns}") {
			continue
		}
		ips := make([]string, 0, len(s.Bootstrap))
		for _, b := range s.Bootstrap {
			b = strings.TrimSpace(b)
			if b != "" {
				ips = append(ips, b)
			}
		}
		s.Bootstrap = ips
		out = append(out, s)
	}
	if len(out) == 0 {
		return Defaults()
	}
	return out
}

func normalizeDomains(list []string) []string {
	if len(list) == 0 {
		return DefaultDomains()
	}
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, d := range list {
		d = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(d), ".")))
		if d == "" || seen[d] {
			continue
		}
		if strings.Contains(d, " ") || strings.Contains(d, "/") {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	if len(out) == 0 {
		return DefaultDomains()
	}
	return out
}

// Load reads settings.json from dir. Unknown fields are ignored. A corrupt
// file is renamed settings.json.bad and defaults are returned; the returned
// notice explains that in one line. Missing file means defaults.
func Load(dir string) (s Settings, notice string, err error) {
	s = DefaultSettings()
	path := SettingsPath(dir)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return Normalize(s), "", nil
		}
		return Normalize(s), "", readErr
	}
	var loaded Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		bad := path + ".bad"
		if renameErr := os.Rename(path, bad); renameErr != nil {
			return Normalize(s), "", fmt.Errorf("settings file is corrupt (%v) and could not be renamed: %w", err, renameErr)
		}
		return Normalize(s), "settings file was corrupt; it was renamed to settings.json.bad and defaults were restored", nil
	}
	if loaded.Version > settingsVersion {
		// A future version's file: keep what we understand, ignore the rest.
		loaded.Version = settingsVersion
	}
	return Normalize(loaded), "", nil
}

// Save writes settings atomically to settings.json inside dir
// (temp file then rename). The value written is normalized.
func Save(dir string, s Settings) error {
	s = Normalize(s)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := filepath.Join(dir, ".settings.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, SettingsPath(dir)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// FormatBytes is a helper for diagnostics: sizes in a compact decimal form.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
