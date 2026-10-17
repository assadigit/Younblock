// socks5_test.go — host-runnable tests for the SOCKS5 front door
// (roadmap M3 task 3: CONNECT-only, address types, loopback, no auth,
// fuzz-ish edge cases like the old spec A1; ADR-0008/0009).
package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
)

// tcpEcho starts a plain TCP echo server on host:0 and returns its address.
func tcpEcho(t *testing.T, host string) *net.TCPAddr {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatalf("tcpEcho: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(c)
		}
	}()
	return ln.Addr().(*net.TCPAddr)
}

// halfClose sends FIN without tearing down the read direction, so the
// door sees a truncated-then-ended stream (falls back to a full close on
// exotic conn types).
func halfClose(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	} else {
		c.Close()
	}
}

// socks5Connect performs a full client handshake against the door and
// returns the established connection plus the final REP code (0x00 on
// success). It fails the test on transport errors, not on SOCKS refusals
// — callers assert the REP they expect.
func socks5Connect(t *testing.T, door string, atyp byte, host string, port int) (net.Conn, byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatalf("dial door: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Greeting: one method, no-auth.
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	var method [2]byte
	if _, err := io.ReadFull(conn, method[:]); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if method[0] != 0x05 || method[1] != 0x00 {
		t.Fatalf("method reply = % x, want 05 00 (no-auth)", method)
	}

	// Request.
	req := []byte{0x05, 0x01, 0x00, atyp}
	switch atyp {
	case 0x01:
		ip := net.ParseIP(host)
		if ip == nil {
			t.Fatalf("bad IPv4 literal %q", host)
		}
		req = append(req, ip.To4()...)
	case 0x03:
		if len(host) > 255 {
			t.Fatalf("domain too long")
		}
		req = append(req, byte(len(host)))
		req = append(req, host...)
	case 0x04:
		ip := net.ParseIP(host)
		if ip == nil {
			t.Fatalf("bad IPv6 literal %q", host)
		}
		req = append(req, ip.To16()...)
	default:
		t.Fatalf("bad atyp %#x", atyp)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Reply: 4-byte head, then the bind address per its ATYP, then port.
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		t.Fatalf("read reply head: %v", err)
	}
	if head[0] != 0x05 {
		t.Fatalf("reply version = %#x, want 5", head[0])
	}
	switch head[3] {
	case 0x01:
		if _, err := io.ReadFull(conn, make([]byte, 4)); err != nil {
			t.Fatalf("read reply addr: %v", err)
		}
	case 0x03:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			t.Fatalf("read reply addr len: %v", err)
		}
		if _, err := io.ReadFull(conn, make([]byte, n[0])); err != nil {
			t.Fatalf("read reply addr: %v", err)
		}
	case 0x04:
		if _, err := io.ReadFull(conn, make([]byte, 16)); err != nil {
			t.Fatalf("read reply addr: %v", err)
		}
	default:
		t.Fatalf("reply atyp = %#x", head[3])
	}
	if _, err := io.ReadFull(conn, make([]byte, 2)); err != nil {
		t.Fatalf("read reply port: %v", err)
	}
	return conn, head[1]
}

