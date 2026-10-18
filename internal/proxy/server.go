// Package proxy is the local HTTP proxy (file 01, section 7): a raw
// net.Listener (not http.Server), one goroutine per connection with
// recover, CONNECT tunnels with a fragmented first TLS record, and the
// local PAC/health endpoints.
package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/stats"
)

// Status and error strings the proxy sends or returns.
const (
	statusConnectEstablished = "HTTP/1.1 200 Connection established\r\n\r\n"
	statusBadGateway         = "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	statusUnavailable        = "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	statusMethodNotAllowed   = "HTTP/1.1 405 Method Not Allowed\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	statusNotFound           = "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	statusBadRequest         = "HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
)

// ErrPortBusy is returned when no port could be bound.
var ErrPortBusy = errors.New("PORT_BUSY")

// Resolver is what the proxy needs from the DoH resolver.
type Resolver interface {
	Lookup(ctx context.Context, host string) ([]net.IP, error)
}

// Logger is the narrow logging interface (satisfiable by *slog.Logger).
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Config wires the proxy to its dependencies.
type Config struct {
	Host string
	Port int
	// AutoPort tries 4526-4545 and then any free port when Port is busy.
	AutoPort bool
	// MaxConns caps concurrent tunnels; above it the proxy replies 503.
	MaxConns int32

	RequestTimeout     time.Duration
	FirstRecordTimeout time.Duration
	// FirstByteTimeout bounds the origin's silence after the first
	// record was written upstream, for processed (routed) hosts only
	// (M8-s, ADR-0018). Zero → the config default; negative → disabled
	// (the plain copy shape, every tunnel's behavior before M8-s).
	FirstByteTimeout time.Duration
	IdleTimeout      time.Duration
	MaxHeaderBytes   int64

	Resolver           Resolver
	Dialer             *netx.Dialer
	DialTimeoutOverall time.Duration
	FragmentPreset     fragment.Preset
	FragmentConfig     fragment.Config
	RNG                fragment.RNG
	Stats              *stats.Counters
	Logger             Logger
	LogHostnames       bool
	Domains            []string
	Mode               config.Mode
}

// Server is a startable, stoppable local proxy.
type Server struct {
	cfg   Config
	stats *stats.Counters

	mu      sync.Mutex
	ln      net.Listener
	port    int
	running bool
	tunnels map[*tunnelCloser]struct{}
	wg      sync.WaitGroup

	// SOCKS5 door (socks5.go): a second, optional loopback listener on the
	// same relay. nil = closed (the default on every desktop build).
	socksLn   net.Listener
	socksPort int

	active int32
}

// Start binds the listener (with the AutoPort fallback) and serves until
// Stop. Calling Start on a running server is a no-op.
func Start(cfg Config) (*Server, error) {
	if cfg.Stats == nil {
		return nil, errors.New("proxy: stats required")
	}
	if cfg.MaxConns < 1 {
		cfg.MaxConns = config.MaxConcurrentConnections
	}
	if cfg.MaxHeaderBytes < 1 {
		cfg.MaxHeaderBytes = config.MaxHeaderBytes
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = config.RequestReadTimeout
	}
	if cfg.FirstRecordTimeout <= 0 {
		cfg.FirstRecordTimeout = config.FirstRecordReadTimeout
	}
	if cfg.FirstByteTimeout == 0 {
		cfg.FirstByteTimeout = config.FirstByteTimeout
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = config.IdleTimeout
	}
	if cfg.DialTimeoutOverall <= 0 {
		cfg.DialTimeoutOverall = config.DialTimeoutOverall
	}
	if !cfg.FragmentPreset.Valid() {
		cfg.FragmentPreset = fragment.Balanced
	}
	if len(cfg.Domains) == 0 {
		cfg.Domains = config.DefaultDomains()
	}
	s := &Server{cfg: cfg, stats: cfg.Stats, tunnels: make(map[*tunnelCloser]struct{})}
	if err := s.Start(); err != nil {
		return nil, err
	}
	return s, nil
}

// Start binds (or rebinds) the listener.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	ln, port, err := s.listenLocked()
	if err != nil {
		return err
	}
	s.ln = ln
	s.port = port
	s.running = true
	s.wg.Add(1)
	go s.acceptLoop(ln)
	if s.cfg.Logger != nil {
		s.cfg.Logger.Info("proxy listening", "host", s.cfg.Host, "port", port)
	}
	return nil
}

