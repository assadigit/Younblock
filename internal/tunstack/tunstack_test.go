// tunstack_test.go — host-runnable tests for the userspace stack (docs/
// android-port 06 §3 unit tests + the host half of the M4 integration
// list; ADR-0008/0009/0010).
//
// The through-path test builds the whole chain with two netstacks
// joined by a packet-preserving in-memory wire (the synthetic TUN):
//
//      phone netstack (gonet TCP/UDP client) → wire → tunstack
//        → SOCKS5 door (the real internal/proxy Server) → test resolver
//        → netx dialer → local echo origin
//
// That is the device traffic path minus the kernel TUN fd and the
// real DoH — both replaced by in-memory equivalents, nothing else
// stubbed. The on-device gates (real DoH, QUIC fallback, protect())
// stay M4 device measurements (doc 09's standing blocker).
package tunstack

import (
        "context"
        "encoding/binary"
        "errors"
        "io"
        "net"
        "strconv"
        "sync"
        "testing"
        "time"

        "gvisor.dev/gvisor/pkg/tcpip"
        "gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
        "gvisor.dev/gvisor/pkg/tcpip/header"
        "gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
        "gvisor.dev/gvisor/pkg/tcpip/stack"
        "gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
        "gvisor.dev/gvisor/pkg/tcpip/transport/udp"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/fragment"
        "github.com/assadigit/Younblock/internal/netx"
        "github.com/assadigit/Younblock/internal/proxy"
        "github.com/assadigit/Younblock/internal/stats"
)

// TestHandshakeTimeoutDefaultCoversDoorBudget pins the M8-n invariant: the
// stack's default handshake budget must cover the SOCKS5 door's own honest
// setup budget (DoH resolve: DoHTimeout per server × the default server
// count, plus the upstream dial's DialTimeoutOverall) and must stay in
// lockstep with config.TunHandshakeTimeout, which the production facade
// passes. A value shorter than the door's budget aborts flows mid-setup on
// Android while the same flows succeed on the desktop doors — the
// first-video defect this test keeps from coming back.
func TestHandshakeTimeoutDefaultCoversDoorBudget(t *testing.T) {
        wireA, wireB := crossedWires()
        defer wireA.Close()
        defer wireB.Close()
        s, err := New(Config{
                Wire:      wireA,
                ProxyAddr: "127.0.0.1:1", // never dialed in this test
        })
        if err != nil {
                t.Fatalf("New: %v", err)
        }
        defer s.Stop()

        if s.cfg.HandshakeTimeout != config.TunHandshakeTimeout {
                t.Errorf("default HandshakeTimeout = %v, want config.TunHandshakeTimeout %v",
                        s.cfg.HandshakeTimeout, config.TunHandshakeTimeout)
        }
        // The budget must cover the door's worst-case honest setup: every DoH
        // server tried at its per-server timeout, then the full dial walk.
        doorBudget := config.DoHTimeout*time.Duration(len(config.Defaults())) + config.DialTimeoutOverall
        if s.cfg.HandshakeTimeout <= doorBudget {
                t.Errorf("default HandshakeTimeout %v does not cover the door budget %v "+
                        "(DoH %d×%v + dial %v)", s.cfg.HandshakeTimeout, doorBudget,
                        len(config.Defaults()), config.DoHTimeout, config.DialTimeoutOverall)
        }
}

// ---------------------------------------------------------------- wire

// packetChan is one direction of an in-memory packet wire: one packet
// per send/recv, boundaries preserved (net.Pipe is a stream — it would
// smear packets; a TUN never does).
type packetChan struct {
        ch     chan []byte
        closed chan struct{}
        once   sync.Once
}

func newPacketChan() *packetChan {
        return &packetChan{ch: make(chan []byte, 256), closed: make(chan struct{})}
}

func (c *packetChan) send(b []byte) error {
        pkt := append([]byte(nil), b...)
        select {
        case c.ch <- pkt:
                return nil
        case <-c.closed:
                return net.ErrClosed
        }
}

func (c *packetChan) recv(buf []byte) (int, error) {
        select {
        case pkt := <-c.ch:
                return copy(buf, pkt), nil
        case <-c.closed:
                return 0, io.EOF
        }
}

