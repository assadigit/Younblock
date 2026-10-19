package proxy

// The M8-s first-byte budget (ADR-0018), pinned by test: a tunnel to a
// processed host whose origin never answers must fail FAST — both ends
// closed, the error counted, the reason baked into the report's log
// message, and the host's cached answer set demoted exactly like a total
// dial failure (the M8-o loop extended to silent origins) — while
// unrouted relays keep the plain idle-timeout semantics and origins that
// answer within the budget flow normally. The round's both-ways proof:
// with the tunnel.go restructure stashed (the pre-M8-s relay), the
// silent processed flow hangs to the idle timeout and neither the reason
// line nor the demotion arrives in time.

// The M8-o recording resolver (dialfeedback_test.go's invalidatingResolver)
// is reused as-is: the budget extends the same Invalidate capability to
// origins that connect but never speak.

import (
	"bytes"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// delayedSpeakServer accepts, stays silent for delay, then echoes.
func delayedSpeakServer(t *testing.T, delay time.Duration) string {
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
				time.Sleep(delay)
				buf := make([]byte, 4096)
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
	return ln.Addr().String()
}

// pollLines waits for the ring to carry `want` lines with the prefix.
func pollLines(log *recLogger, prefix string, want int, timeout time.Duration) []string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := log.lines(prefix); len(got) >= want {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	return log.lines(prefix)
}

func TestFirstByteTimeoutFailsSilentRoutedTunnelFast(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		// M8-t: this test pins the TERMINAL failure shape; the salvage
		// (ADR-0019) has its own tests below.
		c.FirstByteSalvageDials = -1
		// M8-w: this test pins the M8-s terminal exactly — one origin,
		// one budget, no raced candidates either.
		c.FirstByteRaceDials = -1
		c.IdleTimeout = 5 * time.Second
		c.Resolver = &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	})
	// www.youtube.com is on the default list: the tunnel is processed.
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// The client must see the flow die well inside the 5 s idle timeout:
	// the budget is 250 ms.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err == nil {
		t.Fatal("expected the silent processed tunnel to close")
	}
	// The evidence lines (the reason baked into the message — the ring
	// drops attrs, the M8-r lesson).
	fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
	if len(fbs) == 0 {
		t.Fatalf("no first-byte-failed line: %+v", log.lines("WARN "))
	}
	if !strings.Contains(fbs[0], "reason=timeout") {
		t.Fatalf("first-byte-failed line missing reason=timeout: %q", fbs[0])
	}
	dones := pollLines(log, "INFO tunnel done ", 1, 2*time.Second)
	if len(dones) == 0 {
		t.Fatal("no done line (the flow must still report its totals)")
	}
	if !strings.Contains(dones[0], "down=0 B") {
		t.Fatalf("done line must show down=0 B: %q", dones[0])
	}
}

func TestFirstByteFailureInvalidatesSuspectSet(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteSalvageDials = -1 // the terminal shape, exactly one attempt
		c.FirstByteRaceDials = -1    // ...and exactly one origin (the M8-s shape, no raced round)
		c.IdleTimeout = 5 * time.Second
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err == nil {
		t.Fatal("expected the silent processed tunnel to close")
	}
	// The demotion lands with the failure: the set connected but never
	// spoke, so it is suspect and the next lookup re-queries the servers.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := res.invalidatedHosts(); len(got) > 0 {
			if got[0] != "www.youtube.com" {
				t.Fatalf("invalidated %q, want the routed host", got[0])
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Invalidate was never called for the silent origin (the M8-o loop must extend to first-byte failures)")
}

func TestFirstByteBudgetSparesUnroutedHosts(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 900 * time.Millisecond
		c.Resolver = &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	})
	// example.test is NOT on the default list: a plain relay, never the
	// app's processing (D46) — and never the budget.
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("example.test", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// Past the budget, before the idle timeout: the flow must still be up.
	time.Sleep(500 * time.Millisecond)
	conn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("unexpected data from the silent origin")
	} else if !isTimeout(err) {
		t.Fatalf("unrouted flow closed early (%v); the budget must never touch a plain relay", err)
	}
	// And it still dies by the idle timeout — the plain shape's semantics.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("expected the idle close")
	}
	if got := log.lines("WARN tunnel first-byte failed "); len(got) != 0 {
		t.Fatalf("unrouted flow must not log first-byte failures: %q", got[0])
	}
}

func TestFirstByteWithinBudgetFlowsNormally(t *testing.T) {
	up := delayedSpeakServer(t, 120*time.Millisecond)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 400 * time.Millisecond
		c.Resolver = &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
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
		t.Fatalf("a late-but-within-budget first byte must flow: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch")
	}
	fbs := pollLines(log, "INFO tunnel first-byte ", 1, 2*time.Second)
	if len(fbs) == 0 {
		t.Fatal("no first-byte line for the within-budget flow")
	}
	if strings.Contains(fbs[0], "failed") {
		t.Fatalf("within-budget flow logged a failure: %q", fbs[0])
	}
}

