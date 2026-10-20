//go:build linux

package netx

// The TCP_INFO snapshot contract (M8-af, ADR-0043, spec W1), pinned by
// test: the kernel's own per-socket counters are readable through the
// raw fd (BytesAcked/Sent/Retrans/Unacked/Rtt/MSS/Rcvd), a real
// exchange moves exactly the bytes the test sent and echoed, and the
// verdict classifier applies the debug.24 decision rule:
//
//   - acked did not move past A while retrans/unacked grew ->
//     upstream-lost (the request never reached the server);
//   - acked moved but rcvd did not -> downstream-lost (the request
//     arrived, the answer is being dropped or throttled);
//   - both moved -> both-ok;
//   - anything else -> unknown, never a guess.
//
// Both-ways proof: with the classifier's acked-comparison stashed,
// the verdict pins FAIL (downstream-lost flips to unknown); with the
// getsockopt wiring stashed, the snapshot test FAILS on the
// unavailable error.

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSnapshotTCPInfoOnLiveSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	payload := []byte("younblock-tcpinfo-probe")
	echoed := make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4096)
		n, rerr := c.Read(buf)
		if n > 0 {
			_, _ = c.Write(buf[:n])
		}
		if rerr != nil {
			return
		}
		close(echoed)
		c.Read(buf) // hold until the probe side closes
	}()

	d := New(2*time.Second, 4*time.Second, nil)
	conn, err := d.DialIP(context.Background(), "tcp", net.ParseIP("127.0.0.1"), ln.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	a, err := SnapshotTCPInfo(conn)
	if err != nil {
		t.Fatalf("snapshot A: %v (the raw dialed origin must be readable)", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := readFull(conn, got); err != nil {
		t.Fatal(err)
	}
	select {
	case <-echoed:
	case <-time.After(time.Second):
		t.Fatal("the echo never completed")
	}
	b, err := SnapshotTCPInfo(conn)
	if err != nil {
		t.Fatalf("snapshot B: %v", err)
	}
	// The exchange moved exactly the payload both ways.
	if b.Acked < a.Acked+uint64(len(payload)) {
		t.Fatalf("acked %d -> %d, want at least +%d (the payload was ACKed)", a.Acked, b.Acked, len(payload))
	}
	if b.Rcvd < a.Rcvd+uint64(len(payload)) {
		t.Fatalf("rcvd %d -> %d, want at least +%d (the echo was received)", a.Rcvd, b.Rcvd, len(payload))
	}
	if b.MSS == 0 {
		t.Fatal("mss = 0, want the kernel's send MSS")
	}
}

func readFull(c net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func TestSnapshotTCPInfoUnavailableOnWrappers(t *testing.T) {
	// A wrapper that hides the fd answers the honest unavailable error
	// — the proxy's evidence holder is always handed the RAW origin.
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
			go func() {
				buf := make([]byte, 64)
				for {
					if _, e := c.Read(buf); e != nil {
						return
					}
				}
			}()
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	type hider struct{ net.Conn }
	if _, err := SnapshotTCPInfo(hider{conn}); err != ErrTCPInfoUnavailable {
		t.Fatalf("wrapper snapshot err = %v, want ErrTCPInfoUnavailable", err)
	}
}

func TestClassifyTCPVerdict(t *testing.T) {
	cases := []struct {
		name string
		a, b TCPInfo
		want string
	}{
		{
			name: "upstream-lost: acked frozen, retrans grew",
			a:    TCPInfo{Acked: 100, Retrans: 0, Unacked: 0},
			b:    TCPInfo{Acked: 100, Retrans: 3, Unacked: 2},
			want: "upstream-lost",
		},
		{
			name: "upstream-lost: acked frozen, unacked grew alone",
			a:    TCPInfo{Acked: 100, Retrans: 0, Unacked: 0},
			b:    TCPInfo{Acked: 100, Retrans: 0, Unacked: 4},
			want: "upstream-lost",
		},
		{
			name: "downstream-lost: acked moved, rcvd frozen",
			a:    TCPInfo{Acked: 100, Rcvd: 2800},
			b:    TCPInfo{Acked: 500, Rcvd: 2800},
			want: "downstream-lost",
		},
		{
			name: "both-ok: both directions carried",
			a:    TCPInfo{Acked: 100, Rcvd: 2800},
			b:    TCPInfo{Acked: 500, Rcvd: 9000},
			want: "both-ok",
		},
		{
			name: "unknown: acked frozen, nothing grew",
			a:    TCPInfo{Acked: 100, Rcvd: 2800},
			b:    TCPInfo{Acked: 100, Rcvd: 2800},
			want: "unknown",
		},
		{
			name: "unknown: acked went backwards (kernel counter rollover or a fresh socket)",
			a:    TCPInfo{Acked: 500, Rcvd: 100},
			b:    TCPInfo{Acked: 100, Rcvd: 100},
			want: "unknown",
		},
	}
	for _, c := range cases {
		if got := ClassifyTCPVerdict(c.a, c.b); got != c.want {
			t.Fatalf("%s: verdict = %s, want %s", c.name, got, c.want)
		}
	}
}
