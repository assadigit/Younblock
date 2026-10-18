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
	"io"
	"net"
	"strings"
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
