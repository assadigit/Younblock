//go:build !windows

package main

// registerEmbeddedFonts is the non-Windows no-op (the embedded Space
// Grotesk weights are a GDI concern; tests and non-Windows hosts never
// paint). See fonts_windows.go for the real one.
func registerEmbeddedFonts() {}
