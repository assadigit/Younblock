package proxy

// The M8-x shapes (ADR-0022), pinned by test:
//
//  1. The tiered stall watch — a flow that has PROVEN itself (payload
//     delivered, or one idle gap survived and closed by fresh activity)
//     is closed only at the RICH budget and QUIETLY: no answer-set
//     demotion, no error count, tier=rich on the stall line. The
//     debug.14 build's second report (versionCode 13,
//     2026-10-19T11:43Z) is the evidence: thirteen kills in ~70 s of
//     log, every one a data-carrying pooled connection (2.1 MB, 1.6 MB,
//     723.6 KB, 633.7 KB, 162.6 KB down) murdered at the lean 15 s
//     budget mid-pacing — the user's "going next and back in the same
//     video lags" regression.
//
//  2. The lean tier keeps the exact M8-v semantics: first byte plus
//     headers, never woke, silent to the lean budget — closed LOUD
//     (demotion + error count + tier=lean) — and a negative
//     StallRichTimeout pins the M8-v shape for every flow.
//
// Both-ways proof: with the tiering stashed (the M8-v watch), the two
// escalation tests FAIL — the rich flow dies at the lean budget and
// the pacing flow never survives its second gap — while the lean and
// disabled guards pass on both sides, pinning exactly what changed
// and nothing else.

import (
        "bytes"
        "io"
        "net"
        "strings"
        "testing"
        "time"
)

// richThenSilentOrigin echoes the first flight, then delivers a fixed
// payload, then holds the connection totally silent — the pooled video
// connection's shape (real data delivered, then pacing) after its last
// segment.
func richThenSilentOrigin(t *testing.T, payload int) string {
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
                                // The flight: echo until the 69-byte budget is
                                // spent — the fragmented hello's pieces, however
                                // many reads they take (the M8-af fixture fix:
                                // a single-read echo depended on the scheduler
                                // coalescing the pieces and flaked under -race).
                                echoed := 0
                                for echoed < 5+64 {
                                        n, rerr := c.Read(buf)
                                        if n > 0 {
                                                if _, werr := c.Write(buf[:n]); werr != nil {
                                                        return
                                                }
                                                echoed += n
                                        }
                                        if rerr != nil {
                                                return
                                        }
                                }
                                big := bytes.Repeat([]byte{0x42}, payload)
                                if _, werr := c.Write(big); werr != nil {
                                        return
                                }
                                for { // the pacing hold: no more data until someone kills the flow
                                        if _, rerr := c.Read(buf); rerr != nil {
                                                return
                                        }
                                }
                        }(c)
                }
        }()
        return ln.Addr().String()
}

