package proxy

// The M8-w shapes (ADR-0021), pinned by test:
//
//   1. The first-dial race — a processed host's flow races its primary
//      origin against FirstByteRaceDials extra candidates FROM THE FLOW'S
//      START: the candidates dial distinct origins of the answer set
//      immediately (rotated lists), replay the client's first record
//      byte-exact, and the first origin to speak wins. The debug.14
//      report named the serial front's cost: attempt 0 still walked its
//      budget ALONE before the M8-v salvage race launched (six timeout
//      terminals at 5320–5576 ms = the two-budget chain), while
//      CONCURRENT flows to the same hosts answered at 72–162 ms in the
//      same seconds — the dirt is per-origin, not per-moment.
//
//   2. The raced terminal — an all-dead answer set fails at ~1 budget
//      (the round runs in parallel), not the M8-v serial-then-race 2x.
//
//   3. The quiet primary — a clean primary that speaks first wins and
//      nothing is logged (no raced line, no retries, no salvage): the
//      race costs a healthy flow nothing observable.
//
// Both-ways proof: with the tunnel.go raced round stashed (the M8-v
// shape), tests 1 and 2 FAIL — the dirty-primary flow recovers only
// through the salvage (retry + salvaged lines, a full budget later)
// and the all-dead terminal closes at ~2x the budget — and every guard
// (quiet primary, unrouted host, racing disabled, byte-exactness,
// demotion) passes on both sides, pinning exactly what changed and
// nothing else.

import (
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// M8-w shape 1: the first-dial race saves a dirty primary.
// ---------------------------------------------------------------------------

// TestFirstDialRaceSavesDirtyPrimary: the primary origin is silent
// (the debug.14 report's dead-origin shape) and a live echo sits on the
// answer set's second address. With racing enabled (the default), the
// extras dial from t=0 and the flow must deliver the echo through the
// WINNING candidate — well inside one budget, with the raced line and
// none of the salvage lines (the raced round replaced the salvage).
// Pre-fix (the M8-v shape) this FAILS: the flow recovers only through
// the serial attempt-0 budget + the raced salvage re-dials (retry +
// salvaged lines, a full budget later).
func TestFirstDialRaceSavesDirtyPrimary(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0) // the primary's corpse
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	log := &recLogger{}
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{
		net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"),
	}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 5 * time.Second
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the raced flow must deliver the echo: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch through the raced winner")
	}
	// The raced line is the save's evidence; the salvage lines must all
	// be absent — the raced round replaced the salvage (the budget is
	// spent at the start, so its rounds never run).
	raced := pollLines(log, "INFO tunnel first-byte raced ", 1, 2*time.Second)
	if len(raced) == 0 || !strings.Contains(raced[0], "dials=2") {
		t.Fatalf("raced line missing/wrong: %+v", log.lines("INFO tunnel first-byte"))
	}
	for _, bad := range []string{
		"WARN tunnel first-byte retry ",
		"INFO tunnel first-byte salvaged ",
		"WARN tunnel first-byte failed ",
	} {
		if got := log.lines(bad); len(got) != 0 {
			t.Fatalf("the raced save must not log %q: %q", strings.TrimSpace(bad), got[0])
		}
	}
	// The flow is still a working relay through the winner.
	again := make([]byte, len(record))
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, again); err != nil {
		t.Fatalf("the flow must stay up after the raced save: %v", err)
	}
	if string(again) != string(record) {
		t.Fatal("second echo mismatch")
	}
	// The dirty primary demoted the answer set (the M8-o loop extends to
	// raced deaths — the origin was silent, whatever its sibling did).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if inv := res.invalidatedHosts(); len(inv) > 0 && inv[0] == "www.youtube.com" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Invalidate was never called for the dirty primary (the M8-o loop must extend to the raced round)")
}

