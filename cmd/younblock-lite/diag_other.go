//go:build !windows

// diag_other.go — the non-Windows half of the run log: `go test` and
// the honest refusal path keep compiling, with the version line
// answered from the runtime and failures reported on stderr.
package main

import (
	"fmt"
	"os"
	"runtime"
)

func windowsVersionString() string { return runtime.GOOS }

func diagFatal(err any) {
	fmt.Fprintln(os.Stderr, "Younblock Lite could not start:", err)
}
