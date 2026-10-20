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
	"sort"
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
	// StallRichPendingTimeout is the fast budget for a rich flow that
	// died with the client's request OUTSTANDING (M8-ad, ADR-0037): the
	// player's next range request riding a dead pooled conn — the
	// mid-video wedge ("goes loading; refresh fixes it"). The kill
	// demotes the good mark (definite dead-with-work evidence) but stays
	// quiet: no error count, tier=rich cause=pending. Zero → the config
	// default; negative (or ≥ StallRichTimeout) → disabled, pending
	// rich kills wait the full rich budget (the M8-x/M8-ac semantics).
	StallRichPendingTimeout time.Duration
	// TrickleWindow is the rolling window of delivered-down bytes the
	// trickle detector watches on rich flows (M8-ad, ADR-0037): the
	// per-connection throttle shape (bytes keep arriving, the down stamp
	// keeps refreshing, the stall watch never fires) gets a quiet close
	// so the app's retry lands on a fresh, unthrottled conn. Zero → the
	// config default; negative → the detector is off.
	TrickleWindow time.Duration
	// TrickleMinBytes is the byte floor a rich flow must deliver inside
	// TrickleWindow to count as alive (M8-ad, ADR-0037). Zero → the
	// config default; negative → the detector is off.
	TrickleMinBytes int64
	// TrickleFloorBytes is the delivered-down total that arms the
	// trickle detector (M8-ad, ADR-0037). Zero → the config default;
	// negative → the detector is off.
	TrickleFloorBytes int64
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

	// GoodIPSubstitution (M8-ab, ADR-0031): a *.googlevideo.com flow
	// whose raced round draws only its own hostname's rotating answer
	// set gets ONE substitute candidate instead — pool origins the
	// engine has SEEN speak (probe verdicts and real flow first-bytes),
	// not in the current answer set, not penalized, freshest verdict
	// first. The substitute replays the identical fragmented
	// ClientHello (SNI unchanged — the client's own hello) onto the
	// pool origin: Google's video edge serves any *.googlevideo.com
	// name, the Iranian hosts-file IP-substitution practice rides
	// exactly this. Positive → the per-flow substitute candidate cap;
	// zero → the config default; negative → disabled (the exact
	// debug.19 shape).
	GoodIPSubstitution int

	// GvideoBreakerTerminals / GvideoBreakerWindow / GvideoBreakerCooldown
	// (M8-ae, ADR-0041): the googlevideo EMISSION BREAKER. Terminals
	// counted in the window → a cooldown during which googlevideo flows
	// fail fast at both doors (zero wire) and the probe lane stays
	// paused, so the app's player re-fetches and rotates hostnames —
	// the reference implementations' only recovery — and any
	// per-source SNI flag the censor accumulated decays in the quiet.
	// Zero values take the config defaults; a NEGATIVE terminals value
	// disables the breaker (the exact debug.22 shape).
	GvideoBreakerTerminals int
	GvideoBreakerWindow    time.Duration
	GvideoBreakerCooldown  time.Duration

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

	// The origin-IP penalty box (M8-y, ADR-0023; M8-aa escalates it,
	// ADR-0028): origin IP → penalized until, with the strike count
	// that escalates repeat offenders. Written by the classified
	// first-byte deaths (the origin connected and never spoke) and by
	// the origin-probe pass's silent/reset verdicts; read when every
	// dial list is built; bounded FIFO, guarded by deadMu.
	deadMu    sync.Mutex
	dead      map[string]deadMark
	deadOrder []string

	// The verified-live table (M8-aa, ADR-0028; M8-ab extends the
	// marks with the origin's service, ADR-0031): origin IP → good
	// mark (preferred until, googlevideo flag), written by the
	// origin-probe pass's "answered" verdicts AND by real flow
	// first-bytes (stronger evidence — the session's own wire shape
	// spoke back), read by liveIPs to put origins the engine has
	// SEEN speak at the FRONT of every dial list, and by the raced
	// round's substitute candidate (the cross-origin escape: the
	// gvideo-flagged pool serves any *.googlevideo.com flow). Bounded
	// FIFO like the box, guarded by goodMu. Equally a preference,
	// never a verdict — an IP's absence costs it nothing but first
	// place.
	goodMu    sync.Mutex
	good      map[string]goodMark
	goodOrder []string

	// goodRot counts every liveIPs/goodSubstitutes call that rotates
	// (M8-ac, ADR-0034): the anti-concentration turn — the debug.20
	// report's www.youtube.com ran 17 flows at 17 failures with the
	// good-front pinning every primary and rotation onto the same
	// verified origin while the connection-killer worked the lane.
	// Consecutive calls rotate the live-front so consecutive flows
	// spread across the verified origins instead of funneling onto
	// one.
	goodRot atomic.Int64

	// The googlevideo emission breaker (M8-ae, ADR-0041): the
	// recent-terminal ring and the cooldown's end. fed by failFlow
	// (every googlevideo terminal), read at BOTH doors and by the
	// engine's probe lane.
	gvideoBreak gvideoBreakState
}

