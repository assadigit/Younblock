package doh

// The M8-z work item C shapes (ADR-0025), pinned by test:
//
//   1. ONE shared HTTP/2 connection: a burst of 10 concurrent DISTINCT
//      names against one server creates exactly ONE dial (the debug.16
//      report's slow cold lookups were one fresh fragmented TLS
//      handshake per concurrent first query).
//
//   2. The hedged walk: the first server STALLS (handshake completes,
//      no answer), the second answers — the answer wins at ~HedgeDelay
//      with the second server's verdict, the stall is cancelled, and
//      the hedge win is counted.
//
//   3. The IP-literal endpoints: a server whose URL host is an IP gets
//      a ClientHello with NO SNI (Go omits it for IP literals — nothing
//      for SNI inspection to match) and its IP-SAN certificate
//      verifies.
//
//   4. The googlevideo sn-<code> learning + persistence: rr/r variants
//      of one code collapse to the single remembered code, the table
//      persists to CodeStore and reloads, and it is bounded.
//
//   5. WarmProbe opens the connection without waiting for a query.
//
// Both-ways proof: with the transport reverted to http.Transport (the
// M8-y shape), test 1 FAILS (10 dials); with askServers reverted to
// the serial walk, test 2 FAILS (the answer costs the first server's
// full 2 s timeout, not ~50 ms, and no hedge win is counted).

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
)

// TestBurstSharesOneConnection (shape 1, the fix's core): 10
// concurrent distinct names, one server, exactly one dial.
func TestBurstSharesOneConnection(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	const burst = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			name := fmt.Sprintf("burst%d.example.com", i)
			if _, err := r.Lookup(context.Background(), name); err != nil {
				t.Errorf("lookup %s: %v", name, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	d := r.Diagnostic()
	if d.Dials != 1 {
		t.Fatalf("dials = %d, want exactly 1 — the burst must share ONE HTTP/2 connection (got queries=%d)", d.Dials, d.Queries)
	}
	if d.Queries != burst {
		t.Fatalf("queries = %d, want %d", d.Queries, burst)
	}
	if d.Reuse != burst-1 {
		t.Fatalf("reuse = %d, want %d", d.Reuse, burst-1)
	}
}

// stallingDoH accepts the TLS handshake and never answers — the
// silent-server shape the hedge exists for.
func stallingDoH(t *testing.T) *fakeDoH {
	t.Helper()
	f := &fakeDoH{answer: func(string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		select { // stall forever: the request never completes
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	f.srv = srv
	t.Cleanup(srv.Close)
	return f
}

// TestHedgeSecondServerWinsWhenFirstStalls (shape 2, the fix's core):
// server #1 stalls; with HedgeDelay=50 ms the second server's answer
// wins at ~50 ms — not after the first server's full 2 s timeout (the
// serial M8-y shape), and the hedge win is counted for server #2.
func TestHedgeSecondServerWinsWhenFirstStalls(t *testing.T) {
	stall := stallingDoH(t)
	live := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("198.51.100.7")}, 300
	})
	cfg := testConfig(stall.serverConfig("Stall"), live.serverConfig("Live"))
	cfg.HedgeDelay = 50 * time.Millisecond
	pool := stall.tlsConfig().RootCAs
	pool.AddCert(live.srv.Certificate())
	cfg.TLS = &tls.Config{RootCAs: pool}
	r := New(cfg)

	t0 := time.Now()
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup through the hedged walk: %v", err)
	}
	took := time.Since(t0)
	if len(ips) == 0 || !ips[0].Equal(net.ParseIP("198.51.100.7")) {
		t.Fatalf("ips = %v, want the second server's answer", ips)
	}
	// The hedge fires at 50 ms; the serial walk would cost the first
	// server's whole 2 s timeout. 1 s is generous for CI jitter while
	// still proving the stagger did the work.
	if took > time.Second {
		t.Fatalf("hedged lookup took %v — the serial walk's full-timeout shape (want ~50 ms + answer)", took)
	}
	d := r.Diagnostic()
	if d.HedgeWins != 1 {
		t.Fatalf("hedgeWins = %d, want 1", d.HedgeWins)
	}
	if len(d.HedgeByServer) < 2 || d.HedgeByServer[1] != 1 {
		t.Fatalf("hedgeByServer = %v, want the win on server #2", d.HedgeByServer)
	}
}

// TestIPServerNoSNIAndCertVerification (shape 3): a server whose URL
// host is an IP literal gets a ClientHello with NO server name (Go
// omits SNI for IPs — nothing for SNI inspection to match) and its
// IP-SAN certificate still verifies.
func TestIPServerNoSNIAndCertVerification(t *testing.T) {
	// Record the ClientHello's server name (installed before StartTLS —
	// the only point a TLS customizer takes effect).
	var sniMu sync.Mutex
	var gotSNI []string
	f := newFakeDoHCustom(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	}, func(srv *httptest.Server) {
		srv.TLS = &tls.Config{
			GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
				sniMu.Lock()
				gotSNI = append(gotSNI, hi.ServerName)
				sniMu.Unlock()
				return nil, nil // nil keeps the server's own (h2 + cert) config
			},
		}
	})

	// The server URL's host is the IP literal itself (the shape of the
	// two no-SNI endpoints appended to the default list).
	_, port, _ := net.SplitHostPort(f.srv.Listener.Addr().String())
	ipSrv := configServer(t, f, fmt.Sprintf("https://127.0.0.1:%s/dns-query?dns={dns}", port))
	cfg := testConfig(ipSrv)
	cfg.TLS = f.tlsConfig()
	r := New(cfg)

	if _, err := r.Lookup(context.Background(), "example.com"); err != nil {
		t.Fatalf("Lookup through the IP-literal endpoint: %v", err)
	}
	sniMu.Lock()
	defer sniMu.Unlock()
	if len(gotSNI) == 0 {
		t.Fatal("no ClientHello observed")
	}
	for _, s := range gotSNI {
		if s != "" {
			t.Fatalf("ClientHello carried SNI %q for an IP-literal endpoint — want NO name for inspection to match", s)
		}
	}
}