// listenLocked implements the AutoPort rule: the configured port, then
// 4526-4545, then any free port.
func (s *Server) listenLocked() (net.Listener, int, error) {
	first := s.cfg.Port
	if first < 1 || first > 65535 {
		first = config.ListenPort
	}
	try := func(port int) (net.Listener, int, error) {
		ln, err := net.Listen("tcp", net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", port)))
		if err != nil {
			return nil, 0, err
		}
		return ln, port, nil
	}
	if ln, port, err := try(first); err == nil {
		return ln, port, nil
	}
	if !s.cfg.AutoPort {
		return nil, 0, fmt.Errorf("%w: %d", ErrPortBusy, first)
	}
	for p := 4526; p <= 4545; p++ {
		if ln, port, err := try(p); err == nil {
			if s.cfg.Logger != nil {
				s.cfg.Logger.Info("configured port busy; using another", "port", port)
			}
			return ln, port, nil
		}
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(s.cfg.Host, "0"))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrPortBusy, err)
	}
	if s.cfg.Logger != nil {
		s.cfg.Logger.Info("configured port busy; using a random free port", "port", ln.Addr().(*net.TCPAddr).Port)
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}

// SetPACData updates the mode and domain list used to build /proxy.pac
// (mode changes apply immediately while On).
func (s *Server) SetPACData(mode config.Mode, domains []string) {
	s.mu.Lock()
	s.cfg.Mode = mode
	s.cfg.Domains = domains
	s.mu.Unlock()
}

// SetFragment updates the first-record fragmentation (method changes apply
// immediately while On).
func (s *Server) SetFragment(preset fragment.Preset, cfg fragment.Config) {
	s.mu.Lock()
	s.cfg.FragmentPreset = preset
	s.cfg.FragmentConfig = cfg
	s.mu.Unlock()
}

// SetResolver swaps the secure-DNS resolver used for future tunnels (a
// changed DoH list applies while On; D44).
func (s *Server) SetResolver(r Resolver) {
	s.mu.Lock()
	s.cfg.Resolver = r
	s.mu.Unlock()
}

// resolverData returns the current resolver.
func (s *Server) resolverData() Resolver {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Resolver
}

// pacData returns the current mode and domains.
func (s *Server) pacData() (config.Mode, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Mode, s.cfg.Domains
}

// routedHost reports whether host gets the app's processing — the
// first-record fragmentation and the http→https redirect. It is the
// one decision every door shares: the mode/list rule (D46) minus the
// never-processed TLDs, which win over every mode AND over the user's
// domain list — every host under .ir relays untouched, whatever the
// settings say (D83).
func (s *Server) routedHost(host string) bool {
	if config.UnderDirectTLD(host) {
		return false
	}
	mode, domains := s.pacData()
	return mode == config.ModeAll || MatchAnyDomain(host, domains)
}

// fragmentData returns the current preset and config.
func (s *Server) fragmentData() (fragment.Preset, fragment.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.FragmentPreset, s.cfg.FragmentConfig
}

// Port reports the actual listening port.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Addr reports the listening address (host:port).
func (s *Server) Addr() string {
	return net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.Port()))
}

// acceptLoop accepts connections until its listener closes. It takes the
// listener as a parameter so a Stop+Start cycle never races on s.ln.
func (s *Server) acceptLoop(ln net.Listener) {
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
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return
			}
			if s.cfg.Logger != nil {
				s.cfg.Logger.Warn("accept failed", "reason", errKind(err))
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		s.wg.Add(1)
		go s.handle(conn)
	}
}

// handle serves one connection with panic recovery (file 03, R2).
func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	s.stats.ConnOpened()
	defer s.stats.ConnClosed()
	defer func() {
		if p := recover(); p != nil {
			s.stats.Panic()
			if s.cfg.Logger != nil {
				s.cfg.Logger.Error("connection handler panicked and was recovered", "panic", fmt.Sprint(p))
			}
			conn.Close()
		}
	}()
	defer conn.Close()

	if atomic.AddInt32(&s.active, 1) > s.cfg.MaxConns {
		atomic.AddInt32(&s.active, -1)
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("connection limit reached; replying 503", "limit", s.cfg.MaxConns)
		}
		s.reply(conn, statusUnavailable)
		return
	}
	defer atomic.AddInt32(&s.active, -1)

	conn.SetReadDeadline(time.Now().Add(s.cfg.RequestTimeout))
	br := bufio.NewReader(io.LimitReader(conn, s.cfg.MaxHeaderBytes))
	req, err := parseRequest(br)
	if err != nil {
		if isTimeout(err) {
			return // the client never sent anything; close quietly
		}
		s.reply(conn, statusBadRequest)
		return
	}
	rt := decide(req, s.Port())
	switch rt.kind {
	case routeConnect:
		conn.SetReadDeadline(time.Time{})
		s.handleTunnel(conn, br, rt.target)
		return
	case routeRedirect:
		// Absolute-form http:// request. Hosts this app routes keep the
		// http->https redirect (file 01 §7); anything else (youtube mode
		// with the manual system proxy sending every host here, or a
		// never-processed .ir host in any mode — D46/D83) is forwarded
		// like a normal proxy so plain-http sites keep working.
		if s.routedHost(req.URL.Hostname()) {
			s.reply(conn, "HTTP/1.1 302 Found\r\nLocation: "+rt.https+"\r\nContent-Length: 0\r\n\r\n")
			return
		}
		conn.SetReadDeadline(time.Time{})
		s.forwardRequest(conn, br, req)
		return
	case routePAC:
		mode, domains := s.pacData()
		body := PAC(s.Port(), domains, mode)
		s.reply(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/x-ns-proxy-autoconfig\r\nContent-Length: "+
			fmt.Sprintf("%d", len(body))+"\r\n\r\n"+body)
		return
	case routeHealth:
		s.reply(conn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok")
		return
	case routeNotFound:
		s.reply(conn, statusNotFound)
		return
	default:
		s.reply(conn, statusMethodNotAllowed)
		return
	}
}

