package proxy

// The M8-y shapes (ADR-0023), pinned by test:
//
//   1. The origin-IP penalty box — a classified first-byte death (the
//      origin connected and never spoke) marks its IP, and the NEXT
//      flow's dial list skips it: the debug.16 report measured the
//      re-gamble this ends (~17 timeout terminals at 2892-4304 ms plus
//      ~8 resets, all through the raced round, and the second video's
//      NEW rr hostname paying the full discovery cost again — the
//      per-flow dead-IP map died with its flow while the authoritative
//      answer kept returning the same blackholed cache-node IPs).
//
//   2. The raced deaths mark too — an all-dead raced round (primary +
//      extras, every candidate silent to its budget) marks the origin,
//      so the follow-up flow with a live sibling on the answer set
//      never touches the corpse.
//
//   3. The box is a preference, never a verdict — an all-penalized
//      list dials in FULL (the gamble continues exactly as before,
//      never an instant fail).
//
//   4. The disabled knob pins the exact M8-x shape (every flow
//      re-gambles the whole answer set).
//
// Both-ways proof: with tunnel.go's marking stashed (the M8-x shape),
// tests 1 and 2 FAIL — the second flow's primary lands the corpse again
// (one more accept on the dead listener, no echo for flow 1's budget)
// — and the guards (3, 4) pass on both sides, pinning exactly what
// changed and nothing else.

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// settableResolver is a resolver whose answer can change mid-test (the
// second video's fresh rr hostname resolving to a different set).
type settableResolver struct {
	mu  sync.Mutex
	ips []net.IP
}

func (s *settableResolver) Lookup(context.Context, string) ([]net.IP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ips, nil
}

func (s *settableResolver) set(ips []net.IP) {
	s.mu.Lock()
	s.ips = ips
	s.mu.Unlock()
}

// TestDeadIPPreferenceSparesTheNextFlow (M8-y shape 1, the serial
// path): flow 1's primary lands the silent front IP and dies at the
// budget (racing and salvage disabled so the mark comes from the serial
// death); flow 2 — SAME answer set, fresh flow — must skip the corpse
// and dial the live echo directly. Pre-fix this FAILS: flow 2's primary
// lands the corpse again (a second accept on the silent listener) and
// burns its whole budget before dying.
func TestDeadIPPreferenceSparesTheNextFlow(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1    // serial attempt 0: the mark's simplest witness
		c.FirstByteSalvageDials = -1 // the death is terminal — terminal deaths mark too
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"),
		}}
	})

	// Flow 1: the primary lands the corpse, the budget kills it, the IP
	// is marked.
	c1 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	c1.Write(tlsLikeRecord(64))
	c1.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := c1.Read(make([]byte, 32)); err == nil {
		t.Fatal("flow 1 must terminal on the dead origin")
	}
	c1.Close()
	if n := corpse.count(); n != 1 {
		t.Fatalf("flow 1 corpse accepts = %d, want 1", n)
	}

	// Flow 2: the penalty box must spare the corpse — the echo answers
	// with no budget burn, no second terminal.
	c2 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer c2.Close()
	record := tlsLikeRecord(64)
	if _, err := c2.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	c2.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	if _, err := io.ReadFull(c2, got); err != nil {
		t.Fatalf("flow 2 must reach the live origin through the filtered list: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch through the filtered flow")
	}
	if n := corpse.count(); n != 1 {
		t.Fatalf("flow 2 corpse accepts = %d, want still 1 (the box spared it)", n)
	}
	if lines := log.lines("WARN tunnel first-byte failed "); len(lines) != 1 {
		t.Fatalf("first-byte failures = %d, want exactly 1 (flow 1's only): %+v", len(lines), lines)
	}
}

