package doh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"

	"golang.org/x/net/dns/dnsmessage"
)

// fakeDoH is an httptest TLS server speaking the DoH GET wire format.
type fakeDoH struct {
	srv    *httptest.Server
	hits   atomic.Int32
	answer func(name string) ([]net.IP, uint32)
	fail   atomic.Bool // when true, return 500
}

func newFakeDoH(t *testing.T, answer func(name string) ([]net.IP, uint32)) *fakeDoH {
	t.Helper()
	f := &fakeDoH{answer: answer}
	mux := http.NewServeMux()
	mux.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		if f.fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		q := r.URL.Query().Get("dns")
		if q == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		wire, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(q)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var p dnsmessage.Parser
		hdr, err := p.Start(wire)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		question, err := p.Question()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// The v1 request carries the extra parameters; assert them.
		if r.URL.Query().Get("type") != "A" || r.URL.Query().Get("ct") != "application/dns-message" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("accept") != "application/dns-message" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ips, ttl := f.answer(question.Name.String())
		b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
			ID:               hdr.ID,
			Response:         true,
			RecursionDesired: hdr.RecursionDesired,
			RCode:            dnsmessage.RCodeSuccess,
		})
		if err := b.StartQuestions(); err != nil {
			t.Error(err)
		}
		if err := b.Question(question); err != nil {
			t.Error(err)
		}
		if err := b.StartAnswers(); err != nil {
			t.Error(err)
		}
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				ar := dnsmessage.AResource{A: [4]byte{v4[0], v4[1], v4[2], v4[3]}}
				if err := b.AResource(dnsmessage.ResourceHeader{
					Name:  question.Name,
					Type:  dnsmessage.TypeA,
					Class: dnsmessage.ClassINET,
					TTL:   ttl,
				}, ar); err != nil {
					t.Error(err)
				}
			}
		}
		msg, err := b.Finish()
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("content-type", "application/dns-message")
		w.Write(msg)
	})
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeDoH) serverConfig(name string) config.DoHServer {
	return config.DoHServer{
		Name:      name,
		URL:       f.srv.URL + "/dns-query?dns={dns}",
		Bootstrap: []string{"127.0.0.1"},
	}
}

func (f *fakeDoH) tlsConfig() *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(f.srv.Certificate())
	return &tls.Config{RootCAs: pool}
}

func testConfig(servers ...config.DoHServer) Config {
	return Config{
		Servers:        servers,
		Timeout:        2 * time.Second,
		BlockPrivate:   true,
		TLS:            nil, // set per test
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:            fragment.NewSafeRNG(randSource()),
	}
}

func randSource() fragment.RNG {
	return fragment.NewSafeRNG(newRand(42))
}

func newRand(seed int64) fragment.RNG {
	// A tiny deterministic source.
	return &fixedRand{seed: seed}
}

type fixedRand struct {
	seed int64
}

func (f *fixedRand) Intn(n int) int {
	f.seed = (f.seed*1103515245 + 12345) & 0x7fffffff
	if n <= 0 {
		return 0
	}
	return int(f.seed % int64(n))
}

func TestResolverParsesARecords(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("93.184.216.35")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("ips = %v, want 2 records", ips)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Errorf("ips[0] = %v, want 93.184.216.34", ips[0])
	}
	if r.LastServerName() != "Fake" {
		t.Errorf("LastServerName = %q, want Fake", r.LastServerName())
	}
}

func TestResolverCacheHit(t *testing.T) {
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	for i := 0; i < 5; i++ {
		if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
			t.Fatalf("Lookup %d: %v", i, err)
		}
	}
	if got := f.hits.Load(); got != 1 {
		t.Errorf("server hits = %d, want 1 (cache should serve the rest)", got)
	}
}

func TestResolverFallbackAfterFirstServerFails(t *testing.T) {
	bad := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("1.2.3.4")}, 300
	})
	bad.fail.Store(true)
	good := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(bad.serverConfig("Bad"), good.serverConfig("Good"))
	// Both fake servers share one TLS pool? No: each has its own cert.
	// Combine both roots in one pool.
	pool := x509.NewCertPool()
	pool.AddCert(bad.srv.Certificate())
	pool.AddCert(good.srv.Certificate())
	cfg.TLS = &tls.Config{RootCAs: pool}
	r := New(cfg)
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("ips = %v, want the good server's answer", ips)
	}
	if r.LastServerName() != "Good" {
		t.Errorf("LastServerName = %q, want Good", r.LastServerName())
	}
	// Sticky: the next lookup must start at Good and never touch Bad again.
	for i := 0; i < 3; i++ {
		if _, err := r.Lookup(context.Background(), "other.com"); err != nil {
			t.Fatalf("Lookup %d: %v", i, err)
		}
	}
	if got := bad.hits.Load(); got != 1 {
		t.Errorf("bad server hits = %d, want 1 (sticky should avoid it)", got)
	}
}

