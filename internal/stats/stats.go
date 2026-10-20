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

	// M8-z (ADR-0024 work item B / ADR-0027 §8.3-8.4): the plain-relay
	// safety net's kill count, the dial RTT classes, and the slow
	// first-byte count the timeline window reads.
	plainFirstByteTimeouts atomic.Int64
	dialNear               atomic.Int64
	dialFar                atomic.Int64
	firstByteSlow          atomic.Int64

	// M8-aa (ADR-0028/0029/0030): the stall tier split (lean vs rich
	// kills), the raced/salvaged save counts, the routed-flow starts the
	// auto-timeline heuristic reads, and the origin-probe pass's own
	// verdict counters.
	stallKillLean atomic.Int64
	stallKillRich atomic.Int64
	racedSaves    atomic.Int64
	salvagedSaves atomic.Int64
	routedFlows   atomic.Int64
	probePasses   atomic.Int64
	probeIPs      atomic.Int64
	probeAnswered atomic.Int64
	probeSilent   atomic.Int64
	probeReset    atomic.Int64

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

// PlainFirstByteTimeout counts one plain-relay TLS flow closed by the
// 8 s safety net (M8-z, ADR-0024 work item B): the origin never sent a
// byte after the client's first record.
func (c *Counters) PlainFirstByteTimeout() { c.plainFirstByteTimeouts.Add(1) }

// DialNear counts an origin dial under 200 ms (ADR-0027 §8.3.4: the
// rtt-class split of the dial cohort — near vs far cache nodes).
func (c *Counters) DialNear() { c.dialNear.Add(1) }

// DialFar counts an origin dial of 200 ms or more.
func (c *Counters) DialFar() { c.dialFar.Add(1) }

// FirstByteSlow counts a first byte slower than 1 s (ADR-0027 §8.3.6:
// the timeline window's "slow flows" metric).
func (c *Counters) FirstByteSlow() { c.firstByteSlow.Add(1) }

// StallKillLean counts one lean-tier stall kill (M8-aa, ADR-0029: the
// report's tier split — the never-proved flow's mid-response death).
func (c *Counters) StallKillLean() { c.stallKillLean.Add(1) }

// StallKillRich counts one rich-tier hygiene close (M8-aa, ADR-0029).
func (c *Counters) StallKillRich() { c.stallKillRich.Add(1) }

// RacedSave counts one flow the first-dial race saved (an extra
// candidate delivered the first byte; M8-aa, ADR-0030's counter).
func (c *Counters) RacedSave() { c.racedSaves.Add(1) }

// SalvagedSave counts one flow a salvage round saved (M8-aa, ADR-0028's
// raced-path salvage included).
func (c *Counters) SalvagedSave() { c.salvagedSaves.Add(1) }

// RoutedFlowStart counts one established budgeted (class=routed)
// tunnel (M8-aa, ADR-0030: the auto-timeline heuristic's burst signal).
func (c *Counters) RoutedFlowStart() { c.routedFlows.Add(1) }

// ProbePass counts one completed origin-probe pass (M8-aa, ADR-0028).
func (c *Counters) ProbePass() { c.probePasses.Add(1) }

// ProbeIP counts one answer IP the origin-probe pass verified.
func (c *Counters) ProbeIP() { c.probeIPs.Add(1) }

// ProbeAnswered counts a probe verdict of "answered".
func (c *Counters) ProbeAnswered() { c.probeAnswered.Add(1) }

// ProbeSilent counts a probe verdict of "silent".
func (c *Counters) ProbeSilent() { c.probeSilent.Add(1) }

// ProbeReset counts a probe verdict of "reset".
func (c *Counters) ProbeReset() { c.probeReset.Add(1) }

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
	BytesDown        int64 `json:"bytesDown"`
	BytesUp          int64 `json:"bytesUp"`
	ConnsTotal       int64 `json:"connsTotal"`
	ConnsLive        int64 `json:"connsLive"`
	ConnErrors       int64 `json:"connErrors"`
	DnsOK            int64 `json:"dnsOK"`
	DnsFail          int64 `json:"dnsFail"`
	Panics           int64 `json:"panics"`
	PlainFirstByteTo int64 `json:"plainFirstByteTimeouts"`
	DialNear         int64 `json:"dialNear"`
	DialFar          int64 `json:"dialFar"`
	FirstByteSlow    int64 `json:"firstByteSlow"`
	// M8-aa (ADR-0028/0029/0030): the tier split, the saves, the routed
	// flow starts, and the origin-probe verdicts.
	StallKillLean int64     `json:"stallKillLean"`
	StallKillRich int64     `json:"stallKillRich"`
	RacedSaves    int64     `json:"racedSaves"`
	SalvagedSaves int64     `json:"salvagedSaves"`
	RoutedFlows   int64     `json:"routedFlows"`
	ProbePasses   int64     `json:"probePasses"`
	ProbeIPs      int64     `json:"probeIPs"`
	ProbeAnswered int64     `json:"probeAnswered"`
	ProbeSilent   int64     `json:"probeSilent"`
	ProbeReset    int64     `json:"probeReset"`
	StartedAt     time.Time `json:"startedAt"`
}

// Diagnostic returns every counter, for the diagnostic report only.
func (c *Counters) Diagnostic() All {
	return All{
		BytesDown:        c.bytesDown.Load(),
		BytesUp:          c.bytesUp.Load(),
		ConnsTotal:       c.connsTotal.Load(),
		ConnsLive:        c.connsLive.Load(),
		ConnErrors:       c.connErrors.Load(),
		DnsOK:            c.dnsOK.Load(),
		DnsFail:          c.dnsFail.Load(),
		Panics:           c.panics.Load(),
		PlainFirstByteTo: c.plainFirstByteTimeouts.Load(),
		DialNear:         c.dialNear.Load(),
		DialFar:          c.dialFar.Load(),
		FirstByteSlow:    c.firstByteSlow.Load(),
		StallKillLean:    c.stallKillLean.Load(),
		StallKillRich:    c.stallKillRich.Load(),
		RacedSaves:       c.racedSaves.Load(),
		SalvagedSaves:    c.salvagedSaves.Load(),
		RoutedFlows:      c.routedFlows.Load(),
		ProbePasses:      c.probePasses.Load(),
		ProbeIPs:         c.probeIPs.Load(),
		ProbeAnswered:    c.probeAnswered.Load(),
		ProbeSilent:      c.probeSilent.Load(),
		ProbeReset:       c.probeReset.Load(),
		StartedAt:        c.startedAt,
	}
}

// StartedAt reports when this counter set began.
func (c *Counters) StartedAt() time.Time { return c.startedAt }