// startSocks5Door opens the door on a free loopback port on an existing
// test server and returns its address.
func startSocks5Door(t *testing.T, srv *Server) string {
	t.Helper()
	if err := srv.StartSocks5("127.0.0.1", 0); err != nil {
		t.Fatalf("StartSocks5: %v", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", srv.Socks5Port())
}

func TestSocks5LoopbackOnly(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	for _, host := range []string{"0.0.0.0", "192.168.1.5", "10.0.0.1", "", "::"} {
		err := srv.StartSocks5(host, 0)
		if err == nil {
			t.Errorf("StartSocks5(%q) succeeded; the door must refuse non-loopback binds", host)
			srv.StopSocks5()
			continue
		}
		if !errors.Is(err, ErrPortBusy) {
			t.Logf("StartSocks5(%q) = %v (non-port-busy refusal is fine)", host, err)
		}
	}
	// Loopback forms are accepted.
	for _, host := range []string{"127.0.0.1", "127.0.0.2", "::1", "localhost"} {
		if err := srv.StartSocks5(host, 0); err != nil {
			t.Errorf("StartSocks5(%q) = %v, want success", host, err)
			continue
		}
		if got := srv.Socks5Addr(); got == "" {
			t.Errorf("Socks5Addr() empty after StartSocks5(%q)", host)
		}
		srv.StopSocks5()
		if srv.Socks5Port() != 0 {
			t.Errorf("Socks5Port() = %d after StopSocks5, want 0", srv.Socks5Port())
		}
	}
	// A busy fixed port maps to ErrPortBusy (the engine surfaces PORT_BUSY).
	occupier, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupier.Close()
	busyPort := occupier.Addr().(*net.TCPAddr).Port
	err = srv.StartSocks5("127.0.0.1", busyPort)
	if !errors.Is(err, ErrPortBusy) {
		t.Fatalf("StartSocks5 on busy port = %v, want ErrPortBusy", err)
	}
}

func TestSocks5ConnectRelayDomain(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)
	echo := tcpEcho(t, "127.0.0.1")

	// Domain address type through the fake resolver: the full chain
	// handshake → resolve (staticResolver) → dial → tunnel copy.
	conn, rep := socks5Connect(t, door, 0x03, "echo.test", echo.Port)
	if rep != 0x00 {
		t.Fatalf("REP = %#x, want 0x00 (succeeded)", rep)
	}
	marker := "ping via socks5"
	if _, err := conn.Write([]byte(marker)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(marker))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != marker {
		t.Fatalf("echo = %q, want %q", got, marker)
	}
}

func TestSocks5AddressMatrix(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)
	echoV4 := tcpEcho(t, "127.0.0.1")

	lnV6, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback available on this host")
	}
	t.Cleanup(func() { lnV6.Close() })
	go func() {
		for {
			c, err := lnV6.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { defer c.Close(); io.Copy(c, c) }(c)
		}
	}()
	echoV6 := lnV6.Addr().(*net.TCPAddr)

	cases := []struct {
		name string
		atyp byte
		host string
		port int
	}{
		{"IPv4 literal skips DNS", 0x01, "127.0.0.1", echoV4.Port},
		{"domain via resolver", 0x03, "echo.test", echoV4.Port},
		{"IPv6 literal skips DNS", 0x04, "::1", echoV6.Port},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn, rep := socks5Connect(t, door, tc.atyp, tc.host, tc.port)
			if rep != 0x00 {
				t.Fatalf("REP = %#x, want 0x00", rep)
			}
			marker := fmt.Sprintf("marker-%s", tc.name)
			if _, err := conn.Write([]byte(marker)); err != nil {
				t.Fatal(err)
			}
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			got := make([]byte, len(marker))
			if _, err := io.ReadFull(conn, got); err != nil {
				t.Fatalf("read echo: %v", err)
			}
			if string(got) != marker {
				t.Fatalf("echo = %q, want %q", got, marker)
			}
		})
	}
}

func TestSocks5NoAuthOnly(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)

	// A client offering only username/password auth is refused with 05 FF.
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ff [2]byte
	if _, err := io.ReadFull(conn, ff[:]); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if ff[0] != 0x05 || ff[1] != 0xFF {
		t.Fatalf("method reply = % x, want 05 ff (no acceptable method)", ff)
	}
	// The door closes right after.
	buf := make([]byte, 8)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(buf); err != io.EOF {
		t.Fatalf("expected EOF after 05 ff, got %v", err)
	}

	// A client offering no-auth among others is accepted.
	conn2, rep := socks5Connect(t, door, 0x01, "127.0.0.1", tcpEcho(t, "127.0.0.1").Port)
	if rep != 0x00 {
		t.Fatalf("REP = %#x with mixed method offer, want 0x00", rep)
	}
	conn2.Close()
}

func TestSocks5ConnectOnly(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)
	echo := tcpEcho(t, "127.0.0.1")

	for _, cmd := range []byte{0x02 /* BIND */, 0x03 /* UDP ASSOCIATE */} {
		conn, err := net.DialTimeout("tcp", door, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
			t.Fatal(err)
		}
		var m [2]byte
		if _, err := io.ReadFull(conn, m[:]); err != nil {
			t.Fatal(err)
		}
		if m[1] != 0x00 {
			t.Fatalf("greeting unexpectedly refused: % x", m)
		}
		ip := net.ParseIP("127.0.0.1").To4()
		req := []byte{0x05, cmd, 0x00, 0x01}
		req = append(req, ip...)
		req = append(req, byte(echo.Port>>8), byte(echo.Port))
		if _, err := conn.Write(req); err != nil {
			t.Fatal(err)
		}
		var head [4]byte
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if _, err := io.ReadFull(conn, head[:]); err != nil {
			t.Fatalf("cmd %#x: %v", cmd, err)
		}
		if head[1] != 0x07 {
			t.Fatalf("cmd %#x: REP = %#x, want 0x07 (command not supported)", cmd, head[1])
		}
		conn.Close()
	}

	// Unknown address type (0x02) is refused with 0x08.
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	var m [2]byte
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00, 0x02, 1, 2, 3, 4, 5, 0x00, 0x50}); err != nil {
		t.Fatal(err)
	}
	var head [4]byte
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1] != 0x08 {
		t.Fatalf("atyp 0x02: REP = %#x, want 0x08 (address type not supported)", head[1])
	}
}

