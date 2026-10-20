package proxy

// The M8-ac shapes (ADR-0034/0035/0036), pinned by test:
//
//   1. The anti-concentration rotation — two consecutive liveIPs
//      calls over the same answer set lead with DIFFERENT
//      verified-live origins: the debug.20 report's www.youtube.com
//      ran 17 flows at 17 failures, every one funneled onto the same
//      pinned good-front while the connection-killer worked the lane.
//      Pre-fix the front is static — the concentration shape itself.
//
//   2. The reset-class death — a good-marked origin whose flow dies
//      by RESET keeps its good mark (the reset is evidence about ONE
//      connection under the flow-killing regime, not the origin) and
//      boxes only briefly (a within-wave skip). Pre-fix (the
//      debug.20 shape) the death demotes the mark and boxes for the
//      full 60 s — the pool drain the report measured (gvideo=1).
//
//   3. The pending/idle stall split — a lean stall kill whose client
//      spoke after the origin's last byte (a request the origin never
//      answered) DEMOTES the origin's good mark; a kill whose client
//      stayed quiet does NOT. The split rides the line
//      (cause=pending|idle) and the counters.
//
//   4. The box rehabilitation — a boxed origin IP that ANSWERS the
//      probe pass's re-verification is cleared from the box, marked
//      good, and counted: the debug.20 report's smoking gun was
//      penalized=12 while the one-shot dead-origin probe said those
//      same IPs answer. Pre-fix (the debug.20 shape) the pass skips
//      ALL boxed IPs — the box stays wrong for its whole 5-30-minute
//      TTL.
//
//   5. The substitute rotation — consecutive goodSubstitutes calls
//      lead with different pool origins (the substitution bet spreads
//      across the pool instead of always hammering its freshest mark).
//
// Both-ways proof: the rotation neutered (a static front), tests 1
// and 5 FAIL on the repeated front; the reset class neutered (reset
// demotes like the old timeout class), test 2 FAILS on the dropped
// mark; the pending demote neutered, test 3 FAILS on the surviving
// mark; the rehabilitation neutered (boxed IPs skipped), test 4
// FAILS on the stale box and the zero counter.

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/stats"
)

// TestLiveIPsRotatesTheFront (M8-ac shape 1): consecutive dial lists
// over the same answer set lead with different verified-live origins.
func TestLiveIPsRotatesTheFront(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
	})
	srv.markGoodIP("192.0.2.10", false)
	srv.markGoodIP("192.0.2.11", false)
	srv.markGoodIP("192.0.2.12", false)
	in := []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("192.0.2.11"), net.ParseIP("192.0.2.12"), net.ParseIP("192.0.2.13")}
	first := srv.liveIPs(in)
	second := srv.liveIPs(in)
	third := srv.liveIPs(in)
	if len(first) != 4 || len(second) != 4 || len(third) != 4 {
		t.Fatalf("rotation must not drop or duplicate candidates: %v %v %v", first, second, third)
	}
	if first[0].Equal(second[0]) || second[0].Equal(third[0]) || first[0].Equal(third[0]) {
		t.Fatalf("the live front is STATIC — the concentration shape the debug.20 report measured: %v %v %v", first, second, third)
	}
	// The neutral tail keeps its order.
	for _, list := range [][]net.IP{first, second, third} {
		if !list[3].Equal(in[3]) {
			t.Fatalf("the neutral tail must keep its order: %v", list)
		}
	}
}

// resetListenerAt binds exactly ip:port and RESETS every connection
// right after reading its first bytes (SetLinger(0) — the RST shape).
func resetListenerAt(t *testing.T, ip string, port int) int {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				_, _ = c.Read(buf)
				tc := c.(*net.TCPConn)
				_ = tc.SetLinger(0)
				_ = tc.Close() // the RST
			}(c)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// TestResetDeathKeepsTheMark (M8-ac shape 2): a flow-death of class
// reset boxes the origin briefly but NEVER drops its good mark — the
// temporal-signal ruling. The debug.20 shape (demote + 60 s) drained
// the pool to gvideo=1 while the one-shot dead-origin probe said the
// boxed IPs answer.
func TestResetDeathKeepsTheMark(t *testing.T) {
	port := resetListenerAt(t, "127.0.0.1", 0)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-abc.googlevideo.com": {net.ParseIP("127.0.0.1")},
	}}
	srv, st := newTestServer(t, func(c *Config) {
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1 // the serial path: the primary's death class is the test's subject
		c.FirstByteSalvageDials = -1
	})
	// The origin is verified-live (the pool's own evidence).
	srv.markGoodIP("127.0.0.1", true)
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count before the flow = %d, want 1", got)
	}

	// The flow: the origin resets the first-byte read.
	c := dialConnect(t, srv.Addr(), "rr1---sn-abc.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.GoodIPCount() == 1 && func() bool { l, _ := srv.DeadBoxStats(); return l >= 1 }() {
			// The reset class: boxed (briefly) AND the mark kept.
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after a RESET death = %d, want 1 (the reset is temporal evidence — the mark stays)", got)
	}
	live, _ := srv.DeadBoxStats()
	if live < 1 {
		t.Fatalf("the reset origin must still be boxed (briefly) for the within-wave skip")
	}
	_ = st
}

