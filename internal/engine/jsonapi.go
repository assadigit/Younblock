package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/assadigit/Younblock/internal/config"
)

// appInfo is the GetAppInfo payload (file 01, section 10).
type appInfo struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Platform     string       `json:"platform"`
	Capabilities Capabilities `json:"capabilities"`
	ConfigPath   string       `json:"configPath"`
	LogDir       string       `json:"logDir"`
}

// apiError wraps method results that can fail with a typed code.
type apiError struct {
	Error *string `json:"error"`
}

func errResult(code string) string {
	if code == "" {
		return `{"error":null}`
	}
	b, _ := json.Marshal(code)
	return fmt.Sprintf(`{"error":%s}`, b)
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// GetStateJSON returns the state as JSON (the UI-facing API shape).
func (e *Engine) GetStateJSON() string { return marshalJSON(e.GetState()) }

// StartJSON starts connecting and returns immediately.
func (e *Engine) StartJSON() string {
	if err := e.Start(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// StopJSON stops the engine.
func (e *Engine) StopJSON() string {
	if err := e.Stop(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// ToggleJSON toggles on/off.
func (e *Engine) ToggleJSON() string {
	if err := e.Toggle(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// GetSettingsJSON returns the normalized settings.
func (e *Engine) GetSettingsJSON() string {
	e.mu.Lock()
	s := e.settings
	e.mu.Unlock()
	return marshalJSON(s)
}

// SaveSettingsJSON applies a settings JSON payload, persists it, and applies
// mode/method changes immediately while On. Returns the normalized settings
// or a typed error.
func (e *Engine) SaveSettingsJSON(payload string) string {
	var incoming config.Settings
	if err := json.Unmarshal([]byte(payload), &incoming); err != nil {
		return errResult("UNKNOWN")
	}
	normalized := config.Normalize(incoming)

	e.mu.Lock()
	prev := e.settings
	e.settings = normalized
	running := e.server != nil
	e.mu.Unlock()

	if err := config.Save(e.platform.ConfigDir(), normalized); err != nil {
		e.logger.Error("saving settings failed", "reason", errKind(err))
	}

	// Apply immediately while On (file 01, section 10).
	if running {
		if normalized.Mode != prev.Mode {
			e.server.SetPACData(normalized.Mode, normalized.Advanced.Domains)
			if err := e.platform.Attach(e.port, normalized); err != nil {
				e.logger.Warn("re-attach after mode change failed", "reason", errKind(err))
			}
		}
		if normalized.Method != prev.Method || settingsFragmentChanged(prev, normalized) {
			e.server.SetFragment(e.currentPreset(), e.currentFragmentConfig())
			e.resolver.Load().SetFragment(e.currentPreset(), e.currentFragmentConfig())
		}
	}
	// Autostart is applied whenever it flips, on or off.
	if normalized.StartWithWindows != prev.StartWithWindows {
		if err := e.platform.SetAutostart(normalized.StartWithWindows); err != nil {
			e.logger.Warn("autostart change failed", "reason", errKind(err))
		}
	}
	// A changed secure-DNS list swaps in a resolver bound to the new
	// servers (D44): the change used to only clear the cache while the
	// old servers kept being asked until a restart.
	if !sameDoHList(prev.Advanced.DoHServers, normalized.Advanced.DoHServers) {
		e.swapResolver(normalized.Advanced.DoHServers)
		e.logger.Info("secure-dns list changed; a new resolver is active")
	}
	e.emitState()
	return marshalJSON(normalized)
}

// sameDoHList reports whether two server lists are equal.
func sameDoHList(a, b []config.DoHServer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].URL != b[i].URL {
			return false
		}
		if len(a[i].Bootstrap) != len(b[i].Bootstrap) {
			return false
		}
		for j := range a[i].Bootstrap {
			if a[i].Bootstrap[j] != b[i].Bootstrap[j] {
				return false
			}
		}
	}
	return true
}

func settingsFragmentChanged(a, b config.Settings) bool {
	return a.Advanced.PieceSizeMin != b.Advanced.PieceSizeMin ||
		a.Advanced.PieceSizeMax != b.Advanced.PieceSizeMax ||
		a.Advanced.PauseMs != b.Advanced.PauseMs
}

// RestoreWindowsProxyJSON restores the system proxy state (Help button).
func (e *Engine) RestoreWindowsProxyJSON() string {
	if err := e.platform.RestoreProxy(); err != nil {
		e.logger.Error("restore proxy failed", "reason", errKind(err))
		return errResult("PROXY_SETTING_FAILED")
	}
	e.logger.Info("internet settings restored")
	return errResult("")
}

// GetAppInfoJSON returns app metadata.
func (e *Engine) GetAppInfoJSON() string {
	dir := e.platform.ConfigDir()
	info := appInfo{
		Name:         config.AppName,
		Version:      e.opts.Version,
		Platform:     e.platform.Name(),
		Capabilities: e.platform.Capabilities(),
		ConfigPath:   config.SettingsPath(dir),
		LogDir:       e.logDir(),
	}
	return marshalJSON(info)
}

// logDir returns the log directory path.
func (e *Engine) logDir() string {
	if e.opts.LogDir != "" {
		return e.opts.LogDir
	}
	return filepath.Join(e.platform.ConfigDir(), "logs")
}

// GetDiagnosticReport returns the plain-text, redacted diagnostic report
// (file 03, R20/R21).
func (e *Engine) GetDiagnosticReportJSON() string {
	return marshalJSON(map[string]string{"report": e.GetDiagnosticReport()})
}

// GetUsageJSON returns the round-3 Data-usage summary the menu renders.
// A store that could not be opened answers a typed error — the sheet then
// shows its honest "Could not read the usage history" line and the rest
// of the menu keeps working.
func (e *Engine) GetUsageJSON() string {
	if e.usage == nil {
		return errResult("USAGE_UNAVAILABLE")
	}
	return marshalJSON(e.usage.Summary(e.opts.Now()))
}

// ClearUsageJSON empties the history and the Last-session line: the next
// state event carries an explicit lastSession:null, which is exactly how
// the screen knows to drop the line (the frontend maps an absent field to
// "keep", a present null to "gone").
func (e *Engine) ClearUsageJSON() string {
	if e.usage != nil {
		e.usage.Clear()
	}
	e.mu.Lock()
	e.lastSess = nil
	e.mu.Unlock()
	e.emitState()
	return errResult("")
}

// GetDiagnosticReport builds the report text. It contains only: app version,
// platform, settings summary (no custom lists), self-test status summary,
// counters, measured sleep and the last 100 log lines.
func (e *Engine) GetDiagnosticReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s diagnostic report\n", config.AppName, e.opts.Version)
	fmt.Fprintf(&b, "platform: %s\n", e.platform.Name())
	fmt.Fprintf(&b, "generated: %s\n", e.opts.Now().Format(time.RFC3339))
	s := e.GetState()
	fmt.Fprintf(&b, "state: %s (detail %q)\n", s.Phase, s.Detail)
	fmt.Fprintf(&b, "mode: %s method: %s port: %d\n", s.Mode, s.Method, s.Port)
	if s.LatencyMs != nil {
		fmt.Fprintf(&b, "response time: %d ms\n", *s.LatencyMs)
	}
	if s.Fail != nil {
		fmt.Fprintf(&b, "last failed check: %s (%s)\n", s.Fail.Step, s.Fail.Code)
	}
	fmt.Fprintf(&b, "measured sleep for 1ms request: %d ms\n", e.measuredSleep.Milliseconds())
	d := e.stats.Diagnostic()
	fmt.Fprintf(&b, "counters: conns total %d, live %d, errors %d, panics %d\n",
		d.ConnsTotal, d.ConnsLive, d.ConnErrors, d.Panics)
	fmt.Fprintf(&b, "counters: dns ok %d, fail %d, up %s, down %s\n",
		d.DnsOK, d.DnsFail, config.FormatBytes(d.BytesUp), config.FormatBytes(d.BytesDown))
	fmt.Fprintf(&b, "%s\n", e.lastSelfTestSummary())
	lines := e.logger.Ring.Lines()
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	fmt.Fprintf(&b, "last %d log lines:\n", len(lines))
	for _, l := range lines {
		fmt.Fprintf(&b, "  %s\n", l)
	}
	return b.String()
}

// OpenLogsFolderJSON returns the log folder path; opening a file browser is
// shell behavior (DECISIONS.md) so the UI layer handles it.
func (e *Engine) OpenLogsFolderJSON() string {
	return marshalJSON(map[string]string{"path": e.logDir()})
}

// QuitJSON quits the app.
func (e *Engine) QuitJSON() string {
	go func() {
		time.Sleep(50 * time.Millisecond) // let the reply reach the UI first
		e.Quit()
	}()
	return errResult("")
}