func TestResolverPrivateIPFiltering(t *testing.T) {
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("127.0.0.1")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	_, err := r.Lookup(context.Background(), "blocked.example")
	if err == nil {
		t.Fatal("expected an error for a loopback answer")
	}
	if codeFor(err) != "DNS_BLOCKED_ANSWER" {
		t.Errorf("code = %q, want DNS_BLOCKED_ANSWER", codeFor(err))
	}

	// With filtering off (test-only AllowPrivateTargets path) it resolves.
	cfg2 := testConfig(f.serverConfig("Fake"))
	cfg2.TLS = f.tlsConfig()
	cfg2.BlockPrivate = false
	r2 := New(cfg2)
	ips, err := r2.Lookup(context.Background(), "allowed.example")
	if err != nil {
		t.Fatalf("Lookup with filtering off: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("ips = %v, want 127.0.0.1", ips)
	}
}

func TestResolverAllServersUnreachable(t *testing.T) {
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	f.fail.Store(true)
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	_, err := r.Lookup(context.Background(), "down.example")
	if err == nil {
		t.Fatal("expected DNS_UNREACHABLE")
	}
	if codeFor(err) != "DNS_UNREACHABLE" {
		t.Errorf("code = %q, want DNS_UNREACHABLE", codeFor(err))
	}
}

func TestCacheTTLClampAndStaleServing(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 1 // 1 s TTL
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.Now = func() time.Time { return now }
	r := New(cfg)
	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
	// The 1 s TTL must be clamped up to the 120 s minimum.
	if got := f.hits.Load(); got != 1 {
		t.Fatalf("hits = %d", got)
	}
	now = now.Add(60 * time.Second)
	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
	if got := f.hits.Load(); got != 1 {
		t.Fatalf("after 60s hits = %d, want still 1 (clamped TTL)", got)
	}
	// At 130 s the entry is expired but still stale-able; make the server fail.
	now = now.Add(70 * time.Second)
	f.fail.Store(true)
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("expected stale answer, got %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Errorf("stale ips = %v", ips)
	}
	// Beyond 6 h after expiry there is no stale help anymore.
	now = now.Add(7 * time.Hour)
	_, err = r.Lookup(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected failure once staleness window passed")
	}
}

func TestCacheNegativeTTL(t *testing.T) {
	now := time.Now()
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.Now = func() time.Time { return now }
	r := New(cfg)
	f.fail.Store(true)
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("expected failure")
	}
	hitsAfterFailure := f.hits.Load()
	// Within 15 s the negative cache answers without touching the server.
	now = now.Add(5 * time.Second)
	f.fail.Store(false)
	if _, err := r.Lookup(context.Background(), "example.com"); err == nil {
		t.Fatal("negative cache should still fail within 15s")
	}
	if got := f.hits.Load(); got != hitsAfterFailure {
		t.Errorf("server was hit during negative window: %d vs %d", got, hitsAfterFailure)
	}
	// After 15 s the retry reaches the (now healthy) server.
	now = now.Add(11 * time.Second)
	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatalf("expected success after negative TTL: %v", err)
	}
}

func TestResolverSingleflightCoalesces(t *testing.T) {
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		time.Sleep(100 * time.Millisecond)
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.Lookup(context.Background(), "example.com")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Lookup: %v", err)
		}
	}
	if got := f.hits.Load(); got != 1 {
		t.Errorf("server hits = %d, want 1 (singleflight)", got)
	}
}

func TestResolverIPLiteralSkipsDNS(t *testing.T) {
	cfg := testConfig()
	r := New(cfg)
	ips, err := r.Lookup(context.Background(), "192.0.2.9")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("192.0.2.9")) {
		t.Errorf("ips = %v, want the literal itself", ips)
	}
}

