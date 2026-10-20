package proxy

// The M8-ad shapes (ADR-0037/0038), pinned by test:
//
//   1. The rich PENDING fast kill — a flow that had proven itself
//      (payload delivered) dies with the client's request OUTSTANDING,
//      and the kill fires at StallRichPendingTimeout, not the 90 s
//      hygiene budget. This is the tester's post-debug.21 report
//      exactly ("minute 5 of a 10-minute video goes loading, and as
//      soon as you refresh it fixes"): the player's pooled video conn
//      died mid-life, its next range request rode the corpse, and the
//      app sat wedged far past its own patience. Pre-fix the wedge is
//      bounded only by the full rich budget — minutes of spinner.
//
//   2. The trickle kill — a proven flow whose origin keeps DELIVERING
//      but under the floor (the per-connection throttle: the conn
//      stays open, the down stamp keeps refreshing, the both-silent
//      stall clock never accumulates). The kill closes both ends
//      quietly so the app's retry lands on a fresh, unthrottled conn —
//      engine-side what the tester's REFRESH did by hand. Pre-fix the
//      flow lives to the idle timeout trickling.
//
//   3. The stall kill keeps the answer set — no invalidateDNS on any
//      stall kill (ADR-0038). The debug.20 report measured the churn
//      spiral this created (dns=579-846 ms late-session, 59 DoH dials):
//      every killed flow sent the next request back through the
//      struggling DoH lane for a fresh answer set whose old IPs were
//      fine. The old Python cached DNS forever. Pre-fix the lean kill
//      invalidates; the next request pays the dying lane again.
//
// Both-ways proof: the pending fast budget neutered (disabled), test 1
// FAILS waiting for the kill (the flow rides the 8 s rich budget in
// the test's window); the trickle detector neutered (a MinBytes of
// zero), test 2 FAILS on the surviving flow; the invalidateDNS call
// restored at the kill site, test 3 FAILS on the recorded
// invalidation.

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/stats"
)

