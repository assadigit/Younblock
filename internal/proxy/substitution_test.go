package proxy

// The M8-ab shapes (ADR-0031/0032/0033), pinned by test:
//
//   1. The cross-origin substitution — a *.googlevideo.com flow whose
//      answer set is ALL corpses (the debug.19 report's whole
//      remaining cold-start cost: five retry waves gambling rotated
//      unknowns while the good table held verified origins no dial
//      list ever saw) races ONE substitute candidate from the verified
//      pool and the flow is saved in-engine: the echo comes back
//      byte-exact, the substituted line logs, no terminal. Pre-fix
//      (the debug.19 shape) the same flow terminals on the raced
//      round — the salvage finds nothing untried and the app's next
//      retry wave is the only escape left.
//
//   2. The neutered control — GoodIPSubstitution=-1 is the exact
//      debug.19 shape: the pool marks still exist, the flow still
//      gambles its own answer set, the terminal fires. Passes on both
//      sides of the substitution: with the knob on, the same flow is
//      shape 1.
//
//   3. The pool's evidence class (M8-af, ADR-0045, spec W6 — this
//      round's change): a flow that SUCCEEDS marks its origin
//      verified-live but NOT gvideo-flagged — the mark prefers (front
//      of the dial lists) and NEVER substitutes. Only a MEDIA verdict
//      (MarkMediaVerifiedIP, what the W7 network test writes) feeds
//      the substitution pool. Pre-fix (the debug.23 shape) every
//      first-byte win gvideo-flagged and the pool consumed itself on
//      false positives: 18 substitution wins, every flow <=12.1 KB,
//      17 goodDrops, pool gvideo 4->0.
//
//   4. The demote — a flow death on a good-marked origin drops the
//      mark and boxes the IP (the ADR-0028 behavior the debug.19 code
//      documented but never implemented — without the drop a dead IP
//      resurfaces at the FRONT of every dial list once its box entry
//      lapses, and a poisoned substitution pool the moment the pool
//      started mattering).
//
//   5. The churn signal — a googlevideo flow that terminaled (all-dead
//      raced round + dead salvage) bumps gvideoTerminals, the counter
//      the engine's gvideo loop kicks an immediate probe pass on; a
//      non-googlevideo terminal never does.
//
//   6. The substituted line's privacy — counts and class only, no IP
//      literal, no name (R21).
//
// Both-ways proof: with the tunnel.go substitution hunk stashed (the
// debug.19 shape), tests 1 and 3 FAIL — the flows terminal with the
// report's exact "first-byte failed" signature and the pool origins
// are never dialed — while the guards (2, 5, 6) pass on both sides;
// with the demote hunk stashed, test 4 FAILS on the surviving good
// count; with the churn counter stashed, test 5 FAILS at zero.

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/stats"
)

// hostResolver resolves per-host answer sets (the substitution shapes
// need a DIFFERENT answer set per hostname: the flow's own corpses and
// the probe name's verified origin).
type hostResolver struct {
	mu    sync.Mutex
	hosts map[string][]net.IP
}

func (h *hostResolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ips, ok := h.hosts[host]; ok {
		return ips, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// onceEchoListenerAt binds exactly ip:port — the FIRST accepted
// connection is a full echo (the probe's verdict — "answered"), every
// later one held open and silent (the flow's death — the origin went
// black between the verdict and the flow). The echo must be FULL: a
// fragmented ClientHello arrives as several segments of ONE TLS
// record, and a partial echo leaves the handshake waiting for the
// rest (a "silent" verdict, not an "answered" one). port 0 picks a
// free port; the actual port is returned.
func onceEchoListenerAt(t *testing.T, ip string, port int) (*acceptTimes, int) {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	a := &acceptTimes{}
	var first int32 = 1
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			a.mu.Lock()
			a.times = append(a.times, time.Now())
			a.mu.Unlock()
			isEcho := atomic.CompareAndSwapInt32(&first, 1, 0)
			go func(c net.Conn, isEcho bool) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, rerr := c.Read(buf)
					if n > 0 && isEcho {
						if _, werr := c.Write(buf[:n]); werr != nil {
							return
						}
					}
					if rerr != nil {
						return
					}
				}
			}(c, isEcho)
		}
	}()
	return a, ln.Addr().(*net.TCPAddr).Port
}