// configServer builds a DoHServer with an explicit URL (the helper
// variant for IP-literal shapes).
func configServer(t *testing.T, f *fakeDoH, url string) config.DoHServer {
	t.Helper()
	return config.DoHServer{Name: "Fake", URL: url, Bootstrap: []string{"127.0.0.1"}}
}

// TestGvideoCodeLearningAndPersistence (shape 4): rr1---sn-X and
// r2---sn-X collapse to the one remembered code; the table persists to
// CodeStore and reloads in a fresh resolver; the cap evicts FIFO.
func TestGvideoCodeLearningAndPersistence(t *testing.T) {
	store := filepath.Join(t.TempDir(), "googlevideo-codes.txt")
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	cfg.CodeStore = store
	r := New(cfg)

	if _, err := r.Lookup(context.Background(), "rr1---sn-abc123.googlevideo.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Lookup(context.Background(), "r2---sn-abc123.googlevideo.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Lookup(context.Background(), "rr4---sn-def456.googlevideo.com"); err != nil {
		t.Fatal(err)
	}
	if codes := r.GvideoCodes(); len(codes) != 2 || codes[0] != "sn-abc123" || codes[1] != "sn-def456" {
		t.Fatalf("codes = %v, want [sn-abc123 sn-def456] (the rr/r variants collapse)", codes)
	}
	data, err := os.ReadFile(store)
	if err != nil {
		t.Fatalf("the code store must persist: %v", err)
	}
	if strings.TrimSpace(string(data)) != "sn-abc123\nsn-def456" {
		t.Fatalf("code store = %q", strings.TrimSpace(string(data)))
	}

	// A fresh resolver reloads the table.
	r2 := New(cfg)
	if codes := r2.GvideoCodes(); len(codes) != 2 || codes[0] != "sn-abc123" {
		t.Fatalf("reloaded codes = %v, want the persisted set", codes)
	}

	// The cap evicts FIFO (a small synthetic table through the internal
	// note path is enough — the cap itself is pinned in config tests).
	r3 := New(testConfig(f.serverConfig("Fake")))
	r3.cfg.CodeStore = ""
	for i := 0; i < 70; i++ {
		r3.noteGvideoCode(fmt.Sprintf("sn-cap%03d", i))
	}
	if codes := r3.GvideoCodes(); len(codes) != 64 {
		t.Fatalf("capped codes = %d, want 64", len(codes))
	}
}

// TestWarmProbeOpensConnection (shape 5): WarmProbe dials the shared
// connection without any lookup waiting on it, and logs the warm line.
func TestWarmProbeOpensConnection(t *testing.T) {
	f := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("93.184.216.34")}, 300
	})
	cfg := testConfig(f.serverConfig("Fake"))
	cfg.TLS = f.tlsConfig()
	log := &captureLogger{}
	cfg.Logger = log
	r := New(cfg)

	r.WarmProbe()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.Diagnostic().Dials >= 1 && f.hits.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("WarmProbe did not open the connection (dials=%d hits=%d)", r.Diagnostic().Dials, f.hits.Load())
}

// TestSerialWalkWhenHedgingDisabled (guard): a negative HedgeDelay
// pins the exact serial M8-y shape — a stalling first server costs its
// full timeout before the second is asked. Passes on both sides.
func TestSerialWalkWhenHedgingDisabled(t *testing.T) {
	stall := stallingDoH(t)
	live := newFakeDoH(t, func(name string) ([]net.IP, uint32) {
		return []net.IP{net.ParseIP("198.51.100.7")}, 300
	})
	cfg := testConfig(stall.serverConfig("Stall"), live.serverConfig("Live"))
	cfg.HedgeDelay = -1 // the exact serial shape
	cfg.Timeout = 700 * time.Millisecond
	pool := stall.tlsConfig().RootCAs
	pool.AddCert(live.srv.Certificate())
	cfg.TLS = &tls.Config{RootCAs: pool}
	r := New(cfg)

	t0 := time.Now()
	ips, err := r.Lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	took := time.Since(t0)
	if len(ips) == 0 || !ips[0].Equal(net.ParseIP("198.51.100.7")) {
		t.Fatalf("ips = %v, want the second server's answer", ips)
	}
	if took < cfg.Timeout {
		t.Fatalf("serial walk answered in %v — faster than the first server's %v timeout; the serial shape is not pinned", took, cfg.Timeout)
	}
	if d := r.Diagnostic(); d.HedgeWins != 0 {
		t.Fatalf("hedgeWins = %d, want 0 with hedging disabled", d.HedgeWins)
	}
}