// ---------------------------------------------------------------------------
// M8-t (ADR-0019): the salvage re-dial — a processed host's dead
// first-byte origin is re-dialed INSIDE the same flow (same host, same
// preset — never a method change, which is D102's to rule), the
// client's buffered bytes replayed, so the app never sees the failure.
// Both-ways proof: with the tunnel.go salvage stashed (the M8-s shape),
// every test below FAILS — the client sees the budget close, no retry
// line, one dial — and the two updated M8-s tests above (salvage
// disabled) keep pinning the terminal shape.
// ---------------------------------------------------------------------------

// flakyOriginServer accepts connections where the first silentN stay
// silent (the dead-origin shape: dial completes, hello accepted, never
// one byte back) and every later connection echoes. It counts accepts.
func flakyOriginServer(t *testing.T, silentN int) (string, func() int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var accepts atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if accepts.Add(1) <= int32(silentN) {
				go func(c net.Conn) { // dead: read and hold, never answer
					defer c.Close()
					buf := make([]byte, 4096)
					for {
						if _, rerr := c.Read(buf); rerr != nil {
							return
						}
					}
				}(c)
				continue
			}
			go func(c net.Conn) { // alive: echo
				defer c.Close()
				buf := make([]byte, 4096)
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
	return ln.Addr().String(), func() int { return int(accepts.Load()) }
}

// recordingFlakyOrigin: connection 1 records and stays silent; every
// later connection records and echoes. received(n) returns what
// connection n received — the byte-exactness proof for the replay.
func recordingFlakyOrigin(t *testing.T) (string, func(int) []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var order int
	received := make(map[int][]byte)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			order++
			idx := order
			mu.Unlock()
			go func(c net.Conn, idx int) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, rerr := c.Read(buf)
					if n > 0 {
						mu.Lock()
						received[idx] = append(received[idx], buf[:n]...)
						mu.Unlock()
						if idx >= 2 {
							if _, werr := c.Write(buf[:n]); werr != nil {
								return
							}
						}
					}
					if rerr != nil {
						return
					}
				}
			}(c, idx)
		}
	}()
	return ln.Addr().String(), func(idx int) []byte {
		mu.Lock()
		defer mu.Unlock()
		return append([]byte(nil), received[idx]...)
	}
}

// TestFirstByteSalvageRecoversDeadOrigin: the first origin is silent,
// the salvage re-dials the same host and the flow delivers the echo —
// the client NEVER sees a close. Pre-fix (the M8-s shape), this FAILS:
// the client's read errors at the budget, no retry, no salvage line.
func TestFirstByteSalvageRecoversDeadOrigin(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1 // M8-w: pin the salvage shape (serial attempt 0 + the raced re-dials)
		c.IdleTimeout = 5 * time.Second
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the salvaged flow must deliver the origin's bytes: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch through the salvaged flow")
	}
	// M8-v (ADR-0020): the salvage RACES its remaining re-dials — both
	// launch at once (attempt=1 and attempt=2 logged together, each
	// still counted as an absorbed attempt) and the first origin to
	// speak wins, so this single-answer host sees both dials land.
	// M8-w: the raced first dial is DISABLED here — this test pins the
	// M8-v salvage shape (serial attempt 0, then the raced re-dials);
	// the raced round has its own tests (firstrace_test.go).
	retries := pollLines(log, "WARN tunnel first-byte retry ", 2, 2*time.Second)
	if len(retries) < 2 || !strings.Contains(retries[0], "reason=timeout") || !strings.Contains(retries[0], "attempt=1") || !strings.Contains(retries[1], "attempt=2") {
		t.Fatalf("retry lines wrong: %+v", log.lines("WARN "))
	}
	salvaged := pollLines(log, "INFO tunnel first-byte salvaged ", 1, 2*time.Second)
	if len(salvaged) == 0 || !strings.Contains(salvaged[0], "dials=2") {
		t.Fatalf("salvaged line missing: %+v", log.lines("INFO tunnel first-byte"))
	}
	if fails := log.lines("WARN tunnel first-byte failed "); len(fails) != 0 {
		t.Fatalf("a salvaged flow must not log a terminal failure: %q", fails[0])
	}
	// Exactly three origin dials: the dead one plus the two raced
	// salvage candidates.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && accepts() < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := accepts(); n != 3 {
		t.Fatalf("origin dials = %d, want exactly 3 (1 dead + 2 raced)", n)
	}
	// The app never saw the failure: the flow is still a working relay.
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the flow must stay up after the salvage: %v", err)
	}
	// The dead attempt demoted the host's answer set (the M8-o loop).
	if inv := res.invalidatedHosts(); len(inv) == 0 || inv[0] != "www.youtube.com" {
		t.Fatalf("invalidated = %v, want the routed host", inv)
	}
}