// substFixture builds the escape shape: three corpses as the flow
// host's whole answer set, one echo origin verified good through a
// probe pass on a DIFFERENT googlevideo name (the foreign cache the
// debug.19 session never dialed for its failing host).
func substFixture(t *testing.T) (resolver *hostResolver, corpse1 *acceptTimes, port int) {
	t.Helper()
	c1, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	echoListenerAt(t, "127.0.0.4:"+strconv.Itoa(port))
	resolver = &hostResolver{hosts: map[string][]net.IP{
		"rr3---sn-aaa.googlevideo.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
		"rr1---sn-bbb.googlevideo.com": {net.ParseIP("127.0.0.4")},
	}}
	return resolver, c1, port
}

// primePool arms the substitution pool the W7 way (M8-af, ADR-0045,
// spec W6): the probe pass first verifies the echo origin answers the
// flow-shaped hello (a HANDSHAKE verdict — preference, never the
// pool), then the media verdict (what the network test's answered
// class writes) gvideo-flags it. After this, the substitute candidate
// may draw the origin.
func primePool(t *testing.T, srv *Server, port int) {
	t.Helper()
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"rr1---sn-bbb.googlevideo.com"}, port, 4)
	if sum.IPs != 1 || sum.Answered != 1 {
		t.Fatalf("probe summary = %+v, want ips=1 answered=1 (the echo origin)", sum)
	}
	// The handshake verdict preferred but never flagged (W6)...
	if _, gvideo := srv.GoodIPStats(); gvideo != 0 {
		t.Fatalf("gvideo good pool = %d after a handshake-only verdict, want 0 (W6: media evidence only)", gvideo)
	}
	// ...the MEDIA verdict is what feeds the pool.
	srv.MarkMediaVerifiedIP("127.0.0.4")
	if _, gvideo := srv.GoodIPStats(); gvideo != 1 {
		t.Fatalf("gvideo good pool = %d after the media verdict, want 1", gvideo)
	}
}

// TestSubstitutionEscapesTheDeadAnswerSet (M8-ab shape 1): the raced
// round's substitute candidate delivers the first byte from the
// verified pool and the flow is saved in-engine — the exact 45 s
// escape of the debug.19 report collapsed into one round.
func TestSubstitutionEscapesTheDeadAnswerSet(t *testing.T) {
	resolver, corpse1, port := substFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	primePool(t, srv, port)

	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	rec := tlsLikeRecord(64)
	if _, err := c.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo (the substitute must save the flow): %v", err)
	}
	_ = c.Close()
	if string(got) != string(rec) {
		t.Fatalf("echo mismatch: %d bytes", len(got))
	}
	if failed := log.lines("WARN tunnel first-byte failed "); len(failed) != 0 {
		t.Fatalf("terminal line on a substituted flow: %q", failed[0])
	}
	if saved := pollLines(log, "INFO tunnel first-byte substituted ", 1, 2*time.Second); len(saved) == 0 {
		t.Fatalf("the substituted line must log the escape")
	}
	if d := st.Diagnostic(); d.SubstAttempts < 1 || d.SubstWins != 1 {
		t.Fatalf("substitution counters = attempts %d wins %d, want >=1/1", d.SubstAttempts, d.SubstWins)
	}
	// The raced round covered the corpses (the primary and the one
	// remaining own-set rotation — the substitute replaced the other).
	if n := corpse1.count(); n != 1 {
		t.Fatalf("corpse #1 accepted %d connections, want 1 (the primary alone)", n)
	}
}

