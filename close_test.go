// close_test.go — pins the D93 close contract: the window's close button
// is a REAL SHUTDOWN unless the user opted into the tray, and on a
// platform without our tray it is the quit whatever the setting says.
// The Windows half of the contract (closeHidesToTray(keep) == keep) is
// one line in tray_windows.go, compile-checked for GOOS=windows by the
// build lane; everything runnable on this host is pinned here.
package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// newCloseTestApp builds a real engine over the stub platform in a temp
// dir, optionally applies a settings document (a full document, exactly
// the way every UI saves), and wraps it in the App shell — no Wails
// context, so hideWindow is a no-op and beforeClose's decision is the
// only thing under test.
func newCloseTestApp(t *testing.T, settingsJSON string) *App {
	t.Helper()
	plat := stub.NewWithDir(t.TempDir())
	eng, err := engine.New(engine.Options{Platform: plat, LogDir: t.TempDir()})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(eng.Shutdown)
	if settingsJSON != "" {
		_ = eng.SaveSettingsJSON(settingsJSON)
	}
	return &App{eng: eng}
}

// optIn reads the persisted close setting back out of the engine, the
// way beforeClose does, so a test can prove its document really landed.
func optIn(t *testing.T, a *App) bool {
	t.Helper()
	var s struct {
		KeepInTrayOnClose bool `json:"keepInTrayOnClose"`
	}
	if err := json.Unmarshal([]byte(a.eng.GetSettingsJSON()), &s); err != nil {
		t.Fatalf("engine settings unreadable: %v", err)
	}
	return s.KeepInTrayOnClose
}

func TestKeepInTrayOnCloseParsing(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want bool
	}{
		{"absent field (every pre-D93 file)", `{"version":4,"port":4525}`, false},
		{"explicit false", `{"keepInTrayOnClose":false}`, false},
		{"opted in", `{"keepInTrayOnClose":true}`, true},
		{"beside other fields", `{"port":4530,"keepInTrayOnClose":true,"mode":"all"}`, true},
		{"garbage document", `{not json`, false},
	}
	for _, c := range cases {
		if got := keepInTrayOnClose(c.doc); got != c.want {
			t.Errorf("%s: keepInTrayOnClose(%s) = %v, want %v", c.name, c.doc, got, c.want)
		}
	}
}

func TestCloseHidesToTrayNowhereToHide(t *testing.T) {
	// This host has no tray build (tray_other.go): closing is the quit
	// even when the user opted in — there is nowhere to hide, and an
	// opted-in setting must never strand a window that cannot close.
	if closeHidesToTray(true) || closeHidesToTray(false) {
		t.Fatal("closeHidesToTray must answer false without a tray, whatever the setting says")
	}
}

func TestBeforeCloseQuitsByDefault(t *testing.T) {
	// The D93 headline: with the shipped defaults (and with every
	// settings.json written before D93), the close button allows the
	// close — the app leaves the taskbar and the shutdown path stops
	// the engine and restores the proxy.
	a := newCloseTestApp(t, "")
	if a.beforeClose(context.Background()) {
		t.Fatal("default settings: the close must be a real quit (D93)")
	}
}

func TestBeforeCloseOptInStillQuitsWithoutTray(t *testing.T) {
	// The user opted in, but this platform has no tray to hide into:
	// the honest answer is still the real quit.
	a := newCloseTestApp(t, `{"keepInTrayOnClose":true}`)
	if !optIn(t, a) {
		t.Fatal("the opt-in document did not reach the engine")
	}
	if a.beforeClose(context.Background()) {
		t.Fatal("with no tray on this platform, even an opted-in close must quit")
	}
}

func TestBeforeCloseQuittingAlwaysAllows(t *testing.T) {
	// An in-flight Quit (tray menu, engine-driven): the close is always
	// allowed so the app can actually exit.
	a := newCloseTestApp(t, `{"keepInTrayOnClose":true}`)
	a.quitting = true
	if a.beforeClose(context.Background()) {
		t.Fatal("a quitting app must allow the close")
	}
}
