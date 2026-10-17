// Package stats holds the fixed atomic counters (file 01, section 9; file 03,
// R7). There are deliberately no per-IP maps: nothing here can grow.
package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

// Counters is the single set of application-wide counters.
type Counters struct {
	startedAt time.Time

	bytesDown  atomic.Int64
	bytesUp    atomic.Int64
	connsTotal atomic.Int64
	connsLive  atomic.Int64
	connErrors atomic.Int64
	dnsOK      atomic.Int64
	dnsFail    atomic.Int64
	panics     atomic.Int64

	// rate sampling
	mu         sync.Mutex
	lastDown   int64
	lastUp     int64
	lastSample time.Time
	rateDown   float64
	rateUp     float64
}

// New returns a counter set whose clock starts now.
func New() *Counters {
	return &Counters{startedAt: time.Now(), lastSample: time.Now()}
}

// AddBytesDown counts bytes received from websites.
func (c *Counters) AddBytesDown(n int64) { c.bytesDown.Add(n) }

// AddBytesUp counts bytes sent to websites.
func (c *Counters) AddBytesUp(n int64) { c.bytesUp.Add(n) }

// ConnOpened counts an accepted connection (and raises the live count).
func (c *Counters) ConnOpened() { c.connsTotal.Add(1); c.connsLive.Add(1) }

// ConnClosed lowers the live count when a connection ends.
func (c *Counters) ConnClosed() { c.connsLive.Add(-1) }

// ConnError counts a failed connection (dial failure, protocol error).
func (c *Counters) ConnError() { c.connErrors.Add(1) }

// DnsOK counts a successful secure lookup.
func (c *Counters) DnsOK() { c.dnsOK.Add(1) }

// DnsFail counts a failed secure lookup.
func (c *Counters) DnsFail() { c.dnsFail.Add(1) }

// Panic counts a recovered connection panic.
func (c *Counters) Panic() { c.panics.Add(1) }

// Snapshot is the 1-second UI snapshot (JSON shape from file 01, section 10).
type Snapshot struct {
	BytesDown   int64   `json:"bytesDown"`
	BytesUp     int64   `json:"bytesUp"`
	RateDown    float64 `json:"rateDown"`
	RateUp      float64 `json:"rateUp"`
	ActiveConns int64   `json:"activeConns"`
	UptimeSec   int64   `json:"uptimeSec"`
}

// Totals returns the cumulative relayed byte counters since engine start
// (round-3): the session math reads them without disturbing the rate
// sampling that Sample owns.
func (c *Counters) Totals() (down, up int64) {
	return c.bytesDown.Load(), c.bytesUp.Load()
}

// Sample recomputes the transfer rates and returns the snapshot.
// The engine calls it once per second while the proxy is on.
func (c *Counters) Sample() Snapshot {
	c.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(c.lastSample).Seconds()
	if elapsed >= 0.5 { // ignore ultra-short calls; rates stay stable
		c.rateDown = float64(c.bytesDown.Load()-c.lastDown) / elapsed
		c.rateUp = float64(c.bytesUp.Load()-c.lastUp) / elapsed
		c.lastDown = c.bytesDown.Load()
		c.lastUp = c.bytesUp.Load()
		c.lastSample = now
	}
	snap := Snapshot{
		BytesDown:   c.bytesDown.Load(),
		BytesUp:     c.bytesUp.Load(),
		RateDown:    c.rateDown,
		RateUp:      c.rateUp,
		ActiveConns: c.connsLive.Load(),
		UptimeSec:   int64(now.Sub(c.startedAt).Seconds()),
	}
	c.mu.Unlock()
	return snap
}

// All is every counter, used by the diagnostic report.
type All struct {
	BytesDown  int64     `json:"bytesDown"`
	BytesUp    int64     `json:"bytesUp"`
	ConnsTotal int64     `json:"connsTotal"`
	ConnsLive  int64     `json:"connsLive"`
	ConnErrors int64     `json:"connErrors"`
	DnsOK      int64     `json:"dnsOK"`
	DnsFail    int64     `json:"dnsFail"`
	Panics     int64     `json:"panics"`
	StartedAt  time.Time `json:"startedAt"`
}

// Diagnostic returns every counter, for the diagnostic report only.
func (c *Counters) Diagnostic() All {
	return All{
		BytesDown:  c.bytesDown.Load(),
		BytesUp:    c.bytesUp.Load(),
		ConnsTotal: c.connsTotal.Load(),
		ConnsLive:  c.connsLive.Load(),
		ConnErrors: c.connErrors.Load(),
		DnsOK:      c.dnsOK.Load(),
		DnsFail:    c.dnsFail.Load(),
		Panics:     c.panics.Load(),
		StartedAt:  c.startedAt,
	}
}

// StartedAt reports when this counter set began.
func (c *Counters) StartedAt() time.Time { return c.startedAt }
