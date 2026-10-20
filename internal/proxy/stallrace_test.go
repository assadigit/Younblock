package proxy

// The M8-v shapes (ADR-0020), pinned by test:
//
//   1. The stall kill — a processed host's relay that goes silent in BOTH
//      directions for StallTimeout is closed engine-side (both ends, the
//      stall line, the answer set demoted, the error counted), because the
//      debug.13 report measured the alternative at the app's full ~60 s
//      patience: five video-start flows, first byte plus ~7 KB down, then
//      total silence until the app killed them itself. One-direction
//      silence (a pure download, a pure upload) must NEVER fire; unrouted
//      relays and a negative StallTimeout keep the M8-u behavior exactly.
//
//   2. The salvage race — the remaining salvage re-dials launch in
//      PARALLEL (distinct origins via the rotated live set), the first
//      origin to speak wins, and a silent-host terminal costs ~2x the
//      first-byte budget, never the serial chain's 3x. The serial chain
//      was the debug.13 report's terminal signature: 21 timeout dones at
//      8014-8227 ms = three 2.5 s budgets walked one after another.
//
// Both-ways proof: with the tunnel.go restructure stashed (the M8-t
// serial salvage, no stall watch), the three race/stall tests below
// FAIL — the stalled flow hangs to the idle timeout, the two raced
// re-dials land a full budget apart, and the terminal chain closes at
// 3x the budget — and every guard (streaming, unrouted, disabled,
// winner byte-exactness) passes on both sides, pinning exactly what
// changed and nothing else.

import (
        "io"
        "net"
        "strconv"
        "strings"
        "sync"
        "testing"
        "time"
)

// ---------------------------------------------------------------------------
// M8-v shape 1: the stall kill.
// ---------------------------------------------------------------------------

// stallOriginServer echoes the client's first flight and then goes
// totally silent — the debug.13 report's mid-stream stall shape (first
// byte plus a little data, then nothing until someone kills the flow).
func stallOriginServer(t *testing.T) string {
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
                                }
                                if rerr != nil {
                                        return
                                }
                                for { // the stall: hold the conn open, never answer again
                                        if _, rerr := c.Read(buf); rerr != nil {
                                                return
                                        }
                                }
                        }(c)
                }
        }()
        return ln.Addr().String()
}

// streamOriginServer echoes the first flight and then streams one byte
// per interval — the pure-download relay shape (client silent, the
// origin flowing) the stall watch must never kill.
func streamOriginServer(t *testing.T, interval time.Duration, ticks int) string {
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
                                }
                                if rerr != nil {
                                        return
                                }
                                for i := 0; i < ticks; i++ {
                                        time.Sleep(interval)
                                        if _, werr := c.Write([]byte{byte('a' + i)}); werr != nil {
                                                return
                                        }
                                }
                                for { // after the stream: silent hold (the watch may fire here, by design)
                                        if _, rerr := c.Read(buf); rerr != nil {
                                                return
                                        }
                                }
                        }(c)
                }
        }()
        return ln.Addr().String()
}

