package browser

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestArgsReplicatesLegacyLauncher(t *testing.T) {
	got := Args(`C:\Users\Ali\AppData\Local\Younblock\chrome-profile`, 4525)
	want := []string{
		`--user-data-dir=C:\Users\Ali\AppData\Local\Younblock\chrome-profile`,
		"--proxy-server=127.0.0.1:4525",
		"--proxy-bypass-list=<-loopback>",
		"--no-first-run",
		"--disable-gpu",
		"--disable-software-rasterizer",
		"https://www.youtube.com/",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Args mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestArgsUsesTheGivenPort(t *testing.T) {
	got := Args("/tmp/profile", 4527)
	if got[1] != "--proxy-server=127.0.0.1:4527" {
		t.Fatalf("port not applied: %q", got[1])
	}
}

func TestCandidatesFromEnv(t *testing.T) {
	env := func(k string) string {
		switch k {
		case "ProgramFiles":
			return `C:\Program Files`
		case "LocalAppData":
			return `C:\Users\Ali\AppData\Local`
		default:
			return ""
		}
	}
	got := Candidates(env)
	want := []string{
		filepath.Join(`C:\Program Files`, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(`C:\Users\Ali\AppData\Local`, "Google", "Chrome", "Application", "chrome.exe"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestCandidatesDeduplicatesAndSkipsEmpty(t *testing.T) {
	env := func(k string) string {
		if k == "ProgramFiles" || k == "ProgramW6432" {
			return `C:\Program Files`
		}
		return ""
	}
	got := Candidates(env)
	if len(got) != 1 {
		t.Fatalf("expected one deduplicated candidate, got %q", got)
	}
}

func TestFindReturnsFirstExisting(t *testing.T) {
	candidates := []string{"a", "b", "c"}
	existing := map[string]bool{"b": true}
	probe := func(p string) bool { return existing[p] }
	if got := Find(candidates, probe); got != "b" {
		t.Fatalf("Find = %q, want b", got)
	}
}

func TestFindReturnsEmptyWhenNothingExists(t *testing.T) {
	if got := Find([]string{"a", "b"}, func(string) bool { return false }); got != "" {
		t.Fatalf("Find = %q, want empty", got)
	}
}
