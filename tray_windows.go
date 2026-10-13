//go:build windows

// tray_windows.go runs the system tray beside the Wails event loop using
// energye/systray's external-loop mode (file 01, section 9). Three icons
// differ by shape (outlined ring / filled ring / ring with a dot); the
// tooltip carries Off|On|Starting…|Check needed; left click shows or hides
// the window; the right-click menu has Turn on/off, Open Younblock, Quit.
package main

import (
	_ "embed"

	"github.com/assadigit/Younblock/internal/engine"

	"github.com/energye/systray"
	"golang.org/x/sys/windows/registry"
)

//go:embed assets/tray-off.ico
var trayOffIcon []byte

//go:embed assets/tray-on.ico
var trayOnIcon []byte

//go:embed assets/tray-attention.ico
var trayAttentionIcon []byte

//go:embed assets/tray-off-light.ico
var trayOffLightIcon []byte

//go:embed assets/tray-on-light.ico
var trayOnLightIcon []byte

//go:embed assets/tray-attention-light.ico
var trayAttentionLightIcon []byte

// tray holds the menu items we mutate with the state.
type tray struct {
	app       *App
	toggle    *systray.MenuItem
	lightBar  bool
	lastPhase engine.Phase
}

var theTray *tray

// startTray boots the tray loop.
func (a *App) startTray() {
	theTray = &tray{app: a, lightBar: appsUseLightTheme()}
	go func() {
		start, _ := systray.RunWithExternalLoop(onTrayReady, onTrayExit)
		start()
	}()
	// Apply the icon for whatever state the engine is already in.
	a.trayApplyState(a.eng.GetState())
}

// stopTray tears the tray down on shutdown.
func (a *App) stopTray() {
	systray.Quit()
}

func onTrayReady() {
	t := theTray
	if t == nil {
		return
	}
	systray.SetTooltip("Younblock: Off")
	t.applyIcon(engine.PhaseOff)

	mOpen := systray.AddMenuItem("Open Younblock", "Show the Younblock window")
	t.toggle = systray.AddMenuItem("Turn on", "Turn the proxy on or off")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop Younblock completely")

	mOpen.Click(func() { t.app.showWindow() })
	t.toggle.Click(func() {
		t.app.Toggle()
	})
	mQuit.Click(func() { t.app.Quit() })

	// Left click: show or hide the window.
	systray.SetOnClick(func(_ systray.IMenu) {
		t.app.showWindow()
	})
}

func onTrayExit() {
	// Nothing to clean up; the engine shutdown path handles the rest.
}

// trayOnEvent mirrors engine events into tray updates.
func (a *App) trayOnEvent(name, payload string) {
	if name != "state" {
		return
	}
	var st engine.State
	if err := jsonUnmarshalStrict(payload, &st); err != nil {
		return
	}
	a.trayApplyState(st)
}

// trayApplyState updates icon, tooltip and menu label for a state.
func (a *App) trayApplyState(st engine.State) {
	t := theTray
	if t == nil {
		return
	}
	t.applyIcon(st.Phase)
	systray.SetTooltip("Younblock: " + trayStateWord(st))
	if t.toggle != nil {
		label := "Turn on"
		if st.Phase == engine.PhaseOn || st.Phase == engine.PhaseDegraded {
			label = "Turn off"
		}
		if st.Phase == engine.PhaseConnecting || st.Phase == engine.PhaseStopping {
			label = "Turn off"
		}
		t.toggle.SetTitle(label)
	}
}

func (t *tray) applyIcon(phase engine.Phase) {
	var icon []byte
	switch phase {
	case engine.PhaseOn:
		if t.lightBar {
			icon = trayOnLightIcon
		} else {
			icon = trayOnIcon
		}
	case engine.PhaseDegraded:
		if t.lightBar {
			icon = trayAttentionLightIcon
		} else {
			icon = trayAttentionIcon
		}
	default:
		if t.lightBar {
			icon = trayOffLightIcon
		} else {
			icon = trayOffIcon
		}
	}
	systray.SetIcon(icon)
}

// trayStateWord maps the phase to the tooltip word (file 02, section 5).
func trayStateWord(st engine.State) string {
	switch st.Phase {
	case engine.PhaseOn:
		return "On"
	case engine.PhaseConnecting:
		return "Starting…"
	case engine.PhaseDegraded:
		return "Check needed"
	case engine.PhaseStopping:
		return "Off"
	default:
		return "Off"
	}
}

// appsUseLightTheme reports whether Windows apps (and the taskbar) use the
// light theme, so the darker icon variants stay readable.
func appsUseLightTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return v != 0
}