// TestStallKillClosesSilentRelayFlow: the origin answers the first
// flight and then the relay goes silent both ways — the stall kill must
// close the flow at the stall budget, with the stall line, the done
// line and the M8-o demotion. Pre-fix (no stall watch) this FAILS: the
// flow hangs to the 5 s idle timeout and the read at 1.5 s times out
// with the flow still up.
func TestStallKillClosesSilentRelayFlow(t *testing.T) {
        up := stallOriginServer(t)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: this file pins the M8-v shapes; the raced round is firstrace_test.go's
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
        if string(got) != string(record) {
                t.Fatal("echo mismatch")
        }
        // The stall: both directions silent. The kill must close the flow
        // well inside the 5 s idle timeout — the budget is 400 ms.
        conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the stalled flow to close")
        } else if isTimeout(err) {
                t.Fatalf("the flow was still up at 1.5 s — the stall kill did not fire (%v)", err)
        }
        stalls := pollLines(log, "WARN tunnel stall killed ", 1, 2*time.Second)
        if len(stalls) == 0 {
                t.Fatalf("no stall line: %+v", log.lines("WARN "))
        }
        if !strings.Contains(stalls[0], "idle=") {
                t.Fatalf("stall line missing idle=: %q", stalls[0])
        }
        dones := pollLines(log, "INFO tunnel done ", 1, 2*time.Second)
        if len(dones) == 0 {
                t.Fatal("no done line after the stall kill")
        }
        // M8-ad (ADR-0038): the stalled origin's answer set is KEPT — the
        // kill is an origin-conn death (the demote/rotation machinery
        // carries the per-IP evidence), and forcing a re-resolve through
        // the DoH lane fed the churn spiral the debug.20 report measured.
        // The answer set's own refresh belongs to the dial-failure path
        // (M8-o), where a whole set failed to connect.
        time.Sleep(200 * time.Millisecond) // any stray invalidation lands well inside this
        if inv := res.invalidatedHosts(); len(inv) != 0 {
                t.Fatalf("a stall kill must never invalidate the answer set (M8-ad, ADR-0038): %v", inv)
        }
}

// TestStallKillSparesStreamingDownload (guard): a pure download — the
// client never writes again, the origin streams a byte every 100 ms —
// must never be stall-killed, on either side of the fix.
func TestStallKillSparesStreamingDownload(t *testing.T) {
        up := streamOriginServer(t, 100*time.Millisecond, 12) // 1.2 s of streaming
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v shapes
                c.IdleTimeout = 5 * time.Second
                c.StallTimeout = 400 * time.Millisecond
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
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
                t.Fatalf("the first flight must arrive: %v", err)
        }
        // The download flows across many stall budgets (each byte refreshes
        // the down stamp; the client never writes again). All 12 bytes must
        // arrive — 400 ms of both-ways silence never happened.
        stream := make([]byte, 12)
        conn.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(conn, stream); err != nil {
                t.Fatalf("a streaming download must never be stall-killed: %v", err)
        }
        if stalls := log.lines("WARN tunnel stall killed "); len(stalls) != 0 {
                t.Fatalf("a one-direction flow was stall-killed: %q", stalls[0])
        }
}

// TestStallKillDisabledByConfig (guard): a negative StallTimeout pins
// the M8-u behavior exactly — the silent relay lives to the idle
// timeout and no stall line is ever logged.
func TestStallKillDisabledByConfig(t *testing.T) {
        up := stallOriginServer(t)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v shapes
                c.IdleTimeout = 800 * time.Millisecond
                c.StallTimeout = -1
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
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
                t.Fatalf("the first flight must arrive: %v", err)
        }
        // Past the would-be stall budget (400 ms) the flow must still be up —
        // the disabled watch never fires; the idle timeout owns the close.
        conn.SetReadDeadline(time.Now().Add(600 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("unexpected data from the silent origin")
        } else if !isTimeout(err) {
                t.Fatalf("the flow closed before the idle timeout with the stall watch disabled (%v)", err)
        }
        conn.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the idle close")
        }
        if stalls := log.lines("WARN tunnel stall killed "); len(stalls) != 0 {
                t.Fatalf("a disabled stall watch must never log: %q", stalls[0])
        }
}

// TestStallKillSparesUnroutedHosts (guard): the stall watch, like the
// budget and the salvage before it, is part of the processing of routed
// hosts only (the D46 scoping) — an unrouted relay keeps the plain idle
// semantics exactly.
func TestStallKillSparesUnroutedHosts(t *testing.T) {
        up := stallOriginServer(t)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v shapes (unrouted guard — the race, like every other processing, never touches it)
                c.IdleTimeout = 2 * time.Second
                c.StallTimeout = 400 * time.Millisecond
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("example.test", upPort))
        defer conn.Close()
        record := tlsLikeRecord(64)
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        got := make([]byte, len(record))
        conn.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("the unrouted first flight must arrive: %v", err)
        }
        // Past the would-be stall budget the flow must still be up.
        conn.SetReadDeadline(time.Now().Add(900 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("unexpected data from the silent origin")
        } else if !isTimeout(err) {
                t.Fatalf("the unrouted flow closed early (%v); the stall watch must never touch a plain relay", err)
        }
        conn.SetReadDeadline(time.Now().Add(4 * time.Second))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the idle close")
        }
        if stalls := log.lines("WARN tunnel stall killed "); len(stalls) != 0 {
                t.Fatalf("an unrouted flow was stall-killed: %q", stalls[0])
        }
}

