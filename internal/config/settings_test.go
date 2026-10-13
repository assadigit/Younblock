package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultsMatchSpec(t *testing.T) {
	s := DefaultSettings()
	if AppName != "Younblock" {
		t.Errorf("AppName = %q, want Younblock", AppName)
	}
	if ListenHost != "127.0.0.1" {
		t.Errorf("ListenHost = %q, want 127.0.0.1", ListenHost)
	}
	if s.Port != 4525 {
		t.Errorf("default port = %d, want 4525", s.Port)
	}
	if !s.AutoPort {
		t.Error("AutoPort must default to true")
	}
	if s.Mode != ModeYouTube {
		t.Errorf("default mode = %q, want youtube", s.Mode)
	}
	if s.Method != MethodAuto {
		t.Errorf("default method = %q, want auto", s.Method)
	}
	if MaxConcurrentConnections != 1024 {
		t.Errorf("MaxConcurrentConnections = %d, want 1024", MaxConcurrentConnections)
	}
	if IdleTimeout != 300*1e9 { // 300 s in nanoseconds
		t.Errorf("IdleTimeout = %v, want 300s", IdleTimeout)
	}
	if DialTimeoutPerIP != 5*1e9 || DialTimeoutOverall != 12*1e9 {
		t.Errorf("dial timeouts = %v/%v, want 5s/12s", DialTimeoutPerIP, DialTimeoutOverall)
	}
	if len(s.Advanced.DoHServers) != 3 {
		t.Fatalf("default DoH servers = %d, want 3", len(s.Advanced.DoHServers))
	}
	if s.Advanced.DoHServers[0].Name != "RethinkDNS" ||
		s.Advanced.DoHServers[0].URL != "https://sky.rethinkdns.com/?dns={dns}" ||
		s.Advanced.DoHServers[0].Bootstrap[0] != "172.67.162.27" {
		t.Errorf("primary DoH server = %+v, want RethinkDNS with bootstrap 172.67.162.27", s.Advanced.DoHServers[0])
	}
	want := []string{"youtube.com", "youtu.be", "googlevideo.com", "ytimg.com", "ggpht.com",
		"youtube-nocookie.com", "youtubei.googleapis.com"}
	if strings.Join(s.Advanced.Domains, ",") != strings.Join(want, ",") {
		t.Errorf("default domains = %v, want %v", s.Advanced.Domains, want)
	}
	if StartWithWindows || ConnectOnLaunch {
		t.Error("startWithWindows and connectOnLaunch must default to false")
	}
	if BlockPrivateTargets != true {
		t.Error("BlockPrivateTargets must default to true")
	}
	if LogRingLines != 500 || LogFileSizeLimit != 2*1024*1024 || LogFileCount != 3 {
		t.Errorf("logging defaults wrong: ring=%d size=%d count=%d", LogRingLines, LogFileSizeLimit, LogFileCount)
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	dir := t.TempDir()
	s, notice, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if notice != "" {
		t.Errorf("notice = %q, want empty", notice)
	}
	if !reflect.DeepEqual(s, DefaultSettings()) {
		t.Errorf("settings = %+v, want defaults", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := DefaultSettings()
	s.Mode = ModeAll
	s.Method = MethodStrong
	s.Port = 4530
	s.FirstRunDone = true
	s.Advanced.LogDomains = true
	s.Advanced.PauseMs = 7
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, notice, err := Load(dir)
	if err != nil || notice != "" {
		t.Fatalf("Load: %v notice=%q", err, notice)
	}
	if got.Mode != ModeAll || got.Method != MethodStrong || got.Port != 4530 ||
		!got.FirstRunDone || got.Advanced.LogDomains != true || got.Advanced.PauseMs != 7 {
		t.Errorf("round trip lost values: %+v", got)
	}
	// The file must be the only artifact: no temp leftovers.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Errorf("dir contains %v, want only settings.json", entries)
	}
}

func TestLoadCorruptFileRenamedToBad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, notice, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if notice == "" || !strings.Contains(notice, "settings.json.bad") {
		t.Errorf("notice = %q, want a one-line corrupt-file notice", notice)
	}
	if !reflect.DeepEqual(s, DefaultSettings()) {
		t.Errorf("corrupt file must load defaults, got %+v", s)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json.bad")); err != nil {
		t.Errorf("settings.json.bad missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Errorf("settings.json should be gone, stat err = %v", err)
	}
}

func TestLoadUnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	content := `{"version":1,"mode":"all","port":"not a number","futureField":true,"method":"legacy"}`
	// port is the wrong type: json errors on the whole object would corrupt it,
	// so instead use a valid file with an unknown field and a bad port value that
	// json can still decode (string vs int fails the whole unmarshal, which is
	// the corrupt path tested above; here we test unknown fields only).
	content = `{"version":1,"mode":"all","method":"legacy","futureField":true}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Mode != ModeAll || s.Method != MethodLegacy {
		t.Errorf("known fields lost: %+v", s)
	}
}

func TestLoadFutureVersionClamps(t *testing.T) {
	dir := t.TempDir()
	content := `{"version":99,"mode":"manual"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Version != settingsVersion {
		t.Errorf("version = %d, want %d", s.Version, settingsVersion)
	}
	if s.Mode != ModeManual {
		t.Errorf("mode = %q, want manual (known field kept)", s.Mode)
	}
}

func TestNormalizeClampsAndFills(t *testing.T) {
	s := DefaultSettings()
	s.Port = 99999
	s.Mode = "bogus"
	s.Method = "bogus"
	s.Advanced.PauseMs = 5000
	s.Advanced.IdleTimeoutSec = -5
	s.Advanced.PieceSizeMin = 500
	s.Advanced.PieceSizeMax = 100
	s.Advanced.DoHServers = []DoHServer{{Name: "", URL: "https://x/{dns}"}, {Name: "Y", URL: "no-url"}}
	s.Advanced.Domains = []string{"  YouTube.COM. ", "good.example", "good.example", "has space"}
	n := Normalize(s)
	if n.Port != ListenPort {
		t.Errorf("port = %d, want default %d", n.Port, ListenPort)
	}
	if n.Mode != DefaultMode || n.Method != DefaultMethod {
		t.Errorf("mode/method = %q/%q, want defaults", n.Mode, n.Method)
	}
	if n.Advanced.PauseMs != 0 || n.Advanced.IdleTimeoutSec != 0 {
		t.Errorf("pause/idle = %d/%d, want 0/0", n.Advanced.PauseMs, n.Advanced.IdleTimeoutSec)
	}
	if n.Advanced.PieceSizeMin != 100 || n.Advanced.PieceSizeMax != 500 {
		t.Errorf("piece sizes not swapped: %d/%d", n.Advanced.PieceSizeMin, n.Advanced.PieceSizeMax)
	}
	if len(n.Advanced.DoHServers) != len(Defaults()) {
		t.Errorf("invalid DoH list must fall back to defaults, got %v", n.Advanced.DoHServers)
	}
	if strings.Join(n.Advanced.Domains, ",") != "youtube.com,good.example" {
		t.Errorf("domains = %v, want [youtube.com good.example]", n.Advanced.Domains)
	}
}

func TestSaveAtomicNoTempLeftBehindOnFailure(t *testing.T) {
	// A directory used as the file target makes the rename fail.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "settings.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, DefaultSettings()); err == nil {
		t.Fatal("Save should fail when settings.json is a directory")
	}
	if _, err := os.Stat(filepath.Join(dir, ".settings.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}
}