func TestSocks5EdgeCases(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)
	echo := tcpEcho(t, "127.0.0.1")
	ip := net.ParseIP("127.0.0.1").To4()

	// Non-SOCKS5 first byte (an HTTP client pointed at the door): closed
	// quietly, no reply.
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err != io.EOF {
		t.Fatalf("non-SOCKS5 input: expected quiet EOF, got %v", err)
	}
	conn.Close()

	// Wrong version in the greeting.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x04, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err != io.EOF {
		t.Fatalf("version 4 greeting: expected quiet EOF, got %v", err)
	}
	conn.Close()

	// NMETHODS claims 3, only 1 arrives, then EOF.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x03, 0x00}); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err != io.EOF {
		t.Fatalf("truncated methods: expected quiet EOF, got %v", err)
	}
	conn.Close()

	// NMETHODS = 0: 05 FF then close.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		t.Fatal(err)
	}
	var ff [2]byte
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, ff[:]); err != nil {
		t.Fatal(err)
	}
	if ff[1] != 0xFF {
		t.Fatalf("NMETHODS=0: method reply = % x, want 05 ff", ff)
	}
	conn.Close()

	// Truncated request head (2 of 4 bytes) then EOF.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	var m [2]byte
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01}); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err != io.EOF {
		t.Fatalf("truncated request: expected quiet EOF, got %v", err)
	}
	conn.Close()

	// Zero-length domain: REP 0x01.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		t.Fatal(err)
	}
	req := []byte{0x05, 0x01, 0x00, 0x03, 0x00}
	req = append(req, byte(echo.Port>>8), byte(echo.Port))
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}
	var head [4]byte
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1] != 0x01 {
		t.Fatalf("zero-length domain: REP = %#x, want 0x01", head[1])
	}
	conn.Close()

	// Domain claims 40 bytes, 10 arrive, then EOF: quiet close.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		t.Fatal(err)
	}
	req = []byte{0x05, 0x01, 0x00, 0x03, 40}
	req = append(req, []byte("shortname!")...)
	req = append(req, byte(echo.Port>>8), byte(echo.Port))
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 64)); err != io.EOF {
		t.Fatalf("short domain: expected quiet EOF, got %v", err)
	}
	conn.Close()

	// Port 0: REP 0x01.
	conn, err = net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		t.Fatal(err)
	}
	req = []byte{0x05, 0x01, 0x00, 0x01}
	req = append(req, ip...)
	req = append(req, 0x00, 0x00)
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1] != 0x01 {
		t.Fatalf("port 0: REP = %#x, want 0x01", head[1])
	}
}

func TestSocks5ResolverAndDialFailures(t *testing.T) {
	// Resolver failure → REP 0x04 (host unreachable).
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{err: errors.New("dns is down")}
	})
	door := startSocks5Door(t, srv)
	echo := tcpEcho(t, "127.0.0.1")
	conn, rep := socks5Connect(t, door, 0x03, "echo.test", echo.Port)
	if rep != 0x04 {
		t.Fatalf("resolver failure: REP = %#x, want 0x04", rep)
	}
	conn.Close()

	// Dial failure (nothing listens there) → REP 0x01.
	srv2, _ := newTestServer(t, nil)
	door2 := startSocks5Door(t, srv2)
	// Bind, note, close: a surely-dead port a moment later.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	_, rep2 := socks5Connect(t, door2, 0x03, "echo.test", deadPort)
	if rep2 != 0x01 {
		t.Fatalf("dial failure: REP = %#x, want 0x01", rep2)
	}
}

