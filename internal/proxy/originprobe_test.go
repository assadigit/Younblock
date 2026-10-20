package proxy

// The M8-aa shapes (ADR-0028/0029), pinned by test:
//
//   1. The verified prewarm — one origin-probe pass over a name whose
//      answer set holds a silent corpse and a speaking origin: the
//      corpse is marked dead (10-minute direct-evidence TTL), the
//      speaker is marked verified-live, and the NEXT flow's dial list
//      starts on the speaker — the corpse is never dialed at all. The
//      debug.18 report's whole remaining cold-start cost was exactly
//      this gamble bought flow-by-flow; the pass buys it in the
//      background before the first flow arrives.
//
//   2. The neutered control — with DeadIPPenalty=-1 (the box-off
//      M8-x shape) the pass still runs and counts, but the flow
//      gambles the corpse in the resolver's order and terminals on it:
//      the report's exact pre-fix signature, proving the MARKING is
//      what changed the flow's fate.
//
//   3. The raced-path salvage round — an all-dead raced round over a
//      mixed answer set (three corpses the rotations covered plus one
//      live origin they never reached) runs ONE more raced round over
//      exactly the untried origin and the flow is saved in-engine: the
//      retry line, the salvaged line, no terminal, the app never sees
//      the failure.
//
//   4. The fast-terminal guard — a set with nothing untried (every IP
//      already dialed or boxed) keeps the exact M8-w terminal at ~1
//      budget: the salvage never fires on a hopeless host.
//
//   5. Strike escalation — a third mark on the same origin IP
//      escalates its penalty tier while the box stays a preference:
//      an all-penalized list dials in full.
//
//   6. The probe line's privacy — the pass logs counts only: no IP
//      literal, no name, ever (R21).
//
// Both-ways proof: with tunnel.go stashed (the debug.18 shape), test 3
// FAILS — the mixed-set flow terminals on the raced round with the
// report's exact "first-byte failed" signature and the live origin is
// never dialed — while the guards (4, 5, 6) pass on both sides; with
// the probe's marking neutered via DeadIPPenalty=-1 (test 2), test 1
// FAILS with the same terminal signature, pinning exactly what the
// marks change and nothing else.

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/stats"
)

// probeFixture builds the mixed answer set the M8-aa shapes gamble on:
// a silent corpse first in resolver order (the worst case — the
// pre-fix primary dials it), a speaking origin second, both on the
// same port. The probe's tls.Client handshake against the echo origin
// fails as a non-timeout, non-reset error (the echo returns our own
// ClientHello) — classified "answered", exactly the verdict a live
// path earns; the corpse stays silent through the budget — "silent".
func probeFixture(t *testing.T) (corpse *acceptTimes, echoPort int, ips []net.IP) {
	t.Helper()
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echoListenerAt(t, "127.0.0.2:"+strconv.Itoa(port))
	return corpse, port, []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")}
}

// TestOriginProbeCleansThePoolBeforeTheFlow (M8-aa shape 1): the pass
// marks the mixed set's corpse dead and its speaker verified-live, and
// the next flow dials the speaker — the corpse is never accepted, no
// terminal line, the record comes back byte-exact.
func TestOriginProbeCleansThePoolBeforeTheFlow(t *testing.T) {
	corpse, port, ips := probeFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
		// Pin the dial-list preference: racing and salvage off, so the
		// PRIMARY alone decides which origin the flow gambles on.
		c.FirstByteRaceDials = -1
		c.FirstByteSalvageDials = -1
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	// One probe pass, a short per-probe budget (the ctx caps the
	// corpse's silence at well under the production 2.5 s).
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"www.youtube.com"}, port, 4)
	if sum.IPs != 2 || sum.Answered != 1 || sum.Silent != 1 || sum.Reset != 0 {
		t.Fatalf("probe summary = %+v, want ips=2 answered=1 silent=1 reset=0", sum)
	}
	d := st.Diagnostic()
	if d.ProbePasses != 1 || d.ProbeIPs != 2 || d.ProbeAnswered != 1 || d.ProbeSilent != 1 {
		t.Fatalf("probe counters = %+v", d)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good table count = %d, want 1 (the echo origin)", got)
	}

	// The flow: the primary must dial the verified-live origin.
	c := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	rec := tlsLikeRecord(64)
	if _, err := c.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	_ = c.Close()
	if string(got) != string(rec) {
		t.Fatalf("echo mismatch: %d bytes", len(got))
	}
	if n := corpse.count(); n != 1 {
		t.Fatalf("corpse accepted %d connections, want 1 (the probe's own dial) — the flow must skip the dead verdict", n)
	}
	if failed := log.lines("WARN tunnel first-byte failed "); len(failed) != 0 {
		t.Fatalf("terminal line on a probe-cleaned pool: %q", failed[0])
	}
}

// TestOriginProbeMarkingNeuteredGamblesTheCorpse (M8-aa shape 2, the
// neutered control): DeadIPPenalty=-1 is the exact box-off M8-x shape
// — the pass still runs and counts its verdicts, but the marks never
// apply, the flow gambles the corpse in the resolver's order, and the
// terminal carries the debug.18 report's exact signature. Passes on
// both sides of the marking: with the box on, the same flow is shape 1.
func TestOriginProbeMarkingNeuteredGamblesTheCorpse(t *testing.T) {
	corpse, port, ips := probeFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
		c.DeadIPPenalty = -1 // the box off: the M8-x shape exactly
		c.FirstByteRaceDials = -1
		c.FirstByteSalvageDials = -1
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"www.youtube.com"}, port, 4)
	if sum.IPs != 2 || sum.Answered != 1 || sum.Silent != 1 {
		t.Fatalf("probe summary = %+v — the pass itself is knob-independent", sum)
	}
	if d := st.Diagnostic(); d.ProbeIPs != 2 {
		t.Fatalf("probe counters = %+v", d)
	}

	// The flow: nothing marked, the resolver's first answer dials.
	c := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second)
	if len(failed) == 0 {
		t.Fatalf("the box-off flow must terminal on the corpse (the old gamble)")
	}
	if n := corpse.count(); n < 2 {
		t.Fatalf("corpse accepted %d connections, want >= 2 (the probe's dial plus the flow's gamble)", n)
	}
}

