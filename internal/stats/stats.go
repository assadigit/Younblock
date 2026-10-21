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
	// M8-ak (ADR-0050): the auto-timeline's video-scoped signals.
	gvideoFlows      atomic.Int64
	gvideoFirstBytes atomic.Int64
	probePasses      atomic.Int64
	probeIPs         atomic.Int64
	probeAnswered    atomic.Int64
	probeSilent      atomic.Int64
	probeReset       atomic.Int64

	// M8-ab (ADR-0031/0032/0033): the cross-origin substitution's
	// attempts and wins, and the googlevideo terminal count the
	// gvideo loop's churn watch kicks a probe pass on. M8-ai
	// (ADR-0048) splits the wins by size: media-carried (big) vs the
	// fast small verdict answer (small).
	substAttempts   atomic.Int64
	substWins       atomic.Int64
	substBig        atomic.Int64
	substSmall      atomic.Int64
	gvideoTerminals atomic.Int64

	// M8-ac (ADR-0034/0035/0036): the stall kill's pending/idle split
	// (was the origin dead with work outstanding, or just quiet), the
	// good-mark drop count (the pool's churn), the box's rehabilitation
	// count, and the DoH brownout periods.
	stallLeanPending   atomic.Int64
	stallLeanIdle      atomic.Int64
	goodDrops          atomic.Int64
	probeRehabilitated atomic.Int64
	dohBrownouts       atomic.Int64

	// M8-ad (ADR-0037): the rich tier's own cause split — the pending
	// fast kill (a proven conn died with the player's request
	// outstanding; the mid-video wedge's recovery bound) and the
	// trickle kill (a proven conn being per-connection throttled; the
	// refresh-fixes-it shape detected engine-side).
	stallRichPending atomic.Int64
	stallTrickle     atomic.Int64

	// M8-ae (ADR-0040/0041/0042): the wire-discipline round's meters —
	// every googlevideo-SNI ClientHello the engine put on the wire
	// (flow candidates, substitutes, probe jobs: the volume number
	// the censor actually sees) and the emission breaker's arms.
	gvideoHellos    atomic.Int64
	gvideoCooldowns atomic.Int64

	// M8-ag (ADR-0046): the breaker's escapes — scout flows let
	// through an armed cooldown, and cooldowns a speaking wire
	// lifted early.
	gvideoScouts atomic.Int64
	gvideoLifts  atomic.Int64

	// M8-ah (ADR-0047): the dead-cluster memory and the payload-fed
	// pool — clusters marked dead, flows fast-failed at the door for
	// a dead cluster, the periodic re-probes, rehabilitations, and
	// origins media-verified by delivered payload. M8-ai (ADR-0048)
	// adds the verdict lane's door engagements — dead-cluster flows
	// the pool served instead of refusing.
	gvideoClusterDeads    atomic.Int64
	gvideoClusterFastFail atomic.Int64
	gvideoClusterVerdicts atomic.Int64
	gvideoClusterProbes   atomic.Int64
	gvideoClusterRehabs   atomic.Int64
	gvideoClusterSeeds    atomic.Int64
	gvideoPayloadFeeds    atomic.Int64

	// M8-af (ADR-0043, spec W1): the googlevideo flows' kernel-TCP
	// verdict split — every stalled googlevideo flow classified from
	// its A/B TCP_INFO snapshots (upstream-lost: the request never
	// reached the server; downstream-lost: the request arrived and the
	// answer is being dropped or throttled; both-ok: the transport
	// carried both ways; unknown: no classifiable pair).
	tcpUpstreamLost   atomic.Int64
	tcpDownstreamLost atomic.Int64
	tcpBothOK         atomic.Int64
	tcpUnknown        atomic.Int64

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

// GvideoFlowStart counts one established budgeted tunnel to a
// *.googlevideo.com host (M8-ak, ADR-0050): the auto-timeline
// heuristic's PLAY signal, video-scoped — the app only dials
// googlevideo for videoplayback/redirector machinery, so a
// googlevideo flow after googlevideo quiet is a play attempt where
// any routed flow was (feed images and API calls included).
func (c *Counters) GvideoFlowStart() { c.gvideoFlows.Add(1) }