// TestPendingStallDemotesTheMark (M8-ac shape 3a): a lean stall kill
// whose client spoke after the origin's last byte — a request the
// origin never answered — drops the origin's good mark.
func TestPendingStallDemotesTheMark(t *testing.T) {
	up := stallOriginServer(t) // echoes the first flight, then silence
	upHost, upPort, _ := net.SplitHostPort(up)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.IdleTimeout = 5 * time.Second
		c.StallTimeout = 400 * time.Millisecond
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the origin's first flight must be delivered: %v", err)
	}
	// The winner marked its origin good at the first byte (M8-ab's
	// flow-fed pool): the demote's subject.
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after the first byte = %d, want 1 (the flow-fed mark)", got)
	}
	// The PENDING shape: the client sends a new request the origin
	// never answered - written PAST the pump-stop window but INSIDE
	// the wake gap (StallTimeout/5 = 80 ms here: a write that closes
	// a wider gap would trip the M8-x pacing proof and escalate the
	// flow to the rich tier before the lean kill could fire; the rich
	// tier's own pending kills demote too, but the lean budget is the
	// fast path under test).
	time.Sleep(30 * time.Millisecond)
	if _, err := conn.Write([]byte("unanswered request")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Read(make([]byte, 16))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if lines := log.lines("WARN tunnel stall killed "); len(lines) > 0 {
			if !strings.Contains(lines[0], "cause=pending") {
				t.Fatalf("the stall line must carry the pending cause: %q", lines[0])
			}
			if got := srv.GoodIPCount(); got != 0 {
				t.Fatalf("good count after a PENDING stall kill = %d, want 0 (dead with work outstanding demotes)", got)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the stall kill never fired")
}

// TestIdleStallKeepsTheMark (M8-ac shape 3b): a lean stall kill whose
// client stayed quiet since the origin's last byte keeps the mark —
// the idle shape is ambiguous (pooled-healthy OR mid-body death) and
// the rotation already handles any concentration.
func TestIdleStallKeepsTheMark(t *testing.T) {
	up := stallOriginServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.IdleTimeout = 5 * time.Second
		c.StallTimeout = 400 * time.Millisecond
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the origin's first flight must be delivered: %v", err)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after the first byte = %d, want 1", got)
	}
	// The IDLE shape: the client says nothing more; the origin stays
	// silent; the kill fires at the lean budget.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Read(make([]byte, 16))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if lines := log.lines("WARN tunnel stall killed "); len(lines) > 0 {
			if !strings.Contains(lines[0], "cause=idle") {
				t.Fatalf("the stall line must carry the idle cause: %q", lines[0])
			}
			if got := srv.GoodIPCount(); got != 1 {
				t.Fatalf("good count after an IDLE stall kill = %d, want 1 (ambiguous evidence never demotes)", got)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the stall kill never fired")
}

// TestProbeRehabilitatesTheBoxedIP (M8-ac shape 4): a boxed origin
// that ANSWERS the pass's re-verification is cleared, marked good,
// and counted — the box self-corrects on fresh evidence instead of
// sitting wrong for its whole escalated TTL.
func TestProbeRehabilitatesTheBoxedIP(t *testing.T) {
	_, port, ips := probeFixture(t) // the corpse first, the echo second
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = staticResolver{ips: ips}
	})
	// Box the echo origin the hard way: a full-budget timeout death.
	srv.markDeadIP("127.0.0.2")
	if live, _ := srv.DeadBoxStats(); live != 1 {
		t.Fatalf("box before the pass = %d, want 1", live)
	}
	// One pass over a name whose answers are all already marked (the
	// stable front resolves to the corpse — boxed by the fixture's own
	// probe — and the echo — boxed above): the ONLY job left is the
	// rehabilitation sample.
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"www.youtube.com"}, port, 8)
	if sum.IPs < 1 {
		t.Fatalf("the pass must run at least the rehabilitation job: %+v", sum)
	}
	if d := st.Diagnostic(); d.ProbeRehabilitated != 1 {
		t.Fatalf("rehabilitated counter = %d, want 1 (the boxed echo origin answered)", d.ProbeRehabilitated)
	}
	// The rehabilitated origin itself must be OUT of the box and carrying
	// a good mark — the corpse this same pass probed MAY be boxed (its
	// silent verdict is fresh, correct evidence).
	for _, ip := range srv.PenalizedIPs(8) {
		if ip == "127.0.0.2" {
			t.Fatalf("the answered origin must be CLEARED from the box")
		}
	}
	if got := srv.GoodIPCount(); got < 1 {
		t.Fatalf("the rehabilitated origin must carry a good mark, got %d", got)
	}
	lines := pollLines(log, "INFO origin probe: ", 1, 2*time.Second)
	if len(lines) == 0 || !strings.Contains(lines[0], "rehab=1") {
		t.Fatalf("the probe line must carry the rehabilitation count: %q", lines)
	}
}

// TestSubstitutesRotateThePool (M8-ac shape 5): consecutive
// substitute lists lead with different pool origins.
func TestSubstitutesRotateThePool(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
	})
	srv.markGoodIP("192.0.2.20", true)
	srv.markGoodIP("192.0.2.21", true)
	srv.markGoodIP("192.0.2.22", true)
	own := []net.IP{net.ParseIP("198.51.100.1")}
	first := srv.goodSubstitutes(own, 3)
	second := srv.goodSubstitutes(own, 3)
	third := srv.goodSubstitutes(own, 3)
	if len(first) != 3 || len(second) != 3 || len(third) != 3 {
		t.Fatalf("the full pool must be offered: %v %v %v", first, second, third)
	}
	if first[0].Equal(second[0]) || second[0].Equal(third[0]) || first[0].Equal(third[0]) {
		t.Fatalf("the substitute lead is STATIC — the pool bet concentrates: %v %v %v", first, second, third)
	}
}

// Compile-time guards that the unused-import set stays honest.
var _ = config.MaxHeaderBytes
var _ = stats.New
var _ = atomic.Int64{}