// TestRacedSalvageCoversTheUntriedOrigin (M8-aa shape 3): three
// corpses the raced round's rotations cover, one live origin they never
// reach — the salvage round dials exactly the untried origin and the
// flow is saved in-engine. Pre-fix this FAILS: the flow terminals on
// the raced round (the debug.18 report's ~2.6 s wave, one app retry
// away from a fresh gamble) and the live origin is never dialed.
func TestRacedSalvageCoversTheUntriedOrigin(t *testing.T) {
	c1, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	echoListenerAt(t, "127.0.0.4:"+strconv.Itoa(port))
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"), net.ParseIP("127.0.0.4")}
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
		c.FirstByteTimeout = 250 * time.Millisecond
		// Racing on (the default shape): primary + 2 extras cover the
		// first three rotations; the fourth origin is the untried one.
	})

	c := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	rec := tlsLikeRecord(64)
	if _, err := c.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo (the salvage must save the flow): %v", err)
	}
	_ = c.Close()
	if string(got) != string(rec) {
		t.Fatalf("echo mismatch: %d bytes", len(got))
	}
	if retries := pollLines(log, "WARN tunnel first-byte retry ", 1, 2*time.Second); len(retries) == 0 {
		t.Fatalf("no retry line — the salvage round must log its attempt")
	}
	if saved := pollLines(log, "INFO tunnel first-byte salvaged ", 1, 2*time.Second); len(saved) == 0 {
		t.Fatalf("no salvaged line — the untried origin must save the flow")
	}
	if failed := log.lines("WARN tunnel first-byte failed "); len(failed) != 0 {
		t.Fatalf("terminal line on a saved flow: %q", failed[0])
	}
	// The raced round covered the three corpses (one accept each: the
	// primary and both extras); the live origin took the salvage's dial.
	if n := c1.count(); n != 1 {
		t.Fatalf("corpse #1 accepted %d connections, want 1 (the primary)", n)
	}
}

// TestRacedTerminalStaysFastWhenNothingUntried (M8-aa shape 4, the
// guard): every IP of the answer set is dialed or boxed when the raced
// round dies — the terminal keeps the exact M8-w shape, no salvage
// round, no second budget. Passes on both sides of the fix.
func TestRacedTerminalStaysFastWhenNothingUntried(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3")}
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
		c.FirstByteTimeout = 250 * time.Millisecond
	})

	start := time.Now()
	c := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second)
	if len(failed) == 0 {
		t.Fatalf("the hopeless set must terminal (nothing untried)")
	}
	if retries := log.lines("WARN tunnel first-byte retry "); len(retries) != 0 {
		t.Fatalf("salvage fired on a hopeless set: %q", retries[0])
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("terminal took %v — the hopeless set must keep ~1 budget, never 2", elapsed)
	}
}

// TestStrikeEscalation (M8-aa shape 5): the third mark on the same IP
// escalates its penalty tier (DeadBoxStats counts strikes >= 2 as
// escalated), a single mark does not, and the box stays a preference —
// an all-penalized list dials in full, never an instant fail.
func TestStrikeEscalation(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
	})
	srv.markDeadIP("192.0.2.10")
	srv.markDeadIP("192.0.2.10")
	srv.markDeadIP("192.0.2.10")
	srv.markDeadIP("192.0.2.11")
	live, escalated := srv.DeadBoxStats()
	if live != 2 {
		t.Fatalf("live marks = %d, want 2", live)
	}
	if escalated != 1 {
		t.Fatalf("escalated marks = %d, want 1 (the three-strike IP, not the single)", escalated)
	}
	// Never a refusal: the fully-penalized list gambles in full.
	in := []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("192.0.2.11")}
	out := srv.liveIPs(in)
	if len(out) != 2 || !out[0].Equal(in[0]) || !out[1].Equal(in[1]) {
		t.Fatalf("all-penalized list must dial in full, got %v", out)
	}
}

// TestProbeLineCountsAndPrivacy (M8-aa shape 6): the pass's one log
// line carries counts only — no IP literal, no name (R21) — and the
// counters agree with the summary.
func TestProbeLineCountsAndPrivacy(t *testing.T) {
	_, port, ips := probeFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
	})
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"probe-a.test"}, port, 4)
	lines := pollLines(log, "INFO origin probe: ", 1, 3*time.Second)
	if len(lines) == 0 {
		t.Fatalf("the pass must log its summary line")
	}
	line := lines[0]
	for _, want := range []string{"names=1", "ips=2", "answered=1", "silent=1", "reset=0"} {
		if !strings.Contains(line, want) {
			t.Fatalf("probe line %q missing %s", line, want)
		}
	}
	if strings.Contains(line, "127.0.0.") || strings.Contains(line, "probe-a.test") {
		t.Fatalf("probe line leaks an IP or a name (R21): %q", line)
	}
	if d := st.Diagnostic(); d.ProbePasses != 1 || d.ProbeIPs != int64(sum.IPs) {
		t.Fatalf("counters = %+v, summary = %+v", d, sum)
	}
}

// Compile-time guard that the unused-import set stays honest.
var _ = config.MaxHeaderBytes
var _ = stats.New