// GvideoFirstByte counts one first byte delivered on a
// *.googlevideo.com host through any path (M8-ak, ADR-0050): the
// auto-timeline heuristic's DATA signal — with the substitution lane
// retired, a googlevideo first byte is a live video flow answering,
// the direct play->video-data measurement the debug.28 report's
// residual investigation needs (feed bursts of 1.4 MB on i.ytimg
// used to close pairs that carried no video at all).
func (c *Counters) GvideoFirstByte() { c.gvideoFirstBytes.Add(1) }

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

// SubstAttempt counts one substitute candidate launched by a raced
// round (M8-ab, ADR-0031): a pool origin dialed in place of one
// own-set rotation for a *.googlevideo.com flow.
func (c *Counters) SubstAttempt() { c.substAttempts.Add(1) }

// SubstWin counts one flow whose substitute candidate delivered the
// first byte and won the round (M8-ab, ADR-0031) — the counter the
// next report reads to say the escape worked in the field.
func (c *Counters) SubstWin() { c.substWins.Add(1) }

// SubstBig counts one substituted flow that carried real media —
// downstream bytes at or above PayloadVerifiedMinimum, the same bar
// that certifies the origin (M8-ai, ADR-0048). big>0 in a report says
// the cross-origin lane carried payload at least once; big=0 beside
// small>0 says the lane is purely the fast-verdict lane.
func (c *Counters) SubstBig() { c.substBig.Add(1) }

// SubstSmall counts one substituted flow that ended below the media
// bar (M8-ai, ADR-0048): the fast small answer — the 403/redirect
// verdict shape both the debug.23 and debug.26 reports measured on
// every cross-cluster substitute win.
func (c *Counters) SubstSmall() { c.substSmall.Add(1) }

// GvideoTerminal counts one googlevideo flow that terminaled after
// its raced round AND its untried salvage all died (M8-ab, ADR-0032):
// the churn signal the gvideo loop's 1 s watch kicks an immediate
// origin-probe pass on.
func (c *Counters) GvideoTerminal() { c.gvideoTerminals.Add(1) }

// GvideoHello counts one googlevideo-SNI ClientHello the engine put
// on the wire (M8-ae, ADR-0042): every raced candidate, every
// substitute, every serial attempt, every probe job — the volume
// number the censor's per-source flag actually accumulates on. The
// next report reads it against routedFlows: the reference
// implementations put ~1 hello per flow; debug.22 put ~4 plus bursts.
func (c *Counters) GvideoHello() { c.gvideoHellos.Add(1) }

// GvideoCooldown counts one emission-breaker arm (M8-ae, ADR-0041):
// GvideoBreakerTerminals googlevideo terminals inside the window
// flip the engine to wire-silent on googlevideo for one cooldown so
// the app's hostname rotation (and any per-source flag decay) can
// actually happen. Cooldowns>0 in a report = the breaker caught a
// flag state that debug.22 rode into "never opens again".
func (c *Counters) GvideoCooldown() { c.gvideoCooldowns.Add(1) }

// GvideoScout counts one googlevideo flow the door let through an
// armed cooldown as its single wire probe (M8-ag, ADR-0046): the
// app's own request, not synthetic emission — the debug.24 report's
// breaker sat blind for 45 s while the player spent its retry budget
// against the door.
func (c *Counters) GvideoScout() { c.gvideoScouts.Add(1) }

// GvideoLift counts one cooldown an early escape ended (M8-ag,
// ADR-0046): a googlevideo origin spoke (first byte through any path),
// falsifying the arm's premise for right now — the breaker's
// self-correction in the wire-alive direction.
func (c *Counters) GvideoLift() { c.gvideoLifts.Add(1) }

// GvideoClusterDead counts one cache cluster marked dead (M8-ah,
// ADR-0047): GvideoClusterDeadHosts distinct hosts of the same
// sn-cluster silent-terminalled inside the window — the measured
// blackhole signature.
func (c *Counters) GvideoClusterDead() { c.gvideoClusterDeads.Add(1) }

// GvideoClusterFastFail counts one flow the door refused for a
// dead-marked cluster with nothing better to offer it (M8-ah,
// ADR-0047): the instant 502 that keeps the app's hostname rotation
// cycling instead of burning a 3 s first-byte budget per attempt.
func (c *Counters) GvideoClusterFastFail() { c.gvideoClusterFastFail.Add(1) }