// reply writes one prebuilt response with a write deadline, then closes.
func (s *Server) reply(conn net.Conn, response string) {
	_ = writeFull(conn, []byte(response), s.cfg.RequestTimeout)
}

// handleTunnel resolves and dials, then runs the tunnel copy phase.
func (s *Server) handleTunnel(client net.Conn, br *bufio.Reader, target string) {
	host, portStr, err := splitHostPort(target)
	if err != nil {
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port < 1 || port > 65535 {
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}

	tDNS := time.Now()
	ips, err := s.resolve(host)
	dnsMS := time.Since(tDNS).Milliseconds()
	if err != nil {
		if s.cfg.Logger != nil {
			// The reason rides in the message: the log ring that feeds the
			// diagnostic report drops attrs (M8-r — the debug.10 report
			// showed these lines bare, unable to say why the tunnel
			// lookup failed).
			s.cfg.Logger.Warn("dns lookup failed for tunnel reason=" + errKind(err))
		}
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}

	tDial := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
	upstream, err := s.cfg.Dialer.DialHost(ctx, "tcp", host, port, ips)
	cancel()
	dialMS := time.Since(tDial).Milliseconds()
	if err != nil {
		if s.cfg.Logger != nil {
			if s.cfg.LogHostnames {
				s.cfg.Logger.Warn("tunnel dial failed", "host", host, "reason", errKind(err))
			} else {
				s.cfg.Logger.Warn("tunnel dial failed", "reason", errKind(err))
			}
		}
		s.invalidateDNS(host) // M8-o: the whole set failed; re-query next time
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}
	s.logTunnelUp(host, dnsMS, dialMS)

	if err := writeFull(client, []byte(statusConnectEstablished), s.cfg.RequestTimeout); err != nil {
		s.countConnError()
		upstream.Close()
		return
	}

	// Only the routed hosts are fragmented; everything else relays
	// untouched (the manual system proxy sends every host here — D46),
	// and the never-processed TLDs stay untouched in every mode, even
	// when the user's own list names them (.ir — D83).
	fragmentTLS := s.routedHost(host)

	clientSide := &bufferedConn{Conn: client, prefix: drainBuffered(br)}
	s.runBidirectionalCopy(clientSide, upstream, host, fragmentTLS)
}

// forwardRequest relays one plain-http request to its origin server like a
// normal forward proxy (used for hosts outside the routed list in youtube
// mode, so plain-http sites keep working through the manual system proxy;
// D46). The request is rebuilt in origin-form and sent with
// "Connection: close" — one exchange per connection keeps the relay simple
// and correct; the browser simply opens a new connection for the next
// request.
func (s *Server) forwardRequest(client net.Conn, br *bufio.Reader, req *http.Request) {
	_ = br // the body reads through the same bufio.Reader already
	host := req.URL.Hostname()
	if host == "" {
		s.reply(client, statusBadRequest)
		return
	}
	port := 80
	if p := req.URL.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
			port = n
		}
	}

	ips, err := s.resolve(host)
	if err != nil {
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
	upstream, err := s.cfg.Dialer.DialHost(ctx, "tcp", host, port, ips)
	cancel()
	if err != nil {
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("forward dial failed", "reason", errKind(err))
		}
		s.invalidateDNS(host) // M8-o: the whole set failed; re-query next time
		s.reply(client, statusBadGateway)
		s.countConnError()
		return
	}
	defer upstream.Close()

	// Rebuild the request in origin-form with hop-by-hop headers removed.
	out := req.Clone(context.Background())
	out.URL = &url.URL{Path: req.URL.Path, RawPath: req.URL.RawPath, RawQuery: req.URL.RawQuery}
	if req.Host != "" {
		out.Host = req.Host // the client's Host header (port included)
	} else {
		out.Host = req.URL.Host
	}
	out.Close = true
	for _, h := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		out.Header.Del(h)
	}
	if req.Body != nil {
		out.Body = req.Body
	}
	upstream.SetWriteDeadline(time.Now().Add(s.cfg.RequestTimeout))
	if err := out.Write(upstream); err != nil {
		s.countConnError()
		return
	}
	upstream.SetWriteDeadline(time.Time{})

	// Relay the residual request body (client -> origin) and the response
	// (origin -> client) until the origin finishes; then end the exchange.
	clientSide := &bufferedConn{Conn: client, prefix: nil}
	cIdle := &idleConn{Conn: clientSide, timeout: s.cfg.IdleTimeout}
	uIdle := &idleConn{Conn: upstream, timeout: s.cfg.IdleTimeout}
	relayMeter := &flowMeter{} // counted but not logged: one plain-http exchange, not a tunnel
	bodyDone := make(chan struct{})
	go func() {
		defer close(bodyDone)
		s.copyDir(cIdle, uIdle, true, relayMeter, nil)
		closeWrite(upstream)
	}()
	s.copyDir(uIdle, cIdle, false, relayMeter, nil)
	// The response is fully relayed; nothing more can usefully happen.
	client.Close()
	upstream.Close()
	<-bodyDone
}

