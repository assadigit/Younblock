// Package tunstack is the userspace packet/stream adapter (docs/android-port
// 04 §2.5/§7, ADR-0008/0009/0010; roadmap M4): a gVisor netstack fed by
// the TUN wire that turns IP packets back into TCP flows and hands each
// flow to the SOCKS5 front door — the same tunnel path the browser door
// uses (resolve → netx dial → first-record fragmentation → copy).
//
// ADR-0008's measured selection landed on gVisor-direct (D75): the
// tun2socks engine package could not express this product's policy
// through a CONNECT-only door (its UDP model demands a UDP-capable
// proxy, and its global-singleton lifecycle conflicts with the engine's
// restart cycles). The forwarder wiring here follows the same
// production-proven pattern tun2socks' core uses — handlers before NIC,
// spoofing+promiscuous so flows to any destination are accepted.
//
// Policy inside the tunnel (ADR-0009), enforced here:
//
//	TCP   → every flow enters the SOCKS5 door. Fake-pool destinations
//	        are reversed to their DOMAIN (ATYP 0x03) so the door's
//	        MatchAnyDomain fragmentation decision matches the browser
//	        path; literal-IP destinations cross as IPv4 ATYP 0x01
//	        (honest: no domain, no fragmentation); unmapped fake
//	        addresses are RST immediately (fail-fast, no doomed dial).
//	UDP53 → the virtual DNS responder (dns.go): A=fake, AAAA empty,
//	        other types empty, malformed FORMERR/drop.
//	UDP*  → dropped (not forwarded): return false lets the stack send
//	        ICMP port-unreachable where it would — the fail-fast form
//	        of drop; routed apps fall back to TCP (the QUIC gate is a
//	        device measurement, M4 on-device).
//	IPv6  → never enabled on this stack: v6 packets die at the NIC
//	        (ADR-0010's fail-safe; the service still routes ::/0 into
//	        the TUN so v6 cannot bypass it).
//
// Loop safety (ADR-0011) is unchanged by this package: the engine's
// upstream dials go through the one netx dialer (protect()ed); this
// stack's ONLY outbound socket is the loopback dial to the door, which
// never enters the TUN.
package tunstack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// Logger is the narrow logging interface (satisfiable by *slog.Logger).
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Config wires the stack to its dependencies. Wire and ProxyAddr are
// required; everything else has defaults.
type Config struct {
	// Wire is the TUN stream: one raw IP packet per Read/Write. On
	// Android the facade passes os.NewFile(fd, "tun").
	Wire io.ReadWriteCloser
	// MTU of the TUN (0 → 1500). Must match what the service set on the
	// VpnService builder.
	MTU uint32
	// ProxyAddr is the SOCKS5 door, "127.0.0.1:port" (loopback-only by
	// construction, proxy.Server.StartSocks5).
	ProxyAddr string
	// DNSPort is the destination port the virtual DNS answers on
	// (default 53).
	DNSPort uint16
	// MaxFlows caps concurrent TCP flows before the door is even asked
	// (default 1024, matching the door's own cap; over-cap SYN → RST).
	MaxFlows int
	// DNSMaxDomains bounds the fake-IP table (default 4096).
	DNSMaxDomains int
	// HandshakeTimeout bounds the door dial + SOCKS5 exchange.
	// It must cover the door's own honest setup budget — the DoH
	// resolve plus the upstream dial the door completes before
	// replying — or flows that the desktop doors deliver get
	// aborted here mid-setup (the M8-n defect). Default
	// defaultHandshakeTimeout, kept in lockstep with
	// config.TunHandshakeTimeout (the test pins the equality).
	HandshakeTimeout time.Duration
	// Logger may be nil.
	Logger Logger
}

// Stats is a point-in-time snapshot of the stack counters (diagnostics;
// no user-facing claims ride on these — ADR-0007).
type Stats struct {
	FlowsOpened      int64
	FlowsRSTUnmapped int64
	FlowsOverCap     int64
	DoorFailures     int64
	DNSQueries       int64
	UDPDropped       int64
}

// defaultHandshakeTimeout is the stack's backstop for a caller that
// omits HandshakeTimeout: the SOCKS5 door's full honest setup budget
// (DoH resolve: DoHTimeout × servers ≈ 9 s; upstream dial:
// DialTimeoutOverall 12 s) plus slack — the same derivation as
// config.TunHandshakeTimeout, which the production facade passes
// explicitly. TestHandshakeTimeoutDefaultCoversDoorBudget pins the
// lockstep with the config constant.
const defaultHandshakeTimeout = 30 * time.Second