// GvideoClusterVerdict counts one dead-cluster flow the door passed
// to the pool — the VERDICT LANE's engagement (M8-ai, ADR-0048): the
// flow rides a substitute and answers on live wire instead of taking
// an instant reset. verdicts= read against fastFails= says whether
// the lane held through the storm; the debug.26 report read 150
// fast-fails beside a lane that had starved on demoted pool marks.
func (c *Counters) GvideoClusterVerdict() { c.gvideoClusterVerdicts.Add(1) }

// GvideoClusterProbe counts one flow the door let through to a
// dead-marked cluster as its periodic re-test (M8-ah, ADR-0047).
func (c *Counters) GvideoClusterProbe() { c.gvideoClusterProbes.Add(1) }

// GvideoClusterRehab counts one dead mark an answered origin cleared
// (M8-ah, ADR-0047).
func (c *Counters) GvideoClusterRehab() { c.gvideoClusterRehabs.Add(1) }

// GvideoClusterSeed counts one dead-cluster mark seeded from the
// persisted file (M8-an, ADR-0053): the last session's blackhole
// measurement carried into this one — the seeded= half of the
// report's dead=, read beside it, says the first video of the
// session met an already-marked tunnel instead of re-paying the
// discovery.
func (c *Counters) GvideoClusterSeed() { c.gvideoClusterSeeds.Add(1) }

// GvideoPayloadFeed counts one origin media-verified by delivered
// payload (M8-ah, ADR-0047): a googlevideo flow that carried
// PayloadVerifiedMinimum downstream bytes — the pool's automatic
// food, no synthetic probe.
func (c *Counters) GvideoPayloadFeed() { c.gvideoPayloadFeeds.Add(1) }

// TCPUpstreamLost counts one stalled googlevideo flow whose kernel TCP
// evidence says the request never reached the server (acked did not
// move past the request snapshot while retransmits/unacked grew) —
// the censor drops upstream data after the handshake (M8-af, ADR-0043,
// spec W1). Fragmentation tuning will not fix this class.
func (c *Counters) TCPUpstreamLost() { c.tcpUpstreamLost.Add(1) }

// TCPDownstreamLost counts one stalled googlevideo flow whose request
// ARRIVED (acked moved) but whose answer never came back (rcvd did
// not) — response bytes throttled or dropped on the way in (M8-af,
// ADR-0043, spec W1).
func (c *Counters) TCPDownstreamLost() { c.tcpDownstreamLost.Add(1) }

// TCPBothOK counts one stalled googlevideo flow whose TCP carried
// both directions — the transport worked; the server or HTTP layer is
// the suspect (M8-af, ADR-0043, spec W1).
func (c *Counters) TCPBothOK() { c.tcpBothOK.Add(1) }

// TCPUnknown counts one stalled googlevideo flow without a
// classifiable snapshot pair — never a guess (M8-af, ADR-0043).
func (c *Counters) TCPUnknown() { c.tcpUnknown.Add(1) }

// StallLeanPending counts one lean stall kill whose client had spoken
// after the origin's last byte (M8-ac, ADR-0036): a request the origin
// never answered — the flow died with work outstanding, the definite
// dead-origin shape.
func (c *Counters) StallLeanPending() { c.stallLeanPending.Add(1) }

// StallLeanIdle counts one lean stall kill whose client had been quiet
// since the origin's last byte (M8-ac, ADR-0036): the flow was idle
// when it died — EITHER the app's pooled connection sitting healthy
// between requests (the kill costs a re-dial later) OR a response that
// stopped mid-body (the origin died anyway). The split is the next
// report's evidence for which world we are in.
func (c *Counters) StallLeanIdle() { c.stallLeanIdle.Add(1) }

// StallRichPending counts one rich-tier stall kill fired at the
// pending fast budget (M8-ad, ADR-0037): a flow that had proven
// itself died with the client's request outstanding — the definite
// dead-with-work shape on a proven conn, the mid-video wedge the
// tester's post-debug.21 report named ("minute 5 … goes loading …
// refresh fixes it"), and the kill that bounds its recovery at
// StallRichPendingTimeout instead of the player's own patience.
func (c *Counters) StallRichPending() { c.stallRichPending.Add(1) }

