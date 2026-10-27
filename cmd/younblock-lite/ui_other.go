//go:build !windows

// ui_other.go — the honest refusal on non-Windows hosts: the lite
// desktop edition is a Windows app (the whole point is the
// dependency-free Win32 window). Linux already has the web edition
// (younblock-linux-*) and the native window edition; keeping this stub
// lets `go test ./cmd/younblock-lite` run everywhere for the
// OS-neutral halves (view.go, app.go) — the same reason the engine
// keeps a stub platform.
package main

import (
        "fmt"
        "os"
)

// runUI refuses honestly instead of pretending.
func runUI(app *App) {
        fmt.Println("The lite desktop edition is a Windows app.")
        fmt.Println("On Linux, use the web edition (younblock-linux-*) or the native window edition.")
        os.Exit(1)
}
