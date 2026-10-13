// Package logx provides the in-memory log ring and the rotating log file
// (file 01, section 9; file 03, R21). Nothing here ever writes URLs, paths,
// headers or payloads; hostnames reach a log line only when the caller has
// the logDomains setting on.
package logx

import "sync"

// Ring is a fixed-size in-memory ring of pre-formatted log lines. It feeds
// the diagnostic report and is cleared when the engine stops (R21).
type Ring struct {
	mu    sync.Mutex
	lines []string
	next  int // index of the oldest entry
	n     int // number of stored entries
}

// NewRing returns a ring that keeps the last n lines.
func NewRing(n int) *Ring {
	if n < 1 {
		n = 1
	}
	return &Ring{lines: make([]string, n)}
}

// Append stores one formatted line.
func (r *Ring) Append(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n < len(r.lines) {
		r.lines[(r.next+r.n)%len(r.lines)] = line
		r.n++
		return
	}
	r.lines[r.next] = line
	r.next = (r.next + 1) % len(r.lines)
}

// Lines returns the stored lines, oldest first.
func (r *Ring) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, r.n)
	for i := 0; i < r.n; i++ {
		out = append(out, r.lines[(r.next+i)%len(r.lines)])
	}
	return out
}

// Clear removes every stored line.
func (r *Ring) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n = 0
	r.next = 0
}

// Len returns how many lines are stored.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}
