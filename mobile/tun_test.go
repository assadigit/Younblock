// tun_test.go — host-runnable tests for the facade's TUN lifecycle
// (roadmap M4): the phase gates, the fd handover, the door on the
// ENGINE's server, and the DNS round-trip through the facade-built
// stack. The stack's own through-path (TCP flows to the door, the
// virtual DNS policy table, IPv6 fail-safe, stop-while-active) is
// exhaustively tested in internal/tunstack; here the new surface is
// what the facade adds: phase gating, fd→wire construction, the door
// belonging to the engine's live server, and lifecycle idempotency.
//
// The TUN side of the test harness is a SOCK_SEQPACKET socketpair:
// unlike a stream pair it preserves packet boundaries — the same
// datagram contract a real VpnService TUN fd has (IFF_NO_PI). The
// facade sets the fd non-blocking before os.NewFile so the netpoller
// owns it; that requirement (a blocking fd's pump cannot be unwound by
// Close — Stop would hang) is a behavior this suite verifies, not an
// assumption: the first version of StartTun missed it and this test
// deadlocked in Stop, which is exactly what it is for.
package mobile

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/health"
)

// ---- TUN-side harness ----

// tunPair returns the test's end of a packet-preserving socketpair; the
// other fd is returned as the int the facade will receive (StartTun
// takes ownership of it).
func tunPair(t *testing.T) (*os.File, int32) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
	if err != nil {
		if errors.Is(err, syscall.EPROTONOSUPPORT) {
			t.Skipf("SOCK_SEQPACKET unsupported here: %v", err)
		}
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() { _ = os.NewFile(uintptr(fds[0]), "tun-test-side").Close() })
	return os.NewFile(uintptr(fds[0]), "tun-test-side"), int32(fds[1])
}

// ipv4UDP wraps one UDP payload in an IPv4 packet (header checksum
// computed — netstack verifies it; the UDP checksum is 0, which IPv4
// permits).
func ipv4UDP(src, dst net.IP, sport, dport uint16, payload []byte) []byte {
	udp := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[8:], payload)

	total := 20 + len(udp)
	pkt := make([]byte, total)
	pkt[0] = 0x45 // v4, IHL 5
	binary.BigEndian.PutUint16(pkt[2:4], uint16(total))
	pkt[4] = 0x42 // id
	pkt[8] = 64   // TTL
	pkt[9] = 17   // UDP
	copy(pkt[12:16], src.To4())
	copy(pkt[16:20], dst.To4())
	copy(pkt[20:], udp)

	// Header checksum.
	var sum uint32
	for i := 0; i < 20; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(pkt[i : i+2]))
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	binary.BigEndian.PutUint16(pkt[10:12], ^uint16(sum))
	return pkt
}

// dnsQueryWire builds the DNS query wire bytes (one A question).
func dnsQueryWire(id uint16, name string) []byte {
	q := make([]byte, 0, 12+len(name)+6)
	var hdr [12]byte
	binary.BigEndian.PutUint16(hdr[0:2], id)
	binary.BigEndian.PutUint16(hdr[2:4], 0x0100) // RD
	binary.BigEndian.PutUint16(hdr[4:6], 1)
	q = append(q, hdr[:]...)
	start := 0
	for i := 0; i <= len(name); i++ {
		if i == len(name) || name[i] == '.' {
			if i > start {
				q = append(q, byte(i-start))
				q = append(q, name[start:i]...)
			}
			start = i + 1
		}
	}
	q = append(q, 0, 0x00, 0x01, 0x00, 0x01) // root, A, IN
	return q
}

// udpPayloadOf extracts the UDP payload from one IPv4 packet.
func udpPayloadOf(t *testing.T, pkt []byte) []byte {
	t.Helper()
	if len(pkt) < 28 {
		t.Fatalf("short packet (%d bytes)", len(pkt))
	}
	ihl := int(pkt[0]&0x0f) * 4
	if len(pkt) < ihl+8 {
		t.Fatalf("packet %d < ihl %d + 8", len(pkt), ihl)
	}
	return pkt[ihl+8:]
}

// ---- engine harness (same fakes as the M3 protect test) ----