// Stack is one TUN-fed netstack. Construct, use, Stop; not restartable
// (the facade builds a fresh one per engine cycle, mirroring how the
// proxy server itself is per-cycle).
type Stack struct {
	cfg     Config
	ns      *stack.Stack
	wire    *wireEndpoint
	dns     *dnsMapper
	stopped atomic.Bool

	// flowSem caps in-flight TCP flows; nil never (default installed).
	flowSem chan struct{}
	flows   int64 // atomic

	flowsRST int64
	overCap  int64
	doorFail int64
	dnsCount int64
	udpDrops int64
}

// New builds the stack around the wire. Packets flow as soon as the
// NIC is created (the wire pump starts on attach); call Stop to unwind.
func New(cfg Config) (*Stack, error) {
	if cfg.Wire == nil {
		return nil, errors.New("tunstack: Wire required")
	}
	if cfg.ProxyAddr == "" {
		return nil, errors.New("tunstack: ProxyAddr required")
	}
	if cfg.MTU == 0 {
		cfg.MTU = 1500
	}
	if cfg.DNSPort == 0 {
		cfg.DNSPort = 53
	}
	if cfg.MaxFlows <= 0 {
		cfg.MaxFlows = 1024
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = defaultHandshakeTimeout
	}

	s := &Stack{
		cfg:     cfg,
		dns:     newDNSMapper(cfg.DNSMaxDomains),
		flowSem: make(chan struct{}, cfg.MaxFlows),
	}

	ns := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})
	s.ns = ns
	s.wire = newWireEndpoint(cfg.Wire, cfg.MTU)

	// Transport handlers BEFORE the NIC (see package doc).
	tcpFwd := tcp.NewForwarder(ns, 0 /* default rcv wnd */, cfg.MaxFlows, s.handleTCP)
	ns.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)
	udpFwd := udp.NewForwarder(ns, s.handleUDP)
	ns.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)

	const nicID = 1
	if err := ns.CreateNIC(nicID, s.wire); err != nil {
		s.wire.Close()
		return nil, fmt.Errorf("tunstack: create NIC: %s", err)
	}
	// Accept flows to ANY destination and let endpoints bind any source
	// (the TUN's whole job); one primary address exists so netstack can
	// build routes (tun2socks' multicast note — the address is a route
	// anchor, not a claim about the network).
	if err := ns.SetPromiscuousMode(nicID, true); err != nil {
		s.wire.Close()
		return nil, fmt.Errorf("tunstack: promiscuous: %s", err)
	}
	if err := ns.SetSpoofing(nicID, true); err != nil {
		s.wire.Close()
		return nil, fmt.Errorf("tunstack: spoofing: %s", err)
	}
	if err := ns.AddProtocolAddress(nicID,
		tcpip.ProtocolAddress{
			Protocol: ipv4.ProtocolNumber,
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   tcpip.AddrFrom4([4]byte{10, 111, 0, 1}),
				PrefixLen: 32,
			},
		},
		stack.AddressProperties{PEB: stack.CanBePrimaryEndpoint},
	); err != nil {
		s.wire.Close()
		return nil, fmt.Errorf("tunstack: address: %s", err)
	}
	ns.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicID},
	})

	if cfg.Logger != nil {
		cfg.Logger.Info("tunstack up", "mtu", cfg.MTU, "door", cfg.ProxyAddr)
	}
	return s, nil
}

// Stats returns a snapshot of the counters.
func (s *Stack) Stats() Stats {
	return Stats{
		FlowsOpened:      atomic.LoadInt64(&s.flows),
		FlowsRSTUnmapped: atomic.LoadInt64(&s.flowsRST),
		FlowsOverCap:     atomic.LoadInt64(&s.overCap),
		DoorFailures:     atomic.LoadInt64(&s.doorFail),
		DNSQueries:       atomic.LoadInt64(&s.dnsCount),
		UDPDropped:       atomic.LoadInt64(&s.udpDrops),
	}
}

// Stop unwinds the stack: closes the NICs (which fails every live flow
// into its goroutine's copy loop), stops the wire pumps and releases
// the fd. Idempotent; safe from any goroutine.
func (s *Stack) Stop() {
	if !s.stopped.CompareAndSwap(false, true) {
		return
	}
	s.ns.Close() // endpoints fail; flow goroutines see EOF/err and exit
	s.wire.Close()
	s.wire.wait()
	if s.cfg.Logger != nil {
		s.cfg.Logger.Info("tunstack stopped", "stats", fmt.Sprintf("%+v", s.Stats()))
	}
}

