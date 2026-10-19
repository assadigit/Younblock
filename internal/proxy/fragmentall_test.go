package proxy

// The M8-z work item A shapes (ADR-0024), pinned by test:
//
//   1. FragmentAllRouted on → a NON-listed host's first TLS record is
//      fragmented (the plain cohort the debug.16 evidence named: the
//      ad/attestation/Google-API hosts the YouTube app needs at first
//      play, killed by SNI inspection because nothing fragmented
//      them) — AND the flow does NOT use the budgeted path (no
//      first-byte budget death while the origin stays silent past
//      FirstByteTimeout; the plain safety net of work item B owns
//      that cohort instead).
//
//   2. FragmentAllRouted off → yesterday's rule byte for byte: the
//      non-listed host's record arrives in ONE read.
//
//   3. A direct-TLD host (.ir, D83) is never fragmented in either
//      setting.
//
//   4. A listed (budgeted) host is budgeted in both settings — the
//      expensive machinery never follows FragmentAllRouted.
//
// Both-ways proof: with fragmentHost neutered (the M8-y shape — the
// doors ask routedHost alone), test 1 FAILS (the record arrives in one
// read, the exact blocker shape the report hypothesized) and test 4's
// budget assertion FAILS if the budgeted gate is widened to
// fragmentTLS. Tests 2 and 3 pass on both sides, pinning exactly what
// changed and nothing else.

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/fragment"
)