// startServingEngine builds a facade engine over the local fakes and
// drives it to the serving phase ("on"). Returns the facade, the
// counting protector and a plain TCP echo port reachable through the
// engine's resolver (the fake DoH answers every A question 127.0.0.1,
// so CONNECT <name>:echoPort lands on the echo origin).
func startServingEngine(t *testing.T) (*Engine, *countingProtector) {
	t.Helper()
	dir := t.TempDir()
	cert, pool := newLeafCert(t)
	dohPort := httpsServer(t, cert, fakeDoHHandler())

	upMux := http.NewServeMux()
	upMux.HandleFunc("/generate_204", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	upMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	upPort := httpsServer(t, cert, upMux)

	// Plain echo origin for the door-dial proof (the tunnel carries raw
	// bytes; TLS is between the endpoints, which this test does not need).
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen: %v", err)
	}
	t.Cleanup(func() { echoLn.Close() })
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()

	settings := config.Settings{
		Version: 1,
		Mode:    config.ModeYouTube,
		Advanced: config.Advanced{
			DoHServers: []config.DoHServer{{
				Name:      "FakeDoH",
				URL:       fmt.Sprintf("https://127.0.0.1:%d/dns-query?dns={dns}", dohPort),
				Bootstrap: []string{"127.0.0.1"},
			}},
			Domains: config.DefaultDomains(),
		},
	}
	if err := config.Save(dir, settings); err != nil {
		t.Fatal(err)
	}

	protector := &countingProtector{}
	newTLS := func() *tls.Config { return &tls.Config{RootCAs: pool} }
	m, err := newEngineForTest(dir, protector, &recordingSink{}, func(o *engine.Options) {
		o.AllowPrivateTargets = true
		o.ResolverTLS = newTLS()
		o.WatchdogInterval = time.Hour
		o.HealthOverrides = func(c *health.Config) {
			c.PrimaryURL = fmt.Sprintf("https://www.youtube.com:%d/generate_204", upPort)
			c.FallbackURL = fmt.Sprintf("https://www.youtube.com:%d/fallback", upPort)
			c.VideoHost = fmt.Sprintf("redirector.googlevideo.com:%d", upPort)
			c.DNSName = "www.youtube.com"
			c.VideoTLS = newTLS()
			c.WebTLS = newTLS()
		}
	})
	if err != nil {
		t.Fatalf("newEngineForTest: %v", err)
	}
	t.Cleanup(m.Shutdown)

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}
	waitForPhase(t, m, engine.PhaseOn, 40*time.Second)

	// Stash the echo port where the test's door dial can find it.
	t.Setenv("YB_TEST_ECHO_PORT", strconv.Itoa(echoLn.Addr().(*net.TCPAddr).Port))
	return m, protector
}

// TestTunLifecycle — the facade contract, end to end against the real
// engine: gates, fd handover, DNS round-trip through the facade-built
// stack, a SOCKS5 CONNECT through the ENGINE's door resolving via the
// ENGINE's (fake) DoH and dialing protect()ed, and clean teardown.
func TestTunLifecycle(t *testing.T) {
	m, protector := startServingEngine(t)
	echoPort, _ := strconv.Atoi(os.Getenv("YB_TEST_ECHO_PORT"))

	// Gate: not serving? It is serving — the negative gate is checked
	// against a fresh engine instead (below, in TestTunRequiresServing).

	tun, fd := tunPair(t)

	// Gate: negative fd.
	if got := m.StartTun(-1, 1500); got != `{"error":"BAD_FD"}` {
		t.Fatalf("StartTun(-1) = %s", got)
	}

	// Happy path: bind the fd.
	got := m.StartTun(fd, 1500)
	if !strings.Contains(got, `"error":null`) || !strings.Contains(got, `"door":"127.0.0.1:`) {
		t.Fatalf("StartTun = %s", got)
	}
	door := doorAddrOf(t, got)

	// Gate: a second stack on a live one is refused.
	_, fd2 := tunPair(t)
	if g2 := m.StartTun(fd2, 1500); g2 != `{"error":"TUN_ACTIVE"}` {
		t.Fatalf("second StartTun = %s", g2)
	}

	// DNS round-trip through the facade-built stack: one query packet in
	// (to 8.8.8.8:53 — any destination works, ADR-0009's UDP:53 rule is
	// port-scoped, not address-scoped), one fake-pool answer out.
	q := dnsQueryWire(0x2A2A, "www.youtube.com")
	if _, err := tun.Write(ipv4UDP(net.ParseIP("10.111.0.2"), net.ParseIP("8.8.8.8"), 49152, 53, q)); err != nil {
		t.Fatalf("dns write: %v", err)
	}
	_ = tun.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 2000)
	n, err := tun.Read(buf)
	if err != nil {
		t.Fatalf("dns read: %v", err)
	}
	resp := udpPayloadOf(t, buf[:n])
	if len(resp) < 12+4 {
		t.Fatalf("short dns response (%d)", len(resp))
	}
	if binary.BigEndian.Uint16(resp[0:2]) != 0x2A2A {
		t.Fatalf("dns id = %#x", binary.BigEndian.Uint16(resp[0:2]))
	}
	if an := binary.BigEndian.Uint16(resp[6:8]); an != 1 {
		t.Fatalf("dns ANCOUNT = %d", an)
	}
	ip := net.IP(resp[len(resp)-4:])
	if ip[0] != 198 || (ip[1] != 18 && ip[1] != 19) {
		t.Fatalf("answer %s outside the fake pool", ip)
	}

	// The stack is alive and counting.
	if st := m.TunStats(); !strings.Contains(st, `"active":true`) || !strings.Contains(st, `"dnsQueries":1`) {
		t.Fatalf("TunStats = %s", st)
	}

	// The door is the ENGINE's door: a hand-rolled SOCKS5 CONNECT with a
	// domain target resolves through the engine's (fake) DoH — landing on
	// 127.0.0.1 — and the upstream dial is protect()ed (ADR-0011 through
	// the facade, on top of the M3 call-counting proof).
	before := protector.count()
	conn, err := net.DialTimeout("tcp", door, 3*time.Second)
	if err != nil {
		t.Fatalf("door dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := socks5Greet(conn); err != nil {
		t.Fatalf("socks5 greet: %v", err)
	}
	domain := "www.youtube.com"
	req := []byte{5, 1, 0, 3, byte(len(domain))}
	req = append(req, domain...)
	req = append(req, byte(echoPort>>8), byte(echoPort))
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("socks5 request: %v", err)
	}
	if err := socks5SkipReply(conn); err != nil {
		t.Fatalf("socks5 reply: %v", err)
	}
	if _, err := conn.Write([]byte("ping through the door")); err != nil {
		t.Fatalf("payload write: %v", err)
	}
	echo := make([]byte, 21)
	if _, err := io.ReadFull(conn, echo); err != nil {
		t.Fatalf("echo read: %v", err)
	}
	if string(echo) != "ping through the door" {
		t.Fatalf("echo = %q", echo)
	}
	if after := protector.count(); after <= before {
		t.Fatalf("door upstream dial was not protected (%d → %d)", before, after)
	}

	// Teardown: StopTun idempotent, stats go quiet-active.
	if g3 := m.StopTun(); g3 != `{"error":null}` {
		t.Fatalf("StopTun = %s", g3)
	}
	if g4 := m.StopTun(); g4 != `{"error":null}` {
		t.Fatalf("second StopTun = %s", g4)
	}
	if st := m.TunStats(); !strings.Contains(st, `"active":false`) {
		t.Fatalf("TunStats after stop = %s", st)
	}

	// A fresh StartTun works after the stop (the service reconnect path).
	tun2, fd3 := tunPair(t)
	_ = tun2
	if g5 := m.StartTun(fd3, 1500); !strings.Contains(g5, `"error":null`) {
		t.Fatalf("StartTun after stop = %s", g5)
	}
	// Shutdown with a live stack must unwind cleanly (StopTun inside).
	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown hung with a live TUN stack")
	}
	if st := m.TunStats(); !strings.Contains(st, `"active":false`) {
		t.Fatalf("TunStats after shutdown = %s", st)
	}
}

