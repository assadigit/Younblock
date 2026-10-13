// Package browser holds the pure logic for the optional "open a dedicated
// Chrome session" feature (user-requested; DECISIONS.md D40): the command
// line and the Chrome lookup rules. The actual process spawn is shell
// behavior and lives in the app package, so everything here is
// platform-neutral and testable (file 01, section 3, rule 3).
package browser

import (
	"path/filepath"
	"strconv"
	"strings"
)

// ProfileDirName is the folder (inside the app's config dir) that holds the
// dedicated Chrome profile, so the session never touches the user's normal
// Chrome data.
const ProfileDirName = "chrome-profile"

// StartURL is the page the dedicated session opens.
const StartURL = "https://www.youtube.com/"

// Args builds the Chrome command line for a dedicated session through the
// local proxy. The flags replicate the old app's launcher exactly
// (legacy/frontend.py, recorded in DECISIONS.md D2/D40):
//
//	--user-data-dir=<profile>            separate profile, so the flags apply
//	                                     while the user's Chrome keeps running
//	--proxy-server=127.0.0.1:<port>      send everything to this app
//	--proxy-bypass-list=<-loopback>      keep local addresses direct
//	--no-first-run                       no first-run tabs
//	--disable-gpu, --disable-software-rasterizer   the old app's stability flags
func Args(profileDir string, port int) []string {
	return []string{
		"--user-data-dir=" + profileDir,
		"--proxy-server=127.0.0.1:" + strconv.Itoa(port),
		"--proxy-bypass-list=<-loopback>",
		"--no-first-run",
		"--disable-gpu",
		"--disable-software-rasterizer",
		StartURL,
	}
}

// Candidates lists the Chrome executable paths to probe, in order, built
// from the standard environment variables. env returns the value of a
// variable ("" when unset); tests inject a fake.
func Candidates(env func(string) string) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(dir string) {
		if dir == "" {
			return
		}
		p := filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe")
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, v := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		add(env(v))
	}
	add(env("LocalAppData"))
	return out
}

// Find returns the first candidate that exists (probe checks one path;
// tests inject a fake). It returns "" when Chrome is not found.
func Find(candidates []string, probe func(string) bool) string {
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" && probe(c) {
			return c
		}
	}
	return ""
}