func (c *packetChan) close() {
        c.once.Do(func() { close(c.closed) })
}

// xWire is one end of the crossed pair: reads in, writes out.
type xWire struct {
        in, out *packetChan
}

func (w *xWire) Read(b []byte) (int, error) { return w.in.recv(b) }
func (w *xWire) Write(b []byte) (int, error) {
        if err := w.out.send(b); err != nil {
                return 0, err
        }
        return len(b), nil
}
func (w *xWire) Close() error {
        w.in.close()
        w.out.close()
        return nil
}

// crossedWires returns an io.ReadWriteCloser pair with packet
// semantics: what one side writes, the other reads, one packet at a
// time.
func crossedWires() (a, b *xWire) {
        ab := newPacketChan()
        ba := newPacketChan()
        return &xWire{in: ab, out: ba}, &xWire{in: ba, out: ab}
}

// ---------------------------------------------------------------- door

// recordingResolver answers every host with the origin address and
// remembers the names it was asked for (the domain-arrival assertion).
type recordingResolver struct {
        mu     sync.Mutex
        asked  []string
        origin net.IP
}

func (r *recordingResolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
        r.mu.Lock()
        r.asked = append(r.asked, host)
        r.mu.Unlock()
        return []net.IP{r.origin}, nil
}

func (r *recordingResolver) names() []string {
        r.mu.Lock()
        defer r.mu.Unlock()
        return append([]string(nil), r.asked...)
}

// startEcho starts a TCP echo origin listening on every interface, so
// both the loopback resolver path and the literal-IP path can reach it.
func startEcho(t *testing.T) *net.TCPAddr {
        t.Helper()
        ln, err := net.Listen("tcp", "0.0.0.0:0")
        if err != nil {
                t.Fatalf("echo listen: %v", err)
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

// echoPort extracts the echo origin's port (nil-guarded).
func echoPort(t *testing.T, echo *net.TCPAddr) int {
        t.Helper()
        if echo == nil {
                t.Fatal("no echo origin")
        }
        return echo.Port
}

// hostIP returns a non-loopback local IPv4 (for the literal-IP path; a
// loopback literal would be martian-dropped at the TUN stack — the same
// reason real apps' loopback traffic never enters a VPN).
func hostIP(t *testing.T) net.IP {
        t.Helper()
        addrs, err := net.InterfaceAddrs()
        if err != nil {
                t.Fatalf("interface addrs: %v", err)
        }
        for _, a := range addrs {
                if ipn, ok := a.(*net.IPNet); ok {
                        if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLoopback() {
                                return ip4
                        }
                }
        }
        t.Skip("no non-loopback local IPv4 — cannot exercise the literal path")
        return nil
}

// testDoor builds a real proxy Server (the same construction the door's
// own tests use) with the SOCKS5 door open and a resolver pointing at
// the echo origin.
func testDoor(t *testing.T) (doorAddr string, resolver *recordingResolver, origin *net.TCPAddr) {
        t.Helper()
        echo := startEcho(t)
        res := &recordingResolver{origin: net.ParseIP("127.0.0.1")}
        srv, err := proxy.Start(proxy.Config{
                Host:               "127.0.0.1",
                Port:               0,
                AutoPort:           true,
                MaxConns:           64,
                RequestTimeout:     2 * time.Second,
                FirstRecordTimeout: 2 * time.Second,
                IdleTimeout:        5 * time.Second,
                MaxHeaderBytes:     config.MaxHeaderBytes,
                Resolver:           res,
                Dialer:             netx.New(2*time.Second, 4*time.Second, nil),
                FragmentPreset:     fragment.Balanced,
                FragmentConfig:     fragment.Config{Min: 80, Max: 200, Pause: 0},
                RNG:                fragment.NewDefaultRNG(),
                Stats:              stats.New(),
                Domains:            config.DefaultDomains(),
                Mode:               config.ModeYouTube,
        })
        if err != nil {
                t.Fatalf("proxy.Start: %v", err)
        }
        t.Cleanup(func() { srv.StopWithTimeout() })
        if err := srv.StartSocks5("127.0.0.1", 0); err != nil {
                t.Fatalf("StartSocks5: %v", err)
        }
        t.Cleanup(srv.StopSocks5)
        return net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Socks5Port())), res, echo
}

