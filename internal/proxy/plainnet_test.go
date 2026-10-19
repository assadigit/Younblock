package proxy

// The M8-z work item B shapes (ADR-0024, the plain-relay safety net),
// pinned by test:
//
//   1. A NOT-budgeted TLS flow whose origin accepts and stays silent
//      is closed at (roughly) PlainFirstByteTimeout after the
//      ClientHello went upstream, counted in plainFirstByteTimeouts,
//      with the "tunnel plain first-byte failed class=plain
//      reason=timeout" line — never left hanging to the app's ~60 s
//      patience.
//
//   2. An origin that answers within the budget is untouched: the
//      flow completes normally, no kill line, no counter.
//
//   3. A non-TLS flow (plain HTTP shape) never sees the net — the
//      idle timeout owns its silence.
//
//   4. A negative PlainFirstByteTimeout pins the exact pre-M8-z plain
//      shape: only the idle timeout.
//
// Both-ways proof: with the guard's arming stashed (the pre-M8-z
// shape), test 1 FAILS — the client read stays blocked well past the
// budget (the flow hangs to the idle timeout, the blocker's exact
// on-device shape) and the counter stays 0. Tests 2-4 pass on both
// sides, pinning exactly what changed and nothing else.

import (
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPlainFirstByteTimeoutClosesSilentTLSOrigin (shape 1, the fix's
// core): silent origin, non-listed host, short net budget (200 ms,
// injected through the config knob — the shipped default is 8 s).
func TestPlainFirstByteTimeoutClosesSilentTLSOrigin(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.PlainFirstByteTimeout = 200 * time.Millisecond
		c.IdleTimeout = 5 * time.Second // far past the net: a hang would show
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", strconv.Itoa(port)))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	// The net must close the flow at ~200 ms — 1.5 s is 7.5x the budget
	// and far under the 5 s idle: a blocked read here IS the hang.
	conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 32)); err == nil {
		t.Fatal("the silent plain TLS origin must be closed by the safety net")
	} else if isTimeoutErr(err) {
		t.Fatalf("the read timed out on the CLIENT deadline — the flow hung past the net's budget (%v)", err)
	}
	// The counter and the WARN line land from the reader goroutine in
	// that order (counter first) — poll for BOTH before asserting, so a
	// counter-visible-but-line-pending instant cannot flake the check.
	deadline := time.Now().Add(2 * time.Second)
	var lines []string
	for time.Now().Before(deadline) {
		lines = log.lines("WARN tunnel plain first-byte failed ")
		if st.Diagnostic().PlainFirstByteTo == 1 && len(lines) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := st.Diagnostic().PlainFirstByteTo; n != 1 {
		t.Fatalf("plainFirstByteTimeouts = %d, want 1", n)
	}
	if len(lines) != 1 {
		t.Fatalf("plain kill line = %+v, want exactly one", lines)
	}
	if want := "tunnel plain first-byte failed class=plain reason=timeout"; !strings.Contains(lines[0], want) {
		t.Fatalf("plain kill line = %q, want prefix %q", lines[0], want)
	}
	// The net does NOT penalize the origin's IP (work item B's rule): a
	// second flow to the same corpse still dials it (no dead-IP skip).
	conn2 := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", strconv.Itoa(port)))
	conn2.Write(tlsLikeRecord(64))
	conn2.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	conn2.Read(make([]byte, 32)) // closed by the net again
	conn2.Close()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && corpse.count() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := corpse.count(); n != 2 {
		t.Fatalf("corpse accepts = %d, want 2 (the net never marks IPs)", n)
	}
}

// TestPlainFirstByteAnsweredWithinBudgetUntouched (shape 2, guard):
// an origin that answers at ~100 ms with the net at 400 ms completes
// normally — first byte delivered, no kill, counter 0. Passes on both
// sides.
func TestPlainFirstByteAnsweredWithinBudgetUntouched(t *testing.T) {
	up := delayedEchoOrigin(t, 100*time.Millisecond)
	_, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.PlainFirstByteTimeout = 400 * time.Millisecond
		c.IdleTimeout = 2 * time.Second
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", upPort))
	defer conn.Close()
	record := tlsLikeRecord(64)
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("the answering origin must be untouched by the net: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch through the guarded plain flow")
	}
	if n := st.Diagnostic().PlainFirstByteTo; n != 0 {
		t.Fatalf("plainFirstByteTimeouts = %d, want 0", n)
	}
	if lines := log.lines("WARN tunnel plain first-byte failed "); len(lines) != 0 {
		t.Fatalf("kill lines on an answering origin: %+v", lines)
	}
}

// TestPlainFirstByteNonTLSFlowsKeepIdleSemantics (shape 3, guard): a
// NON-TLS first message (an HTTP-ish payload) to a silent origin does
// NOT trip the net — the flow outlives the net's budget and dies at
// the idle timeout. Passes on both sides.
func TestPlainFirstByteNonTLSFlowsKeepIdleSemantics(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0)
	srv, st := newTestServer(t, func(c *Config) {
		c.PlainFirstByteTimeout = 150 * time.Millisecond
		c.IdleTimeout = 600 * time.Millisecond
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("plain.example.com", strconv.Itoa(port)))
	defer conn.Close()
	// A plain HTTP GET shape: readFirstRecord passes it through as
	// non-TLS (no 0x16 record header).
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	// 3x the net budget: the flow must STILL be open (only the idle
	// timeout may close it, at 600 ms).
	conn.SetReadDeadline(time.Now().Add(450 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("unexpected data from a silent origin")
	} else if !isTimeoutErr(err) {
		t.Fatalf("non-TLS flow closed early (%v) — the net must not touch non-TLS shapes", err)
	}
	if n := st.Diagnostic().PlainFirstByteTo; n != 0 {
		t.Fatalf("plainFirstByteTimeouts = %d, want 0 (non-TLS flows are not the net's business)", n)
	}
}

// TestPlainFirstByteDisabledByConfig (shape 4, guard): a negative knob
// pins the exact pre-M8-z plain shape — the silent TLS origin hangs to
// the idle timeout, no counter, no line. Passes on both sides.
func TestPlainFirstByteDisabledByConfig(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0)
	srv, st := newTestServer(t, func(c *Config) {
		c.PlainFirstByteTimeout = -1
		c.IdleTimeout = 500 * time.Millisecond
	})
	conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("ads.example.com", strconv.Itoa(port)))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("unexpected data")
	} else if !isTimeoutErr(err) {
		t.Fatalf("closed early with the net disabled (%v) — the idle timeout alone must own the close", err)
	}
	if n := st.Diagnostic().PlainFirstByteTo; n != 0 {
		t.Fatalf("plainFirstByteTimeouts = %d, want 0 with the net disabled", n)
	}
}

// delayedEchoOrigin accepts one connection at a time and echoes after
// the delay (the answering-within-budget shape).
func delayedEchoOrigin(t *testing.T, delay time.Duration) string {
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
					time.Sleep(delay)
					c.Write(buf[:n])
				}
				if rerr != nil {
					return
				}
				io.Copy(c, c)
			}(c)
		}
	}()
	return ln.Addr().String()
}
