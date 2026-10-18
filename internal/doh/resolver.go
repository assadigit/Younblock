// Package doh resolves names through DNS-over-HTTPS (file 01, section 6),
// replicating the old Python app's request shape exactly: a GET request to
// `...?dns=<base64url, no padding>` with an `accept: application/dns-message`
// header plus the extra `type=A&ct=application/dns-message` query parameters.
//
// The DoH connection dials the server's bootstrap IPs directly through
// netx.Dialer (never through our own proxy) and its TLS ClientHello is
// fragmented with the fragment.Conn wrapper.
package doh

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/sync/singleflight"
)

// Sentinel error kinds; the engine maps them to UI error codes.
var (
	// ErrUnreachable means no secure-DNS server answered.
	ErrUnreachable = errors.New("DNS_UNREACHABLE")
	// ErrBlockedAnswer means every answer was in a private/reserved range.
	ErrBlockedAnswer = errors.New("DNS_BLOCKED_ANSWER")
	// ErrNoAnswer means the server answered with no usable A records.
	ErrNoAnswer = errors.New("no usable answer")
	// ErrRecentlyFailed means the negative cache answered.
	ErrRecentlyFailed = errors.New("lookup failed recently")
)

// Config sets up a resolver.
type Config struct {
	// Servers are tried in order until one answers.
	Servers []config.DoHServer
	// Timeout bounds one server attempt (including its connection).
	Timeout time.Duration
	// BlockPrivate drops answers in private/reserved ranges.
	BlockPrivate bool
	// QueryAAAA also asks for AAAA records (off by default; IPv4 only).
	QueryAAAA bool
	// TLS overrides the client TLS config (tests inject a trust pool here).
	TLS *tls.Config
	// Control is the netx socket hook (nil on desktop).
	Control netx.ControlFunc
	// FragmentPreset and FragmentConfig shape the DoH ClientHello splitting.
	FragmentPreset fragment.Preset
	FragmentConfig fragment.Config
	// RNG drives fragmentation randomness.
	RNG fragment.RNG
	// Logger receives one line per server failure and fallback (never names).
	Logger interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
	}
	// LogHostnames allows hostnames in those log lines (default off).
	LogHostnames bool
	// Stats receives the dnsOK / dnsFail counts (may be nil in tests).
	Stats interface {
		DnsOK()
		DnsFail()
	}
	// Now is the clock (tests inject a fake).
	Now func() time.Time
}

// Resolver answers lookups from cache, singleflighted server queries, or stale
// entries when everything fails.
type Resolver struct {
	cfg      Config
	client   *http.Client
	cache    *cache
	sf       singleflight.Group
	sticky   atomic.Int32 // index of the last server that answered
	lastGood atomic.Value // string: name of the last server that answered
	lastTTL  atomic.Int64 // nanoseconds of the most recent successful answer
	mu       sync.Mutex   // guards server URL host map construction
	hosts    map[string]int

	fragMu     sync.Mutex
	fragPreset fragment.Preset
	fragConfig fragment.Config

	resets atomic.Int32
}

// New builds a ready resolver.
func New(cfg Config) *Resolver {
	if cfg.Timeout <= 0 {
		cfg.Timeout = config.DoHTimeout
	}
	if len(cfg.Servers) == 0 {
		cfg.Servers = config.Defaults()
	}
	r := &Resolver{cfg: cfg, cache: newCache(config.DNSCacheMaxEntries, cfg.Now),
		fragPreset: cfg.FragmentPreset, fragConfig: cfg.FragmentConfig}
	transport := &http.Transport{
		// Explicitly no proxy: DoH never loops through our own proxy port (R5).
		Proxy:               nil,
		ForceAttemptHTTP2:   true,
		DialContext:         r.dialBootstrap,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 2,
	}
	if cfg.TLS != nil {
		transport.TLSClientConfig = cfg.TLS.Clone()
	}
	r.client = &http.Client{Transport: transport}
	r.hosts = make(map[string]int, len(cfg.Servers))
	for i, s := range cfg.Servers {
		if u, err := url.Parse(s.URL); err == nil && u.Hostname() != "" {
			r.hosts[u.Hostname()] = i
		}
	}
	return r
}