// TestRacedDeathsMarkTheIP (M8-y shape 2, the raced round): the answer
// set is a single silent IP, so the primary and both raced extras all
// land the corpse and the round terminals — every candidate's death
// marks the IP. The set then learns a live sibling (the next video's
// fresh hostname): the follow-up flow must dial the live origin with NO
// further accept on the corpse. Pre-fix this FAILS: the follow-up
// primary lands the corpse again (a fourth accept) and only the raced
// extras save the flow.
func TestRacedDeathsMarkTheIP(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	res := &settableResolver{}
	res.set([]net.IP{net.ParseIP("127.0.0.1")})
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.IdleTimeout = 5 * time.Second
		c.Resolver = res // racing at the default (2 extras)
	})

	// Flow 1: primary + 2 extras, all on the corpse, all silent to the
	// budget — the round terminals and every death marks the IP.
	c1 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	c1.Write(tlsLikeRecord(64))
	c1.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := c1.Read(make([]byte, 32)); err == nil {
		t.Fatal("flow 1 must terminal on the all-dead round")
	}
	c1.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && corpse.count() < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := corpse.count(); n != 3 {
		t.Fatalf("flow 1 corpse accepts = %d, want 3 (primary + 2 raced extras)", n)
	}

	// The next video's hostname resolves to corpse + a live sibling.
	res.set([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")})

	// Flow 2: the box filters the corpse out — the primary IS the live
	// origin and the echo arrives with no further corpse accept.
	c2 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	defer c2.Close()
	record := tlsLikeRecord(64)
	if _, err := c2.Write(record); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(record))
	c2.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	if _, err := io.ReadFull(c2, got); err != nil {
		t.Fatalf("flow 2 must reach the live origin: %v", err)
	}
	if string(got) != string(record) {
		t.Fatal("echo mismatch")
	}
	// Let the round's reaping settle, then the corpse must still be at 3.
	time.Sleep(150 * time.Millisecond)
	if n := corpse.count(); n != 3 {
		t.Fatalf("flow 2 corpse accepts = %d, want still 3 (the raced deaths' mark spared it)", n)
	}
	if lines := log.lines("WARN tunnel first-byte failed "); len(lines) != 1 {
		t.Fatalf("first-byte failures = %d, want exactly 1 (flow 1's terminal): %+v", len(lines), lines)
	}
}

// TestDeadIPAllPenalizedStillDials (guard): the box is a preference,
// never a verdict — when EVERY address of the set is penalized (or the
// set is a single dead IP), the list dials in FULL and the flow
// terminals honestly at its budget, exactly as before. Passes on both
// sides of the fix.
func TestDeadIPAllPenalizedStillDials(t *testing.T) {
	up, accepts := flakyOriginServer(t, 1<<30) // every conn silent
	upHost, upPort, _ := net.SplitHostPort(up)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.FirstByteSalvageDials = -1
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	for i := 0; i < 2; i++ {
		c := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", upPort))
		c.Write(tlsLikeRecord(64))
		c.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
		if _, err := c.Read(make([]byte, 32)); err == nil {
			t.Fatalf("flow %d must terminal on the dead origin", i+1)
		}
		c.Close()
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && accepts() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := accepts(); n != 2 {
		t.Fatalf("origin dials = %d, want 2 — an all-penalized set must dial in full, never instant-fail", n)
	}
	if lines := log.lines("WARN tunnel first-byte failed "); len(lines) != 2 {
		t.Fatalf("first-byte failures = %d, want 2: %+v", len(lines), lines)
	}
}

// TestDeadIPPenaltyDisabledByConfig (guard): a negative DeadIPPenalty
// pins the exact M8-x shape — no marking, no filtering — so the second
// flow's primary lands the corpse again and burns its budget. Passes on
// both sides of the fix.
func TestDeadIPPenaltyDisabledByConfig(t *testing.T) {
	corpse, port := silentListenerAt(t, "127.0.0.1", 0)
	echoListenerAt(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.FirstByteTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1
		c.FirstByteSalvageDials = -1
		c.DeadIPPenalty = -1 // the exact M8-x shape
		c.IdleTimeout = 5 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"),
		}}
	})
	// Flow 1: terminal on the corpse (nothing marked — the knob is off).
	c1 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	c1.Write(tlsLikeRecord(64))
	c1.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := c1.Read(make([]byte, 32)); err == nil {
		t.Fatal("flow 1 must terminal")
	}
	c1.Close()
	// Flow 2: the unfiltered list gambles again — the primary lands the
	// corpse and the flow dies at the budget (no echo).
	c2 := rawTunnel(t, srv.Addr(), net.JoinHostPort("www.youtube.com", strconv.Itoa(port)))
	c2.Write(tlsLikeRecord(64))
	c2.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := c2.Read(make([]byte, 32)); err == nil {
		t.Fatal("flow 2 must terminal on the re-gambled corpse (the disabled shape)")
	}
	c2.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && corpse.count() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if n := corpse.count(); n != 2 {
		t.Fatalf("corpse accepts = %d, want 2 (the disabled knob re-gambles)", n)
	}
	if lines := log.lines("WARN tunnel first-byte failed "); len(lines) != 2 {
		t.Fatalf("first-byte failures = %d, want 2: %+v", len(lines), lines)
	}
}