func TestSocks5FragmentedFirstRecord(t *testing.T) {
	// The "same tunnel path" proof: a routed domain's first TLS record
	// arrives at the upstream in several reads; an unrouted host's arrives
	// in one. Mirrors the HTTP-door proof in behavior_test.go (same
	// fragment bounds and 5 ms pause, so the pieces cannot coalesce).
	srv, _ := newTestServer(t, func(c *Config) {
		c.FragmentConfig = fragment.Config{Min: 80, Max: 200, Pause: 5 * time.Millisecond}
	})
	door := startSocks5Door(t, srv)

	recLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { recLn.Close() })
	rec := &recordingListener{Listener: recLn}
	go func() {
		for {
			c, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { // drain so the copy phase proceeds
				defer c.Close()
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	upPort := recLn.Addr().(*net.TCPAddr).Port

	// A synthetic TLS record: 5-byte header claiming 200 body bytes.
	record := make([]byte, 5+200)
	record[0], record[1], record[2] = 0x16, 0x03, 0x03
	record[3], record[4] = 0x00, 200

	// Routed host (youtube mode, default domain list).
	conn, rep := socks5Connect(t, door, 0x03, "www.youtube.com", upPort)
	if rep != 0x00 {
		t.Fatalf("routed: REP = %#x, want 0x00", rep)
	}
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	deadline := time.Now().Add(3 * time.Second)
	for {
		rc := rec.latest()
		if rc != nil && rc.reads.Load() >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("routed host's first record never arrived fragmented (reads so far: %v)", rec.latest())
		}
		time.Sleep(10 * time.Millisecond)
	}
	conn.Close()

	// Unrouted host: one read, unfragmented.
	rec2Ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rec2Ln.Close() })
	rec2 := &recordingListener{Listener: rec2Ln}
	go func() {
		for {
			c, err := rec2.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	upPort2 := rec2Ln.Addr().(*net.TCPAddr).Port

	conn2, rep2 := socks5Connect(t, door, 0x03, "example.org", upPort2)
	if rep2 != 0x00 {
		t.Fatalf("unrouted: REP = %#x, want 0x00", rep2)
	}
	if _, err := conn2.Write(record); err != nil {
		t.Fatal(err)
	}
	halfClose(conn2)
	time.Sleep(200 * time.Millisecond)
	if rc := rec2.latest(); rc == nil || rc.reads.Load() != 1 {
		got := int64(-1)
		if rc != nil {
			got = rc.reads.Load()
		}
		t.Fatalf("unrouted host's first record arrived in %d reads, want 1 (unfragmented)", got)
	}
	conn2.Close()
}

// TestSocks5DirectTLDStaysUnfragmented: the .ir rule (D83) through the
// Android door — a domain-type CONNECT to a .ir host relays untouched
// even in route-all mode, the mode that fragments everything else.
// Mirrors TestSocks5FragmentedFirstRecord's shape.
func TestSocks5DirectTLDStaysUnfragmented(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.Mode = config.ModeAll
		c.FragmentConfig = fragment.Config{Min: 80, Max: 200, Pause: 5 * time.Millisecond}
	})
	door := startSocks5Door(t, srv)

	recLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { recLn.Close() })
	rec := &recordingListener{Listener: recLn}
	go func() {
		for {
			c, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	upPort := recLn.Addr().(*net.TCPAddr).Port

	record := make([]byte, 5+200)
	record[0], record[1], record[2] = 0x16, 0x03, 0x03
	record[3], record[4] = 0x00, 200

	conn, rep := socks5Connect(t, door, 0x03, "example.ir", upPort)
	if rep != 0x00 {
		t.Fatalf(".ir: REP = %#x, want 0x00", rep)
	}
	if _, err := conn.Write(record); err != nil {
		t.Fatal(err)
	}
	halfClose(conn)
	time.Sleep(200 * time.Millisecond)
	if rc := rec.latest(); rc == nil || rc.reads.Load() != 1 {
		got := int64(-1)
		if rc != nil {
			got = rc.reads.Load()
		}
		t.Fatalf(".ir first record arrived in %d reads, want 1 (never fragmented, even in all mode)", got)
	}
	conn.Close()
}

func TestSocks5ConnLimit(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) { c.MaxConns = 1 })
	door := startSocks5Door(t, srv)
	echo := tcpEcho(t, "127.0.0.1")

	// First client: a live, idle tunnel (holds the single slot).
	first, rep := socks5Connect(t, door, 0x03, "echo.test", echo.Port)
	if rep != 0x00 {
		t.Fatalf("first: REP = %#x, want 0x00", rep)
	}
	defer first.Close()

	// Second client: over the shared cap → REP 0x01.
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	// The over-limit reply is the full 10-byte failure form (05 01 00 01
	// 0.0.0.0 0 0); the greeting reader sees "05 01" as its method reply,
	// so read the whole thing and check the REP slot.
	reply := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read over-limit reply: %v", err)
	}
	if reply[0] != 0x05 || reply[1] != 0x01 || reply[3] != 0x01 {
		t.Fatalf("over-limit reply = % x, want 05 01 .. 01 (general failure)", reply)
	}
}

func TestSocks5StopWithServer(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	door := startSocks5Door(t, srv)
	if err := srv.StopWithTimeout(); err != nil {
		t.Fatalf("StopWithTimeout: %v", err)
	}
	if srv.Socks5Port() != 0 || srv.Socks5Addr() != "" {
		t.Fatalf("door still reported open after Stop: port=%d addr=%q", srv.Socks5Port(), srv.Socks5Addr())
	}
	conn, err := net.DialTimeout("tcp", door, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("door still accepting after Stop")
	}
}