// richStallOriginServer echoes the first flight, streams enough
// payload to prove the flow (rich by bytes), then holds the conn open
// in total silence — the proven conn that died mid-life.
func richStallOriginServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
				defer c.Close()
				buf := make([]byte, 4096)
				n, rerr := c.Read(buf)
				if n > 0 {
					if _, werr := c.Write(buf[:n]); werr != nil {
						return
					}
					// The rich proof: past StallRichBytes in one burst.
					if _, werr := c.Write(make([]byte, 64*1024)); werr != nil {
						return
					}
				}
				if rerr != nil {
					return
				}
				for { // the death: hold the conn open, never answer again
					if _, rerr := c.Read(buf); rerr != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

// trickleOriginServer echoes the first flight, streams a fast fill
// (past the floor), then throttles to a crawl — the per-connection
// police shape: bytes keep arriving, never enough.
func trickleOriginServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
				defer c.Close()
				buf := make([]byte, 4096)
				n, rerr := c.Read(buf)
				if n > 0 {
					if _, werr := c.Write(buf[:n]); werr != nil {
						return
					}
					// The fast fill: the floor, delivered at line rate.
					if _, werr := c.Write(make([]byte, 100*1024)); werr != nil {
						return
					}
				}
				if rerr != nil {
					return
				}
				chunk := make([]byte, 400)
				for { // the throttle: ~2.7 KB/s, forever
					time.Sleep(150 * time.Millisecond)
					if _, werr := c.Write(chunk); werr != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

// TestRichPendingKillAtTheFastBudget (M8-ad shape 1): a proven flow
// whose client's request went unanswered is closed at the pending fast
// budget — the demote lands (dead with work outstanding), the app's
// conn dies NOW (its retry is the recovery), and the answer set is
// never invalidated.
func TestRichPendingKillAtTheFastBudget(t *testing.T) {
	up := richStallOriginServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.IdleTimeout = 5 * time.Second
		c.StallTimeout = 400 * time.Millisecond
		c.StallRichTimeout = 8 * time.Second // the hygiene budget the wedge used to ride
		c.StallRichPendingTimeout = 600 * time.Millisecond
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	// Drain continuously — the app keeps reading its video bytes.
	drained := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, conn)
		drained <- err
	}()
	// The winner marked its origin good at the first byte (M8-ab's
	// flow-fed pool): the demote's subject.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.GoodIPCount() == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after the first byte = %d, want 1 (the flow-fed mark)", got)
	}
	// The PENDING shape: the player's next range request on the pooled
	// conn — the origin never answers it.
	time.Sleep(150 * time.Millisecond)
	if _, err := conn.Write([]byte("the player's next range request")); err != nil {
		t.Fatal(err)
	}
	// The kill must fire at the FAST budget (~600 ms), not the 8 s
	// hygiene budget: the line is the proof, the app-side close is the
	// recovery the tester used to deliver by hand (the refresh).
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if lines := log.lines("WARN tunnel stall killed "); len(lines) > 0 {
			if !strings.Contains(lines[0], "tier=rich") || !strings.Contains(lines[0], "cause=pending") {
				t.Fatalf("the rich pending kill line must carry tier=rich cause=pending: %q", lines[0])
			}
			if got := srv.GoodIPCount(); got != 0 {
				t.Fatalf("good count after a rich PENDING kill = %d, want 0 (dead with work outstanding demotes)", got)
			}
			d := st.Diagnostic()
			if d.StallRichPending != 1 || d.StallKillRich != 1 {
				t.Fatalf("rich pending counters = %+v %+v, want 1 and 1", d.StallRichPending, d.StallKillRich)
			}
			if hosts := res.invalidatedHosts(); len(hosts) != 0 {
				t.Fatalf("a stall kill must never invalidate the answer set (ADR-0038): %v", hosts)
			}
			// The app's conn must be dead — the retry path is open.
			select {
			case <-drained:
				return
			case <-time.After(2 * time.Second):
				t.Fatalf("the app-side conn survived the kill — the wedge stays")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the rich pending kill never fired (the wedge rides the hygiene budget)")
}

// TestTrickleKillClosesTheThrottledFlow (M8-ad shape 2): a proven flow
// being per-connection throttled (bytes keep arriving, under the
// floor) is closed quietly — no demote (a fresh conn to the same
// origin runs at full speed), no error count, and the app's conn dies
// so its retry lands on a fresh conn.
func TestTrickleKillClosesTheThrottledFlow(t *testing.T) {
	up := trickleOriginServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.IdleTimeout = 5 * time.Second
		c.StallTimeout = 400 * time.Millisecond // tick 100 ms
		c.StallRichTimeout = 8 * time.Second    // the hygiene budget must NOT own this kill
		c.StallRichPendingTimeout = -1          // isolate: the trickle path is under test
		c.TrickleWindow = 1 * time.Second       // ring of 10 snapshots at the 100 ms tick
		c.TrickleMinBytes = 4 * 1024            // the floor: 2.7 KB/s of throttle lands under it
		c.TrickleFloorBytes = 64 * 1024         // the fast fill arms the detector
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	drained := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, conn)
		drained <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.GoodIPCount() == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after the first byte = %d, want 1", got)
	}
	// The throttle runs; the detector must close the flow inside its
	// window past the fill (~1-1.5 s), not the 8 s hygiene budget.
	deadline = time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if lines := log.lines("WARN tunnel stall killed "); len(lines) > 0 {
			if !strings.Contains(lines[0], "tier=rich") || !strings.Contains(lines[0], "cause=trickle") {
				t.Fatalf("the trickle kill line must carry tier=rich cause=trickle: %q", lines[0])
			}
			if got := srv.GoodIPCount(); got != 1 {
				t.Fatalf("good count after a trickle kill = %d, want 1 (policing follows the conn, never demotes)", got)
			}
			d := st.Diagnostic()
			if d.StallTrickle != 1 {
				t.Fatalf("trickle counter = %d, want 1", d.StallTrickle)
			}
			if hosts := res.invalidatedHosts(); len(hosts) != 0 {
				t.Fatalf("a stall kill must never invalidate the answer set (ADR-0038): %v", hosts)
			}
			select {
			case <-drained:
				return
			case <-time.After(2 * time.Second):
				t.Fatalf("the app-side conn survived the trickle kill — the crawl continues")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the trickle kill never fired (the throttled flow crawled past the window)")
}

// TestStallKillKeepsTheAnswerSet (M8-ad shape 3, ADR-0038): the lean
// stall kill — the historical invalidateDNS site — leaves the
// resolver's cache alone: the kill is an origin-conn death, the per-IP
// demote already carries the evidence, and the answer set's own
// refresh belongs to the dial-failure path (M8-o) where a whole set
// failed to connect.
func TestStallKillKeepsTheAnswerSet(t *testing.T) {
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
	// The idle shape: nobody speaks again; the lean kill fires.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Read(make([]byte, 16))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if lines := log.lines("WARN tunnel stall killed "); len(lines) > 0 {
			if hosts := res.invalidatedHosts(); len(hosts) != 0 {
				t.Fatalf("the stall kill invalidated the answer set (the churn spiral's feed): %v", hosts)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the lean stall kill never fired")
}

// Compile-time guards that the unused-import set stays honest.
var _ = stats.New