// fragTarget serves one recording origin at a time: it accepts, drains
// (counting reads via the recordingListener), and never answers — the
// silent-origin shape the budgeted/plain distinctions are measured
// against.
func fragTarget(t *testing.T) (*recordingListener, string) {
	t.Helper()
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingListener{Listener: rawLn}
	go func() {
		for {
			c, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	t.Cleanup(func() { rawLn.Close() })
	return rec, strconv.Itoa(rawLn.Addr().(*net.TCPAddr).Port)
}

// waitForReads waits until the latest recording connection has at
// least min reads, then settles and returns the settled count. The
// deadline is generous: under a full-suite parallel run the tunnel
// setup competes for CPU with every other package's tests.
func waitForReads(t *testing.T, rec *recordingListener, min int64) int64 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rc := rec.latest(); rc != nil {
			if reads, _ := rc.stats(); reads >= min {
				time.Sleep(150 * time.Millisecond) // let any further pieces land
				reads, _ = rc.stats()
				return reads
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0
}

// TestFragmentAllRoutedFragmentsNonListedHost (shape 1a, the fix's
// core): FragmentAllRouted on, youtube mode, a NON-listed host — the
// first TLS record must arrive at the origin in more than one read.
// Pre-fix (fragmentHost → routedHost alone) this FAILS: one read.
func TestFragmentAllRoutedFragmentsNonListedHost(t *testing.T) {
	rec, port := fragTarget(t)
	srv, _ := newTestServer(t, func(c *Config) {
		c.FragmentAllRouted = true
		c.IdleTimeout = 2 * time.Second
		// A pause between pieces keeps them separable at the reader
		// under a loaded run (Pause 0 lets the origin coalesce pieces
		// into one read — the count would flake, not the behavior).
		c.FragmentConfig = fragment.Config{Min: 80, Max: 200, Pause: 25 * time.Millisecond}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", port))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(400)); err != nil {
		t.Fatal(err)
	}
	if reads := waitForReads(t, rec, 2); reads < 2 {
		t.Fatalf("FragmentAllRouted: non-listed host's first record arrived in %d reads, want >= 2 (fragmented)", reads)
	}
}

// TestFragmentAllRoutedDoesNotUseBudgetedPath (shape 1b): the plain
// cohort stays OUT of the budgeted machinery. A silent origin with
// FirstByteTimeout=250 ms: budgeted hosts terminal at the budget
// ("tunnel first-byte failed"); this flow must NOT — it stays open
// past the budget (the work item B net owns its silence instead; here
// the net is disabled so the flow simply lives). Pre-fix this passes
// trivially (the host was never budgeted); post-fix it pins that
// fragmentHost alone did NOT widen the budgeted gate.
func TestFragmentAllRoutedDoesNotUseBudgetedPath(t *testing.T) {
	_, port := fragTarget(t)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.FragmentAllRouted = true
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.PlainFirstByteTimeout = -1 // the safety net is item B's own test's business
		c.IdleTimeout = 2 * time.Second
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", port))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// 3x the budget: no budgeted death line, no conn error for the flow.
	time.Sleep(750 * time.Millisecond)
	if lines := log.lines("WARN tunnel first-byte failed "); len(lines) != 0 {
		t.Fatalf("plain flow hit the budgeted machinery: %+v", lines)
	}
}

// TestFragmentAllRoutedOffKeepsOldRelay (shape 2, guard): the setting
// off keeps yesterday's rule byte for byte — the non-listed host's
// record arrives in ONE read. Passes on both sides of the fix.
func TestFragmentAllRoutedOffKeepsOldRelay(t *testing.T) {
	rec, port := fragTarget(t)
	srv, _ := newTestServer(t, func(c *Config) {
		c.FragmentAllRouted = false
		c.IdleTimeout = 2 * time.Second
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", port))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(400)); err != nil {
		t.Fatal(err)
	}
	if reads := waitForReads(t, rec, 1); reads != 1 {
		t.Fatalf("FragmentAllRouted off: record arrived in %d reads, want exactly 1 (the old plain relay)", reads)
	}
}

// TestFragmentAllRoutedNeverTouchesDirectTLD (shape 3, guard): a .ir
// host is never fragmented, whatever FragmentAllRouted says (D83).
func TestFragmentAllRoutedNeverTouchesDirectTLD(t *testing.T) {
	for _, on := range []bool{true, false} {
		rec, port := fragTarget(t)
		srv, _ := newTestServer(t, func(c *Config) {
			c.FragmentAllRouted = on
			c.IdleTimeout = 2 * time.Second
		})
		conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("example.ir", port))
		if _, err := conn.Write(tlsLikeRecord(400)); err != nil {
			t.Fatal(err)
		}
		if reads := waitForReads(t, rec, 1); reads != 1 {
			t.Fatalf("FragmentAllRouted=%v: .ir host's record arrived in %d reads, want 1 (D83: never processed)", on, reads)
		}
		conn.Close()
		srv.StopWithTimeout()
	}
}

// TestBudgetedListedHostInBothSettings (shape 4, guard): a YouTube-list
// host is budgeted whether FragmentAllRouted is on or off — the
// expensive machinery follows budgetedHost alone. A silent origin
// terminals at the budget with the budgeted death line in BOTH
// settings.
func TestBudgetedListedHostInBothSettings(t *testing.T) {
	for _, on := range []bool{true, false} {
		_, port := fragTarget(t)
		log := &recLogger{}
		srv, _ := newTestServer(t, func(c *Config) {
			c.FragmentAllRouted = on
			c.Logger = log
			c.FirstByteTimeout = 250 * time.Millisecond
			c.FirstByteRaceDials = -1
			c.FirstByteSalvageDials = -1
			c.IdleTimeout = 2 * time.Second
		})
		conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", port))
		conn.Write(tlsLikeRecord(64))
		conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
		if _, err := conn.Read(make([]byte, 32)); err == nil {
			t.Fatalf("FragmentAllRouted=%v: listed host must terminal on the silent origin", on)
		}
		conn.Close()
		lines := log.lines("WARN tunnel first-byte failed ")
		if len(lines) != 1 {
			t.Fatalf("FragmentAllRouted=%v: budgeted failures = %d, want 1: %+v", on, len(lines), lines)
		}
		if !strings.Contains(lines[0], "class=routed") {
			t.Fatalf("listed host's failure line must carry class=routed: %q", lines[0])
		}
		srv.StopWithTimeout()
	}
}

// TestClassTagRidesEveryTunnelLine (ADR-0027 §8.2): the up/first-byte/
// done lines carry class=routed|plain in the MESSAGE text (the report's
// ring drops attrs — the tag must survive it).
func TestClassTagRidesEveryTunnelLine(t *testing.T) {
	up, _ := flakyOriginServer(t, 1<<30) // silent origin: plain flow, no kill
	_, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.IdleTimeout = 300 * time.Millisecond
		c.PlainFirstByteTimeout = -1 // let the idle timeout close it: done line still lands
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("plain.example.com", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, err := conn.Read(make([]byte, 16)); err != nil {
			break
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(log.lines("INFO tunnel done ")) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, prefix := range []string{"INFO tunnel up ", "INFO tunnel done "} {
		lines := log.lines(prefix)
		if len(lines) == 0 {
			t.Fatalf("no %q line logged: %+v", strings.TrimSpace(prefix), log.lines(prefix))
		}
		if !strings.Contains(lines[0], "class=plain") {
			t.Fatalf("%q line missing class=plain in the message: %q", strings.TrimSpace(prefix), lines[0])
		}
	}
	if lines := log.lines("INFO tunnel up "); !strings.Contains(lines[0], "rtt-class=") {
		t.Fatalf("tunnel up line missing rtt-class: %q", lines[0])
	}
	fmt.Fprint(io.Discard, "") // keep fmt/io imported for fragTarget
}