// ---------------------------------------------------------------------------
// M8-v shape 2: the salvage race.
// ---------------------------------------------------------------------------

// acceptTimes records every accept's time on one listener.
type acceptTimes struct {
        mu    sync.Mutex
        times []time.Time
}

func (a *acceptTimes) snapshot() []time.Time {
        a.mu.Lock()
        defer a.mu.Unlock()
        return append([]time.Time(nil), a.times...)
}

func (a *acceptTimes) count() int {
        a.mu.Lock()
        defer a.mu.Unlock()
        return len(a.times)
}

// silentListenerAt binds exactly ip:port, accepts forever, logs every
// accept time, and holds every conn open and silent (the dead-origin
// shape). port 0 picks a free port; the actual port is returned.
func silentListenerAt(t *testing.T, ip string, port int) (*acceptTimes, int) {
        t.Helper()
        ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(func() { ln.Close() })
        a := &acceptTimes{}
        go func() {
                for {
                        c, err := ln.Accept()
                        if err != nil {
                                return
                        }
                        a.mu.Lock()
                        a.times = append(a.times, time.Now())
                        a.mu.Unlock()
                        go func(c net.Conn) {
                                defer c.Close()
                                buf := make([]byte, 4096)
                                for {
                                        if _, rerr := c.Read(buf); rerr != nil {
                                                return
                                        }
                                }
                        }(c)
                }
        }()
        return a, ln.Addr().(*net.TCPAddr).Port
}

// echoListenerAt binds exactly addr and echoes every connection.
func echoListenerAt(t *testing.T, addr string) {
        t.Helper()
        ln, err := net.Listen("tcp", addr)
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

// TestSalvageRaceRedialsInParallel: three loopback origins, all silent;
// attempt 0 dies on the first and the two salvage re-dials must land on
// the other two AT THE SAME TIME — the serial chain re-dials one budget
// apart, and that is exactly the pre-fix failure this test pins.
func TestSalvageRaceRedialsInParallel(t *testing.T) {
        l1, port := silentListenerAt(t, "127.0.0.1", 0)
        l2, _ := silentListenerAt(t, "127.0.0.2", port)
        l3, _ := silentListenerAt(t, "127.0.0.3", port)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v salvage-race shape (serial attempt 0, then the raced re-dials)
                c.IdleTimeout = 5 * time.Second
                c.Resolver = staticResolver{ips: []net.IP{
                        net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
                }}
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
        if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
                t.Fatal(err)
        }
        conn.SetReadDeadline(time.Now().Add(2 * time.Second))
        if _, err := conn.Read(make([]byte, 64)); err == nil {
                t.Fatal("expected the exhausted flow to close")
        }
        fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
        if len(fbs) == 0 || !strings.Contains(fbs[0], "reason=timeout") {
                t.Fatalf("terminal failure line wrong: %+v", log.lines("WARN "))
        }
        // The full attempt count still ran: 1 + 2 raced dials, one origin each.
        deadline := time.Now().Add(2 * time.Second)
        for time.Now().Before(deadline) && l1.count()+l2.count()+l3.count() < 3 {
                time.Sleep(10 * time.Millisecond)
        }
        if n := l1.count() + l2.count() + l3.count(); n != 3 {
                t.Fatalf("origin accepts = %d, want 3 (attempt 0 + 2 raced)", n)
        }
        // THE race evidence: the two salvage candidates accepted together.
        // The serial chain cannot — its second re-dial starts only after the
        // first re-dial's full budget dies (a full FirstByteTimeout apart).
        t2 := l2.snapshot()
        t3 := l3.snapshot()
        if len(t2) != 1 || len(t3) != 1 {
                t.Fatalf("salvage accepts: ip2=%d ip3=%d, want 1 each", len(t2), len(t3))
        }
        delta := t2[0].Sub(t3[0])
        if delta < 0 {
                delta = -delta
        }
        if delta >= 150*time.Millisecond {
                t.Fatalf("the two salvage re-dials landed %v apart — the serial chain's shape, not the race's", delta)
        }
}

