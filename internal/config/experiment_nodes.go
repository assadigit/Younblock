// Package config — the optional foreign-cluster experiment file
// (M8-af, ADR-0044, spec W8): Ali supplies a file of googlevideo
// hostnames collected OUTSIDE Iran (yt-dlp -g on a foreign node);
// "Run network test" probes those addresses with their own real SNI
// and the table shows foreign-cluster rows — the §7 decision table's
// "the cluster the geo-DNS gives is harmed" discriminator.
//
// The file lives in the repo at internal/config/experiment_nodes.txt
// and is EMBEDDED in the binary: Ali commits it, the round's build
// carries it, and the on-device resolution goes through the normal
// DoH path (the hostnames are resolved where the test runs, never
// pre-resolved abroad). It ships EMPTY with a comment explaining it —
// an empty file is skipped silently (W8's own rule), so every build
// without Ali's data behaves exactly as if the lane did not exist.
package config

import (
	_ "embed"
	"strings"
)

// experimentNodesFS is the embedded foreign-node hostname file.
//
//go:embed experiment_nodes.txt
var experimentNodesFS string

// ExperimentNodesMax bounds the embedded list: one network test
// probes at most this many foreign hostnames (two answer addresses
// each) — the test is an operator-pressed measurement, and the bound
// keeps even a fat file inside the same few-minute wall budget.
const ExperimentNodesMax = 4

// ExperimentNodes returns the foreign-cluster experiment hostnames
// (trimmed, de-duplicated, capped): one googlevideo hostname per
// line, '#' comment lines and blanks skipped. Empty when Ali has not
// supplied the file's contents yet — the network test then skips the
// foreign lane silently (spec W8).
func ExperimentNodes() []string {
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(experimentNodesFS, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// One hostname per line; anything that is not a bare hostname
		// (spaces, slashes) is a mistake in the file, not data.
		if strings.ContainsAny(line, " \t/") {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
		if len(out) >= ExperimentNodesMax {
			break
		}
	}
	return out
}