// ---------------------------------------------------------------- phone

// phoneStack is the client-side netstack: an ordinary IPv4 host
// (10.111.0.2) whose only link is the crossed wire.
type phoneStack struct {
        ns   *stack.Stack
        wire *xWire // the phone's end of the crossed pair
}

func newPhone(t *testing.T, tunSideWire *xWire) *phoneStack {
        t.Helper()
        ns := stack.New(stack.Options{
                NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
                TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
        })
        phoneWire := tunSideWire // crossed: phone reads what tunstack writes
        ep := newWireEndpoint(phoneWire, 1500, nil)
        const nicID = 1
        if err := ns.CreateNIC(nicID, ep); err != nil {
                t.Fatalf("phone NIC: %s", err)
        }
        if err := ns.AddProtocolAddress(nicID,
                tcpip.ProtocolAddress{
                        Protocol: ipv4.ProtocolNumber,
                        AddressWithPrefix: tcpip.AddressWithPrefix{
                                Address:   tcpip.AddrFrom4([4]byte{10, 111, 0, 2}),
                                PrefixLen: 32,
                        },
                },
                stack.AddressProperties{PEB: stack.CanBePrimaryEndpoint},
        ); err != nil {
                t.Fatalf("phone address: %s", err)
        }
        ns.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicID}})
        t.Cleanup(func() { ns.Close() })
        return &phoneStack{ns: ns, wire: phoneWire}
}

// dialTCP dials from the phone through the wire (deadline-bounded).
func (p *phoneStack) dialTCP(t *testing.T, ip net.IP, port uint16) *gonet.TCPConn {
        t.Helper()
        ip4 := ip.To4()
        if ip4 == nil {
                t.Fatalf("dialTCP: %s is not IPv4", ip)
        }
        ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
        defer cancel()
        conn, err := gonet.DialTCPWithBind(ctx, p.ns, tcpip.FullAddress{},
                tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}), Port: port},
                ipv4.ProtocolNumber)
        if err != nil {
                t.Fatalf("phone dial %s:%d: %v", ip, port, err)
        }
        t.Cleanup(func() { conn.Close() })
        return conn
}

// dialUDP dials a UDP endpoint from the phone.
func (p *phoneStack) dialUDP(t *testing.T, ip net.IP, port uint16) *gonet.UDPConn {
        t.Helper()
        ip4 := ip.To4()
        if ip4 == nil {
                t.Fatalf("dialUDP: %s is not IPv4", ip)
        }
        conn, err := gonet.DialUDP(p.ns, nil,
                &tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}), Port: port},
                ipv4.ProtocolNumber)
        if err != nil {
                t.Fatalf("phone udp dial: %v", err)
        }
        t.Cleanup(func() { conn.Close() })
        return conn
}

// newTun builds the tunstack under test around the other wire end.
func newTun(t *testing.T, door string, mutate func(*Config)) (*Stack, *xWire) {
        t.Helper()
        tunWire, phoneSide := crossedWires()
        cfg := Config{Wire: tunWire, MTU: 1500, ProxyAddr: door}
        if mutate != nil {
                mutate(&cfg)
        }
        s, err := New(cfg)
        if err != nil {
                t.Fatalf("tunstack.New: %v", err)
        }
        t.Cleanup(s.Stop)
        return s, phoneSide
}

// ---------------------------------------------------------------- DNS unit tests

// dnsQuery builds a wire-format query: header + one question.
func dnsQuery(id uint16, name string, qtype uint16) []byte {
        q := make([]byte, 0, 12+len(name)+6)
        var hdr [12]byte
        binary.BigEndian.PutUint16(hdr[0:2], id)
        binary.BigEndian.PutUint16(hdr[2:4], 0x0100) // RD
        binary.BigEndian.PutUint16(hdr[4:6], 1)      // QD
        q = append(q, hdr[:]...)
        for _, label := range splitLabels(name) {
                q = append(q, byte(len(label)))
                q = append(q, label...)
        }
        q = append(q, 0) // root
        q = append(q, byte(qtype>>8), byte(qtype))
        q = append(q, 0x00, 0x01) // IN
        return q
}

