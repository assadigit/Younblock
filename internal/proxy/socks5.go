// socks5.go — the SOCKS5 front door (docs/android-port 04 §2.6,
// ADR-0008/0009; roadmap M3 task 3).
//
// The HTTP door (server.go) is what a browser speaks to. The TUN path
// needs something else: the userspace stack reassembles the phone's IP
// packets into TCP flows and hands each flow to a SOCKS5 server on
// loopback. This file adds that second door to the SAME relay: one
// Server, two listeners, one tunnels map, one Stop — the SOCKS door is
// started explicitly (StartSocks5) and stays off by default, so desktop
// behavior is unchanged byte-for-byte until an Android caller opens it.
//
// Protocol surface (deliberately minimal, RFC 1928):
//   - CONNECT only (BIND and UDP ASSOCIATE are refused with REP 0x07);
//   - no authentication (method 0x00 only — the door is loopback-only,
//     same trust boundary as the HTTP door, which has no auth either);
//   - address types IPv4 (0x01), domain (0x03) and IPv6 (0x04);
//   - everything after the handshake walks the exact HTTP-CONNECT path:
//     resolve through the DoH resolver (IP literals skip DNS), dial
//     through the one netx.Dialer (protected on Android), fragment the
//     first TLS record for routed domains, copy both ways with the idle
//     timeout. Fragmentation follows the mode rules minus the
//     never-processed TLDs — with the domain address type the stack's
//     fake-DNS lets the same routedHost decision the browser path makes
//     apply here too (.ir relays untouched in every mode — D83).
package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

// SOCKS5 wire constants (RFC 1928).
const (
	socks5Version = 0x05

	socks5MethodNoAuth          = 0x00
	socks5MethodNoneAcceptable  = 0xFF
	socks5CmdConnect            = 0x01
	socks5AtypIPv4              = 0x01
	socks5AtypDomain            = 0x03
	socks5AtypIPv6              = 0x04
	socks5RepSucceeded          = 0x00
	socks5RepGeneralFailure     = 0x01
	socks5RepHostUnreachable    = 0x04
	socks5RepCommandUnsupported = 0x07
	socks5RepAddressUnsupported = 0x08
)

// errSocks5Malformed marks a client that violated the protocol shape; it
// never reaches a client — it only selects "close quietly" inside the
// door.
var errSocks5Malformed = errors.New("socks5: malformed handshake")

// StartSocks5 opens the SOCKS5 front door on host:port. The door is
// loopback-only by construction (doc 04 §2.6): anything routable is
// refused before a socket exists. port 0 asks the kernel for a free
// port; a busy fixed port is reported as ErrPortBusy so the engine's
// existing error mapping surfaces PORT_BUSY. Calling it twice is a no-op.
func (s *Server) StartSocks5(host string, port int) error {
	if !isLoopbackHost(host) {
		return fmt.Errorf("proxy: the socks5 door is loopback-only; refusing to bind %q", host)
	}
	if port < 0 || port > 65535 {
		port = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.socksLn != nil {
		return nil // already open
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("%w: socks5 %s:%d (%v)", ErrPortBusy, host, port, err)
	}
	s.socksLn = ln
	s.socksPort = ln.Addr().(*net.TCPAddr).Port
	s.wg.Add(1)
	go s.socks5AcceptLoop(ln)
	if s.cfg.Logger != nil {
		s.cfg.Logger.Info("socks5 door listening", "host", host, "port", s.socksPort)
	}
	return nil
}

// StopSocks5 closes the SOCKS5 door (idempotent). The HTTP door and the
// tunnels are untouched; in-flight SOCKS tunnels run to their normal end.
func (s *Server) StopSocks5() {
	s.mu.Lock()
	ln := s.socksLn
	s.socksLn = nil
	s.socksPort = 0
	s.mu.Unlock()
	if ln != nil {
		ln.Close()
	}
}

// Socks5Port reports the door's port (0 when closed).
func (s *Server) Socks5Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.socksPort
}

// Socks5Addr reports the door's host:port ("" when closed).
func (s *Server) Socks5Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.socksLn == nil {
		return ""
	}
	return net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.socksPort))
}

// isLoopbackHost reports whether host is a loopback literal (or the
// reserved name "localhost", which the door also accepts for tests and
// tooling convenience — it still only ever binds loopback).
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// socks5AcceptLoop accepts connections until its listener closes; it
// mirrors acceptLoop's error handling (drives on through transient
// accept errors, exits on ErrClosed).
func (s *Server) socks5AcceptLoop(ln net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if s.cfg.Logger != nil {
				s.cfg.Logger.Warn("socks5 accept failed", "reason", errKind(err))
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		s.wg.Add(1)
		go s.serveSocks5(conn)
	}
}