// TestFirstDialRaceWinnerDeliversByteExact (guard): the raced winner's
// replay must be byte-exact — the client's first record re-fragmented
// through the fresh fragment conn's first Write — and the bytes the
// client sent DURING the race (frozen in the socket, exactly the M8-v
// salvage race's semantics) must flow to the winner in order through
// the relay.
func TestFirstDialRaceWinnerDeliversByteExact(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0) // the primary's corpse
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	srv, _ := newTestServer(t, func(c *Config) {
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"),
		}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer conn.Close()
	record := tlsLikeRecord(64)
	extra := []byte("EARLY-DATA-0123456789ABCDEF")
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(extra); err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte(nil), record...), extra...)
	got := make([]byte, len(want))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the raced flow must deliver record+extra: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("the raced winner's delivery is not byte-exact")
	}
}

// ---------------------------------------------------------------------------
// M8-w shape 2: the raced terminal is bounded at ~1 budget.
// ---------------------------------------------------------------------------

// TestFirstDialRaceTerminalIsBounded: every origin of the answer set is
// silent. The raced round runs its candidates in parallel, so the flow
// must fail at ~1 budget — inside 1.8x — while the M8-v shape (serial
// attempt 0 + the raced salvage) closes at ~2x (a full budget later).
// Pre-fix this FAILS: the close arrives a full budget later.
func TestFirstDialRaceTerminalIsBounded(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1<<30) // every connection silent
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 300 * time.Millisecond
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// 550 ms: one budget (300) plus the dial and slack. The M8-v shape
	// (300 serial + 300 raced) cannot land inside it.
	conn.SetReadDeadline(time.Now().Add(550 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 64)); err == nil {
		t.Fatal("expected the exhausted flow to close")
	} else if isTimeout(err) {
		t.Fatalf("the flow was still up 550 ms in — the serial-then-race shape, not the raced round's ~1 budget")
	}
	// The full candidate count still ran: 1 primary + 2 raced extras.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && accepts() < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := accepts(); n != 3 {
		t.Fatalf("origin dials = %d, want 3 (1 primary + 2 raced)", n)
	}
	fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
	if len(fbs) == 0 || !strings.Contains(fbs[0], "reason=timeout") {
		t.Fatalf("terminal line wrong: %+v", log.lines("WARN "))
	}
	dones := pollLines(log, "INFO tunnel done ", 1, 2*time.Second)
	if len(dones) == 0 || !strings.Contains(dones[0], "down=0 B") {
		t.Fatalf("done line must show down=0 B: %+v", dones)
	}
	// The raced round replaced the salvage: no retry lines, no salvage.
	if got := log.lines("WARN tunnel first-byte retry "); len(got) != 0 {
		t.Fatalf("the raced round must not log salvage retries: %q", got[0])
	}
}

// ---------------------------------------------------------------------------
// M8-w guards: the quiet primary, the unrouted host, the disabled knob.
// ---------------------------------------------------------------------------

// delayedEchoListenerAt binds exactly ip:port and echoes every
// connection only after delay — the "slow but live" origin that gives
// the primary a provable head start.
func delayedEchoListenerAt(t *testing.T, ip string, port int, delay time.Duration) {
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
				defer c.Close()
				buf := make([]byte, 4096)
				n, _ := c.Read(buf)
				if n == 0 {
					return
				}
				time.Sleep(delay)
				if _, werr := c.Write(buf[:n]); werr != nil {
					return
				}
				for {
					n, rerr := c.Read(buf)
					if n > 0 {
						if _, werr := c.Write(buf[:n]); werr != nil {
							return
						}
					}
					if rerr != nil {
						return
					}
				}
			}(c)
		}
	}()
}