// handleTCP receives every otherwise-unclaimed IPv4 TCP SYN flow.
func (s *Stack) handleTCP(r *tcp.ForwarderRequest) {
	id := r.ID()
	dst := tcpIPToNet(id.LocalAddress)
	dstPort := int(id.LocalPort)

	// Flow cap before any endpoint exists (RST is the honest answer to a
	// flow we refuse to carry).
	select {
	case s.flowSem <- struct{}{}:
	default:
		atomic.AddInt64(&s.overCap, 1)
		r.Complete(true)
		return
	}

	// Where is this flow going? Fake-pool destinations reverse to their
	// domain (the door then resolves via DoH and applies the same
	// fragmentation rules as the browser path); literal destinations
	// cross as-is; unmapped fakes are RST immediately.
	target := dst.String()
	if fakeIPReports(dst) {
		domain, ok := s.dns.domainFor(dst)
		if !ok {
			atomic.AddInt64(&s.flowsRST, 1)
			<-s.flowSem
			r.Complete(true)
			return
		}
		target = domain
	}

	var wq waiter.Queue
	ep, terr := r.CreateEndpoint(&wq)
	if terr != nil {
		<-s.flowSem
		r.Complete(true)
		return
	}
	r.Complete(false)
	ep.SocketOptions().SetKeepAlive(true)
	idle := tcpip.KeepaliveIdleOption(60 * time.Second)
	_ = ep.SetSockOpt(&idle)
	interval := tcpip.KeepaliveIntervalOption(30 * time.Second)
	_ = ep.SetSockOpt(&interval)
	_ = ep.SetSockOptInt(tcpip.KeepaliveCountOption, 9)

	atomic.AddInt64(&s.flows, 1)
	go s.pumpFlow(gonet.NewTCPConn(&wq, ep), target, dstPort)
}

// pumpFlow carries one established phone-side flow to the door.
func (s *Stack) pumpFlow(phone *gonet.TCPConn, target string, port int) {
	defer atomic.AddInt64(&s.flows, -1)
	defer func() { _ = phone.Close() }()
	defer func() { <-s.flowSem }()

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.HandshakeTimeout)
	door, err := socks5Dial(ctx, s.cfg.ProxyAddr, target, port)
	cancel()
	if err != nil {
		atomic.AddInt64(&s.doorFail, 1)
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("tunstack: door dial failed", "reason", errKind(err))
		}
		return // phone.Close sends the fail-fast FIN/RST
	}
	defer func() { _ = door.Close() }()

	// Splice both ways with half-close propagation: EOF on one side
	// closes only that side's write direction, so HTTP-style
	// request/response turns and TLS close_notify both survive.
	done := make(chan struct{}, 2)
	go func() {
		copyFlow(door, phone)
		done <- struct{}{}
	}()
	go func() {
		copyFlow(phone, door)
		done <- struct{}{}
	}()
	<-done
	closeWrite(door)
	<-done
}

// copyFlow copies src→dst until EOF or error (idle handling is the
// door's job on its side; the phone side rides the door's timeouts).
func copyFlow(dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
}

// closeWrite half-closes w when it can (both *net.TCPConn and
// *gonet.TCPConn implement it); a full close is the fallback.
func closeWrite(w io.Writer) {
	if cw, ok := w.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	if c, ok := w.(io.Closer); ok {
		_ = c.Close()
	}
}

// handleUDP receives every otherwise-unclaimed IPv4 UDP flow: DNS to
// the configured port is answered by the virtual resolver; everything
// else is dropped (ADR-0009).
func (s *Stack) handleUDP(r *udp.ForwarderRequest) bool {
	if r.ID().LocalPort != s.cfg.DNSPort {
		atomic.AddInt64(&s.udpDrops, 1)
		return false // not forwarded; the stack may ICMP-unreachable it
	}
	var wq waiter.Queue
	ep, terr := r.CreateEndpoint(&wq)
	if terr != nil {
		return false
	}
	conn := gonet.NewUDPConn(&wq, ep)
	go s.serveDNS(conn)
	return true
}

// serveDNS answers queries on one phone-side UDP flow until it goes
// quiet (clients may pipeline a few queries per socket).
func (s *Stack) serveDNS(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return // timeout, closed or dead: flow over
		}
		atomic.AddInt64(&s.dnsCount, 1)
		if resp, ok := respondDNS(s.dns, buf[:n]); ok {
			if _, err := conn.Write(resp); err != nil {
				return
			}
		}
	}
}

// tcpIPToNet converts a tcpip.Address to a net.IP.
func tcpIPToNet(a tcpip.Address) net.IP {
	if a.Len() == 4 {
		return net.IP(append([]byte(nil), a.AsSlice()...)).To4()
	}
	return net.IP(append([]byte(nil), a.AsSlice()...))
}

// errKind reduces an error to a short, hostless reason (the same shape
// the proxy package logs).
func errKind(err error) string {
	if err == nil {
		return ""
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return "closed"
	}
	msg := err.Error()
	for i := 0; i < len(msg); i++ {
		if msg[i] == ':' {
			return msg[:i]
		}
	}
	return msg
}