// TestSubstitutionNeuteredKeepsTheOldTerminal (M8-ab shape 2, the
// neutered control): GoodIPSubstitution=-1 is the exact debug.19
// shape — the pool exists, the flow never draws from it, the terminal
// carries the report's signature. Passes on both sides: with the knob
// on, the same flow is shape 1.
func TestSubstitutionNeuteredKeepsTheOldTerminal(t *testing.T) {
	resolver, corpse1, port := substFixture(t)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
		c.GoodIPSubstitution = -1 // the substitution off: the debug.19 shape exactly
	})
	primePool(t, srv, port)

	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second)
	if len(failed) == 0 {
		t.Fatalf("the knob-off flow must terminal on its own dead answer set")
	}
	if saved := log.lines("INFO tunnel first-byte substituted "); len(saved) != 0 {
		t.Fatalf("substituted line with the knob off: %q", saved[0])
	}
	// The rotations' DialHost staggers cancel at each candidate's
	// first success, so every candidate dials exactly its list head:
	// the primary's corpse takes one connection, and no pool origin is
	// ever dialed.
	if n := corpse1.count(); n != 1 {
		t.Fatalf("corpse #1 accepted %d connections, want 1 (the primary alone)", n)
	}
}

// TestFlowSuccessFeedsTheSubstitutionPool (M8-ab shape 3, REWIRED by
// M8-af/ADR-0045/W6): a flow that succeeds marks its origin
// verified-live — but the gvideo pool stays EMPTY (first-byte evidence
// is handshake evidence; the debug.23 report measured that pool
// consuming itself on exactly these false positives), so a later flow
// to an all-corpse answer set TERMINALS — and only an explicit MEDIA
// verdict (the W7 network test's answered class) arms the substitute
// that saves the flow after it.
func TestFlowSuccessFeedsTheSubstitutionPool(t *testing.T) {
	c1, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	echoListenerAt(t, "127.0.0.5:"+strconv.Itoa(port))
	_ = c1 // flow 2's primary still dials corpse #1; the count is shape 1's business
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-ccc.googlevideo.com": {net.ParseIP("127.0.0.5")},
		"rr9---sn-ddd.googlevideo.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})

	// Flow 1: the foreign cache answers on its own hostname.
	c1c := dialConnect(t, srv.Addr(), "rr1---sn-ccc.googlevideo.com", port)
	rec := tlsLikeRecord(64)
	if _, err := c1c.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c1c.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c1c, got); err != nil {
		t.Fatalf("read echo (flow 1 must succeed on its own origin): %v", err)
	}
	_ = c1c.Close()
	// W6 (M8-af): the successful flow's origin is preferred (good
	// total 1) but the gvideo pool is EMPTY — first-byte evidence
	// never substitutes again.
	if total, gvideo := srv.GoodIPStats(); total != 1 || gvideo != 0 {
		t.Fatalf("good pool = %d (gvideo %d) after a successful flow, want 1/0 (W6: preference, never the pool)", total, gvideo)
	}

	// Flow 2: a different googlevideo host, all-corpse answers — NO
	// substitute engages (the pool is empty), the flow terminals.
	c2 := dialConnect(t, srv.Addr(), "rr9---sn-ddd.googlevideo.com", port)
	_, _ = c2.Write(rec)
	_ = c2.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c2.Read(make([]byte, 128))
	_ = c2.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second); len(failed) == 0 {
		t.Fatalf("the empty pool must leave the all-corpse flow to its terminal (W6)")
	}
	if saved := log.lines("INFO tunnel first-byte substituted "); len(saved) != 0 {
		t.Fatalf("a substituted line with an empty pool: %q", saved[0])
	}
	if d := st.Diagnostic(); d.SubstAttempts != 0 {
		t.Fatalf("substAttempts = %d with an empty pool, want 0", d.SubstAttempts)
	}

	// Flow 3: the MEDIA verdict arms the pool (the W7 evidence
	// class), and the same all-corpse host is saved by the
	// substitute — the escape works, on real evidence now.
	srv.MarkMediaVerifiedIP("127.0.0.5")
	c3 := dialConnect(t, srv.Addr(), "rr9---sn-ddd.googlevideo.com", port)
	if _, err := c3.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c3.SetReadDeadline(time.Now().Add(4 * time.Second))
	got3 := make([]byte, len(rec))
	if _, err := io.ReadFull(c3, got3); err != nil {
		t.Fatalf("read echo (the media-armed pool must save the flow): %v", err)
	}
	_ = c3.Close()
	if string(got3) != string(rec) {
		t.Fatalf("echo mismatch: %d bytes", len(got3))
	}
	if saved := pollLines(log, "INFO tunnel first-byte substituted ", 1, 2*time.Second); len(saved) == 0 {
		t.Fatalf("the media-armed substitution must log its escape")
	}
	if d := st.Diagnostic(); d.SubstWins != 1 {
		t.Fatalf("substWins = %d, want 1", d.SubstWins)
	}
}

