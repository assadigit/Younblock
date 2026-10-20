package proxy

// The M8-ae shapes (ADR-0040/0041/0042), pinned by test:
//
//  1. The good-front skip — a flow whose primary dial landed on a
//     verified-live origin sends exactly ONE hello (the reference
//     implementations' steady-state shape): no raced extras, no
//     substitute candidate. Pre-fix (the debug.22 shape) the same
//     healthy flow fanned out 3-4 parallel hellos on exactly the
//     traffic a per-source SNI flag accumulates on.
//
//  2. Once per round — an all-dead raced round demotes the answer set
//     exactly ONCE through the DoH lane (plus the M8-s terminal's own
//     invalidation), not once per dead candidate: the 4x resolver
//     churn the debug.20/21 decay measured as dns=579-846 ms.
//
//  3. The emission breaker — consecutive googlevideo terminals arm a
//     cooldown; during it a googlevideo flow fails fast at the door
//     with ZERO wire (the corpses' accept counts stand perfectly
//     still), a NON-gvideo host sails through untouched (the feed
//     keeps refreshing through the cooldown — the whole point), and
//     once the cooldown lapses the next googlevideo flow dials again.
//
//  4. The probe clamp — ProbeOrigins never exceeds the trickle budget
//     (config.OriginProbePerPass) no matter how large a budget the
//     caller passes; every probe hello rides the same gvideoHellos
//     meter the flows do.
//
// Both-ways proof: with the good-front hunk stashed, test 1 FAILS on
// the echo origin's accept count (3, not 1); with the once-per-round
// hunk stashed, test 2 FAILS on the invalidate count (4, not 2); with
// the door hunk stashed, test 3 FAILS on the corpse count (the wire
// spoke during the cooldown); with the clamp stashed, test 4 FAILS on
// 16 probes, not 10.

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingEchoListenerAt binds addr as a FULL echo (every connection,
// byte-exact) and counts accepts — the good-front shape needs the
// echo on every conn (the raced losers speak too) plus the count.
func countingEchoListenerAt(t *testing.T, addr string) *acceptTimes {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
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
	return a
}

// invalidateCounter is the hostResolver plus a counted Invalidate —
// the once-per-round shape's witness.
type invalidateCounter struct {
	mu     sync.Mutex
	hosts  map[string][]net.IP
	calls  atomic.Int32
	latest atomic.Value // string
}

func (h *invalidateCounter) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ips, ok := h.hosts[host]; ok {
		return ips, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func (h *invalidateCounter) Invalidate(host string) {
	h.calls.Add(1)
	h.latest.Store(host)
}

func (h *invalidateCounter) count() int { return int(h.calls.Load()) }

// TestGoodFrontSendsOneHello (M8-ae shape 1): the primary landed on a
// verified-live origin — the raced round never launches, ONE hello
// carries the flow, and the meter counts exactly the probes plus that
// one flow hello.
func TestGoodFrontSendsOneHello(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echo := countingEchoListenerAt(t, "127.0.0.5:"+strconv.Itoa(port))
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-eee.googlevideo.com": {net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.5")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 400 * time.Millisecond
	})
	// The probe pass verifies the echo origin good (and boxes the
	// corpse silent) — the front the primary then lands on.
	pctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"rr1---sn-eee.googlevideo.com"}, port, 4)
	if sum.Answered != 1 {
		t.Fatalf("probe summary = %+v, want answered=1 (the echo origin)", sum)
	}
	corpseAfterPrime := corpse.count()
	echoAfterPrime := echo.count()

	c := dialConnect(t, srv.Addr(), "rr1---sn-eee.googlevideo.com", port)
	rec := tlsLikeRecord(64)
	if _, err := c.Write(rec); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	_ = c.Close()
	if string(got) != string(rec) {
		t.Fatalf("echo mismatch: %d bytes", len(got))
	}
	// THE shape: exactly one connection to the verified origin, zero
	// extra dials to the corpse (the raced round never launched).
	if n := echo.count() - echoAfterPrime; n != 1 {
		t.Fatalf("echo origin accepted %d flow connections, want 1 (one hello, no fan-out)", n)
	}
	if n := corpse.count() - corpseAfterPrime; n != 0 {
		t.Fatalf("corpse accepted %d flow connections, want 0 (the race never launched)", n)
	}
	// The meter: two probe hellos (corpse + echo) plus the flow's one.
	if d := st.Diagnostic(); d.GvideoHellos != 3 {
		t.Fatalf("gvideoHellos = %d, want 3 (2 probes + the flow's single hello)", d.GvideoHellos)
	}
	if failed := log.lines("WARN tunnel first-byte failed "); len(failed) != 0 {
		t.Fatalf("terminal line on a saved flow: %q", failed[0])
	}
}

// TestRacedDeathInvalidatesOncePerRound (M8-ae shape 2): an all-dead
// round demotes the answer set once (the round's own invalidation)
// plus the terminal's M8-s invalidation — never once per dead
// candidate.
func TestRacedDeathInvalidatesOncePerRound(t *testing.T) {
	_, port := silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	resolver := &invalidateCounter{hosts: map[string][]net.IP{
		"rr5---sn-hhh.googlevideo.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
	}}
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 150 * time.Millisecond
		c.GoodIPSubstitution = -1 // no pool: the round is the own-set alone
	})
	c := dialConnect(t, srv.Addr(), "rr5---sn-hhh.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second); len(failed) == 0 {
		t.Fatalf("the all-corpse flow must terminal")
	}
	// The round (3 dead candidates -> ONE invalidation) plus the
	// M8-s terminal site: exactly 2, never the per-candidate 4.
	if n := resolver.count(); n != 2 {
		t.Fatalf("invalidateDNS called %d times, want 2 (once per round + the terminal)", n)
	}
}