// StallTrickle counts one rich-tier trickle kill (M8-ad, ADR-0037): a
// proven flow whose rolling window kept delivering bytes — but under
// the trickle floor, the per-connection throttle shape the stall watch
// could never see (the down stamp kept refreshing). Closed quietly so
// the player's retry lands on a fresh, unthrottled connection.
func (c *Counters) StallTrickle() { c.stallTrickle.Add(1) }

// GoodDrop counts one verified-live mark dropped by a death's demote
// (M8-ac, ADR-0034): the good pool's churn — the debug.20 report's
// pool drained to gvideo=1 because every terminal's deaths demoted
// the very marks its wins had written.
func (c *Counters) GoodDrop() { c.goodDrops.Add(1) }

// ProbeRehabilitated counts one boxed origin IP the probe pass
// re-verified as ANSWERED and therefore cleared (M8-ac, ADR-0034) —
// the number that says the box was wrong, directly.
func (c *Counters) ProbeRehabilitated() { c.probeRehabilitated.Add(1) }

// DoHBrownout counts one DoH brownout period entered (M8-ac,
// ADR-0035): the resolver's connections churning fast enough that
// the engine paused its background lanes.
func (c *Counters) DoHBrownout() { c.dohBrownouts.Add(1) }

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
	StallKillLean int64 `json:"stallKillLean"`
	StallKillRich int64 `json:"stallKillRich"`
	RacedSaves    int64 `json:"racedSaves"`
	SalvagedSaves int64 `json:"salvagedSaves"`
	RoutedFlows   int64 `json:"routedFlows"`
	// M8-ak (ADR-0050): the auto-timeline's video-scoped signals.
	GvideoFlows      int64 `json:"gvideoFlows"`
	GvideoFirstBytes int64 `json:"gvideoFirstBytes"`
	ProbePasses      int64 `json:"probePasses"`
	ProbeIPs         int64 `json:"probeIPs"`
	ProbeAnswered    int64 `json:"probeAnswered"`
	ProbeSilent      int64 `json:"probeSilent"`
	ProbeReset       int64 `json:"probeReset"`
	// M8-ab (ADR-0031/0032/0033); M8-ai (ADR-0048) splits the wins by
	// size class — media-carried vs the fast small verdict answer.
	SubstAttempts   int64 `json:"substAttempts"`
	SubstWins       int64 `json:"substWins"`
	SubstBig        int64 `json:"substBig"`
	SubstSmall      int64 `json:"substSmall"`
	GvideoTerminals int64 `json:"gvideoTerminals"`
	// M8-ac (ADR-0034/0035/0036): the stall kill's pending/idle split,
	// the good-mark drops, the box rehabilitations, the DoH brownouts.
	StallLeanPending   int64 `json:"stallLeanPending"`
	StallLeanIdle      int64 `json:"stallLeanIdle"`
	GoodDrops          int64 `json:"goodDrops"`
	ProbeRehabilitated int64 `json:"probeRehabilitated"`
	DoHBrownouts       int64 `json:"dohBrownouts"`

	// M8-ad (ADR-0037): the rich tier's cause split — the pending
	// fast kill and the trickle kill.
	StallRichPending int64 `json:"stallRichPending"`
	StallTrickle     int64 `json:"stallTrickle"`
	// M8-ae (ADR-0042): the wire-discipline meters.
	GvideoHellos    int64 `json:"gvideoHellos"`
	GvideoCooldowns int64 `json:"gvideoCooldowns"`
	// M8-ag (ADR-0046): the breaker's two escapes — the scout flows
	// that probed an armed cooldown, and the early lifts a speaking
	// wire earned.
	GvideoScouts int64 `json:"gvideoScouts"`
	GvideoLifts  int64 `json:"gvideoLifts"`
	// M8-ah (ADR-0047): the dead-cluster memory and the payload-fed
	// pool — the report's "gvideo clusters:" line reads these.
	// M8-ai (ADR-0048) adds the verdict lane's door engagements.
	GvideoClusterDeads    int64 `json:"gvideoClusterDeads"`
	GvideoClusterFastFail int64 `json:"gvideoClusterFastFail"`
	GvideoClusterVerdicts int64 `json:"gvideoClusterVerdicts"`
	GvideoClusterProbes   int64 `json:"gvideoClusterProbes"`
	GvideoClusterRehabs   int64 `json:"gvideoClusterRehabs"`
	GvideoClusterSeeds    int64 `json:"gvideoClusterSeeds"`
	GvideoPayloadFeeds    int64 `json:"gvideoPayloadFeeds"`
	// M8-af (ADR-0043, spec W1): the stalled googlevideo flows' kernel
	// TCP verdict split.
	TCPUpstreamLost   int64     `json:"tcpUpstreamLost"`
	TCPDownstreamLost int64     `json:"tcpDownstreamLost"`
	TCPBothOK         int64     `json:"tcpBothOK"`
	TCPUnknown        int64     `json:"tcpUnknown"`
	StartedAt         time.Time `json:"startedAt"`
}