// TestFlowDeathDropsTheStaleGoodMark (M8-ab shape 4): an origin the
// probe verified good goes black; the next flow's death drops the good
// mark AND boxes the IP — the demote the ADR-0028 comment promised and
// the debug.19 code never ran.
func TestFlowDeathDropsTheStaleGoodMark(t *testing.T) {
	// The once-echo: the probe's dial is answered (the good mark), the
	// flow's later dials are held silent (the death that must demote).
	_, port := onceEchoListenerAt(t, "127.0.0.6", 0)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-eee.googlevideo.com": {net.ParseIP("127.0.0.6")},
		"rr2---sn-fff.googlevideo.com": {net.ParseIP("127.0.0.6")},
	}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	pctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if sum := srv.ProbeOrigins(pctx, []string{"rr1---sn-eee.googlevideo.com"}, port, 4); sum.Answered != 1 {
		t.Fatalf("probe summary = %+v, want the once-echo answered", sum)
	}
	if got := srv.GoodIPCount(); got != 1 {
		t.Fatalf("good count after the probe = %d, want 1", got)
	}

	// The flow to the second name dials the now-black origin and dies.
	c := dialConnect(t, srv.Addr(), "rr2---sn-fff.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	if got := srv.GoodIPCount(); got != 0 {
		t.Fatalf("good count after the flow death = %d, want 0 (the demote)", got)
	}
	if live, _ := srv.DeadBoxStats(); live < 1 {
		t.Fatalf("the dead origin must be boxed alongside the demote")
	}
}

// TestGvideoTerminalCounted (M8-ab shape 5): an all-dead googlevideo
// flow bumps the churn counter; an all-dead non-googlevideo flow never
// does.
func TestGvideoTerminalCounted(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr3---sn-ggg.googlevideo.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
		"www.youtube.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})

	// The googlevideo terminal: the churn signal.
	c := dialConnect(t, srv.Addr(), "rr3---sn-ggg.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second); len(failed) == 0 {
		t.Fatalf("the all-dead set must terminal")
	}
	if d := st.Diagnostic(); d.GvideoTerminals != 1 {
		t.Fatalf("gvideoTerminals = %d, want 1", d.GvideoTerminals)
	}

	// The non-googlevideo terminal: never the churn signal.
	c2 := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	_, _ = c2.Write(tlsLikeRecord(64))
	_ = c2.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c2.Read(make([]byte, 128))
	_ = c2.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 2, 3*time.Second); len(failed) < 2 {
		t.Fatalf("the second all-dead set must terminal too")
	}
	if d := st.Diagnostic(); d.GvideoTerminals != 1 {
		t.Fatalf("gvideoTerminals = %d after a non-gvideo terminal, want 1 (unchanged)", d.GvideoTerminals)
	}
}

// TestSubstitutedLinePrivacy (M8-ab shape 6): the substituted line
// carries class and counts only — no IP literal, no name (R21).
func TestSubstitutedLinePrivacy(t *testing.T) {
	resolver, _, port := substFixture(t)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	primePool(t, srv, port)

	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = io.ReadFull(c, make([]byte, 64))
	_ = c.Close()
	saved := pollLines(log, "INFO tunnel first-byte substituted ", 1, 2*time.Second)
	if len(saved) == 0 {
		t.Fatalf("the substituted line must log")
	}
	line := saved[0]
	if !strings.Contains(line, "class=routed") || !strings.Contains(line, "dials=1") {
		t.Fatalf("substituted line missing its shape: %q", line)
	}
	if strings.Contains(line, "127.0.0.4") || strings.Contains(line, "googlevideo.com") {
		t.Fatalf("substituted line leaks an IP or a name (R21): %q", line)
	}
}

// Compile-time guard that the unused-import set stays honest.
var _ = config.MaxHeaderBytes
var _ = stats.New
