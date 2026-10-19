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

// settingsVersion is the settings file schema version. v2 (M6, doc 04
// §9) added the per-app allow-list `apps`; v3 (D86) was the one-time
// migration of the debug.1-era empty list to the YouTube default: a
// v1/v2 file's absent-or-empty apps field was the SHIPPED default
// under D83 ("no allow-list default"), never a user choice, so it
// migrated to the D86 default [com.google.android.youtube]. v4 (D87)
// extends the default to the four browsers (Ali's second ruling) and
// migrates the v3-era shipped state with it: a v3 file still holding
// EXACTLY the D86 default was written by the debug.2 build itself
// (fresh default or the v2-empty migration), never curated by its
// owner, so it upgrades to the D87 default. A v3 file's empty list
// (they removed the last entry) and every custom list — v1/v2 lists
// with entries included — are the user's own and stay untouched. The
// NO_TARGET_APPS door owns what happens at connect, never a
// restore-to-default.
//
// v5 (M8-z, ADR-0024) adds `fragmentAllRouted` — fragment EVERY TLS
// first record of a routed app's flows, not just the YouTube-list
// hosts — with a PLATFORM-DEPENDENT default: true on Android (the
// per-app allow-list means every flow the TUN hands the door belongs
// to a routed app, and the debug.16 evidence says the un-fragmented
// plain cohort is where the first video's minute went), false on every
// desktop (behavior unchanged there — hard constraint 5). The default
// is applied by LoadFor when it knows the platform: a fresh install
// (no file) and any pre-v5 file on Android both start fragmented-all;
// a v5 file records the owner's explicit choice either way. v5 also
// carries the debug-tester LogHostnames default (ADR-0027 §8.1): ONLY
// a DebugTesterBuild (the -ldflags -X lane) on Android turns
// `logDomains` on for the shipped state — a release build never does,
// and an owner's saved logDomains survives verbatim in every build.
const settingsVersion = 5

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
	Version          int    `json:"version"`
	Mode             Mode   `json:"mode"`
	Method           Method `json:"method"`
	Port             int    `json:"port"`
	AutoPort         bool   `json:"autoPort"`
	StartWithWindows bool   `json:"startWithWindows"`
	ConnectOnLaunch  bool   `json:"connectOnLaunch"`
	// FragmentAllRouted (M8-z, ADR-0024, schema v5): fragment the first
	// TLS record of EVERY non-direct-TLD host a routed app sends — not
	// just the YouTube domain list. Default true on Android (LoadFor
	// applies it), false on every desktop edition; the budgeted shape
	// (first-byte budget, salvage, race, dead-IP penalty, tiered stall
	// kill) stays scoped to the YouTube-list hosts exactly as before —
	// fragmentation for everything, the expensive machinery only where
	// it pays. Desktop editions persist the field but the engine wires
	// it through like any setting; the desktop default (false) keeps
	// desktop behavior byte-for-byte.
	FragmentAllRouted bool `json:"fragmentAllRouted"`
	// KeepInTrayOnClose (D93): closing the window hides into the
	// system tray instead of quitting — OPT-IN since D93. The shipped
	// default is false: the close button is a real shutdown, so the
	// app leaves the taskbar unless its owner turned this on in
	// Settings. Every file written before D93 lacks the field, which
	// loads as exactly that default — the behavior the user demanded,
	// not a choice the file ever recorded, so no schema-version
	// migration was owed when it landed (the D93-era file schema stays
	// as it was; v5 below is owed to FragmentAllRouted alone).
	KeepInTrayOnClose bool `json:"keepInTrayOnClose"`
	// OpenBrowserOnConnect opens a dedicated Chrome session (separate
	// profile, proxy flags) when the engine turns on. Default off.
	OpenBrowserOnConnect bool `json:"openBrowserOnConnect"`
	FirstRunDone         bool `json:"firstRunDone"`
	CloseNoticeShown     bool `json:"closeNoticeShown"`
	// Apps is the Android per-app allow-list (ADR-0003, schema v2,
	// default D87): the package names whose TCP+DNS the VpnService
	// routes through the engine. The list starts with the YouTube
	// app and the Chrome, Firefox, Opera and Brave browsers (Ali's
	// rulings D86 + D87); removing every entry is a user choice that
	// persists — the M7 NO_TARGET_APPS door (D81) refuses that connect
	// closed. Desktop editions persist the field but never read it —
	// one settings.json serves every edition, and the classic screen's
	// spread-based saves round-trip it untouched.
	Apps     []string `json:"apps"`
	Advanced Advanced `json:"advanced"`
}

