// diag.go — the lite edition's run log (D117, the runs-anywhere round).
//
// Why it exists: v1.6.12 died at launch on the owner's machine with
// not one visible word — a -H windowsgui process has no console, so a
// panic anywhere before the message loop simply vanished. From 1.6.13
// every launch writes younblock-lite.log (truncated per run, so it
// never grows) as close to the exe as permissions allow — the exe's
// own folder first, then the user's cache dir, then the temp dir —
// and every startup milestone, fallback door and panic lands in it.
// A startup failure additionally raises a real MessageBox with the
// error and the log's path, so "it didn't run" always comes with a
// sentence that says why.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/assadigit/Younblock/internal/config"
)

var (
	diagMu   sync.Mutex
	diagPath string
)

// diagInit opens this run's log: the first writable candidate wins.
// Called from main before anything can fail.
func diagInit() {
	var candidates []string
	if exe, err := os.Executable(); err == nil && filepath.Dir(exe) != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "younblock-lite.log"))
	}
	if cache, err := os.UserCacheDir(); err == nil {
		candidates = append(candidates, filepath.Join(cache, "YounblockLite", "younblock-lite.log"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "younblock-lite.log"))

	for _, p := range candidates {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			continue
		}
		if _, err := f.WriteString(diagHeader()); err != nil {
			f.Close()
			continue
		}
		f.Close()
		diagMu.Lock()
		diagPath = p
		diagMu.Unlock()
		return
	}
}

// diagHeader is the log's first line: what launched, where from, on
// what. One line, human-readable, enough to identify the machine.
func diagHeader() string {
	exe := "unknown"
	if e, err := os.Executable(); err == nil {
		exe = e
	}
	return fmt.Sprintf("%s %s lite %s log — %s/%s — %s — %s\n",
		config.AppName, "desktop", config.AppVersion,
		runtime.GOOS, runtime.GOARCH, windowsVersionString(), exe)
}

// diagLog appends one timestamped line. Safe from any goroutine; a
// no-op when no log could be opened (nothing may ever panic in here).
func diagLog(format string, args ...any) {
	diagMu.Lock()
	defer diagMu.Unlock()
	if diagPath == "" {
		return
	}
	f, err := os.OpenFile(diagPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("2006-01-02 15:04:05.000")+" "+format+"\n", args...)
}

// diagWhere returns the log's path for the failure dialogs (empty
// when logging was impossible).
func diagWhere() string {
	diagMu.Lock()
	defer diagMu.Unlock()
	return diagPath
}