// Lookup returns the addresses for host.
func (r *Resolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return nil, fmt.Errorf("empty name")
	}
	if ip := net.ParseIP(host); ip != nil {
		// IP literals skip DNS entirely (old app behavior, file 01 section 2).
		return []net.IP{ip}, nil
	}
	res := r.cache.lookup(host)
	switch res.state {
	case stateFresh:
		if r.cfg.Stats != nil {
			r.cfg.Stats.DnsOK()
		}
		return res.ips, nil
	case stateNegative:
		if r.cfg.Stats != nil {
			r.cfg.Stats.DnsFail()
		}
		return nil, fmt.Errorf("%w: %s", ErrRecentlyFailed, res.code)
	case stateStale:
		// A fresh lookup is still attempted below; stale answers are only
		// served when the servers fail.
	}

	v, err, _ := r.sf.Do(host, func() (any, error) {
		ips, server, ttl, err := r.askServers(ctx, host)
		if err == nil {
			r.cache.putSuccess(host, ips, ttl)
			r.sticky.Store(int32(server))
			r.lastGood.Store(r.cfg.Servers[server].Name)
			return ips, nil
		}
		// Serve stale if we still may (R5).
		if stale := r.cache.lookup(host); stale.state == stateStale {
			if r.cfg.Logger != nil {
				// The reason rides in the message: the log ring that feeds the
				// diagnostic report drops attrs (M8-r — the debug.10 report
				// showed these lines bare).
				r.cfg.Logger.Warn("dns servers failed; serving stale answer reason=" + errKind(err))
			}
			return stale.ips, nil
		}
		r.cache.putNegative(host, codeFor(err))
		return nil, err
	})
	if err != nil {
		if r.cfg.Stats != nil {
			r.cfg.Stats.DnsFail()
		}
		return nil, err
	}
	if r.cfg.Stats != nil {
		r.cfg.Stats.DnsOK()
	}
	ips, _ := v.([]net.IP)
	return ips, nil
}

// LastServerName reports which secure-DNS server answered last.
func (r *Resolver) LastServerName() string {
	if v, ok := r.lastGood.Load().(string); ok && v != "" {
		return v
	}
	if len(r.cfg.Servers) > 0 {
		return r.cfg.Servers[0].Name
	}
	return ""
}

// SetFragment updates the ClientHello fragmentation used for new DoH
// connections (method changes apply immediately while On).
func (r *Resolver) SetFragment(p fragment.Preset, cfg fragment.Config) {
	r.fragMu.Lock()
	r.fragPreset = p
	r.fragConfig = cfg
	r.fragMu.Unlock()
}

// fragmentData returns the current fragmentation settings.
func (r *Resolver) fragmentData() (fragment.Preset, fragment.Config) {
	r.fragMu.Lock()
	defer r.fragMu.Unlock()
	return r.fragPreset, r.fragConfig
}

// Invalidate drops a host's still-fresh cached answer to stale (M8-o,
// ADR-0014 — the dial-failure feedback loop). The proxy calls this when
// it dialed every address of the cached set and none connected: on a
// filtered network that answer set is suspect, and the client's next
// retry should get a fresh server answer, not the same set until TTL
// expiry (the 120 s minimum TTL otherwise turns one bad answer into a
// minutes-long retry loop — the rr*.googlevideo.com hostnames map to
// few addresses of one cache node each, so a filtered node poisons the
// whole set). Serve-stale is preserved: if the servers then fail, the
// old answer is still better than nothing. IP literals and negatives
// are no-ops (literals are never cached; negatives already re-query at
// their own short expiry).
func (r *Resolver) Invalidate(host string) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || net.ParseIP(host) != nil {
		return
	}
	r.cache.invalidate(host)
}

// Reset clears the cache and the sticky server (used when the network
// changes; file 03, R19).
func (r *Resolver) Reset() {
	r.cache.reset()
	r.sticky.Store(0)
	r.lastGood = atomic.Value{}
	r.resets.Add(1)
}

// ResetCount reports how many times Reset ran (tests and diagnostics).
func (r *Resolver) ResetCount() int32 { return r.resets.Load() }

