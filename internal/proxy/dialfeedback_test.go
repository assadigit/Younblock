// dialfeedback_test.go — host-runnable tests for the dial-failure → DNS
// feedback loop (M8-o, ADR-0014): when a door dials every address of the
// set resolve() returned and none connect, the door must tell the resolver
// so the client's next attempt re-queries the servers instead of being
// served the same suspect set until the 120 s minimum TTL expires (the
// amplifier behind the per-video stall loop on filtered networks).
package proxy

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// invalidatingResolver is staticResolver plus a recording Invalidate.
type invalidatingResolver struct {
	staticResolver

	mu          sync.Mutex
	invalidated []string
}

func (r *invalidatingResolver) Invalidate(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalidated = append(r.invalidated, host)
}

func (r *invalidatingResolver) invalidatedHosts() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.invalidated...)
}

// deadPort reserves a loopback port and releases it, leaving nothing
// listening (a dial to it refuses instantly — the fast form of "every
// address of the set failed").
func deadPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("deadPort reserve: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// TestSocks5DialFailureInvalidatesDNS: the SOCKS5 door (the TUN stack's
// door) reports the host behind a total dial failure.
func TestSocks5DialFailureInvalidatesDNS(t *testing.T) {
	port := deadPort(t)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = res
	})
	if err := srv.StartSocks5("127.0.0.1", 0); err != nil {
		t.Fatalf("StartSocks5: %v", err)
	}
	t.Cleanup(srv.StopSocks5)

	_, rep := socks5Connect(t, srv.Socks5Addr(), socks5AtypDomain, "video.example", port)
	if rep == 0x00 {
		t.Fatalf("REP = 00 (succeeded) against a dead port; want a refusal")
	}
	got := res.invalidatedHosts()
	if len(got) != 1 || got[0] != "video.example" {
		t.Fatalf("invalidated = %v, want exactly [video.example]", got)
	}
}

// TestConnectDialFailureInvalidatesDNS: the HTTP-CONNECT door (the browser
// path) reports the same way.
func TestConnectDialFailureInvalidatesDNS(t *testing.T) {
	port := deadPort(t)
	res := &invalidatingResolver{staticResolver: staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = res
	})

	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial door: %v", err)
	}
	defer conn.Close()
	target := fmt.Sprintf("video.example:%d", port)
	req := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	br := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if len(status) < 12 || status[9:12] != "502" {
		t.Fatalf("status = %q, want 502 Bad Gateway", status)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(res.invalidatedHosts()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := res.invalidatedHosts()
	if len(got) != 1 || got[0] != "video.example" {
		t.Fatalf("invalidated = %v, want exactly [video.example]", got)
	}
}

// TestDialFailureWithoutInvalidatorIsGraceful: a resolver without the
// optional Invalidate (the tests' minimal fakes, and any third-party
// Resolver implementation) must keep working — the capability probe
// skips silently.
func TestDialFailureWithoutInvalidatorIsGraceful(t *testing.T) {
	port := deadPort(t)
	srv, _ := newTestServer(t, nil) // staticResolver has no Invalidate
	if err := srv.StartSocks5("127.0.0.1", 0); err != nil {
		t.Fatalf("StartSocks5: %v", err)
	}
	t.Cleanup(srv.StopSocks5)

	_, rep := socks5Connect(t, srv.Socks5Addr(), socks5AtypDomain, "video.example", port)
	if rep == 0x00 {
		t.Fatalf("REP = 00 (succeeded) against a dead port; want a refusal")
	}
	// No panic, no hang: the dial failure path completed with a clean
	// refusal — that is the whole assertion (the staticResolver answered
	// Lookups normally throughout, per the newTestServer default).
}