// DefaultSettings returns the built-in settings.
func DefaultSettings() Settings {
	return Settings{
		Version:  settingsVersion,
		Mode:     DefaultMode,
		Method:   DefaultMethod,
		Port:     ListenPort,
		AutoPort: AutoPort,
		Apps:     DefaultApps(),
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
	s.Apps = normalizeApps(s.Apps)
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

// maxApps caps the per-app allow-list: every entry becomes one
// addAllowedApplication call at TUN establish; a runaway list would slow
// the establish for no product value (the declared scope is a handful of
// apps). Entries beyond the cap are dropped in first-seen order.
const maxApps = 32

// normalizeApps validates the per-app allow-list (schema v2, ADR-0003):
// trimmed, shape-checked Android package names, de-duplicated in
// first-seen order, capped. An empty result is KEPT — under D87 the
// fresh default is five apps (YouTube + the four browsers), so a list
// that arrives empty (or all-invalid) here was EMPTIED by its owner
// (the UI row, a partial save payload) and empty is the honest
// persisted state; the M7 NO_TARGET_APPS door (D81) fails that connect
// closed. The empty slice is non-nil so it marshals as [] — an
// explicit, honest empty list, never a route-everything fallback (a
// VpnService.Builder with zero allowed applications would route EVERY
// app, which is why the Kotlin side refuses to establish on an empty
// list instead of dropping the per-app restriction). The never-restored
// rule stands: the D87 default lives in DefaultApps/DefaultSettings and
// the v4 Load migration, never in this validator.
func normalizeApps(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, pkg := range list {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] || !validAndroidPackage(pkg) {
			continue
		}
		if len(out) == maxApps {
			break
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	return out
}

// validAndroidPackage reports whether s looks like an Android package
// name: dot-separated segments of letters, digits and underscores, each
// segment starting with a letter, at least two segments — the shape
// Android's package world accepts (the VpnService builder rejects a name
// it cannot resolve at establish time; validating here keeps the file
// honest and the UI's feedback early). Case is preserved: package names
// are case-sensitive on Android.
func validAndroidPackage(s string) bool {
	if len(s) < 3 || len(s) > 256 {
		return false
	}
	segments := strings.Split(s, ".")
	if len(segments) < 2 {
		return false
	}
	for _, seg := range segments {
		if len(seg) == 0 {
			return false
		}
		for i, r := range seg {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
				// a letter is valid anywhere in a segment
			case r >= '0' && r <= '9', r == '_':
				if i == 0 {
					return false // a segment starts with a letter
				}
			default:
				return false
			}
		}
	}
	return true
}

// d86DefaultApps is the shipped default of the v3 era (D86): the
// YouTube app alone. Kept as its own value — not a DefaultApps prefix
// assumption — so the v4 migration's "was this file ever customized?"
// test stays honest even if the D87 default's order changes later.
var d86DefaultApps = []string{"com.google.android.youtube"}

// isD86DefaultApps reports whether list is exactly the D86-era default,
// in order, with nothing added: the state the debug.2 build itself wrote
// (the fresh v3 default, or the v2-empty migration's result). Only that
// state upgrades to the D87 default; anything else on a v3 file was
// touched by its owner and survives verbatim.
func isD86DefaultApps(list []string) bool {
	if len(list) != len(d86DefaultApps) {
		return false
	}
	for i, pkg := range list {
		if pkg != d86DefaultApps[i] {
			return false
		}
	}
	return true
}

// Load reads settings.json from dir with no platform awareness (the
// desktop editions and the tests): no Android v5 defaults are applied.
// See LoadFor for the platform-aware variant.
func Load(dir string) (s Settings, notice string, err error) {
	return LoadFor(dir, "")
}

// LoadFor reads settings.json from dir with the loading platform's name
// ("android" for the mobile facade's engine; anything else — including
// the empty string the desktop Load passes — is a non-Android build).
// Unknown fields are ignored. A corrupt file is renamed settings.json.bad
// and defaults are returned; the returned notice explains that in one
// line. Missing file means defaults — with the platform's v5 defaults
// applied (Android: fragmentAllRouted on; a debug-tester build also
// logs hostnames by default, ADR-0027 §8.1). A pre-v5 file on Android
// migrates the same way once: the absent field was never a choice the
// file recorded. A v5 file is the owner's word — fragmentAllRouted and
// logDomains are read verbatim whatever the build.
func LoadFor(dir string, platform string) (s Settings, notice string, err error) {
	android := platform == "android"
	s = DefaultSettings()
	path := SettingsPath(dir)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			applyPlatformDefaults(&s, android)
			return Normalize(s), "", nil
		}
		applyPlatformDefaults(&s, android)
		return Normalize(s), "", readErr
	}
	var loaded Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		bad := path + ".bad"
		if renameErr := os.Rename(path, bad); renameErr != nil {
			applyPlatformDefaults(&s, android)
			return Normalize(s), "", fmt.Errorf("settings file is corrupt (%v) and could not be renamed: %w", err, renameErr)
		}
		applyPlatformDefaults(&s, android)
		return Normalize(s), "settings file was corrupt; it was renamed to settings.json.bad and defaults were restored", nil
	}
	if loaded.Version > settingsVersion {
		// A future version's file: keep what we understand, ignore the rest.
		loaded.Version = settingsVersion
	}
	// The v4 migration (D87), the same one-time mechanism the v3 one
	// used, in two branches:
	//
	//   - Every v1/v2 file whose apps list is absent or empty carries
	//     the SHIPPED default of its era (D83's apps:[] — debug.1's
	//     entry path refused friendly names, so "empty" in a v2 file
	//     almost always means "never successfully configured"). It
	//     now fills the D87 default directly.
	//   - A v3 file still holding EXACTLY the D86 default
	//     [com.google.android.youtube] carries the state the debug.2
	//     build itself wrote — the fresh default or the v2-empty
	//     migration — never a curation (a v1/v2 file with entries was
	//     hand-typed under a refusing entry path, and is kept; a v3
	//     list that differs from the era default was edited). It
	//     upgrades to the D87 default, which is exactly Ali's
	//     requested outcome on the one device carrying debug.2.
	//
	// A v3 empty (the user removed the last entry — D86's agency
	// ruling) and every other custom list are left alone, as is any
	// v4+ file: under its own schema version, empty means agency.
	if loaded.Version < settingsVersion {
		if len(loaded.Apps) == 0 && loaded.Version < 3 {
			loaded.Apps = DefaultApps()
		} else if loaded.Version == 3 && isD86DefaultApps(loaded.Apps) {
			loaded.Apps = DefaultApps()
		}
		// The v5 migration (M8-z, ADR-0024/0027): only on Android, only
		// the state no owner ever chose. fragmentAllRouted was absent
		// from every pre-v5 file, so its false there is the shipped
		// default, never a decision — Android's default is true.
		// logDomains EXISTED in v4 files, so it is NOT migrated here:
		// only a DebugTesterBuild's fresh-install default turns it on
		// (applyPlatformDefaults on the missing-file path), never a
		// file that recorded its owner's off.
		if android && loaded.Version < 5 {
			loaded.FragmentAllRouted = true
		}
	}
	return Normalize(loaded), "", nil
}

// applyPlatformDefaults fills the loading platform's shipped defaults
// for fields a fresh install cannot record (M8-z, ADR-0024/0027):
// Android starts fragmentAllRouted on; a DebugTesterBuild (the
// -ldflags lane, never a release) also starts with hostnames in the
// diagnostic log — the visible "DEBUG BUILD" notice line rides in the
// report itself. Non-Android platforms keep DefaultSettings verbatim.
func applyPlatformDefaults(s *Settings, android bool) {
	if !android {
		return
	}
	s.FragmentAllRouted = true
	if IsDebugTesterBuild() {
		s.Advanced.LogDomains = true
	}
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
