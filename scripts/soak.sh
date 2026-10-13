#!/usr/bin/env bash
# soak: the 5,000-tunnel soak test (file 01, section 11; file 03, R2/R7)
# run verbosely with the race detector, plus the goroutine/fd/heap report.
set -euo pipefail
cd "$(dirname "$0")/.."
go test -race -run 'TestSoak' -v -timeout 900s ./internal/proxy/