// serveSocks5 serves one SOCKS5 connection with the same guards as the
// HTTP door's handle(): panic recovery (R2), the shared MaxConns cap
// (over-limit gets REP 0x01 instead of 503 — SOCKS has no status line),
// and the shared counters.
func (s *Server) serveSocks5(conn net.Conn) {
	defer s.wg.Done()
	s.stats.ConnOpened()
	defer s.stats.ConnClosed()
	defer func() {
		if p := recover(); p != nil {
			s.stats.Panic()
			if s.cfg.Logger != nil {
				s.cfg.Logger.Error("socks5 handler panicked and was recovered", "panic", fmt.Sprint(p))
			}
			conn.Close()
		}
	}()
	defer conn.Close()

	if atomic.AddInt32(&s.active, 1) > s.cfg.MaxConns {
		atomic.AddInt32(&s.active, -1)
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("socks5 connection limit reached; replying general failure", "limit", s.cfg.MaxConns)
		}
		s.socks5Reply(conn, socks5RepGeneralFailure)
		return
	}
	defer atomic.AddInt32(&s.active, -1)

	// The whole handshake is bounded by the request timeout, like the
	// HTTP header read.
	conn.SetReadDeadline(time.Now().Add(s.cfg.RequestTimeout))
	defer conn.SetReadDeadline(time.Time{})

	br := bufio.NewReader(conn)
	if err := s.socks5Greeting(conn, br); err != nil {
		return // malformed or unacceptable: the reply (if any) was sent
	}
	host, port, ok := s.socks5Request(conn, br)
	if !ok {
		return
	}

	// From here on this is the HTTP-CONNECT tunnel path, verbatim:
	// resolve (DoH; literals skip DNS) → dial (netx, protected) →
	// reply established → copy with the first-record rule.
	tDNS := time.Now()
	ips, err := s.resolve(host)
	dnsMS := time.Since(tDNS).Milliseconds()
	if err != nil {
		if s.cfg.Logger != nil {
			// The reason rides in the message (M8-y, the M8-r lesson the
			// HTTP door already learned: the log ring that feeds the
			// diagnostic report drops attrs — the debug.16 report showed
			// 50 bare "socks5 dns lookup failed" lines that could not say
			// why the door lookups failed).
			s.cfg.Logger.Warn("socks5 dns lookup failed reason=" + errKind(err))
		}
		s.socks5Reply(conn, socks5RepHostUnreachable)
		s.countConnError()
		return
	}
	ips = s.liveIPs(ips) // the penalty box prefers live origins (M8-y, ADR-0023)
	tDial := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
	upstream, err := s.cfg.Dialer.DialHost(ctx, "tcp", host, port, ips)
	cancel()
	dialMS := time.Since(tDial).Milliseconds()
	if err != nil {
		if s.cfg.Logger != nil {
			if s.cfg.LogHostnames {
				s.cfg.Logger.Warn("socks5 dial failed", "host", host, "reason", errKind(err))
			} else {
				s.cfg.Logger.Warn("socks5 dial failed", "reason", errKind(err))
			}
		}
		s.invalidateDNS(host) // M8-o: the whole set failed; re-query next time
		s.socks5Reply(conn, socks5RepGeneralFailure)
		s.countConnError()
		return
	}
	s.logTunnelUp(host, dnsMS, dialMS)

	s.socks5ReplyAddr(conn, upstream.LocalAddr())

	// The one routed-host decision, shared with the HTTP door: the
	// mode/list rule minus the never-processed TLDs — a .ir host relays
	// untouched whatever the mode or list says (D83).
	fragmentTLS := s.routedHost(host)
	clientSide := &bufferedConn{Conn: conn, prefix: drainBuffered(br)}
	s.runBidirectionalCopy(clientSide, upstream, host, port, fragmentTLS)
}

