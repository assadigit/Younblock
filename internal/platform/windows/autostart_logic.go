package windows

// Autostart manages the Younblock entry under
// HKCU\Software\Microsoft\Windows\CurrentVersion\Run (file 01, section 8).
// The entry is a single named value, so it can never duplicate; when the
// exe moves, the stored path is rewritten on the next launch.

// RunKeyPath is the registry key holding per-user launch entries.
const RunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// AutostartValueName is the one value we own there.
const AutostartValueName = "Younblock"

// MinimizedFlag is appended to the autostart command.
const MinimizedFlag = "--minimized"

// autostartCommand builds the command line stored in the Run key:
// "C:\full path\younblock.exe" --minimized — plain quotes around the path
// (spaces), no Go-style escaping.
func autostartCommand(exePath string) string {
	return "\"" + exePath + "\" " + MinimizedFlag
}

// setAutostarmCommand writes or removes the autostart entry. Re-writing an
// existing entry both deduplicates and handles a moved exe.
func setAutostart(enabled bool, exePath string, key internetSettings) error {
	if !enabled {
		_, exists, err := key.readString(AutostartValueName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		return key.deleteValue(AutostartValueName)
	}
	return key.writeString(AutostartValueName, autostartCommand(exePath))
}

// rewriteIfMoved updates the stored command when the current exe path
// differs from the stored one (file 01, section 8: "on every launch, if
// enabled and the stored path differs from the current exe path, rewrite
// it"). It reports whether a rewrite happened.
func rewriteIfMoved(exePath string, key internetSettings) (bool, error) {
	current, exists, err := key.readString(AutostartValueName)
	if err != nil || !exists {
		return false, err
	}
	want := autostartCommand(exePath)
	if current == want {
		return false, nil
	}
	return true, key.writeString(AutostartValueName, want)
}

// autostartEnabled reads whether our entry exists.
func autostartEnabled(key internetSettings) (bool, error) {
	_, exists, err := key.readString(AutostartValueName)
	return exists, err
}