// TestEmissionBreakerQuietsTheWire (M8-ae shape 3): two terminals arm
// the breaker; the cooldown's googlevideo flow gets its 502 with ZERO
// wire while a NON-gvideo host sails through the same window; the
// cooldown's lapse restores the dial.
func TestEmissionBreakerQuietsTheWire(t *testing.T) {
	corpse1, port := silentListenerAt(t, "127.0.0.1", 0)
	corpse2, _ := silentListenerAt(t, "127.0.0.2", port)
	echo := countingEchoListenerAt(t, "127.0.0.5:"+strconv.Itoa(port))
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr2---sn-fff.googlevideo.com": {net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")},
		"www.example.com":              {net.ParseIP("127.0.0.5")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 150 * time.Millisecond
		c.FirstByteRaceDials = -1    // serial: the terminal's simplest witness
		c.FirstByteSalvageDials = -1 // attempt 0 only: fail fast, terminal now
		c.GvideoBreakerTerminals = 2
		c.GvideoBreakerWindow = 5 * time.Second
		c.GvideoBreakerCooldown = 400 * time.Millisecond
	})
	// Two all-dead googlevideo flows: two terminals, one arm.
	for i := 0; i < 2; i++ {
		c := dialConnect(t, srv.Addr(), "rr2---sn-fff.googlevideo.com", port)
		_, _ = c.Write(tlsLikeRecord(64))
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = c.Read(make([]byte, 128))
		_ = c.Close()
	}
	if armed := pollLines(log, "WARN gvideo emission breaker armed", 1, 3*time.Second); len(armed) == 0 {
		t.Fatalf("the second terminal must ARM the breaker")
	}
	if d := st.Diagnostic(); d.GvideoCooldowns != 1 || d.GvideoTerminals != 2 {
		t.Fatalf("cooldowns=%d terminals=%d, want 1/2", d.GvideoCooldowns, d.GvideoTerminals)
	}
	wire := corpse1.count() + corpse2.count()

	// The cooldown's googlevideo flow: 502 at the door, zero wire.
	c, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial door: %v", err)
	}
	_, _ = c.Write([]byte(fmt.Sprintf("CONNECT rr2---sn-fff.googlevideo.com:%d HTTP/1.1\r\nHost: rr2---sn-fff.googlevideo.com:%d\r\n\r\n", port, port)))
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, rerr := c.Read(buf)
	if rerr != nil || !strings.Contains(string(buf[:n]), "502") {
		t.Fatalf("door reply = %q err=%v, want the cooldown's 502", string(buf[:n]), rerr)
	}
	_ = c.Close()
	if door := pollLines(log, "WARN tunnel gvideo cooldown", 1, 2*time.Second); len(door) == 0 {
		t.Fatalf("the door's cooldown line must log")
	}
	if still := corpse1.count() + corpse2.count(); still != wire {
		t.Fatalf("the wire spoke during the cooldown: %d -> %d corpse connections", wire, still)
	}

	// The same window, a NON-gvideo host: full service (the feed keeps
	// refreshing — the breaker only ever silences googlevideo).
	fc := dialConnect(t, srv.Addr(), "www.example.com", port)
	rec := tlsLikeRecord(64)
	if _, err := fc.Write(rec); err != nil {
		t.Fatalf("write non-gvideo record: %v", err)
	}
	_ = fc.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(fc, got); err != nil {
		t.Fatalf("the non-gvideo flow must sail through the cooldown: %v", err)
	}
	_ = fc.Close()
	if string(got) != string(rec) {
		t.Fatalf("non-gvideo echo mismatch")
	}
	if n := echo.count(); n < 1 {
		t.Fatalf("the non-gvideo flow never reached the echo origin (count=%d)", n)
	}

	// The cooldown lapses (400 ms + the proportional jitter's margin):
	// the next googlevideo flow dials again.
	time.Sleep(700 * time.Millisecond)
	gc := dialConnect(t, srv.Addr(), "rr2---sn-fff.googlevideo.com", port)
	_, _ = gc.Write(tlsLikeRecord(64))
	_ = gc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = gc.Read(make([]byte, 128))
	_ = gc.Close()
	if resumed := corpse1.count() + corpse2.count(); resumed <= wire {
		t.Fatalf("the lapsed cooldown must let the next flow dial: %d -> %d", wire, resumed)
	}
}

// TestProbePassIsClampedToTheTrickle (M8-ae shape 4): sixteen
// reset-verdict origins, a caller asking for 999 verifications — the
// pass probes exactly config.OriginProbePerPass of them, and every
// probe hello is counted in the gvideoHellos meter.
func TestProbePassIsClampedToTheTrickle(t *testing.T) {
	port := resetListenerAt(t, "127.0.0.1", 0)
	var ips []net.IP
	for i := 1; i <= 16; i++ {
		if i > 1 {
			resetListenerAt(t, fmt.Sprintf("127.0.0.%d", i), port)
		}
		ips = append(ips, net.ParseIP(fmt.Sprintf("127.0.0.%d", i)))
	}
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr4---sn-ggg.googlevideo.com": ips,
	}}
	srv, st := newTestServer(t, func(c *Config) {
		c.Resolver = resolver
	})
	pctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sum := srv.ProbeOrigins(pctx, []string{"rr4---sn-ggg.googlevideo.com"}, port, 999)
	if sum.IPs > 10 {
		t.Fatalf("probe pass verified %d IPs, want at most the trickle budget (10)", sum.IPs)
	}
	if d := st.Diagnostic(); d.GvideoHellos > 10 {
		t.Fatalf("gvideoHellos = %d from one pass, want at most 10 (the clamp bounds the meter too)", d.GvideoHellos)
	}
}