func splitLabels(name string) []string {
        var out []string
        start := 0
        for i := 0; i <= len(name); i++ {
                if i == len(name) || name[i] == '.' {
                        if i > start {
                                out = append(out, name[start:i])
                        }
                        start = i + 1
                }
        }
        return out
}

func TestDNSResponderPolicy(t *testing.T) {
        m := newDNSMapper(64)

        // A → one fake-pool answer.
        resp, ok := respondDNS(m, dnsQuery(0xBEEF, "www.youtube.com", qtypeA))
        if !ok {
                t.Fatal("A query dropped")
        }
        if got := binary.BigEndian.Uint16(resp[0:2]); got != 0xBEEF {
                t.Fatalf("id = %#x", got)
        }
        if resp[2]&0x80 == 0 {
                t.Fatal("QR not set")
        }
        if rcode := binary.BigEndian.Uint16(resp[2:4]) & 0xF; rcode != 0 {
                t.Fatalf("rcode = %d", rcode)
        }
        if an := binary.BigEndian.Uint16(resp[6:8]); an != 1 {
                t.Fatalf("ANCOUNT = %d, want 1", an)
        }
        // The answer RDATA sits at the tail: ptr+type+class+ttl+len (16) + 4.
        ip := net.IP(resp[len(resp)-4:])
        if !fakeIPReports(ip) {
                t.Fatalf("answer %s outside the fake pool", ip)
        }
        // Reverse-mapped back.
        if dom, ok := m.domainFor(ip); !ok || dom != "www.youtube.com" {
                t.Fatalf("reverse = %q %v", dom, ok)
        }

        // Same name → same address (stable mapping).
        resp2, _ := respondDNS(m, dnsQuery(0x0001, "WwW.YoUtUbE.CoM", qtypeA))
        ip2 := net.IP(resp2[len(resp2)-4:])
        if !ip.Equal(ip2) {
                t.Fatalf("unstable mapping: %s vs %s", ip, ip2)
        }

        // AAAA → NOERROR, zero answers (apps stay IPv4; ADR-0009).
        resp, ok = respondDNS(m, dnsQuery(0xCAFE, "www.youtube.com", qtypeAAAA))
        if !ok {
                t.Fatal("AAAA query dropped")
        }
        if an := binary.BigEndian.Uint16(resp[6:8]); an != 0 {
                t.Fatalf("AAAA ANCOUNT = %d, want 0", an)
        }
        if rcode := binary.BigEndian.Uint16(resp[2:4]) & 0xF; rcode != 0 {
                t.Fatalf("AAAA rcode = %d", rcode)
        }

        // Other types (HTTPS/MX) → NOERROR, zero answers.
        for _, qt := range []uint16{65, 15, 16, 33} {
                resp, _ = respondDNS(m, dnsQuery(0x0102, "www.youtube.com", qt))
                if an := binary.BigEndian.Uint16(resp[6:8]); an != 0 {
                        t.Fatalf("qtype %d ANCOUNT = %d, want 0", qt, an)
                }
        }

        // Truncated question → FORMERR (header echoed, QD=0).
        bad := dnsQuery(0x0103, "www.youtube.com", qtypeA)
        resp, ok = respondDNS(m, bad[:len(bad)-3])
        if !ok {
                t.Fatal("truncated query dropped")
        }
        if rcode := binary.BigEndian.Uint16(resp[2:4]) & 0xF; rcode != rcodeFormErr {
                t.Fatalf("truncated rcode = %d, want FORMERR", rcode)
        }

        // Sub-header garbage → dropped silently.
        if _, ok = respondDNS(m, []byte{1, 2, 3}); ok {
                t.Fatal("garbage answered, want drop")
        }

        // Opcode ≠ QUERY → NOTIMP.
        n := dnsQuery(0x0104, "x.test", qtypeA)
        n[2] |= 0x08 // opcode 1
        resp, _ = respondDNS(m, n)
        if rcode := binary.BigEndian.Uint16(resp[2:4]) & 0xF; rcode != rcodeNotImp {
                t.Fatalf("opcode rcode = %d, want NOTIMP", rcode)
        }

        // QDCOUNT 0 → FORMERR.
        z := dnsQuery(0x0105, "x.test", qtypeA)
        binary.BigEndian.PutUint16(z[4:6], 0)
        resp, _ = respondDNS(m, z)
        if rcode := binary.BigEndian.Uint16(resp[2:4]) & 0xF; rcode != rcodeFormErr {
                t.Fatalf("qd0 rcode = %d, want FORMERR", rcode)
        }
}