// Len reports the cache entry count (diagnostics only).
func (r *Resolver) Len() int { return r.cache.len() }

// askServers tries servers in order starting from the sticky one.
func (r *Resolver) askServers(ctx context.Context, host string) ([]net.IP, int, time.Duration, error) {
	n := len(r.cfg.Servers)
	start := int(r.sticky.Load())
	if start < 0 || start >= n {
		start = 0
	}
	var firstErr error
	var firstFailed string
	var firstFailedIdx int
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if ctx.Err() != nil {
			break
		}
		srv := r.cfg.Servers[idx]
		cctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
		ips, ttl, err := r.queryOne(cctx, srv, host)
		cancel()
		if err == nil {
			if firstFailed != "" && r.cfg.Logger != nil {
				r.cfg.Logger.Info(fmt.Sprintf("dns server failed; using the next one failed=#%d now=#%d", firstFailedIdx+1, idx+1),
					"failed", firstFailed, "nowUsing", srv.Name)
			}
			return ips, idx, ttl, nil
		}
		if firstErr == nil {
			firstErr = err
			firstFailed = srv.Name
			firstFailedIdx = idx
		}
		if r.cfg.Logger != nil {
			// Server index and reason ride in the message so the diagnostic
			// report's log ring (which drops attrs) carries the evidence — the
			// debug.10 report's three bare "dns server failed" lines could not
			// say timeout vs reset vs bad answer (M8-r). The configured name
			// stays an attr for the rotating file log.
			r.cfg.Logger.Warn(fmt.Sprintf("dns server failed server=#%d reason=%s", idx+1, errKind(err)),
				"server", srv.Name)
		}
	}
	if firstErr == nil {
		firstErr = ctx.Err()
	}
	return nil, 0, 0, firstErr
}

// queryOne performs the exact v1-style GET against one server.
func (r *Resolver) queryOne(ctx context.Context, srv config.DoHServer, host string) ([]net.IP, time.Duration, error) {
	wire, id, err := buildQuery(host, r.cfg.QueryAAAA)
	if err != nil {
		return nil, 0, err
	}
	q := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(wire)
	full := strings.Replace(srv.URL, "{dns}", q, 1)
	// The v1 request shape: the extra type/ct parameters ride along (D7).
	sep := "?"
	if u, perr := url.Parse(full); perr == nil && u.RawQuery != "" {
		sep = "&"
	}
	full = full + sep + "type=A&ct=application/dns-message"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("accept", "application/dns-message")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnreachable, errKind(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, 0, fmt.Errorf("%w: status %d", ErrUnreachable, resp.StatusCode)
	}
	media := resp.Header.Get("content-type")
	if i := strings.IndexByte(media, ';'); i >= 0 {
		media = media[:i]
	}
	if strings.TrimSpace(strings.ToLower(media)) != "application/dns-message" {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, 0, fmt.Errorf("%w: bad content-type", ErrUnreachable)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnreachable, errKind(err))
	}
	ips, ttl, err := parseAnswer(body, id, r.cfg.QueryAAAA)
	if err != nil {
		return nil, 0, err
	}
	if r.cfg.BlockPrivate {
		ips = filterPrivate(ips)
		if len(ips) == 0 {
			return nil, 0, fmt.Errorf("%w: all answers filtered", ErrBlockedAnswer)
		}
	}
	return ips, ttl, nil
}

// buildQuery makes the wire-format DNS question.
func buildQuery(host string, withAAAA bool) (wire []byte, id uint16, err error) {
	name, err := dnsmessage.NewName(host + ".")
	if err != nil {
		return nil, 0, fmt.Errorf("invalid name: %w", err)
	}
	id = uint16(time.Now().UnixNano() & 0xffff)
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id, RecursionDesired: true})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, 0, err
	}
	if err := b.Question(dnsmessage.Question{
		Name:  name,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}); err != nil {
		return nil, 0, err
	}
	if withAAAA {
		if err := b.Question(dnsmessage.Question{
			Name:  name,
			Type:  dnsmessage.TypeAAAA,
			Class: dnsmessage.ClassINET,
		}); err != nil {
			return nil, 0, err
		}
	}
	wire, err = b.Finish()
	if err != nil {
		return nil, 0, err
	}
	return wire, id, nil
}

