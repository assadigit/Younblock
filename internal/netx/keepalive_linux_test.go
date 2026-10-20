//go:build linux

package netx

// TestDialIPAppliesKeepAlive (M8-ad, ADR-0037): a dialer carrying the
// keepalive fields installs them on the socket the kernel sees —
// TCP_KEEPIDLE, TCP_KEEPINTVL and TCP_KEEPCNT read back exactly. This
// is the probe contract the whole round rides on: a silently
// blackholed origin surfaces as a read error in ~Idle + Count ×
// Interval instead of the Go default's ~2 h. Both-ways: with the
// SetKeepAliveConfig call neutered, KEEPIDLE reads back the default
// (7200 s on Linux, or at best the runtime's 15) and this test FAILS.
import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDialIPAppliesKeepAlive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						c.Close()
						return
					}
				}
			}(c)
		}
	}()

	d := New(2*time.Second, 4*time.Second, nil)
	d.KeepAliveIdle = 7 * time.Second
	d.KeepAliveInterval = 2 * time.Second
	d.KeepAliveCount = 3

	conn, err := d.DialIP(context.Background(), "tcp", net.ParseIP("127.0.0.1"), ln.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	tc, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatalf("expected *net.TCPConn, got %T", conn)
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var idle, interval, count int
	var rerr error
	ctrl := func(fd uintptr) {
		if idle, rerr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPIDLE); rerr != nil {
			return
		}
		if interval, rerr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPINTVL); rerr != nil {
			return
		}
		count, rerr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPCNT)
	}
	if err := raw.Control(ctrl); err != nil {
		t.Fatal(err)
	}
	if rerr != nil {
		t.Fatalf("getsockopt: %v", rerr)
	}
	if idle != 7 {
		t.Fatalf("TCP_KEEPIDLE = %d, want 7 (OriginKeepAliveIdle applied)", idle)
	}
	if interval != 2 {
		t.Fatalf("TCP_KEEPINTVL = %d, want 2 (OriginKeepAliveInterval applied)", interval)
	}
	if count != 3 {
		t.Fatalf("TCP_KEEPCNT = %d, want 3 (OriginKeepAliveCount applied)", count)
	}
}