func TestDNSMapperEviction(t *testing.T) {
        m := newDNSMapper(2)
        a := m.fakeFor("a.test")
        b := m.fakeFor("b.test")
        if got := len(m.fwd); got != 2 {
                t.Fatalf("size = %d", got)
        }
        c := m.fakeFor("c.test") // evicts a.test (FIFO)
        if _, ok := m.domainFor(a); ok {
                t.Fatal("a.test still mapped after eviction")
        }
        for _, x := range []net.IP{b, c} {
                if _, ok := m.domainFor(x); !ok {
                        t.Fatalf("%s lost its mapping", x)
                }
        }
        // Distinct domains get distinct addresses.
        if b.Equal(c) {
                t.Fatal("duplicate addresses")
        }
        // Re-asking an evicted name works (fresh allocation).
        if ip := m.fakeFor("a.test"); ip.Equal(b) && ip.Equal(c) {
                t.Fatal("no fresh address after eviction")
        }
}

// ---------------------------------------------------------------- through-path

func TestTCPThroughPathDomainFlows(t *testing.T) {
        door, resolver, echo := testDoor(t)
        tun, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // Step 1: resolve like the routed app would — DNS through the stack.
        dns := phone.dialUDP(t, net.ParseIP("10.111.0.53"), 53)
        q := dnsQuery(0x1234, "www.youtube.com", qtypeA)
        if _, err := dns.Write(q); err != nil {
                t.Fatalf("dns write: %v", err)
        }
        buf := make([]byte, 1500)
        _ = dns.SetReadDeadline(time.Now().Add(3 * time.Second))
        n, err := dns.Read(buf)
        if err != nil {
                t.Fatalf("dns read: %v", err)
        }
        ans := net.IP(buf[n-4 : n])
        if !fakeIPReports(ans) {
                t.Fatalf("dns answer %s not a fake address", ans)
        }
        t.Logf("www.youtube.com → %s", ans)

        // Step 2: the app connects to the fake address; the flow must reach
        // the door AS THE DOMAIN (ATYP 0x03), so the door's resolver sees
        // www.youtube.com (and its MatchAnyDomain rule applies there). The
        // app dials the origin's port — the door dials the resolved address
        // at whatever port the app asked for.
        conn := phone.dialTCP(t, ans, uint16(echoPort(t, echo)))
        payload := []byte("hello through the tun")
        if _, err := conn.Write(payload); err != nil {
                t.Fatalf("write: %v", err)
        }
        got := make([]byte, len(payload))
        _ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("read echo: %v", err)
        }
        if string(got) != string(payload) {
                t.Fatalf("echo = %q", got)
        }

        names := resolver.names()
        if len(names) == 0 || names[0] != "www.youtube.com" {
                t.Fatalf("door asked for %v, want [www.youtube.com] (domain must cross the door)", names)
        }
        st := tun.Stats()
        if st.DNSQueries != 1 || st.FlowsOpened < 1 {
                t.Fatalf("stats = %+v", st)
        }
}

func TestTCPThroughPathLiteralIP(t *testing.T) {
        door, resolver, echo := testDoor(t)
        _, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // A literal IP destination: crosses as IPv4 ATYP, the door skips
        // DNS (literal), dials the literal directly. The literal is the
        // host's own non-loopback address — the only literal both stacks
        // and the door can actually reach from here.
        literal := hostIP(t)
        conn := phone.dialTCP(t, literal, uint16(echo.Port))
        payload := []byte("literal destination")
        if _, err := conn.Write(payload); err != nil {
                t.Fatalf("write: %v", err)
        }
        got := make([]byte, len(payload))
        _ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        if _, err := io.ReadFull(conn, got); err != nil {
                t.Fatalf("read: %v", err)
        }
        if string(got) != string(payload) {
                t.Fatalf("echo = %q", got)
        }
        // The literal path must NOT have asked any resolver.
        if names := resolver.names(); len(names) != 0 {
                t.Fatalf("resolver asked for %v on a literal flow", names)
        }
}

