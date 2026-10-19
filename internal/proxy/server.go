// Package proxy is the local HTTP proxy (file 01, section 7): a raw
// net.Listener (not http.Server), one goroutine per connection with
// recover, CONNECT tunnels with a fragmented first TLS record, and the
// local PAC/health endpoints.
package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	// FirstByteSalvageDials bounds how many times a processed host's
	// dead first-byte origin is re-dialed inside the same flow,
	// replaying the client's buffered bytes (M8-t, ADR-0019; same host,
	// same preset — never a method change, which is D102's to rule).
	// M8-v (ADR-0020): the remaining re-dials are raced in parallel,
	// not walked serially. M8-w (ADR-0021): with FirstByteRaceDials
	// enabled (the default) the dial budget is spent racing from the
	// flow's start and the salvage rounds never run; this knob keeps
	// its exact meaning when racing is disabled. Zero → the config
	// default; negative → disabled (the M8-s fail-fast behavior
	// exactly).
	FirstByteSalvageDials int
	// FirstByteRaceDials is how many EXTRA origin candidates a
	// processed host's flow races in parallel with its primary dial
	// from the very start (M8-w, ADR-0021): distinct origins via the
	// rotated answer set, the client's first record replayed
	// byte-exact through each, the first origin to speak wins. Zero →
	// the config default; negative → disabled (the exact M8-v shape:
	// serial attempt 0 + the raced salvage re-dials).
	FirstByteRaceDials int
	// StallTimeout bounds how long a processed host's relay may stay
	// silent in BOTH directions before the flow is closed as a
	// mid-stream stall (M8-v, ADR-0020). Zero → the config default;
	// negative → disabled (the M8-u behavior exactly).
	StallTimeout time.Duration
	// StallRichTimeout is the escalated stall budget for a flow that
	// has proven itself alive — ≥ StallRichBytes delivered down, or
	// one idle gap survived and closed by fresh activity (M8-x,
	// ADR-0022). A rich-tier close is quiet: no demotion, no error
	// count, tier=rich on the stall line. Zero → the config default;
	// negative (or ≤ StallTimeout) → no escalation — the exact M8-v
	// semantics for every flow.
	StallRichTimeout time.Duration
	// StallRichBytes is the flow's delivered-down byte count that
	// escalates the stall budget to StallRichTimeout (M8-x, ADR-0022).
	// Zero → the config default; negative → never escalate on bytes
	// (only a survived idle gap escalates).
	StallRichBytes int64
	// DeadIPPenalty is how long an origin IP that connected but never
	// spoke (the classified first-byte death: a reset at the hello
	// write, or silence to the budget) is skipped when building dial
	// lists (M8-y, ADR-0023). The debug.16 report measured the
	// re-gamble: ~17 timeout terminals at 2892–4304 ms plus ~8 resets,
	// all through the raced round, and the SECOND video's start (a new
	// rr hostname) paying the full discovery cost again — the per-flow
	// dead-IP map died with its flow while the authoritative answer
	// kept returning the same blackholed cache-node IPs. The penalty
	// outlives the flow and crosses hostnames (the blackhole is per
	// origin IP); the box is a preference, never a verdict — a list the
	// filter would empty dials in full. Zero → the config default;
	// negative → disabled (the exact M8-x shape).
	DeadIPPenalty  time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int64

	// FragmentAllRouted (M8-z, ADR-0024 work item A): fragment the
	// first TLS record of EVERY non-direct-TLD host (class=plain
	// included), not just the routed domain list. The budgeted shape —
	// first-byte budget, salvage, race, dead-IP penalty, tiered stall
	// kill — stays scoped to budgetedHost (the YouTube-list hosts)
	// exactly as before: fragmentation for everything, the expensive
	// machinery only where it pays. The engine wires the settings field
	// (false on desktop — byte-for-byte old behavior; true on Android,
	// LoadFor's platform default).
	FragmentAllRouted bool
	// PlainFirstByteTimeout bounds a NOT-budgeted TLS flow's origin
	// silence after the first record went upstream (M8-z, ADR-0024
	// work item B): a TLS server always answers a ClientHello quickly,
	// so silence past this budget closes both sides and counts
	// (plainFirstByteTimeouts) instead of hanging until the app's
	// ~60 s patience — the unseen blocker cohort of the debug.16
	// evidence. Non-TLS flows and flows that already delivered a byte
	// keep the plain idle semantics. Zero → the config default;
	// negative → disabled (the exact pre-M8-z plain shape).
	PlainFirstByteTimeout time.Duration
	// Hosts is the per-host flow table behind the report's Top-10
	// section (M8-z, ADR-0027 §8.3.1); nil = collect nothing.
	Hosts *stats.HostTable

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

	// The origin-IP penalty box (M8-y, ADR-0023): origin IP →
	// penalized until. Written by the classified first-byte deaths (the
	// origin connected and never spoke), read when every dial list is
	// built; bounded FIFO, guarded by deadMu.
	deadMu    sync.Mutex
	dead      map[string]time.Time
	deadOrder []string
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
	if cfg.FirstByteSalvageDials == 0 {
		cfg.FirstByteSalvageDials = config.FirstByteSalvageDials
	}
	if cfg.FirstByteRaceDials == 0 {
		cfg.FirstByteRaceDials = config.FirstByteRaceDials
	}
	if cfg.StallTimeout == 0 {
		cfg.StallTimeout = config.StallTimeout
	}
	if cfg.StallRichTimeout == 0 {
		cfg.StallRichTimeout = config.StallRichTimeout
	}
	if cfg.StallRichBytes == 0 {
		cfg.StallRichBytes = config.StallRichBytes
	}
	if cfg.DeadIPPenalty == 0 {
		cfg.DeadIPPenalty = config.DeadIPPenalty
	}
	if cfg.PlainFirstByteTimeout == 0 {
		cfg.PlainFirstByteTimeout = config.PlainFirstByteTimeout
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
//
// M8-z (ADR-0024) splits the single decision into two: routedHost is
// now exactly budgetedHost — the flows that run the first-byte budget,
// salvage, race, dead-IP penalty and tiered stall kill. See
// fragmentHost for the fragmentation decision.
func (s *Server) routedHost(host string) bool {
	if config.UnderDirectTLD(host) {
		return false
	}
	mode, domains := s.pacData()
	return mode == config.ModeAll || MatchAnyDomain(host, domains)
}

// budgetedHost reports whether host's flows run the budgeted shape
// (M8-z, ADR-0024): exactly yesterday's routedHost result — the
// YouTube-list (or mode=all) hosts, never the direct TLDs. The
// first-byte budget, the salvage/race machinery, the dead-IP penalty
// box and the tiered stall watch stay scoped to these hosts and to
// these hosts ONLY, whatever FragmentAllRouted says: fragmentation
// for everything is cheap and safe; the expensive machinery is not.
func (s *Server) budgetedHost(host string) bool {
	return s.routedHost(host)
}

// fragmentHost reports whether host's first TLS record is fragmented
// (M8-z, ADR-0024 work item A): not under a direct TLD (D83) AND
// (FragmentAllRouted OR the routed domain list match). With
// FragmentAllRouted on (Android's default) every routed app's TLS
// ClientHello rides fragmented — the ad, attestation and Google-API
// hosts the YouTube app needs at first play are exactly the cohort
// the debug.16 report could not see, and SNI inspection kills them
// un-fragmented. With it off, yesterday's rule byte for byte: the
// list hosts only.
func (s *Server) fragmentHost(host string) bool {
	if config.UnderDirectTLD(host) {
		return false
	}
	if s.cfg.FragmentAllRouted {
		return true
	}
	mode, domains := s.pacData()
	return mode == config.ModeAll || MatchAnyDomain(host, domains)
}

// classFor is the tunnel-line class tag (M8-z, ADR-0027 §8.2):
// "routed" for a budgeted (YouTube-list) host, "plain" for everything
// else — including the plain-but-fragmented flows FragmentAllRouted
// creates. The tag rides in the MESSAGE text (the report's log ring
// drops attrs) so every up/first-byte/done/stall line carries it.
func (s *Server) classFor(host string) string {
	if s.budgetedHost(host) {
		return "routed"
	}
	return "plain"
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
	ips = s.liveIPs(ips) // the penalty box prefers live origins (M8-y, ADR-0023)

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
	s.logTunnelUp(host, s.classFor(host), dnsMS, dialMS)

	if err := writeFull(client, []byte(statusConnectEstablished), s.cfg.RequestTimeout); err != nil {
		s.countConnError()
		upstream.Close()
		return
	}

	// The two decisions (M8-z, ADR-0024): fragmentHost fragments the
	// first TLS record (FragmentAllRouted extends it to every
	// non-direct-TLD host — the manual system proxy sends every host
	// here, D46 — while the never-processed TLDs stay untouched in
	// every mode, even when the user's own list names them (.ir —
	// D83)); budgetedHost gates the expensive machinery (exactly the
	// old routedHost result: the list hosts only).
	fragmentTLS := s.fragmentHost(host)
	budgeted := s.budgetedHost(host)

	clientSide := &bufferedConn{Conn: client, prefix: drainBuffered(br)}
	s.runBidirectionalCopy(clientSide, upstream, host, port, fragmentTLS, budgeted)
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
	ips = s.liveIPs(ips) // the penalty box prefers live origins (M8-y, ADR-0023)
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

// deadIPBoxMax bounds the origin-IP penalty box (M8-y, ADR-0023) —
// far above any one session's distinct dead cache nodes, small enough
// to scan trivially.
const deadIPBoxMax = 512

// markDeadIP penalizes one origin IP for DeadIPPenalty (M8-y,
// ADR-0023): the origin connected (or reset at the hello write) and
// never spoke — the classified first-byte death, the pasted reports'
// dead-origin signature. The censor's blackhole is per ORIGIN IP (cache
// nodes shared across rr hostnames), so the knowledge outlives the
// flow and crosses hostnames: the next video's fresh rr name resolving
// to the same blackholed node skips it on sight. Stall kills,
// client-side deaths, teardown-canceled deaths and dial failures never
// mark (no single-IP first-byte evidence). No-op when the knob is
// disabled.
func (s *Server) markDeadIP(ip string) {
	if s.cfg.DeadIPPenalty <= 0 || ip == "" {
		return
	}
	until := time.Now().Add(s.cfg.DeadIPPenalty)
	s.deadMu.Lock()
	defer s.deadMu.Unlock()
	if s.dead == nil {
		s.dead = make(map[string]time.Time)
	}
	if _, ok := s.dead[ip]; !ok {
		for len(s.dead) >= deadIPBoxMax && len(s.deadOrder) > 0 {
			oldest := s.deadOrder[0]
			s.deadOrder = s.deadOrder[1:]
			delete(s.dead, oldest)
		}
		s.deadOrder = append(s.deadOrder, ip)
	}
	s.dead[ip] = until
}

// liveIPs filters penalized origin IPs out of a dial list (M8-y,
// ADR-0023). The box is a preference, never a verdict: a list the
// filter would EMPTY returns unchanged — an all-penalized set gambles
// in full exactly as before, never an instant fail. Single-IP lists
// return untouched (nothing to prefer). No-op when the knob is
// disabled.
func (s *Server) liveIPs(ips []net.IP) []net.IP {
	if s.cfg.DeadIPPenalty <= 0 || len(ips) < 2 {
		return ips
	}
	s.deadMu.Lock()
	now := time.Now()
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if until, ok := s.dead[ip.String()]; ok && now.Before(until) {
			continue
		}
		out = append(out, ip)
	}
	s.deadMu.Unlock()
	if len(out) == 0 {
		return ips
	}
	return out
}

// PenalizedIPs returns up to n origin IPs currently in the dead-IP
// penalty box (M8-z, ADR-0027 §8.3.5: the report's one-shot
// dead-origin probe asks these whether the block is by SNI or by IP —
// the IPs themselves never reach the report, only the verdicts).
func (s *Server) PenalizedIPs(n int) []string {
	if n <= 0 {
		return nil
	}
	now := time.Now()
	s.deadMu.Lock()
	defer s.deadMu.Unlock()
	var out []string
	for _, ip := range s.deadOrder {
		if until, ok := s.dead[ip]; ok && until.After(now) {
			out = append(out, ip)
			if len(out) >= n {
				break
			}
		}
	}
	return out
}

// ProbeDeadOrigin answers whether one origin IP speaks to a plain
// ClientHello (M8-z, ADR-0027 §8.3.5): "answered" (the origin — and
// therefore the path — is alive; the block, if any, is by SNI or
// deeper), "silent" (blackholed by IP), or "reset" (actively refused).
// The hello is dialed through the engine's one netx dialer (ADR-0011
// loop safety: the protect hook applies). sni is the ServerName to
// send ("" sends none — Go omits the extension for an empty name).
func (s *Server) ProbeDeadOrigin(ctx context.Context, ip net.IP, sni string) string {
	conn, err := s.cfg.Dialer.DialHost(ctx, "tcp", ip.String(), 443, []net.IP{ip})
	if err != nil {
		return "silent" // not even a TCP answer: the IP's path is dead
	}
	defer conn.Close()
	// The handshake budget is the ctx (the engine gives each probe 2 s);
	// verification is irrelevant here — only whether bytes come back.
	tlsCfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true}
	tc := tls.Client(conn, tlsCfg)
	err = tc.HandshakeContext(ctx)
	if err == nil {
		return "answered"
	}
	if isTimeout(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return "silent"
	}
	if strings.Contains(err.Error(), "connection reset") {
		return "reset"
	}
	// Any other handshake failure (certificate verification, alert
	// after the ServerHello, EOF mid-flight) means the origin SPOKE —
	// bytes came back within the budget.
	return "answered"
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