// Diagnostic returns every counter, for the diagnostic report only.
func (c *Counters) Diagnostic() All {
	return All{
		BytesDown:             c.bytesDown.Load(),
		BytesUp:               c.bytesUp.Load(),
		ConnsTotal:            c.connsTotal.Load(),
		ConnsLive:             c.connsLive.Load(),
		ConnErrors:            c.connErrors.Load(),
		DnsOK:                 c.dnsOK.Load(),
		DnsFail:               c.dnsFail.Load(),
		Panics:                c.panics.Load(),
		PlainFirstByteTo:      c.plainFirstByteTimeouts.Load(),
		DialNear:              c.dialNear.Load(),
		DialFar:               c.dialFar.Load(),
		FirstByteSlow:         c.firstByteSlow.Load(),
		StallKillLean:         c.stallKillLean.Load(),
		StallKillRich:         c.stallKillRich.Load(),
		RacedSaves:            c.racedSaves.Load(),
		SalvagedSaves:         c.salvagedSaves.Load(),
		RoutedFlows:           c.routedFlows.Load(),
		GvideoFlows:           c.gvideoFlows.Load(),
		GvideoFirstBytes:      c.gvideoFirstBytes.Load(),
		ProbePasses:           c.probePasses.Load(),
		ProbeIPs:              c.probeIPs.Load(),
		ProbeAnswered:         c.probeAnswered.Load(),
		ProbeSilent:           c.probeSilent.Load(),
		ProbeReset:            c.probeReset.Load(),
		SubstAttempts:         c.substAttempts.Load(),
		SubstWins:             c.substWins.Load(),
		SubstBig:              c.substBig.Load(),
		SubstSmall:            c.substSmall.Load(),
		GvideoTerminals:       c.gvideoTerminals.Load(),
		StallLeanPending:      c.stallLeanPending.Load(),
		StallLeanIdle:         c.stallLeanIdle.Load(),
		GoodDrops:             c.goodDrops.Load(),
		ProbeRehabilitated:    c.probeRehabilitated.Load(),
		DoHBrownouts:          c.dohBrownouts.Load(),
		StallRichPending:      c.stallRichPending.Load(),
		StallTrickle:          c.stallTrickle.Load(),
		GvideoHellos:          c.gvideoHellos.Load(),
		GvideoCooldowns:       c.gvideoCooldowns.Load(),
		GvideoScouts:          c.gvideoScouts.Load(),
		GvideoLifts:           c.gvideoLifts.Load(),
		GvideoClusterDeads:    c.gvideoClusterDeads.Load(),
		GvideoClusterFastFail: c.gvideoClusterFastFail.Load(),
		GvideoClusterVerdicts: c.gvideoClusterVerdicts.Load(),
		GvideoClusterProbes:   c.gvideoClusterProbes.Load(),
		GvideoClusterRehabs:   c.gvideoClusterRehabs.Load(),
		GvideoClusterSeeds:    c.gvideoClusterSeeds.Load(),
		GvideoPayloadFeeds:    c.gvideoPayloadFeeds.Load(),
		TCPUpstreamLost:       c.tcpUpstreamLost.Load(),
		TCPDownstreamLost:     c.tcpDownstreamLost.Load(),
		TCPBothOK:             c.tcpBothOK.Load(),
		TCPUnknown:            c.tcpUnknown.Load(),
		StartedAt:             c.startedAt,
	}
}

// StartedAt reports when this counter set began.
func (c *Counters) StartedAt() time.Time { return c.startedAt }