func TestUnmappedFakeAddressRST(t *testing.T) {
        door, _, _ := testDoor(t)
        _, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // A fake-pool address nobody mapped: fail-fast RST, no doomed dial.
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _, err := gonet.DialTCPWithBind(ctx, phone.ns, tcpip.FullAddress{},
                tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{198, 18, 9, 9}), Port: 443},
                ipv4.ProtocolNumber)
        if err == nil {
                t.Fatal("dial to unmapped fake address succeeded")
        }
}

func TestUDPNon53Dropped(t *testing.T) {
        door, _, _ := testDoor(t)
        tun, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // QUIC-shaped flow: UDP to 443 — never forwarded, never answered.
        quic := phone.dialUDP(t, net.ParseIP("8.8.8.8"), 443)
        if _, err := quic.Write([]byte("QUIC-ish datagram")); err != nil {
                t.Fatalf("udp write: %v", err)
        }
        _ = quic.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
        buf := make([]byte, 1500)
        if n, err := quic.Read(buf); err == nil {
                t.Fatalf("udp 443 answered (%d bytes): must be dropped per ADR-0009", n)
        } else if !isTimeoutErr(err) {
                t.Fatalf("udp read err = %v, want timeout", err)
        }
        if got := tun.Stats().UDPDropped; got < 1 {
                t.Fatalf("UDPDropped = %d", got)
        }
}

func TestDNSOverUDPThroughStack(t *testing.T) {
        door, _, _ := testDoor(t)
        _, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        dns := phone.dialUDP(t, net.ParseIP("10.111.0.53"), 53)
        for _, tc := range []struct {
                name   string
                qtype  uint16
                wantAN int
        }{
                {"example.test", qtypeA, 1},
                {"example.test", qtypeAAAA, 0},
                {"example.test", 65 /* HTTPS */, 0},
        } {
                if _, err := dns.Write(dnsQuery(0x0A0A, tc.name, tc.qtype)); err != nil {
                        t.Fatalf("write %s: %v", tc.name, err)
                }
                buf := make([]byte, 1500)
                _ = dns.SetReadDeadline(time.Now().Add(3 * time.Second))
                if _, err := dns.Read(buf); err != nil {
                        t.Fatalf("read %s: %v", tc.name, err)
                }
                if an := binary.BigEndian.Uint16(buf[6:8]); int(an) != tc.wantAN {
                        t.Fatalf("%s qtype %d: ANCOUNT = %d, want %d", tc.name, tc.qtype, an, tc.wantAN)
                }
                if rcode := binary.BigEndian.Uint16(buf[2:4]) & 0xF; rcode != 0 {
                        t.Fatalf("%s: rcode = %d", tc.name, rcode)
                }
        }

        // Sub-header garbage: dropped, never answered.
        if _, err := dns.Write([]byte{1, 2}); err != nil {
                t.Fatalf("garbage write: %v", err)
        }
        _ = dns.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
        if n, err := dns.Read(make([]byte, 1500)); err == nil {
                t.Fatalf("garbage answered (%d bytes)", n)
        } else if !isTimeoutErr(err) {
                t.Fatalf("garbage read err = %v, want timeout", err)
        }
}

func TestIPv6PacketsDropped(t *testing.T) {
        door, _, _ := testDoor(t)
        tun, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // A raw IPv6 packet injected as if the phone sent it: the wire drops
        // it before netstack ever sees a v6 NIC (ADR-0010's in-stack
        // blackhole). Nothing may crash, nothing may come back.
        v6 := make([]byte, 40)
        v6[0] = 0x60 // version 6
        if _, err := phone.wire.Write(v6); err != nil {
                t.Fatalf("v6 write: %v", err)
        }
        time.Sleep(300 * time.Millisecond) // let any bug surface
        st := tun.Stats()
        if st.DNSQueries != 0 || st.FlowsOpened != 0 {
                t.Fatalf("v6 packet produced flows: %+v", st)
        }
}