// socks5Greeting reads and answers the method negotiation. It sends the
// final reply itself on failure (05 FF for unacceptable methods) and
// returns errSocks5Malformed (or the IO error) so the caller can close.
func (s *Server) socks5Greeting(conn net.Conn, br *bufio.Reader) error {
	ver, err := br.ReadByte()
	if err != nil {
		return err
	}
	if ver != socks5Version {
		// Not SOCKS5 at all (an HTTP client pointed here, garbage): close
		// quietly — there is no status line to argue with.
		return errSocks5Malformed
	}
	nMethods, err := br.ReadByte()
	if err != nil {
		return err
	}
	if nMethods == 0 {
		s.socks5ReplyMethodNone(conn)
		return errSocks5Malformed
	}
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return err
	}
	acceptable := false
	for _, m := range methods {
		if m == socks5MethodNoAuth {
			acceptable = true
			break
		}
	}
	if !acceptable {
		// No-auth is the only method this door offers (loopback, same
		// trust boundary as the HTTP door).
		s.socks5ReplyMethodNone(conn)
		return errSocks5Malformed
	}
	reply := []byte{socks5Version, socks5MethodNoAuth}
	if err := writeFull(conn, reply, s.cfg.RequestTimeout); err != nil {
		return err
	}
	return nil
}

// socks5Request reads the CONNECT request and returns the destination
// (ok), or sends the right refusal itself (not ok).
func (s *Server) socks5Request(conn net.Conn, br *bufio.Reader) (host string, port int, ok bool) {
	head := make([]byte, 4) // VER CMD RSV ATYP
	if _, err := io.ReadFull(br, head); err != nil {
		return "", 0, false
	}
	if head[0] != socks5Version {
		return "", 0, false
	}
	if head[1] != socks5CmdConnect {
		// BIND and UDP ASSOCIATE are out of scope by design (ADR-0009:
		// only TCP flows enter the engine; DNS is the stack's UDP/53
		// business, answered through the DoH resolver upstream of this
		// door).
		s.socks5Reply(conn, socks5RepCommandUnsupported)
		return "", 0, false
	}
	switch head[3] {
	case socks5AtypIPv4:
		raw := make([]byte, 4)
		if _, err := io.ReadFull(br, raw); err != nil {
			return "", 0, false
		}
		host = net.IP(raw).String()
	case socks5AtypDomain:
		n, err := br.ReadByte()
		if err != nil {
			return "", 0, false
		}
		if n == 0 {
			s.socks5Reply(conn, socks5RepGeneralFailure) // zero-length name
			return "", 0, false
		}
		raw := make([]byte, n)
		if _, err := io.ReadFull(br, raw); err != nil {
			return "", 0, false
		}
		host = string(raw)
	case socks5AtypIPv6:
		raw := make([]byte, 16)
		if _, err := io.ReadFull(br, raw); err != nil {
			return "", 0, false
		}
		host = net.IP(raw).String()
	default:
		s.socks5Reply(conn, socks5RepAddressUnsupported)
		return "", 0, false
	}
	var pb [2]byte
	if _, err := io.ReadFull(br, pb[:]); err != nil {
		return "", 0, false
	}
	port = int(pb[0])<<8 | int(pb[1])
	if port < 1 || port > 65535 {
		s.socks5Reply(conn, socks5RepGeneralFailure)
		return "", 0, false
	}
	return host, port, true
}

// socks5Reply sends a bare failure reply bound at 0.0.0.0:0.
func (s *Server) socks5Reply(conn net.Conn, rep byte) {
	// 05 REP 00 01 0.0.0.0 00 00
	reply := []byte{socks5Version, rep, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0, 0}
	_ = writeFull(conn, reply, s.cfg.RequestTimeout)
}

// socks5ReplyMethodNone sends 05 FF (no acceptable method).
func (s *Server) socks5ReplyMethodNone(conn net.Conn) {
	_ = writeFull(conn, []byte{socks5Version, socks5MethodNoneAcceptable}, s.cfg.RequestTimeout)
}

// socks5ReplyAddr sends the success reply with BND.ADDR set to the
// upstream socket's real local address — the honest "where the tunnel
// connected from" — falling back to 0.0.0.0:0 when the address is not a
// TCP address or carries no IP.
func (s *Server) socks5ReplyAddr(conn net.Conn, local net.Addr) {
	var atyp byte = socks5AtypIPv4
	raw := make([]byte, 4) // the 0.0.0.0 fallback
	port := 0
	if ta, ok := local.(*net.TCPAddr); ok {
		port = ta.Port
		switch {
		case ta.IP == nil:
			// keep the fallback
		case ta.IP.To4() != nil:
			raw = ta.IP.To4()
		case ta.IP.To16() != nil:
			atyp = socks5AtypIPv6
			raw = ta.IP.To16()
		}
	}
	reply := []byte{socks5Version, socks5RepSucceeded, 0x00, atyp}
	reply = append(reply, raw...)
	reply = append(reply, byte(port>>8), byte(port))
	_ = writeFull(conn, reply, s.cfg.RequestTimeout)
}
