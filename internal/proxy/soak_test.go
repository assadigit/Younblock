package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/stats"
)

// soakCounts returns goroutine and open-descriptor counts (fd count only on
// Linux; elsewhere it reports 0 and the assertion is skipped).
func soakCounts(t *testing.T) (goroutines int, fds int) {
	t.Helper()
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutines = runtime.NumGoroutine()
	fds = 0
	entries, err := os.ReadDir("/proc/self/fd")
	if err == nil {
		fds = len(entries)
	}
	return goroutines, fds
}

func heapInUse(t *testing.T) uint64 {
	t.Helper()
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// soakResolver resolves everything to the local echo server except names
// starting with "fail", which error.
type soakResolver struct{}

func (soakResolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	return nil, nil
}

// TestSoak5000Tunnels is the Phase 1 soak requirement (file 01 section 11;
// file 03 R2/R7): 5,000 short tunnels including failing ones; goroutine and
// handle counts return to baseline and memory stays flat.
func TestSoak5000Tunnels(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test skipped in -short mode")
	}
	up := echoServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)

	st := stats.New()
	cfg := Config{
		Host:               "127.0.0.1",
		Port:               0,
		AutoPort:           true,
		MaxConns:           256,
		RequestTimeout:     2 * time.Second,
		FirstRecordTimeout: 2 * time.Second,
		IdleTimeout:        10 * time.Second,
		MaxHeaderBytes:     config.MaxHeaderBytes,
		Resolver:           soakR{up: upHost},
		Dialer:             netx.New(2*time.Second, 4*time.Second, nil),
		FragmentPreset:     fragment.Balanced,
		FragmentConfig:     fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:                fragment.NewDefaultRNG(),
		Stats:              st,
		Domains:            config.DefaultDomains(),
		Mode:               config.ModeYouTube,
	}
	srv, err := Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.StopWithTimeout()

	// Warm up so one-time setup (pool, TLS certs, scheduler) is excluded.
	for i := 0; i < 20; i++ {
		runSoakTunnel(t, srv.Addr(), upPort, false)
	}
	for i := 0; i < 5; i++ {
		runSoakTunnel(t, srv.Addr(), upPort, true)
	}
	waitForQuiesce(t, srv)

	baseGoroutines, baseFds := soakCounts(t)
	baseHeap := heapInUse(t)

	const total = 5000
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount, failCount := 0, 0
	var unexpected []string
	workers := 20
	batches := total / workers // 5000 / 20 = 250 exactly
	for b := 0; b < batches; b++ {
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				// Every 25th tunnel fails (DNS refuses the name).
				wantFail := i%25 == 0
				err := runSoakTunnelQuiet(t, srv.Addr(), upPort, wantFail)
				mu.Lock()
				defer mu.Unlock()
				if wantFail {
					// The helper returns nil when it got the expected 502.
					if err == nil {
						failCount++
					} else {
						unexpected = append(unexpected, fmt.Sprintf("failing tunnel %d: %v", i, err))
					}
				} else if err == nil {
					okCount++
				} else {
					unexpected = append(unexpected, fmt.Sprintf("tunnel %d: %v", i, err))
				}
			}(b*workers + w)
		}
		wg.Wait()
	}
	if len(unexpected) > 0 {
		for _, u := range unexpected[:min(len(unexpected), 10)] {
			t.Errorf("unexpected soak failure: %s", u)
		}
	}
	if okCount+failCount != total {
		t.Fatalf("completed %d+%d of %d tunnels", okCount, failCount, total)
	}

	waitForQuiesce(t, srv)
	goroutines, fds := soakCounts(t)
	heap := heapInUse(t)

	t.Logf("soak: %d ok, %d failed tunnels; goroutines %d -> %d; fds %d -> %d; heap %d -> %d",
		okCount, failCount, baseGoroutines, goroutines, baseFds, fds, baseHeap, heap)

	if goroutines > baseGoroutines+5 {
		t.Errorf("goroutines did not return to baseline: %d -> %d", baseGoroutines, goroutines)
	}
	if baseFds > 0 && fds > baseFds+5 {
		t.Errorf("file descriptors did not return to baseline: %d -> %d", baseFds, fds)
	}
	growth := int64(heap) - int64(baseHeap)
	if growth > 16<<20 {
		t.Errorf("heap grew by %d bytes (> 16 MB); memory should stay flat", growth)
	}
	if d := st.Diagnostic(); d.Panics != 0 {
		t.Errorf("panics = %d, want 0", d.Panics)
	}
	if d := st.Diagnostic(); d.ConnsTotal < int64(total) {
		t.Errorf("connsTotal = %d, want >= %d", d.ConnsTotal, total)
	}
}

// waitForQuiesce waits until the server's live connection count is zero.
func waitForQuiesce(t *testing.T, srv *Server) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if srv.cfg.Stats.Diagnostic().ConnsLive == 0 {
			return
		}
	}
}

// runSoakTunnel opens one short tunnel and asserts it worked.
func runSoakTunnel(t *testing.T, addr, port string, wantFail bool) error {
	t.Helper()
	if err := runSoakTunnelQuiet(t, addr, port, wantFail); err != nil {
		t.Fatalf("soak tunnel: %v", err)
	}
	return nil
}

// runSoakTunnelQuiet opens one short tunnel without failing the test.
func runSoakTunnelQuiet(t *testing.T, addr, port string, wantFail bool) error {
	name := "site"
	if wantFail {
		name = "fail-site"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	target := name + ":" + port
	if _, err := conn.Write([]byte("CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n")); err != nil {
		return err
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	if wantFail {
		if !strings.Contains(line, "502") {
			return fmt.Errorf("want 502, got %q", strings.TrimSpace(line))
		}
		return nil
	}
	if !strings.Contains(line, "200") {
		return fmt.Errorf("want 200, got %q", strings.TrimSpace(line))
	}
	for {
		l, err := br.ReadString('\n')
		if err != nil || strings.TrimSpace(l) == "" {
			break
		}
	}
	payload := tlsLikeRecord(120)
	if _, err := conn.Write(payload); err != nil {
		return err
	}
	echo := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, echo); err != nil {
		return err
	}
	if string(echo) != string(payload) {
		return errors.New("echo mismatch")
	}
	closeWrite(conn)
	return nil
}

// soakR resolves names: "fail*" errors, everything else maps to the echo IP.
type soakR struct {
	up string
}

func (r soakR) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	if strings.HasPrefix(host, "fail") {
		return nil, errors.New("soak: simulated dns failure")
	}
	return []net.IP{net.ParseIP(r.up)}, nil
}