func TestStopWhileActive(t *testing.T) {
        door, _, echo := testDoor(t)
        tun, phoneSide := newTun(t, door, nil)
        phone := newPhone(t, phoneSide)

        // Full flow up, echoing…
        dns := phone.dialUDP(t, net.ParseIP("10.111.0.53"), 53)
        if _, err := dns.Write(dnsQuery(0x0042, "stop.test", qtypeA)); err != nil {
                t.Fatalf("dns: %v", err)
        }
        buf := make([]byte, 1500)
        _ = dns.SetReadDeadline(time.Now().Add(3 * time.Second))
        n, err := dns.Read(buf)
        if err != nil {
                t.Fatalf("dns read: %v", err)
        }
        conn := phone.dialTCP(t, net.IP(buf[n-4:n]), uint16(echoPort(t, echo)))
        if _, err := conn.Write([]byte("before stop")); err != nil {
                t.Fatalf("write: %v", err)
        }
        _ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        if _, err := conn.Read(make([]byte, 64)); err != nil {
                t.Fatalf("echo read: %v", err)
        }

        // …then the stack is torn down mid-flight.
        tun.Stop()
        tun.Stop() // idempotent

        // What Stop must guarantee here: no traffic PATH survives. The phone
        // may drain what its own netstack already buffered before the cut,
        // and this harness — a netstack with no kernel behind it — has
        // nothing to reset the flow when the link dies (on a device the TUN
        // interface teardown is what resets routed sockets). So reads end in
        // a stall (timeout) or an error, either is fine; what must NOT
        // happen is new data crossing the dead stack.
        _ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
        for {
                _, err := conn.Read(make([]byte, 64))
                if err == nil {
                        continue // drain what was in flight; bounded by the deadline
                }
                break // stall or reset: no live path either way
        }

        // The definitive check: a write after Stop must never echo back.
        if _, werr := conn.Write([]byte("after stop")); werr == nil {
                _ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
                if n, rerr := conn.Read(make([]byte, 64)); rerr == nil && n > 0 {
                        t.Fatalf("data crossed the stack after Stop (%d bytes)", n)
                }
        }
}

func TestFlowCapRSTOverCap(t *testing.T) {
        door, _, echo := testDoor(t)
        tun, phoneSide := newTun(t, door, func(c *Config) { c.MaxFlows = 1 })
        phone := newPhone(t, phoneSide)

        // Two mapped names, so the cap — not the unmapped-RST — is what
        // refuses the second flow.
        dns := phone.dialUDP(t, net.ParseIP("10.111.0.53"), 53)
        ask := func(name string) net.IP {
                if _, err := dns.Write(dnsQuery(0x0043, name, qtypeA)); err != nil {
                        t.Fatalf("dns: %v", err)
                }
                buf := make([]byte, 1500)
                _ = dns.SetReadDeadline(time.Now().Add(3 * time.Second))
                n, err := dns.Read(buf)
                if err != nil {
                        t.Fatalf("dns read: %v", err)
                }
                return net.IP(buf[n-4 : n])
        }
        firstIP := ask("cap.test")
        secondIP := ask("cap2.test")

        // Flow 1: holds the only slot (kept open by the blocked read).
        first := phone.dialTCP(t, firstIP, uint16(echoPort(t, echo)))
        if _, err := first.Write([]byte("hold the slot")); err != nil {
                t.Fatalf("first write: %v", err)
        }

        // Flow 2: over cap → refused (RST), counted.
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _, err := gonet.DialTCPWithBind(ctx, phone.ns, tcpip.FullAddress{},
                tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{secondIP[0], secondIP[1], secondIP[2], secondIP[3]}), Port: 443},
                ipv4.ProtocolNumber)
        if err == nil {
                t.Fatal("second flow over cap was accepted")
        }
        if got := tun.Stats().FlowsOverCap; got < 1 {
                t.Fatalf("FlowsOverCap = %d, want ≥ 1: %+v", got, tun.Stats())
        }
}

// isTimeoutErr matches deadline/timeout failures across gonet (which
// wraps them in *net.OpError with Timeout()==true) and net.
func isTimeoutErr(err error) bool {
        var ne net.Error
        if errors.As(err, &ne) && ne.Timeout() {
                return true
        }
        return errors.Is(err, context.DeadlineExceeded)
}