// TestFirstDialRacePrimaryWinsQuietly (guard): a clean primary that
// speaks first wins and NOTHING is logged — no raced line (the save is
// the primary's own), no retries, no salvage, no failure. The race
// costs a healthy flow nothing observable. The extras sit on
// slower-but-live origins (150 ms), so the primary's head start is
// provable, not a scheduling hope. Passes on both sides of the fix.
func TestFirstDialRacePrimaryWinsQuietly(t *testing.T) {
	port := 0
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err == nil {
		port = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	} else {
		t.Fatal(err)
	}
	echoListenerAt(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(port))) // the primary: instant echo
	delayedEchoListenerAt(t, "127.0.0.2", port, 150*time.Millisecond)    // extra 1: slow but live
	delayedEchoListenerAt(t, "127.0.0.3", port, 150*time.Millisecond)    // extra 2: slow but live
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 2 * time.Second
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the clean-primary flow must deliver the echo: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch")
	}
	// The quiet primary: no raced line (the winner IS the primary), none
	// of the salvage machinery's lines, no failure.
	for _, bad := range []string{
		"INFO tunnel first-byte raced ",
		"WARN tunnel first-byte retry ",
		"INFO tunnel first-byte salvaged ",
		"WARN tunnel first-byte failed ",
	} {
		if got := log.lines(bad); len(got) != 0 {
			t.Fatalf("a clean primary must not log %q: %q", strings.TrimSpace(bad), got[0])
		}
	}
	// The flow is a working relay through the primary.
	again := make([]byte, len(record))
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, again); err != nil {
		t.Fatalf("the flow must stay up through the primary: %v", err)
	}
	if string(again) != string(record) {
		t.Fatal("second echo mismatch")
	}
}

// TestFirstDialRaceSparesUnroutedHosts (guard): the race, like the
// budget and the salvage before it, is part of the processing of
// routed hosts only — an unrouted relay never races and keeps the plain
// idle semantics.
func TestFirstDialRaceSparesUnroutedHosts(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1<<30)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 900 * time.Millisecond
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("example.test", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// Past the budget, before the idle timeout: one dial, no race, the
	// flow still up.
	time.Sleep(600 * time.Millisecond)
	if n := accepts(); n != 1 {
		t.Fatalf("unrouted relay dialed %d times; the race must never touch a plain relay", n)
	}
	for _, bad := range []string{
		"WARN tunnel first-byte ",
		"INFO tunnel first-byte raced ",
		"INFO tunnel first-byte salvaged ",
	} {
		if got := log.lines(bad); len(got) != 0 {
			t.Fatalf("unrouted flow must not log %q: %q", strings.TrimSpace(bad), got[0])
		}
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("expected the idle close")
	}
	if n := accepts(); n != 1 {
		t.Fatalf("unrouted relay dialed %d times after the idle close; want 1", n)
	}
}

// TestFirstDialRaceDisabledByConfig: a negative FirstByteRaceDials pins
// the M8-v behavior exactly — serial attempt 0 through its budget, then
// the raced salvage re-dials recover the flow. The raced round never
// runs (no raced line); the salvage lines are the M8-v shape's.
func TestFirstDialRaceDisabledByConfig(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0) // attempt 0's corpse
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1 // the M8-v shape exactly
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"),
		}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the salvage must still recover the flow with racing disabled: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch through the salvaged flow")
	}
	// The M8-v evidence: the retry lines and the salvaged line — and
	// never the raced line.
	retries := pollLines(log, "WARN tunnel first-byte retry ", 2, 2*time.Second)
	if len(retries) < 2 {
		t.Fatalf("the disabled-racing shape must keep its salvage retries: %+v", log.lines("WARN "))
	}
	salvaged := pollLines(log, "INFO tunnel first-byte salvaged ", 1, 2*time.Second)
	if len(salvaged) == 0 {
		t.Fatalf("the disabled-racing shape must keep its salvaged line: %+v", log.lines("INFO tunnel first-byte"))
	}
	if raced := log.lines("INFO tunnel first-byte raced "); len(raced) != 0 {
		t.Fatalf("a disabled race must never log the raced line: %q", raced[0])
	}
}
