//go:build !windows

// clientsize_other.go — the non-Windows shell runs in a browser during
// development; the window-size correction is a Windows-only concern (D48).
package main

import "fmt"

func ensureDesignClientSize(designW, designH int32) error {
	return fmt.Errorf("not supported on this platform")
}