func TestResolverResetClearsState(t *testing.T) {
	f := newFakeDoH(t, func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)
	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
	if r.Len() != 1 {
		t.Fatalf("cache len = %d, want 1", r.Len())
	}
	r.Reset()
	if r.Len() != 0 {
		t.Errorf("cache len after reset = %d, want 0", r.Len())
	}
}

func TestCacheBoundedEntries(t *testing.T) {
	c := newCache(3, nil)
	for i := 0; i < 10; i++ {
		c.putSuccess(fmt.Sprintf("h%d", i), []net.IP{net.ParseIP("93.184.216.34")}, time.Minute)
	}
	if c.len() != 3 {
		t.Errorf("cache len = %d, want 3", c.len())
	}
	if c.lookup("h9").state != stateFresh {
		t.Errorf("newest entry missing")
	}
	if c.lookup("h0").state != stateMiss {
		t.Errorf("oldest entry should have been evicted")
	}
}

// TestInvalidateForcesRequery (M8-o, ADR-0014): a total dial failure on the
// cached set must drop the fresh entry to stale, so the client's next lookup
// re-queries the servers and can pick up a different answer instead of
// being served the same suspect set until the 120 s minimum TTL expires.
func TestInvalidateForcesRequery(t *testing.T) {
	var f *fakeDoH
	f = newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		if f.hits.Load() >= 2 { // the handler counts this hit before answering
			return []net.IP{net.ParseIP("93.184.216.99")}, 300
		}
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("first Lookup: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("first answer = %v, want 93.184.216.34", ips)
	}
	r.Invalidate("example.com")

	ips, err = r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup after Invalidate: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.99")) {
		t.Fatalf("answer after Invalidate = %v, want the re-queried 93.184.216.99", ips)
	}
	if f.hits.Load() != 2 {
		t.Errorf("server hits = %d, want 2 (the cache must not have served)", f.hits.Load())
	}
}

// TestInvalidateKeepsServeStale: invalidation must not make failures worse —
// when the servers are unreachable after an invalidation, the stale answer
// is still served (R5's serve-stale contract is preserved).
func TestInvalidateKeepsServeStale(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatalf("first Lookup: %v", err)
	}
	f.fail.Store(true)
	r.Invalidate("example.com")

	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup after Invalidate with dead servers: %v (serve-stale must cover it)", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("stale answer = %v, want 93.184.216.34", ips)
	}
}

// TestInvalidateNoops: IP literals (never cached), unknown hosts (nothing
// cached) and negative entries (already re-query at their own short expiry)
// are all no-ops — invalidation only demotes still-fresh success entries.
func TestInvalidateNoops(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	r.Invalidate("93.184.216.34")      // IP literal: no panic, no cache change
	r.Invalidate("never.seen.example") // unknown host: no-op
	r.Invalidate("")                   // empty: no-op

	// A negative entry keeps its own short TTL: Invalidate must not clear it.
	if _, err := r.Lookup(context.Background(), "down.example"); err != nil {
		t.Fatalf("first Lookup of an answerable name: %v", err)
	}
	f.fail.Store(true)
	if _, err := r.Lookup(context.Background(), "poisoned.example"); err == nil {
		t.Fatalf("expected failure with dead server")
	}
	r.Invalidate("poisoned.example")
	if _, err := r.Lookup(context.Background(), "poisoned.example"); err == nil {
		t.Fatalf("negative entry must survive Invalidate (it is not a success entry)")
	}
}

// TestDialBootstrapFailsOverToSecondBootstrapIP: the resolver's own bootstrap
// dial (ADR-0013's racing DialHost since M8-o) must fail over to the next
// bootstrap address when the first one refuses. (The silent-blackhole race
// mechanics themselves are pinned by netx's own tests.)
func TestDialBootstrapFailsOverToSecondBootstrapIP(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	srvCfg := f.serverConfig("Fake")
	// 127.0.0.2:<port> refuses instantly (nothing listens there); the real
	// server is on 127.0.0.1:<port> — same port, both loopback /8.
	srvCfg.Bootstrap = []string{"127.0.0.2", "127.0.0.1"}
	cfg := testConfig(srvCfg)
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup through the second bootstrap: %v", err)
	}
	if !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("ips = %v, want the fake answer", ips)
	}
	if f.hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1", f.hits.Load())
	}
}