// TestTunRequiresServing — the engine must be serving before a TUN can
// bind: the fresh engine is off, StartTun refuses NOT_RUNNING. The fd
// is consumed either way (single-owner rule): after the refusal the
// facade has closed it, so the test's end of the pair reports the peer
// death on the next write.
func TestTunRequiresServing(t *testing.T) {
	dir := t.TempDir()
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	tun, fd := tunPair(t)
	defer tun.Close()
	if got := m.StartTun(fd, 1500); got != `{"error":"NOT_RUNNING"}` {
		t.Fatalf("StartTun while off = %s", got)
	}

	// The refused fd is closed by the facade: a write on the test's end
	// must fail (EPIPE — Go suppresses the signal), not succeed silently
	// into a socket whose peer is gone.
	_ = tun.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := tun.Write([]byte("anyone there?")); err == nil {
		t.Fatal("refused fd was not consumed (write succeeded after NOT_RUNNING)")
	}
}

// doorAddrOf extracts the door address from a StartTun result.
func doorAddrOf(t *testing.T, result string) string {
	t.Helper()
	i := strings.Index(result, `"door":"`)
	if i < 0 {
		t.Fatalf("no door in %s", result)
	}
	rest := result[i+len(`"door":"`):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("unterminated door in %s", result)
	}
	return rest[:j]
}

// socks5Greet performs the no-auth greeting half.
func socks5Greet(conn net.Conn) error {
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	var m [2]byte
	if _, err := io.ReadFull(conn, m[:]); err != nil {
		return err
	}
	if m[0] != 5 || m[1] != 0 {
		return fmt.Errorf("greeting refused: %02x %02x", m[0], m[1])
	}
	return nil
}

// socks5SkipReply reads and validates the CONNECT reply head and skips
// the bound address.
func socks5SkipReply(conn net.Conn) error {
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return err
	}
	if head[0] != 5 {
		return fmt.Errorf("reply version %02x", head[0])
	}
	if head[1] != 0 {
		return fmt.Errorf("door refused: REP %02x", head[1])
	}
	var skip int
	switch head[3] {
	case 1:
		skip = 4 + 2
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return err
		}
		skip = int(n[0]) + 2
	case 4:
		skip = 16 + 2
	default:
		return fmt.Errorf("reply ATYP %02x", head[3])
	}
	if skip > 0 {
		if _, err := io.ReadFull(conn, make([]byte, skip)); err != nil {
			return err
		}
	}
	return nil
}