// pacingEchoOrigin echoes every flight and holds between them — the
// pooled connection's request/response rhythm.
func pacingEchoOrigin(t *testing.T) string {
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

// TestStallRichFlowSurvivesLeanBudget: a flow that delivered real
// payload (≥ StallRichBytes) and then went both-silent must NOT be
// closed at the lean budget — the debug.14 build's murdered-pool
// regression — only at the rich budget, and that close is quiet: no
// demotion, no error count, tier=rich. Pre-fix (the M8-v watch) this
// FAILS: the flow dies at the lean budget, demoted and counted.
func TestStallRichFlowSurvivesLeanBudget(t *testing.T) {
        up := richThenSilentOrigin(t, 4096)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
        // The raced round stays ON here: this is the shipping shape — the
        // tiered watch must hold on the raced path exactly as on the
        // serial one.
        srv, st := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.IdleTimeout = 5 * time.Second
                c.StallTimeout = 400 * time.Millisecond
                c.StallRichTimeout = 1500 * time.Millisecond
                c.StallRichBytes = 2048
                c.Resolver = res
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
        defer conn.Close()
        record := tlsLikeRecord(64)
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        // The first-flight echo plus the 4 KB payload must all arrive —
        // the flow's proof, delivered.
        got := make([]byte, len(record)+4096)
        conn.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("the payload must be delivered: %v", err)
        }
        // Both directions silent now. The lean budget (400 ms) must NOT
        // fire: the flow proved itself. Still up at 900 ms.
        conn.SetReadDeadline(time.Now().Add(900 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("unexpected data from the silent origin")
        } else if !isTimeout(err) {
                t.Fatalf("the rich flow was closed at the LEAN budget — the murdered-pool regression's exact shape (%v)", err)
        }
        // The rich budget (1500 ms from the last activity, ± the 100 ms
        // tick) closes it.
        conn.SetReadDeadline(time.Now().Add(1600 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the rich-budget close")
        } else if isTimeout(err) {
                t.Fatalf("the flow was still up past the rich budget (%v)", err)
        }
        stalls := pollLines(log, "WARN tunnel stall killed ", 1, 2*time.Second)
        if len(stalls) == 0 || !strings.Contains(stalls[0], "tier=rich") {
                t.Fatalf("stall line must carry tier=rich: %+v", stalls)
        }
        // The quiet close: no demotion, no error count.
        if inv := res.invalidatedHosts(); len(inv) != 0 {
                t.Fatalf("a rich-tier close must not demote: %v", inv)
        }
        if errs := st.Diagnostic().ConnErrors; errs != 0 {
                t.Fatalf("a rich-tier close must not count an error: %d", errs)
        }
}

// TestStallWakeEscalatesBudget: a flow whose bytes stay lean but whose
// RHYTHM proves pacing — a gap ≥ the wake gap, closed by a second
// flight — must escalate to the rich budget too. StallRichBytes is
// negative here (bytes never escalate): the wake rule alone is under
// test. Pre-fix this FAILS: the flow dies at the lean budget right
// after its second flight.
func TestStallWakeEscalatesBudget(t *testing.T) {
        up := pacingEchoOrigin(t)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
        srv, st := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1 // the serial shape; the watch is path-independent
                c.IdleTimeout = 6 * time.Second
                c.StallTimeout = 800 * time.Millisecond // wakeGap = 160 ms
                c.StallRichTimeout = 2500 * time.Millisecond
                c.StallRichBytes = -1 // bytes never escalate — only the wake rule can
                c.Resolver = res
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
        defer conn.Close()
        record := tlsLikeRecord(64)
        // Flight 1 — lean bytes (a 64 B echo).
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        got := make([]byte, len(record))
        conn.SetReadDeadline(time.Now().Add(2 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("flight 1 echo: %v", err)
        }
        // A real pacing gap — 500 ms, ≥ the 160 ms wake gap, under the
        // 800 ms lean budget — then flight 2 closes it: the flow has
        // woken once, the pacing proof.
        time.Sleep(500 * time.Millisecond)
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        conn.SetReadDeadline(time.Now().Add(2 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("flight 2 echo: %v", err)
        }
        // Both silent now. The lean budget (800 ms) must NOT fire — the
        // flow proved its pacing. Still up at 500 ms.
        conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("unexpected data from the silent origin")
        } else if !isTimeout(err) {
                t.Fatalf("the pacing flow was closed at the LEAN budget — the murdered-pool regression (%v)", err)
        }
        // The rich budget (2500 ms ± the 200 ms tick) closes it.
        conn.SetReadDeadline(time.Now().Add(2600 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the rich-budget close")
        } else if isTimeout(err) {
                t.Fatalf("the flow was still up past the rich budget (%v)", err)
        }
        stalls := pollLines(log, "WARN tunnel stall killed ", 1, 2*time.Second)
        if len(stalls) == 0 || !strings.Contains(stalls[0], "tier=rich") {
                t.Fatalf("stall line must carry tier=rich: %+v", stalls)
        }
        if inv := res.invalidatedHosts(); len(inv) != 0 {
                t.Fatalf("a rich-tier close must not demote: %v", inv)
        }
        if errs := st.Diagnostic().ConnErrors; errs != 0 {
                t.Fatalf("a rich-tier close must not count an error: %d", errs)
        }
}

// TestStallLeanKillStaysLoud: the dead cohort's shape — first byte
// plus headers, never woke, no payload — keeps the exact M8-v death:
// the lean budget, the demotion, the error count, tier=lean. The
// escalation is ARMED in this test (a rich budget is configured); the
// lean shape must stay lean anyway.
func TestStallLeanKillStaysLoud(t *testing.T) {
        up := stallOriginServer(t)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
        srv, st := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1
                c.IdleTimeout = 5 * time.Second
                c.StallTimeout = 400 * time.Millisecond
                c.StallRichTimeout = 1500 * time.Millisecond // escalation armed — the lean shape stays lean anyway
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
                t.Fatalf("the first flight must arrive: %v", err)
        }
        // Never woke, no payload: the lean budget closes it LOUD.
        conn.SetReadDeadline(time.Now().Add(1200 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the lean close")
        } else if isTimeout(err) {
                t.Fatalf("the flow was still up past the lean budget — the dead cohort must keep dying fast (%v)", err)
        }
        stalls := pollLines(log, "WARN tunnel stall killed ", 1, 2*time.Second)
        if len(stalls) == 0 || !strings.Contains(stalls[0], "tier=lean") {
                t.Fatalf("stall line must carry tier=lean: %+v", stalls)
        }
        // M8-ad (ADR-0038): the lean close counts its error but KEEPS the
        // answer set — no invalidateDNS on any stall kill (the churn
        // spiral's feed, dns=579-846 ms in the debug.20 report).
        deadline := time.Now().Add(2 * time.Second)
        for time.Now().Before(deadline) {
                if errs := st.Diagnostic().ConnErrors; errs == 1 {
                        if inv := res.invalidatedHosts(); len(inv) != 0 {
                                t.Fatalf("the lean close must keep the answer set (M8-ad, ADR-0038): %v", inv)
                        }
                        return
                }
                time.Sleep(20 * time.Millisecond)
        }
        t.Fatalf("the lean close must count exactly one error: errors=%d", st.Diagnostic().ConnErrors)
}

// TestStallRichEscalationDisabledByConfig (guard): a negative
// StallRichTimeout pins the M8-v shape for every flow — even one that
// delivered real payload dies at the LEAN budget, loudly.
func TestStallRichEscalationDisabledByConfig(t *testing.T) {
        up := richThenSilentOrigin(t, 4096)
        upHost, upPort, _ := net.SplitHostPort(up)
        log := &recLogger{}
        res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP(upHost)}}}
        srv, st := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.FirstByteTimeout = 250 * time.Millisecond
                c.FirstByteRaceDials = -1
                c.IdleTimeout = 5 * time.Second
                c.StallTimeout = 400 * time.Millisecond
                c.StallRichTimeout = -1 // no escalation — the exact M8-v semantics
                c.Resolver = res
        })
        conn := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
        defer conn.Close()
        record := tlsLikeRecord(64)
        if _, err := conn.Write(record); err != nil {
                t.Fatal(err)
        }
        got := make([]byte, len(record)+4096)
        conn.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("the payload must be delivered: %v", err)
        }
        // The lean budget closes it despite the payload — M8-v exactly.
        conn.SetReadDeadline(time.Now().Add(1000 * time.Millisecond))
        if _, err := conn.Read(make([]byte, 16)); err == nil {
                t.Fatal("expected the lean close")
        } else if isTimeout(err) {
                t.Fatalf("with escalation disabled the rich-payload flow must still close at the lean budget (%v)", err)
        }
        stalls := pollLines(log, "WARN tunnel stall killed ", 1, 2*time.Second)
        if len(stalls) == 0 || !strings.Contains(stalls[0], "tier=lean") {
                t.Fatalf("stall line must carry tier=lean: %+v", stalls)
        }
        // M8-ad (ADR-0038): the close counts its error but KEEPS the answer
        // set — the stall kill never invalidates (the dial-failure path,
        // M8-o, owns the answer set's refresh).
        deadline := time.Now().Add(2 * time.Second)
        for time.Now().Before(deadline) {
                if errs := st.Diagnostic().ConnErrors; errs == 1 {
                        if inv := res.invalidatedHosts(); len(inv) != 0 {
                                t.Fatalf("the disabled-escalation close must keep the answer set (M8-ad, ADR-0038): %v", inv)
                        }
                        return
                }
                time.Sleep(20 * time.Millisecond)
        }
        t.Fatalf("the disabled-escalation close must count exactly one error: errors=%d", st.Diagnostic().ConnErrors)
}