// resolve returns target addresses; IP literals skip DNS.
func (s *Server) resolve(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	r := s.resolverData()
	if r == nil {
		return nil, errors.New("no resolver configured")
	}
	ips, err := r.Lookup(context.Background(), host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("no addresses returned")
	}
	return ips, nil
}

// invalidateDNS reports a total dial failure to the resolver (M8-o,
// ADR-0014): every address of the set resolve() returned for this host
// failed to connect, so the set is suspect on this network and the next
// lookup should re-query the servers instead of serving the same cached
// set until TTL expiry. Optional capability — resolvers without
// Invalidate (tests' minimal fakes) are skipped; IP literals never
// cached by the real resolver, and it no-ops them itself.
func (s *Server) invalidateDNS(host string) {
	if inv, ok := s.resolverData().(interface{ Invalidate(string) }); ok {
		inv.Invalidate(host)
	}
}

// register tracks a live tunnel so Stop can force-close it.
func (s *Server) register(t *tunnelCloser) {
	s.mu.Lock()
	s.tunnels[t] = struct{}{}
	s.mu.Unlock()
}

// unregister drops a finished tunnel.
func (s *Server) unregister(t *tunnelCloser) {
	s.mu.Lock()
	delete(s.tunnels, t)
	s.mu.Unlock()
}

// Stop closes the listener, waits up to the ctx deadline for tunnels, then
// force-closes the rest. Safe to call repeatedly from any goroutine.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	ln := s.ln
	s.ln = nil
	// The SOCKS5 door, when open, goes down with the same Stop (its
	// handlers are tracked by the same wg below). Captured under the
	// same lock so it cannot race a concurrent StartSocks5/StopSocks5.
	sl := s.socksLn
	s.socksLn = nil
	s.socksPort = 0
	s.mu.Unlock()

	ln.Close()
	if sl != nil {
		sl.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		remaining := make([]*tunnelCloser, 0, len(s.tunnels))
		for t := range s.tunnels {
			remaining = append(remaining, t)
		}
		s.mu.Unlock()
		for _, t := range remaining {
			t.closeBoth()
		}
		if s.cfg.Logger != nil {
			s.cfg.Logger.Info("proxy stopped; force-closed remaining tunnels", "count", len(remaining))
		}
		return nil
	}
}

// StopWithTimeout stops with the spec's 3-second budget.
func (s *Server) StopWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.Stop(ctx)
}

func (s *Server) countConnError() {
	s.stats.ConnError()
}

func writeFull(conn net.Conn, b []byte, timeout time.Duration) error {
	if timeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(timeout))
		defer conn.SetWriteDeadline(time.Time{})
	}
	_, err := conn.Write(b)
	return err
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// errKind reduces an error to a short, hostless/IPless reason string.
func errKind(err error) string {
	if err == nil {
		return ""
	}
	if isTimeout(err) {
		return "timeout"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return "closed"
	}
	msg := err.Error()
	if i := indexByte(msg, ':'); i > 0 {
		msg = msg[:i]
	}
	return msg
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// Running reports whether the listener is up.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