// TestFirstByteSalvageExhaustsToFailFast: every origin is silent; the
// flow runs its bounded attempts (1 + the default 2 re-dials) and then
// fails exactly like M8-s — both ends closed, the reason line, the
// done line with down=0 B, the answer set demoted per dead attempt.
func TestFirstByteSalvageExhaustsToFailFast(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1<<30) // every connection silent
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 150 * time.Millisecond
		c.FirstByteRaceDials = -1 // M8-w: pin the salvage shape (serial attempt 0 + the raced re-dials)
		c.IdleTimeout = 5 * time.Second
		c.Resolver = res
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err == nil {
		t.Fatal("expected the exhausted flow to close")
	}
	fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
	if len(fbs) == 0 || !strings.Contains(fbs[0], "reason=timeout") {
		t.Fatalf("terminal failure line wrong: %+v", log.lines("WARN "))
	}
	dones := pollLines(log, "INFO tunnel done ", 1, 2*time.Second)
	if len(dones) == 0 || !strings.Contains(dones[0], "down=0 B") {
		t.Fatalf("done line must show down=0 B: %+v", log.lines("INFO tunnel done "))
	}
	// Exactly 1 + FirstByteSalvageDials (the default 2) dials — bounded.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && accepts() < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := accepts(); n != 3 {
		t.Fatalf("origin dials = %d, want exactly 3 (1 + 2 salvage re-dials)", n)
	}
	// One demotion per dead attempt (the M8-o loop, per attempt).
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(res.invalidatedHosts()) < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := len(res.invalidatedHosts()); got != 3 {
		t.Fatalf("invalidations = %d, want 3 (one per dead attempt)", got)
	}
}

// TestSalvageReplayIsByteExact: the client's first record plus the
// early data it sent during the wait must arrive at the fresh origin
// byte-exact — the record re-fragmented through the fresh fragment
// conn's first Write, the extras right after.
func TestSalvageReplayIsByteExact(t *testing.T) {
	up, received := recordingFlakyOrigin(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, _ := newTestServer(t, func(c *Config) {
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1 // M8-w: pin the salvage replay's shape (the salvage round owns the replay)
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
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
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the salvaged flow must echo record+extra: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("echo mismatch")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(received(2)) < len(want) {
		time.Sleep(10 * time.Millisecond)
	}
	if !bytes.Equal(received(2), want) {
		t.Fatalf("the fresh origin's replay is not byte-exact:\n got  %q\n want %q", received(2), want)
	}
}

// TestFirstByteSalvageSparesUnroutedHosts (guard): the salvage, like
// the budget before it, is part of the processing of routed hosts only
// — an unrouted relay never re-dials and keeps the plain idle
// semantics.
func TestFirstByteSalvageSparesUnroutedHosts(t *testing.T) {
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
	// Past the budget, before the idle timeout: one dial, no retries,
	// the flow still up.
	time.Sleep(600 * time.Millisecond)
	if n := accepts(); n != 1 {
		t.Fatalf("unrouted relay dialed %d times; the salvage must never touch a plain relay", n)
	}
	if got := log.lines("WARN tunnel first-byte "); len(got) != 0 {
		t.Fatalf("unrouted flow must not log first-byte lines: %q", got[0])
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("expected the idle close")
	}
	if n := accepts(); n != 1 {
		t.Fatalf("unrouted relay dialed %d times after the idle close; want 1", n)
	}
}

// TestFirstByteSalvageDisabledByConfig: a negative
// FirstByteSalvageDials pins the M8-s behavior exactly — one dial, the
// budget's failure, never a retry.
func TestFirstByteSalvageDisabledByConfig(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1<<30)
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 5 * time.Second
		c.FirstByteSalvageDials = -1
		c.FirstByteRaceDials = -1 // M8-w: racing off too — this pins the pure M8-s shape; racing-off-alone is firstrace_test.go's disabled test
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err == nil {
		t.Fatal("expected the disabled-salvage flow to close at the budget")
	}
	fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
	if len(fbs) == 0 || !strings.Contains(fbs[0], "reason=timeout") {
		t.Fatalf("failure line wrong: %+v", log.lines("WARN "))
	}
	if retries := log.lines("WARN tunnel first-byte retry "); len(retries) != 0 {
		t.Fatalf("disabled salvage must never retry: %q", retries[0])
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && accepts() < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := accepts(); n != 1 {
		t.Fatalf("origin dials = %d, want exactly 1", n)
	}
}
