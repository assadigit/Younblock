package doh

// The M8-y shapes (ADR-0023), pinned by test:
//
//   1. The negative re-ask — a DNS_UNREACHABLE negative (the resolver's
//      own path failed, not a verdict about the name) re-asks the
//      servers through the singleflight instead of failing instantly.
//      The debug.16 report measured the lockout: dns ok 142 / fail 50
//      with doorFailures=50, the failure lines pairing at the same
//      millisecond — one DoH blip negative-caching a batch of fresh
//      per-video hostnames, then the negative's whole 15 s TTL of
//      instant kills at the door.
//
//   2. The rate limit — the walk that WROTE the negative counts as the
//      window's probe, so inside the window a negative hit fails fast
//      (never a redundant walk seconds after a failure), and during a
//      real outage the probing stays at one walk per window per host.
//
//   3. Server-verdict negatives (no usable answer) keep the instant
//      fail — the servers SPOKE; re-asking a verdict is pointless.
//
// Both-ways proof: with resolver.go's re-ask stashed (the M8-w shape),
// tests 1 and 2 FAIL — the +2 s lookup dies on the negative without a
// server hit, exactly the report's lockout — and every guard (test 3,
// the rewritten TestCacheNegativeTTL's inside-window half) passes on
// both sides, pinning exactly what changed and nothing else.

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// TestNegativeReaskRecoversAfterBlip (M8-y shape 1): a DoH blip fails a
// lookup and negative-caches the host; the blip passes; the next lookup
// 2 s later must RE-ASK the servers and succeed instead of dying on the
// negative — the report's 15 s lockout collapsed to one walk.
// Pre-fix this FAILS: the +2 s lookup returns ErrRecentlyFailed without
// touching the server.
func TestNegativeReaskRecoversAfterBlip(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.Now = func() time.Time { return now }
	r := New(cfg)

	f.fail.Store(true) // the blip
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected the blip's failure")
	}
	hitsAfterBlip := f.hits.Load()

	// The blip passes. The negative (15 s TTL) is still fresh — but
	// DNS_UNREACHABLE is a path failure, not a name verdict: the lookup
	// must re-ask and recover.
	now = now.Add(2 * time.Second)
	f.fail.Store(false)
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("the re-ask must recover the blip: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("re-ask ips = %v", ips)
	}
	if got := f.hits.Load(); got != hitsAfterBlip+1 {
		t.Fatalf("server hits = %d, want exactly one re-ask walk (+1)", got-hitsAfterBlip)
	}

	// And the answer is cached: an immediate follow-up lookup is a hit.
	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatalf("the recovered answer must cache: %v", err)
	}
	if got := f.hits.Load(); got != hitsAfterBlip+1 {
		t.Fatalf("the recovered answer must be cached: hits = %d", got)
	}
}

// TestNegativeReaskRateLimited (M8-y shape 2): the servers stay down.
// The walk that wrote the negative counts as the window's probe, so a
// hit 2 s later re-asks exactly once (the singleflight coalesces the
// wave), and a hit INSIDE the following window fails fast without a
// walk — during a real outage the probing stays at one walk per window
// per host, never a storm.
func TestNegativeReaskRateLimited(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.Now = func() time.Time { return now }
	r := New(cfg)

	f.fail.Store(true) // a real outage, not a blip
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected the outage's failure (walk 1)")
	}
	// 2 s later: past the window — the negative hit re-asks (walk 2) and
	// fails again; the negative re-caches with a fresh TTL.
	now = now.Add(2 * time.Second)
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected the re-ask's failure (walk 2)")
	}
	if got := f.hits.Load(); got != 2 {
		t.Fatalf("server hits = %d, want 2 (the walk + one re-ask)", got)
	}
	// 500 ms later: inside the new window — the negative answers
	// instantly, no third walk.
	now = now.Add(500 * time.Millisecond)
	_, err := r.Lookup(context.Background(), "example.com")
	if err == nil || !errors.Is(err, ErrRecentlyFailed) {
		t.Fatalf("inside the re-ask window the negative must answer: %v", err)
	}
	if got := f.hits.Load(); got != 2 {
		t.Fatalf("server hits = %d, want still 2 (the window held the probing line)", got)
	}
}

// TestNegativeReaskDisabledByConfig (guard): a negative NegativeReask
// pins the exact M8-w shape — the negative answers until its TTL
// expires, no re-ask ever fires (the server is hit exactly once).
func TestNegativeReaskDisabledByConfig(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.NegativeReask = -1 // the exact M8-w shape
	cfg.Now = func() time.Time { return now }
	r := New(cfg)

	f.fail.Store(true)
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected failure")
	}
	f.fail.Store(false)
	now = now.Add(5 * time.Second)
	_, err := r.Lookup(context.Background(), "example.com")
	if err == nil || !errors.Is(err, ErrRecentlyFailed) {
		t.Fatalf("with the re-ask disabled the negative must answer: %v", err)
	}
	if got := f.hits.Load(); got != 1 {
		t.Fatalf("server hits = %d, want 1 (no re-asks, the M8-w shape)", got)
	}
}

// TestNegativeServerVerdictsDontReask (guard): a negative whose code is
// a SERVER VERDICT (the server answered: no A records) keeps the
// instant fail — the servers spoke, and re-asking a verdict is
// pointless. No extra server hit, whatever the clock says.
func TestNegativeServerVerdictsDontReask(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return nil, 300 // the server speaks: zero A records (ErrNoAnswer)
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.Now = func() time.Time { return now }
	r := New(cfg)

	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected the no-answer failure")
	}
	now = now.Add(5 * time.Second)
	_, err := r.Lookup(context.Background(), "example.com")
	if err == nil || !errors.Is(err, ErrRecentlyFailed) {
		t.Fatalf("a server-verdict negative must keep the instant fail: %v", err)
	}
	if got := f.hits.Load(); got != 1 {
		t.Fatalf("server hits = %d, want 1 (verdicts never re-ask)", got)
	}
}