// deadMark is one penalized origin IP's box entry (M8-aa, ADR-0028):
// the penalty's expiry and how many times this IP has been marked
// dead — the strike count the TTL escalation reads.
type deadMark struct {
	until   time.Time
	strikes int
}

// goodMark is one verified-live origin IP's table entry (M8-ab,
// ADR-0031): the preference's expiry and whether the origin serves
// the googlevideo wildcard — the flag that scopes cross-origin
// substitution to same-service flows.
type goodMark struct {
	until  time.Time
	gvideo bool
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
	if cfg.StallRichPendingTimeout == 0 {
		cfg.StallRichPendingTimeout = config.StallRichPendingTimeout
	}
	if cfg.TrickleWindow == 0 {
		cfg.TrickleWindow = config.TrickleWindow
	}
	if cfg.TrickleMinBytes == 0 {
		cfg.TrickleMinBytes = config.TrickleMinBytes
	}
	if cfg.TrickleFloorBytes == 0 {
		cfg.TrickleFloorBytes = config.TrickleFloorBytes
	}
	if cfg.DeadIPPenalty == 0 {
		cfg.DeadIPPenalty = config.DeadIPPenalty
	}
	if cfg.GoodIPSubstitution == 0 {
		cfg.GoodIPSubstitution = config.GoodIPSubstitution
	}
	if cfg.GvideoBreakerTerminals == 0 {
		cfg.GvideoBreakerTerminals = config.GvideoBreakerTerminals
	}
	if cfg.GvideoBreakerWindow <= 0 {
		cfg.GvideoBreakerWindow = config.GvideoBreakerWindow
	}
	if cfg.GvideoBreakerCooldown <= 0 {
		cfg.GvideoBreakerCooldown = config.GvideoBreakerCooldown
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

	// M8-ae (ADR-0041): the emission breaker's door. The breaker is
	// armed (recent googlevideo terminals) and this is a googlevideo
	// host: reply 502 NOW, zero wire, no resolve, no dial — the app's
	// player re-fetches and rotates hostnames while the cooldown lets
	// any per-source SNI flag decay. The feed's hosts (a different SNI
	// class) pass untouched, which is the whole point.
	if s.gvideoDoorBlocked(host) {
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("tunnel gvideo cooldown (emission breaker: wire quiet)")
		}
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

// isGvideoHost reports whether a host is a Google video cache name
// (M8-ab, ADR-0031): the service whose edge serves ANY
// *.googlevideo.com origin behind the per-cluster names — the scope
// of cross-origin substitution and the gvideo good-pool flag.
func isGvideoHost(host string) bool {
	return strings.HasSuffix(host, ".googlevideo.com")
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

// goodIPTableMax bounds the verified-live table (M8-aa, ADR-0028) —
// the same bound, the same trivial scan.
const goodIPTableMax = 512

// strikeTTL maps a strike count to its escalation floor (M8-aa,
// ADR-0028): the first mark keeps the caller's requested TTL, the
// second and beyond never sit below the escalation tiers. The dead
// box is a PREFERENCE, never a verdict (ADR-0023) — escalation only
// lengthens how long a repeatedly-dead IP is skipped, never whether a
// list may dial it.
func strikeTTL(strikes int) time.Duration {
	tiers := config.DeadIPStrikeEscalation
	if strikes <= 1 {
		return tiers[0]
	}
	if strikes == 2 {
		return tiers[1]
	}
	return tiers[2]
}

// markDeadIP penalizes one origin IP for DeadIPPenalty (M8-y,
// ADR-0023): the origin connected (or reset at the hello write) and
// never spoke — the classified first-byte death, the pasted reports'
// dead-origin signature. Stall kills,
// client-side deaths, teardown-canceled deaths and dial failures never
// mark (no single-IP first-byte evidence). No-op when the knob is
// disabled.
func (s *Server) markDeadIP(ip string) {
	s.markDeadIPTTL(ip, s.cfg.DeadIPPenalty)
}

// markDeadIPClass penalizes one origin IP's first-byte death by its
// EVIDENCE CLASS (M8-ac, ADR-0034): a reset death — the origin (or
// something on the path) actively refused the connection at the
// hello — is now a TEMPORAL signal, not an IP verdict: the debug.20
// report measured the regime change (the one-shot dead-origin probe
// ANSWERED on penalized IPs; 35 of 77 probe verdicts came back
// reset), so a reset boxes the IP only ResetIPPenalty long — a
// within-retry-wave skip that spreads candidates elsewhere — and
// does NOT demote the good mark (a live origin stays preferred; the
// rotation in liveIPs keeps it from being the ONLY origin dialed).
// Every other first-byte death (a full budget of silence, a clean
// close) keeps the M8-ab semantics: DeadIPPenalty + the demote.
func (s *Server) markDeadIPClass(ip, reason string) {
	if ip == "" {
		return
	}
	if reason == "reset" {
		if s.cfg.DeadIPPenalty <= 0 {
			return
		}
		s.boxIP(ip, config.ResetIPPenalty)
		return // no demote: the reset is evidence about ONE connection
	}
	s.markDeadIP(ip)
}

// boxIP writes one origin IP's penalty-box entry with strike
// escalation, WITHOUT touching the good table (M8-ac, ADR-0034 — the
// class-split core of markDeadIPTTL, shared by the reset path and
// the probe's rehabilitation refresh).
func (s *Server) boxIP(ip string, base time.Duration) {
	if s.cfg.DeadIPPenalty <= 0 || base <= 0 || ip == "" {
		return
	}
	s.deadMu.Lock()
	if s.dead == nil {
		s.dead = make(map[string]deadMark)
	}
	m := s.dead[ip]
	m.strikes++
	ttl := base
	if esc := strikeTTL(m.strikes); esc > ttl {
		ttl = esc
	}
	m.until = time.Now().Add(ttl)
	if _, ok := s.dead[ip]; !ok {
		for len(s.dead) >= deadIPBoxMax && len(s.deadOrder) > 0 {
			oldest := s.deadOrder[0]
			s.deadOrder = s.deadOrder[1:]
			delete(s.dead, oldest)
		}
		s.deadOrder = append(s.deadOrder, ip)
	}
	s.dead[ip] = m
	s.deadMu.Unlock()
}

// clearDeadIP removes one origin IP's penalty-box entry entirely
// (M8-ac, ADR-0034): the rehabilitation's write — fresh direct
// evidence the IP answers outranks the stale death that boxed it.
// The FIFO order entry is dropped with it (a later re-box re-learns
// it at the end, its strikes gone).
func (s *Server) clearDeadIP(ip string) {
	if ip == "" {
		return
	}
	s.deadMu.Lock()
	delete(s.dead, ip)
	for i, key := range s.deadOrder {
		if key == ip {
			s.deadOrder = append(s.deadOrder[:i], s.deadOrder[i+1:]...)
			break
		}
	}
	s.deadMu.Unlock()
}

// markDeadIPTTL penalizes one origin IP for the given baseline TTL
// with strike escalation (M8-aa, ADR-0028): a repeat mark while the
// entry lives (or later — the strike count survives expiry until the
// 512-entry FIFO evicts it) escalates the TTL to the escalation floor,
// so a cache node that keeps killing flows stops being re-learned
// every 60 s. A verified-live mark on the same IP is DROPPED (M8-ab,
// ADR-0031 — the behavior ADR-0028's comment promised, actually
// implemented): the death is the fresher evidence, and without the
// drop a dead IP resurfaces at the FRONT of every dial list the
// moment its box entry lapses — the debug.19 code's latent leak, and
// a poisoned substitution pool the moment the pool started mattering.
func (s *Server) markDeadIPTTL(ip string, base time.Duration) {
	if s.cfg.DeadIPPenalty <= 0 || base <= 0 || ip == "" {
		return
	}
	s.boxIP(ip, base)
	// The demote (M8-ab): the box's verdict outranks the good mark
	// while it lives, and the stale preference must not outlive it.
	s.dropGoodIP(ip)
}

// dropGoodIP removes one origin IP's verified-live mark (M8-ab,
// ADR-0031; M8-ac counts the drop, ADR-0034 — the pool's churn
// number the report reads). The table's FIFO order entry is kept —
// a later re-mark refreshes it in place, and expired entries sweep
// cheaply on access.
func (s *Server) dropGoodIP(ip string) {
	if ip == "" {
		return
	}
	s.goodMu.Lock()
	if m, ok := s.good[ip]; ok && m.until.After(time.Now()) {
		m.until = time.Now() // expires on the next access
		s.good[ip] = m
		s.goodMu.Unlock()
		if s.cfg.Stats != nil {
			s.cfg.Stats.GoodDrop()
		}
		return
	}
	s.goodMu.Unlock()
}

// markGoodIP prefers one origin IP for OriginProbeGoodTTL (M8-aa,
// ADR-0028; M8-ab adds the service flag, ADR-0031): the origin-probe
// pass watched this IP answer the exact wire shape a real flow sends,
// so every dial list built while the mark lives starts there. The
// mark NEVER gates anything — an IP without one dials exactly as
// before, first place is its only prize. gvideo marks the origin as
// serving the *.googlevideo.com wildcard: the flag the raced round's
// substitute candidate scopes itself to (same-service substitution
// only; the API fronts keep their own answers).
func (s *Server) markGoodIP(ip string, gvideo bool) {
	if ip == "" {
		return
	}
	s.goodMu.Lock()
	defer s.goodMu.Unlock()
	if s.good == nil {
		s.good = make(map[string]goodMark)
	}
	if _, ok := s.good[ip]; !ok {
		for len(s.good) >= goodIPTableMax && len(s.goodOrder) > 0 {
			oldest := s.goodOrder[0]
			s.goodOrder = s.goodOrder[1:]
			delete(s.good, oldest)
		}
		s.goodOrder = append(s.goodOrder, ip)
	}
	s.good[ip] = goodMark{until: time.Now().Add(config.OriginProbeGoodTTL), gvideo: gvideo}
}

// gvideoBreakState is the googlevideo emission breaker (M8-ae,
// ADR-0041): the ring of recent googlevideo flow terminals and the
// active cooldown's end. The breaker's contract is SILENCE — the
// debug.22 death spiral was the engine answering googlevideo failure
// with MORE googlevideo volume (the app's retry storm × the 4-way
// raced fan-out × the churn kick's 48-handshake pass), which under a
// censor that flags scan-like sources per SNI class is exactly the
// behavior that turns "video laggy" into "video never opens again
// while the feed keeps working". Door-blocks create no terminals, so
// an armed breaker self-extinguishes: the quiet window's stamps age
// out and the next flow resumes service on a decayed flag.
type gvideoBreakState struct {
	mu        sync.Mutex
	terminals []time.Time // recent googlevideo terminal stamps
	until     time.Time   // the cooldown's end (zero = never armed)
}

// noteGvideoTerminal records one googlevideo flow terminal and arms
// the cooldown when GvideoBreakerTerminals of them landed inside
// GvideoBreakerWindow (M8-ae, ADR-0041). Returns true on the arm —
// the caller logs it and counts the cooldown. The arm's jitter is
// proportional (a quarter of the cooldown, capped at 10 s) so
// re-arming staggers; a nil RNG (the test harness) skips it.
func (s *Server) noteGvideoTerminal() bool {
	if s.cfg.GvideoBreakerTerminals <= 0 || s.cfg.GvideoBreakerWindow <= 0 || s.cfg.GvideoBreakerCooldown <= 0 {
		return false // breaker disabled: the exact debug.22 shape
	}
	now := time.Now()
	s.gvideoBreak.mu.Lock()
	defer s.gvideoBreak.mu.Unlock()
	s.gvideoBreak.terminals = append(s.gvideoBreak.terminals, now)
	cut := now.Add(-s.cfg.GvideoBreakerWindow)
	kept := s.gvideoBreak.terminals[:0]
	for _, t := range s.gvideoBreak.terminals {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	s.gvideoBreak.terminals = kept
	if len(s.gvideoBreak.terminals) < s.cfg.GvideoBreakerTerminals || now.Before(s.gvideoBreak.until) {
		return false // under threshold, or already quiet
	}
	cd := s.cfg.GvideoBreakerCooldown
	if jitter := cd / 4; jitter > 0 && s.cfg.RNG != nil {
		if maxJitter := 10 * time.Second; jitter > maxJitter {
			jitter = maxJitter
		}
		if ms := s.cfg.RNG.Intn(int(jitter / time.Millisecond)); ms > 0 {
			cd += time.Duration(ms) * time.Millisecond
		}
	}
	s.gvideoBreak.until = now.Add(cd)
	return true
}

// gvideoCoolingDown reports whether the emission breaker keeps the
// engine silent on googlevideo right now (M8-ae, ADR-0041).
func (s *Server) gvideoCoolingDown() bool {
	s.gvideoBreak.mu.Lock()
	defer s.gvideoBreak.mu.Unlock()
	return time.Now().Before(s.gvideoBreak.until)
}

// GvideoCoolingDown is the exported probe-lane gate (M8-ae,
// ADR-0041): the engine's gvideo loop checks it before every
// origin-probe pass — a pass during the cooldown would be the exact
// churn-kick mistake the round deletes.
func (s *Server) GvideoCoolingDown() bool { return s.gvideoCoolingDown() }

// gvideoDoorBlocked reports whether the emission breaker blocks THIS
// host at the door (M8-ae, ADR-0041): googlevideo hosts only —
// youtube.com, ytimg, ggpht and every other SNI class keeps full
// service through the cooldown, which is why the FEED keeps
// refreshing while the video path goes quiet.
func (s *Server) gvideoDoorBlocked(host string) bool {
	return isGvideoHost(host) && s.gvideoCoolingDown()
}

// goodMarkActive reports whether one origin IP's verified-live mark
// is live right now (M8-ae, ADR-0040): the raced round's skip gate —
// a primary that connected to an origin the engine has SEEN speak
// gets exactly ONE hello (the reference implementations'
// steady-state shape) instead of the four-way fan-out, and the race
// returns only when the front is unknown or the session is young.
func (s *Server) goodMarkActive(ip string) bool {
	if ip == "" {
		return false
	}
	s.goodMu.Lock()
	defer s.goodMu.Unlock()
	m, ok := s.good[ip]
	return ok && m.until.After(time.Now())
}

// liveIPs filters penalized origin IPs out of a dial list and puts
// verified-live ones at its front (M8-y, ADR-0023; M8-aa, ADR-0028).
// The box is a preference, never a verdict: a list the filter would
// EMPTY returns unchanged — an all-penalized set gambles in full
// exactly as before, never an instant fail. Single-IP lists return
// untouched (nothing to prefer). No-op when the knob is disabled.
//
// M8-ac (ADR-0034) rotates the live FRONT per call: the debug.20
// report's www.youtube.com ran 17 flows for 17 failures — every one
// funneled onto the same pinned verified origin (the good-front plus
// the raced rotations starting inside it) while the connection-killer
// worked the lane. Consecutive calls now start the live-front at a
// different verified origin, so consecutive flows spread across the
// origins the engine has SEEN speak instead of concentrating on one.
func (s *Server) liveIPs(ips []net.IP) []net.IP {
	if s.cfg.DeadIPPenalty <= 0 || len(ips) < 2 {
		return ips
	}
	now := time.Now()
	s.deadMu.Lock()
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if m, ok := s.dead[ip.String()]; ok && now.Before(m.until) {
			continue
		}
		out = append(out, ip)
	}
	s.deadMu.Unlock()
	if len(out) == 0 {
		return ips
	}
	// The verified-live front (M8-aa): origins the probe watched answer
	// come first — the primary dial and every rotation then start on
	// the origins the engine has SEEN speak, while the neutral tail
	// keeps its order for the raced rotations to cover. The front
	// itself ROTATES per call (M8-ac): the rotation counter's next
	// value picks which verified origin leads THIS list.
	s.goodMu.Lock()
	var live, neutral []net.IP
	for _, ip := range out {
		if m, ok := s.good[ip.String()]; ok && now.Before(m.until) {
			live = append(live, ip)
		} else {
			neutral = append(neutral, ip)
		}
	}
	s.goodMu.Unlock()
	if len(live) == 0 {
		return out
	}
	if len(live) > 1 {
		rot := int(s.goodRot.Add(1)-1) % len(live)
		if rot > 0 {
			live = append(append([]net.IP(nil), live[rot:]...), live[:rot]...)
		}
	}
	return append(live, neutral...)
}

// DeadBoxStats reports the penalty box's shape for the diagnostic
// report (M8-aa, ADR-0028): how many origin IPs are penalized right
// now, and how many of those carry escalated strikes. Counts only —
// the IPs themselves never leave the box (R21).
func (s *Server) DeadBoxStats() (live, escalated int) {
	now := time.Now()
	s.deadMu.Lock()
	defer s.deadMu.Unlock()
	for _, ip := range s.deadOrder {
		if m, ok := s.dead[ip]; ok && now.Before(m.until) {
			live++
			if m.strikes >= 2 {
				escalated++
			}
		}
	}
	return live, escalated
}

// GoodIPCount reports how many origin IPs are verified-live right now
// (M8-aa, ADR-0028) — the dial lists' preferred front.
func (s *Server) GoodIPCount() int {
	total, _ := s.GoodIPStats()
	return total
}

// GoodIPStats reports the verified-live table's current shape (M8-ab,
// ADR-0031): how many origin IPs are preferred right now, and how
// many of those carry the googlevideo flag — the substitution pool's
// size, the number the report's substitution line and the fast-start
// probe cadence read.
func (s *Server) GoodIPStats() (total, gvideo int) {
	now := time.Now()
	s.goodMu.Lock()
	defer s.goodMu.Unlock()
	for _, ip := range s.goodOrder {
		if m, ok := s.good[ip]; ok && now.Before(m.until) {
			total++
			if m.gvideo {
				gvideo++
			}
		}
	}
	return total, gvideo
}

// GoodGvideoIPList returns the gvideo-flagged verified-live origin IPs
// in FIFO order (M8-ab, ADR-0033): the persistence layer's snapshot —
// the engine writes it to GoogleVideoGoodIPsFile and seeds the next
// session's pool from it.
func (s *Server) GoodGvideoIPList() []string {
	now := time.Now()
	s.goodMu.Lock()
	defer s.goodMu.Unlock()
	var out []string
	for _, ip := range s.goodOrder {
		if m, ok := s.good[ip]; ok && now.Before(m.until) && m.gvideo {
			out = append(out, ip)
		}
	}
	return out
}

// SeedGoodIPs seeds persisted verified-live origin IPs back into the
// good table (M8-ab, ADR-0033): every IP gvideo-flagged with the
// shorter GoodPoolSeedTTL — yesterday's verdict is stale evidence,
// and a seed that went bad overnight self-corrects through the same
// demote-on-death guard every live mark carries.
func (s *Server) SeedGoodIPs(ips []string) {
	if len(ips) == 0 {
		return
	}
	s.goodMu.Lock()
	if s.good == nil {
		s.good = make(map[string]goodMark)
	}
	until := time.Now().Add(config.GoodPoolSeedTTL)
	for _, ip := range ips {
		if ip == "" || net.ParseIP(ip) == nil {
			continue
		}
		if _, ok := s.good[ip]; !ok {
			for len(s.good) >= goodIPTableMax && len(s.goodOrder) > 0 {
				oldest := s.goodOrder[0]
				s.goodOrder = s.goodOrder[1:]
				delete(s.good, oldest)
			}
			s.goodOrder = append(s.goodOrder, ip)
		}
		s.good[ip] = goodMark{until: until, gvideo: true}
	}
	s.goodMu.Unlock()
}

// goodSubstitutes builds one raced round's substitute candidate list
// (M8-ab, ADR-0031): gvideo-flagged verified-live origins that are NOT
// in the flow's current answer set and NOT penalized right now,
// freshest verdict first, capped at max. Google's video edge serves
// any *.googlevideo.com name, so the candidate replays the identical
// fragmented ClientHello — SNI unchanged — onto the pool origin. Nil
// when the knob is off or the pool has nothing new to offer (the
// round then keeps its own-set rotations, exactly the debug.19
// shape). M8-ac (ADR-0034) rotates which verified origins lead the
// list across calls — the same anti-concentration turn liveIPs takes,
// so consecutive substitutes spread across the pool instead of always
// betting its freshest mark.
func (s *Server) goodSubstitutes(own []net.IP, max int) []net.IP {
	if s.cfg.GoodIPSubstitution <= 0 || max <= 0 {
		return nil
	}
	ownSet := make(map[string]bool, len(own))
	for _, ip := range own {
		ownSet[ip.String()] = true
	}
	now := time.Now()
	type cand struct {
		ip    net.IP
		until time.Time
	}
	var out []cand
	s.goodMu.Lock()
	for _, key := range s.goodOrder {
		m, ok := s.good[key]
		if !ok || !m.gvideo || !now.Before(m.until) {
			continue
		}
		if ownSet[key] {
			continue // already a candidate this round
		}
		if ip := net.ParseIP(key); ip != nil {
			out = append(out, cand{ip: ip, until: m.until})
		}
	}
	s.goodMu.Unlock()
	if len(out) == 0 {
		return nil
	}
	// Penalized pool origins are skipped: the box outranks a stale
	// good mark exactly as in every dial list.
	s.deadMu.Lock()
	kept := out[:0]
	for _, c := range out {
		if m, ok := s.dead[c.ip.String()]; ok && now.Before(m.until) {
			continue
		}
		kept = append(kept, c)
	}
	s.deadMu.Unlock()
	out = kept
	if len(out) == 0 {
		return nil
	}
	// Freshest verdict first: the most recently verified pool origin
	// leads the candidate's dial list — then the rotation (M8-ac):
	// consecutive rounds start that ordering at a different pool
	// origin, so the substitute bet spreads too.
	sort.Slice(out, func(i, j int) bool { return out[i].until.After(out[j].until) })
	if len(out) > max {
		out = out[:max]
	}
	if len(out) > 1 {
		rot := int(s.goodRot.Add(1)-1) % len(out)
		if rot > 0 {
			out = append(append([]cand(nil), out[rot:]...), out[:rot]...)
		}
	}
	ips := make([]net.IP, len(out))
	for i, c := range out {
		ips[i] = c.ip
	}
	return ips
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
		if m, ok := s.dead[ip]; ok && m.until.After(now) {
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

// ProbeSummary is one origin-probe pass's outcome (M8-aa, ADR-0028):
// counts only — the pass's names, the answer IPs it verified, and the
// verdicts it handed the box and the good table. No IP, no name, ever
// (R21).
type ProbeSummary struct {
	Names    int
	IPs      int
	Answered int
	Silent   int
	Reset    int
}

// probeOneOrigin answers whether one origin IP speaks to the EXACT wire
// shape a real flow sends (M8-aa, ADR-0028): one fragmented ClientHello
// with the name a flow would carry as SNI, through the engine's one netx
// dialer (ADR-0011 loop safety), inside one OriginProbeBudget. This is
// the verified-prewarm half of the fix — the same evidence a flow's
// dead-origin classification would produce, bought in the background
// where no app flow is waiting on it. The verdict classification mirrors
// ProbeDeadOrigin's (timeout → silent, reset → reset, anything else that
// comes back → answered); unlike the report's one-shot SNI-vs-IP probe,
// the hello here is FRAGMENTED with the live preset, because the tuple
// being measured is the one a real flow will use.
func (s *Server) probeOneOrigin(ctx context.Context, ip net.IP, sni string, port int, preset fragment.Preset, fcfg fragment.Config) string {
	conn, err := s.cfg.Dialer.DialHost(ctx, "tcp", ip.String(), port, []net.IP{ip})
	if err != nil {
		return "silent" // not even a TCP answer: the IP's path is dead
	}
	defer conn.Close()
	// M8-ae (ADR-0042): every googlevideo-SNI hello the engine emits
	// is counted — the probes ride the same emission budget the flows
	// do, and the report's hellos number is the volume the censor
	// actually sees.
	if isGvideoHost(sni) && s.stats != nil {
		s.stats.GvideoHello()
	}
	fconn := fragment.NewConn(conn, preset, fcfg, s.cfg.RNG)
	tlsCfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true}
	tc := tls.Client(fconn, tlsCfg)
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
	// Any other handshake failure means the origin SPOKE within the budget.
	return "answered"
}

// ProbeOrigins runs one origin-probe pass (M8-aa, ADR-0028): for each
// name in priority order, resolve it and verify every answer IP that is
// neither verified-live nor penalized right now — the unknown middles of
// the answer sets, exactly the IPs a cold flow would gamble on. Answered
// verdicts mark the IP verified-live (dial lists start there); silent
// and reset verdicts mark it dead with the probe's direct-evidence TTL
// (strike-escalated like every dead mark). Already-marked IPs are
// skipped both ways — the box's verdict stands until it lapses, and a
// verified-live mark is simply re-learned on a later pass once it does.
// The pass stops at maxIPs verifications or ctx's end; the per-probe
// budget is OriginProbeBudget, the concurrency cap OriginProbeInflight.
// port is the origin port (443 in production; tests inject theirs).
// Returns the counters-only summary; the pass also logs one line.
func (s *Server) ProbeOrigins(ctx context.Context, names []string, port int, maxIPs int) ProbeSummary {
	var sum ProbeSummary
	if len(names) == 0 || maxIPs <= 0 {
		return sum
	}
	// M8-ae (ADR-0041): the pass is a TRICKLE, never a burst — the
	// budget is clamped to config.OriginProbePerPass (10) no matter
	// what the caller asks for. Every job is a full TLS handshake with
	// a real SNI; the tester's reference implementations send zero
	// background handshakes, and the 48-job pass was scan-shaped
	// volume against the very censor the pass was measuring.
	if maxIPs > config.OriginProbePerPass {
		maxIPs = config.OriginProbePerPass
	}
	// The pass's wire shape is captured once (D102's discipline: one
	// preset per pass, the method's current shape — the same one every
	// flow this method serves is sending right now).
	preset, fcfg := s.fragmentData()
	now := time.Now()
	type probeJob struct {
		ip     net.IP
		sni    string
		gvideo bool
		rehab  bool // a re-verification of a currently-boxed IP (M8-ac, ADR-0034)
	}
	var jobs []probeJob
	for _, name := range names {
		if ctx.Err() != nil || len(jobs) >= maxIPs {
			break
		}
		sum.Names++
		ips, err := s.resolve(name)
		if err != nil {
			continue
		}
		gvideo := isGvideoHost(name)
		for _, ip := range dedupeIPs(ips) {
			if len(jobs) >= maxIPs {
				break
			}
			key := ip.String()
			s.deadMu.Lock()
			_, boxed := s.dead[key]
			if boxed {
				if m, ok := s.dead[key]; ok && !now.Before(m.until) {
					boxed = false // expired: the entry is strike memory only
				}
			}
			s.deadMu.Unlock()
			s.goodMu.Lock()
			_, live := s.good[key]
			if live {
				if m, ok := s.good[key]; ok && !now.Before(m.until) {
					live = false // expired: re-learn it
				}
			}
			s.goodMu.Unlock()
			if boxed || live {
				continue
			}
			jobs = append(jobs, probeJob{ip: ip, sni: name, gvideo: gvideo})
		}
	}
	// The rehabilitation sample (M8-ac, ADR-0034): up to
	// ProbeRehabPerPass currently-boxed IPs get their verdict
	// RE-CHECKED — soonest-expiring first — because the debug.20
	// report proved the box now holds stale verdicts (penalized=12
	// while the one-shot dead-origin probe answered on those same
	// IPs): under the flow-killing regime a death says nothing
	// stable about the IP, and the box must clear on fresh evidence,
	// not on a 5-30-minute expiry. An "answered" verdict clears the
	// box entry and marks the IP good (counted); anything else
	// refreshes the box with the class-weighted TTL.
	s.deadMu.Lock()
	type rehabCand struct {
		key   string
		until time.Time
	}
	var rehabs []rehabCand
	for _, key := range s.deadOrder {
		if len(rehabs) >= config.ProbeRehabPerPass || len(jobs) >= maxIPs {
			break
		}
		if m, ok := s.dead[key]; ok && now.Before(m.until) {
			rehabs = append(rehabs, rehabCand{key: key, until: m.until})
		}
	}
	s.deadMu.Unlock()
	sort.Slice(rehabs, func(i, j int) bool { return rehabs[i].until.Before(rehabs[j].until) })
	for _, rc := range rehabs {
		if len(jobs) >= maxIPs {
			break
		}
		if ip := net.ParseIP(rc.key); ip != nil {
			// The rehab hello carries a neutral SNI (it verifies
			// the PATH, not a service), so a rehabilitated mark is
			// never gvideo-flagged — front-of-list preference only,
			// never a substitution candidate (the wildcard-service
			// claim needs the googlevideo-shaped verdict).
			jobs = append(jobs, probeJob{ip: ip, sni: "www.google.com", gvideo: false, rehab: true})
		}
	}
	if len(jobs) == 0 {
		if s.cfg.Stats != nil {
			s.cfg.Stats.ProbePass()
		}
		return sum
	}
	sum.IPs = 0
	sem := make(chan struct{}, config.OriginProbeInflight)
	var wg sync.WaitGroup
	verdicts := make([]string, len(jobs))
	probedIPs := make([]string, len(jobs))
	probeGvideo := make([]bool, len(jobs))
	probeRehab := make([]bool, len(jobs))
	for i, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j probeJob) {
			defer wg.Done()
			defer func() { <-sem }()
			pctx, cancel := context.WithTimeout(ctx, config.OriginProbeBudget)
			defer cancel()
			verdicts[i] = s.probeOneOrigin(pctx, j.ip, j.sni, port, preset, fcfg)
			probedIPs[i] = j.ip.String()
			probeGvideo[i] = j.gvideo
			probeRehab[i] = j.rehab
		}(i, j)
	}
	wg.Wait()
	rehabbed := 0
	for i, v := range verdicts {
		if probedIPs[i] == "" {
			continue // the pass was canceled before this probe launched
		}
		sum.IPs++
		switch v {
		case "answered":
			sum.Answered++
			if probeRehab[i] {
				// The rehabilitation (M8-ac): fresh direct evidence
				// the IP answers outranks the stale death that boxed
				// it — the box entry is cleared entirely, the good
				// mark takes its place, the counter says the box
				// was wrong.
				s.clearDeadIP(probedIPs[i])
				rehabbed++
				if s.cfg.Stats != nil {
					s.cfg.Stats.ProbeRehabilitated()
				}
			}
			s.markGoodIP(probedIPs[i], probeGvideo[i])
			if s.cfg.Stats != nil {
				s.cfg.Stats.ProbeAnswered()
			}
		case "reset":
			sum.Reset++
			// M8-ac (ADR-0034): the probe's fresh reset is the same
			// TEMPORAL signal a flow's reset death is — a brief box,
			// never the old 10-minute IP verdict, and never a demote.
			s.boxIP(probedIPs[i], config.OriginProbeResetTTL)
			if s.cfg.Stats != nil {
				s.cfg.Stats.ProbeReset()
			}
		default:
			sum.Silent++
			s.markDeadIPTTL(probedIPs[i], config.OriginProbeDeadTTL)
			if s.cfg.Stats != nil {
				s.cfg.Stats.ProbeSilent()
			}
		}
		if s.cfg.Stats != nil {
			s.cfg.Stats.ProbeIP()
		}
	}
	if s.cfg.Stats != nil {
		s.cfg.Stats.ProbePass()
	}
	if s.cfg.Logger != nil {
		// R21: counts only — never a name, never an IP.
		s.cfg.Logger.Info(fmt.Sprintf("origin probe: names=%d ips=%d answered=%d silent=%d reset=%d rehab=%d",
			sum.Names, sum.IPs, sum.Answered, sum.Silent, sum.Reset, rehabbed))
	}
	return sum
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
