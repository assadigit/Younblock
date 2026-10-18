package netx

import (
	"context"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestDialHostTriesEveryAddressInOrder(t *testing.T) {
	// The good listener sits on 127.0.0.2; dialing the same port on 127.0.0.1
	// is refused (nothing listens there), proving the second address is tried.
	good, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("127.0.0.2 not bindable on this host: %v", err)
	}
	defer good.Close()
	goodAddr := good.Addr().(*net.TCPAddr)

	d := New(500*time.Millisecond, 2*time.Second, nil)
	conn, err := d.DialHost(context.Background(), "tcp", "example.test", goodAddr.Port,
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")})
	if err != nil {
		t.Fatalf("DialHost: %v", err)
	}
	conn.Close()
}

func TestDialHostAllFailReportsLastError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	l.Close()
	time.Sleep(50 * time.Millisecond)

	d := New(200*time.Millisecond, 1*time.Second, nil)
	_, err = d.DialHost(context.Background(), "tcp", "example.test", addr.Port,
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")})
	if err == nil {
		t.Fatal("expected an error when every address fails")
	}
	if want := "all 2 addresses failed"; len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", err.Error(), want)
	}
}

func TestDialHostEmptyAddressList(t *testing.T) {
	d := New(time.Second, time.Second, nil)
	if _, err := d.DialHost(context.Background(), "tcp", "example.test", 443, nil); err == nil {
		t.Fatal("expected error for empty address list")
	}
}

func TestDialHostHonorsOverallTimeout(t *testing.T) {
	// Three black-hole addresses: unroutable TEST-NET IPs will time out.
	ips := []net.IP{
		net.ParseIP("192.0.2.1"),
		net.ParseIP("192.0.2.2"),
		net.ParseIP("192.0.2.3"),
	}
	d := New(2*time.Second, 1*time.Second, nil)
	start := time.Now()
	_, err := d.DialHost(context.Background(), "tcp", "slow.test", 81, ips)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("overall timeout not honored: %v", elapsed)
	}
}

// TestDialHostRacesPastBlackholedFrontAddress pins the M8-n fix: a
// silently-blackholed first address (the filtered-network shape — no RST,
// the SYN just dies) must cost the fallback stagger, not a full
// PerIPTimeout. Before the fix this dial needed ≥5 s (sequential walk);
// after it the second address starts at 750 ms and wins by ~1 s.
func TestDialHostRacesPastBlackholedFrontAddress(t *testing.T) {
	good, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("127.0.0.2 not bindable on this host: %v", err)
	}
	defer good.Close()
	goodAddr := good.Addr().(*net.TCPAddr)

	// 192.0.2.1 is TEST-NET: the SYN blackholes (the same property
	// TestDialHostHonorsOverallTimeout above relies on in this suite).
	d := New(5*time.Second, 10*time.Second, nil)
	start := time.Now()
	conn, err := d.DialHost(context.Background(), "tcp", "front.test", goodAddr.Port,
		[]net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("127.0.0.2")})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("DialHost: %v", err)
	}
	conn.Close()
	// The stagger fires the second attempt at 750 ms; the winner lands
	// well under 2 s. The old sequential walk could not return before
	// the 5 s PerIPTimeout of the blackholed front address.
	if elapsed >= 2*time.Second {
		t.Errorf("blackholed front address cost %v; fast failover did not engage (want < 2s)", elapsed)
	}
}

// TestDialHostWinnerCancelsLosers pins the cleanup half of the race: when
// the front address wins cleanly, the blackholed back address must not
// hold the call open (its attempt is canceled the moment the winner lands).
func TestDialHostWinnerCancelsLosers(t *testing.T) {
	good, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("127.0.0.2 not bindable on this host: %v", err)
	}
	defer good.Close()
	goodAddr := good.Addr().(*net.TCPAddr)

	d := New(3*time.Second, 6*time.Second, nil)
	start := time.Now()
	conn, err := d.DialHost(context.Background(), "tcp", "clean.test", goodAddr.Port,
		[]net.IP{net.ParseIP("127.0.0.2"), net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("DialHost: %v", err)
	}
	conn.Close()
	// Without cancel-on-winner the call would wait out the losers'
	// stagger timers; the cancel unwinds them immediately.
	if elapsed >= 2*time.Second {
		t.Errorf("winning dial held open %v by canceled losers (want < 2s)", elapsed)
	}
}

func TestControlHookCalled(t *testing.T) {
	var calls atomic.Int32
	control := func(network, address string, c syscall.RawConn) error {
		calls.Add(1)
		return nil
	}
	good, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer good.Close()
	port := good.Addr().(*net.TCPAddr).Port

	d := New(500*time.Millisecond, time.Second, control)
	conn, err := d.DialHost(context.Background(), "tcp", "example.test", port,
		[]net.IP{net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("DialHost: %v", err)
	}
	defer conn.Close()
	if calls.Load() != 1 {
		t.Errorf("control hook called %d times, want 1", calls.Load())
	}
}