// parseAnswer collects every A (and optionally AAAA) record.
func parseAnswer(body []byte, id uint16, withAAAA bool) ([]net.IP, time.Duration, error) {
	var p dnsmessage.Parser
	hdr, err := p.Start(body)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: bad message", ErrUnreachable)
	}
	if hdr.ID != id {
		return nil, 0, fmt.Errorf("%w: id mismatch", ErrUnreachable)
	}
	if hdr.RCode != dnsmessage.RCodeSuccess {
		return nil, 0, fmt.Errorf("%w: rcode %d", ErrNoAnswer, hdr.RCode)
	}
	if err := p.SkipAllQuestions(); err != nil && !errors.Is(err, dnsmessage.ErrSectionDone) {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnreachable, errKind(err))
	}
	answers, err := p.AllAnswers()
	if err != nil && !errors.Is(err, dnsmessage.ErrSectionDone) {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnreachable, errKind(err))
	}
	var ips []net.IP
	ttl := uint32(0)
	haveTTL := false
	for _, ans := range answers {
		switch rr := ans.Body.(type) {
		case *dnsmessage.AResource:
			ips = append(ips, net.IP{rr.A[0], rr.A[1], rr.A[2], rr.A[3]})
		case *dnsmessage.AAAAResource:
			if withAAAA {
				ip := make(net.IP, 16)
				copy(ip, rr.AAAA[:])
				ips = append(ips, ip)
			}
		}
		if !haveTTL || ans.Header.TTL < ttl {
			ttl = ans.Header.TTL
			haveTTL = true
		}
	}
	if len(ips) == 0 {
		return nil, 0, fmt.Errorf("%w: no A records", ErrNoAnswer)
	}
	if !haveTTL {
		ttl = 60
	}
	return ips, time.Duration(ttl) * time.Second, nil
}

// filterPrivate drops loopback, private, link-local, multicast and
// unspecified addresses (file 01, section 4: Block private targets).
func filterPrivate(ips []net.IP) []net.IP {
	out := ips[:0:0]
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
			ip.IsInterfaceLocalMulticast() {
			continue
		}
		out = append(out, ip)
	}
	return out
}

// dialBootstrap is the http.Transport DialContext: it ignores system DNS and
// dials the URL host's bootstrap IPs directly through netx, wrapping the
// connection so the DoH ClientHello itself is fragmented.
func (r *Resolver) dialBootstrap(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	idx, known := r.hosts[host]
	r.mu.Unlock()
	if !known {
		return nil, fmt.Errorf("no bootstrap addresses for %s", host)
	}
	srv := r.cfg.Servers[idx]
	ips := make([]net.IP, 0, len(srv.Bootstrap))
	for _, b := range srv.Bootstrap {
		if ip := net.ParseIP(b); ip != nil {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("server has no valid bootstrap addresses")
	}
	d := netx.New(r.cfg.Timeout, r.cfg.Timeout, r.cfg.Control)
	// Race the bootstrap IPs Happy-Eyeballs-style (ADR-0013's dialer,
	// M8-o): a filtered network that blackholes the front bootstrap
	// silently must not cost a full Timeout before the next IP — the
	// same reasoning as the door's DialHost, on the resolver's own path.
	conn, err := d.DialHost(ctx, network, host, port, ips)
	if err != nil {
		return nil, err
	}
	return fragment.NewConn(conn, r.cfg.FragmentPreset, r.cfg.FragmentConfig, r.cfg.RNG), nil
}

// codeFor maps an error to its UI code string.
func codeFor(err error) string {
	switch {
	case errors.Is(err, ErrBlockedAnswer):
		return ErrBlockedAnswer.Error()
	case errors.Is(err, ErrNoAnswer):
		return ErrNoAnswer.Error()
	default:
		return ErrUnreachable.Error()
	}
}

// errKind reduces an error to a short, hostless/IPless reason string.
func errKind(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		return "network"
	}
	msg := err.Error()
	if i := strings.IndexByte(msg, ':'); i > 0 {
		msg = msg[:i]
	}
	return msg
}