// TestSalvageRaceTerminalIsBounded: every origin silent; the exhausted
// flow must close at ~2x the budget (attempt 0 + the raced dials), not
// the serial chain's 3x. Pre-fix this FAILS: the close arrives a full
// budget later.
func TestSalvageRaceTerminalIsBounded(t *testing.T) {
        up, accepts := flakyOriginServer(t, 1<<30) // every connection silent
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 300 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v salvage-race shape
                c.IdleTimeout = 5 * time.Second
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
        if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
                t.Fatal(err)
        }
        // 800 ms: two budgets plus slack. The serial chain (3 x 300 ms plus
        // dials) cannot land inside it; the race (300 + 300) does.
        conn.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 64)); err == nil {
                t.Fatal("expected the exhausted flow to close")
        } else if isTimeout(err) {
                t.Fatalf("the flow was still up 800 ms in — the serial chain's 3x-budget shape, not the race's ~2x")
        }
        deadline := time.Now().Add(2 * time.Second)
        for time.Now().Before(deadline) && accepts() < 3 {
                time.Sleep(10 * time.Millisecond)
        }
        if n := accepts(); n != 3 {
                t.Fatalf("origin dials = %d, want 3 (1 + 2 raced)", n)
        }
        fbs := pollLines(log, "WARN tunnel first-byte failed ", 1, 2*time.Second)
        if len(fbs) == 0 || !strings.Contains(fbs[0], "reason=timeout") {
                t.Fatalf("terminal line wrong: %+v", log.lines("WARN "))
        }
        dones := pollLines(log, "INFO tunnel done ", 1, 2*time.Second)
        if len(dones) == 0 || !strings.Contains(dones[0], "down=0 B") {
                t.Fatalf("done line must show down=0 B: %+v", dones)
        }
}

// TestSalvageRaceWinnerDeliversByteExact (guard): attempt 0 dies on a
// silent origin; the race's winner (the echo origin) must deliver the
// client's record plus its early data byte-exact, log the save, and
// leave the flow a working relay.
func TestSalvageRaceWinnerDeliversByteExact(t *testing.T) {
        _, port := silentListenerAt(t, "127.0.0.1", 0) // attempt 0's corpse
        echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // M8-w: pin the M8-v salvage-race shape
                c.IdleTimeout = 5 * time.Second
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")}}
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
        conn.SetReadDeadline(time.Now().Add(4 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("the raced salvage must deliver the echo: %v", err)
        }
        if string(got) != string(want) {
                t.Fatal("echo mismatch through the raced winner")
        }
        salvaged := pollLines(log, "INFO tunnel first-byte salvaged ", 1, 2*time.Second)
        if len(salvaged) == 0 || !strings.Contains(salvaged[0], "dials=2") {
                t.Fatalf("salvaged line wrong: %+v", log.lines("INFO tunnel first-byte"))
        }
        if fails := log.lines("WARN tunnel first-byte failed "); len(fails) != 0 {
                t.Fatalf("a raced save must not log a terminal failure: %q", fails[0])
        }
        // The flow is still a working relay after the race.
        again := make([]byte, len(record))
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        conn.SetReadDeadline(time.Now().Add(2 * time.Second))
        if _, err := io.ReadFull(conn, again); err != nil {
                t.Fatalf("the flow must stay up after the raced salvage: %v", err)
        }
        if string(again) != string(record) {
                t.Fatal("second echo mismatch")
        }
}
