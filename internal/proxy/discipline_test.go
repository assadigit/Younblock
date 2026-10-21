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
//  3. The emission breaker — consecutive googlevideo terminals from
//     TWO DISTINCT sn-clusters (M8-ah, ADR-0047's arm gate: a
//     single-cluster burst is the blackhole signature) arm a
//     cooldown; its FIRST googlevideo flow is the scout (M8-ag,
//     ADR-0046: the app's own request probes the wire once), the
//     SECOND fails fast at the door with ZERO wire (the corpses'
//     accept counts stand perfectly still), a NON-gvideo host sails
//     through untouched (the feed keeps refreshing through the
//     cooldown — the whole point), and once the cooldown lapses the
//     next googlevideo flow dials again. A media-verified pool
//     serving the flow would pass too (the pool escape — pinned from
//     the substitution side by TestExperimentSwitches' primePool).
//
//  5. The dead-cluster memory (M8-ah, ADR-0047) — three distinct
//     hosts of one sn-cluster silent-terminal inside the window mark
//     the CLUSTER dead; the next host of that cluster gets an
//     INSTANT 502 at the door with zero wire (no 3 s first-byte
//     burn), the media-verified pool overrides the mark, and an
//     answered own-dial rehabilitates the cluster while the big
//     echo's payload media-verifies its origin (the pool's automatic
//     food).
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
// 16 probes, not 10. M8-ag both-ways: with the scout claim stashed,
// test 3 FAILS on the corpse count staying flat (the scout's dial
// never happened); with the lift hook stashed, the M8-af shape-2
// test FAILS on the missing lift line.

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
		"rr2---sn-fff-1.googlevideo.com": {net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")},
		"rr9---sn-ggg-1.googlevideo.com": {net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")},
		"www.example.com":                {net.ParseIP("127.0.0.5")},
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
		c.GvideoBreakerCooldown = 2 * time.Second // long enough that no pass below can be a lapse
	})
	// Two all-dead googlevideo flows, one per cluster (M8-ah, ADR-0047:
	// the arm gate needs terminals from TWO distinct sn-clusters — a
	// single-cluster death burst is the blackhole signature cluster
	// memory owns, not the per-source flag the breaker exists for):
	// two terminals, two clusters, one arm.
	for _, h := range []string{"rr2---sn-fff-1", "rr9---sn-ggg-1"} {
		c := dialConnect(t, srv.Addr(), h+".googlevideo.com", port)
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

	// M8-ag (ADR-0046): the cooldown's FIRST googlevideo flow is
	// the SCOUT — the app's own request probes the flag state the
	// arm guessed. Against corpses it terminal exactly like any
	// flow (the dial lands, the first byte never comes), feeding
	// the ring a third terminal — never a re-arm: in-cooldown
	// feeds extend nothing.
	sc := dialConnect(t, srv.Addr(), "rr2---sn-fff-1.googlevideo.com", port)
	_, _ = sc.Write(tlsLikeRecord(64))
	_ = sc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = sc.Read(make([]byte, 128))
	_ = sc.Close()
	if d := st.Diagnostic(); d.GvideoScouts != 1 {
		t.Fatalf("gvideoScouts = %d, want 1 (the first cooldown flow is the scout)", d.GvideoScouts)
	}
	if probed := corpse1.count() + corpse2.count(); probed != wire+1 {
		t.Fatalf("the scout must dial once: %d -> %d corpse connections", wire, probed)
	}
	if d := st.Diagnostic(); d.GvideoCooldowns != 1 {
		t.Fatalf("gvideoCooldowns = %d after the scout's terminal, want 1 (in-cooldown feeds never re-arm)", d.GvideoCooldowns)
	}

	// The SECOND flow inside the cooldown: 502 at the door, zero
	// wire (the scout is spent, the pool is empty — the corpses
	// never media-verified anything).
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
	if still := corpse1.count() + corpse2.count(); still != wire+1 {
		t.Fatalf("the wire spoke during the cooldown: %d -> %d corpse connections", wire+1, still)
	}

	// M8-ag (ADR-0046) pool escape: a media-verified address — the
	// W7 evidence class, one the corpses never provided — lets the
	// NEXT googlevideo flow through the SAME active cooldown:
	// verified wire outranks the arm (the debug.24 report's breaker
	// starved a healthy pool behind its own door).
	srv.MarkMediaVerifiedIP("127.0.0.9")
	pc := dialConnect(t, srv.Addr(), "rr2---sn-fff-1.googlevideo.com", port)
	_, _ = pc.Write(tlsLikeRecord(64))
	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = pc.Read(make([]byte, 128))
	_ = pc.Close()
	if moved := corpse1.count() + corpse2.count(); moved != wire+2 {
		t.Fatalf("the pool-served flow must pass the door and dial: %d -> %d corpse connections", wire+1, moved)
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

	// The cooldown lapses (2 s + margin): the door serves googlevideo
	// again (the pool may carry it too — the lapse is what the count
	// proves either way: the arm is over).
	time.Sleep(2300 * time.Millisecond)
	gc := dialConnect(t, srv.Addr(), "rr2---sn-fff-1.googlevideo.com", port)
	_, _ = gc.Write(tlsLikeRecord(64))
	_ = gc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = gc.Read(make([]byte, 128))
	_ = gc.Close()
	if resumed := corpse1.count() + corpse2.count(); resumed <= wire+1 {
		t.Fatalf("the lapsed cooldown must let the next flow dial: %d -> %d", wire+1, resumed)
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

// TestClusterMemoryFastFailsTheDeadCluster (M8-ah, ADR-0047): three
// DISTINCT hosts of one sn-cluster silent-terminal inside the window →
// the CLUSTER is marked dead; the next host of that cluster gets the
// door's INSTANT 502 with zero wire (the debug.25 report's cold start
// was ~21 three-second burns on the dead sn-qxau5 set interleaved with
// 7 breaker cooldowns — the fast-fail replaces every burn with ~0 s);
// the media-verified pool overrides the mark; and an answered own-dial
// rehabilitates the cluster while the echo's delivered payload
// media-verifies its origin (the pool's automatic food — the debug.25
// session starved an empty pool beside scout-carried 1.8 MB flows).
func TestClusterMemoryFastFailsTheDeadCluster(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echo := countingEchoListenerAt(t, "127.0.0.5:"+strconv.Itoa(port))
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-zzz-1.googlevideo.com": {net.ParseIP("127.0.0.1")},
		"rr2---sn-zzz-2.googlevideo.com": {net.ParseIP("127.0.0.1")},
		"rr3---sn-zzz-3.googlevideo.com": {net.ParseIP("127.0.0.1")},
		"rr4---sn-zzz-4.googlevideo.com": {net.ParseIP("127.0.0.1")},
		"rr5---sn-zzz-5.googlevideo.com": {net.ParseIP("127.0.0.5")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 150 * time.Millisecond
		c.FirstByteRaceDials = -1    // serial: the silent terminal's simplest witness
		c.FirstByteSalvageDials = -1 // attempt 0 only
		// The breaker never arms here (cluster memory is the subject);
		// the ring still collects, and the M8-ah dead-cluster feed
		// exclusion keeps it from ever seeing these deaths.
		c.GvideoBreakerTerminals = 100
		c.GvideoClusterDeadHosts = 3
		c.GvideoClusterDeadWindow = time.Minute
		c.GvideoClusterDeadTTL = time.Minute
		c.GvideoClusterReprobeEvery = time.Hour // the probe never fires inside the test
		c.PayloadVerifiedMinimum = 4096         // the echo's 8 KB flight certifies
	})
	// Three distinct hosts of sn-zzz, each silent-terminal.
	for _, h := range []string{"rr1---sn-zzz-1", "rr2---sn-zzz-2", "rr3---sn-zzz-3"} {
		c := dialConnect(t, srv.Addr(), h+".googlevideo.com", port)
		_, _ = c.Write(tlsLikeRecord(64))
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = c.Read(make([]byte, 128))
		_ = c.Close()
	}
	if marked := pollLines(log, "WARN gvideo cluster marked dead", 1, 3*time.Second); len(marked) == 0 {
		t.Fatalf("the third distinct dead host must mark the CLUSTER dead")
	}
	if d := st.Diagnostic(); d.GvideoClusterDeads != 1 {
		t.Fatalf("gvideoClusterDeads = %d, want 1", d.GvideoClusterDeads)
	}
	wire := corpse.count()
	if wire != 3 {
		t.Fatalf("the three discovery flows must have dialed: corpse connections = %d, want 3", wire)
	}

	// The fourth host of the SAME cluster: instant 502 at the door,
	// zero wire — no dial, no 3 s burn.
	start := time.Now()
	fc, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial door: %v", err)
	}
	_, _ = fc.Write([]byte(fmt.Sprintf("CONNECT rr4---sn-zzz-4.googlevideo.com:%d HTTP/1.1\r\nHost: rr4---sn-zzz-4.googlevideo.com:%d\r\n\r\n", port, port)))
	_ = fc.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, rerr := fc.Read(buf)
	_ = fc.Close()
	if rerr != nil || !strings.Contains(string(buf[:n]), "502") {
		t.Fatalf("door reply = %q err=%v, want the dead cluster's fast-fail 502", string(buf[:n]), rerr)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("the fast-fail must be instant, took %v", elapsed)
	}
	if still := corpse.count(); still != wire {
		t.Fatalf("the fast-fail must not dial: %d -> %d corpse connections", wire, still)
	}
	if ff := pollLines(log, "WARN gvideo cluster fast-fail", 1, 2*time.Second); len(ff) == 0 {
		t.Fatalf("the fast-fail line must log")
	}
	if d := st.Diagnostic(); d.GvideoClusterFastFail != 1 {
		t.Fatalf("gvideoClusterFastFail = %d, want 1", d.GvideoClusterFastFail)
	}

	// The media-verified pool overrides the mark: the next flow of the
	// dead cluster PASSES the door and rides the substitute lane's
	// pass (its own corpse dial still burns its 150 ms — the door
	// decision is what is pinned here).
	srv.MarkMediaVerifiedIP("127.0.0.9")
	pc := dialConnect(t, srv.Addr(), "rr4---sn-zzz-4.googlevideo.com", port)
	_, _ = pc.Write(tlsLikeRecord(64))
	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = pc.Read(make([]byte, 128))
	_ = pc.Close()
	if moved := corpse.count(); moved != wire+1 {
		t.Fatalf("the pool-served flow must pass the door and dial: %d -> %d corpse connections", wire, moved)
	}

	// An answered own-dial rehabilitates the cluster (the mark clears)
	// while the echo's delivered payload media-verifies its origin —
	// the pool's automatic food.
	gc := dialConnect(t, srv.Addr(), "rr5---sn-zzz-5.googlevideo.com", port)
	_, _ = gc.Write(tlsLikeRecord(64))
	_ = gc.SetReadDeadline(time.Now().Add(3 * time.Second))
	first := make([]byte, 256)
	fn, _ := gc.Read(first) // the echo's first flight: the first byte that rehabilitates
	big := make([]byte, 8192)
	_, _ = gc.Write(big) // the payload: echoed back, delivered downstream
	_ = gc.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := 0
	for got < len(big) {
		cn, gerr := gc.Read(big)
		if cn > 0 {
			got += cn
		}
		if gerr != nil {
			break
		}
	}
	_ = gc.Close()
	if fn == 0 {
		t.Fatalf("the echo flow never got its first byte")
	}
	if rehabbed := pollLines(log, "INFO gvideo cluster rehabilitated", 1, 2*time.Second); len(rehabbed) == 0 {
		t.Fatalf("the own-dial answer must REHABILITATE the cluster")
	}
	if d := st.Diagnostic(); d.GvideoClusterRehabs != 1 {
		t.Fatalf("gvideoClusterRehabs = %d, want 1", d.GvideoClusterRehabs)
	}
	if fed := pollLines(log, "INFO gvideo origin media-verified by payload", 1, 3*time.Second); len(fed) == 0 {
		t.Fatalf("the 8 KB echo delivery must media-verify the origin by payload")
	}
	if d := st.Diagnostic(); d.GvideoPayloadFeeds != 1 {
		t.Fatalf("gvideoPayloadFeeds = %d, want 1", d.GvideoPayloadFeeds)
	}
	if _, gvideo := srv.GoodIPStats(); gvideo < 2 {
		t.Fatalf("the pool must hold both media-verified origins (the marked 127.0.0.9 and the payload-fed echo), gvideo = %d", gvideo)
	}
	if served := echo.count(); served < 1 {
		t.Fatalf("the rehab flow never reached the echo origin (count=%d)", served)
	}
}
