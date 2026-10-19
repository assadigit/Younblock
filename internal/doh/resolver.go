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
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/http2"
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
	// HedgeDelay is the stagger between servers in one hedged walk
	// (M8-z, ADR-0025 §6.3): server N+1 launches HedgeDelay after
	// server N when N has not answered yet; a fast failure launches
	// the next server immediately. Zero → the config default
	// (config.DoHHedgeDelay); negative → no hedging (the exact serial
	// M8-y shape: the next server is asked only when the previous one
	// failed).
	HedgeDelay time.Duration
	// CodeStore is the file the learned googlevideo sn-<code> list
	// persists to (M8-z, ADR-0025 §6.4; empty → memory-only). The
	// engine points it at <configDir>/googlevideo-codes.txt.
	CodeStore string
	// NegativeReask is how often a host whose negative-cache entry says
	// DNS_UNREACHABLE may RE-ASK the servers instead of failing
	// instantly (M8-y, ADR-0023): that code is evidence about the
	// resolver's own path, not a verdict about the name, so a hit
	// re-asks through the singleflight — at most one walk per window per
	// host (bounded probing during a real outage; the blip shape recovers
	// in one walk instead of dying for the negative's whole TTL).
	// Server-verdict negatives (DNS_BLOCKED_ANSWER, no usable answer)
	// never re-ask. Zero → the config default; negative → disabled (the
	// exact M8-w shape).
	NegativeReask time.Duration
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

	// M8-z (ADR-0025 §8.3.3): the DoH diagnostics the report renders —
	// dial/query/hedge counters and the recent per-server failure
	// records (server number, phase, reason, latency).
	queries   atomic.Int64
	dials     atomic.Int64
	hedgeWins atomic.Int64
	failMu    sync.Mutex
	failRecs  []FailureRecord

	// Per-server hedge wins (index-aligned with cfg.Servers; grown
	// lazily under failMu).
	hedgeByServer []int

	// The warm/rewarm state (§6.2): one warm in flight at a time, and
	// a re-warm scheduled after a connection-phase failure (within 1 s,
	// debounced).
	warming  atomic.Bool
	rewarmAt atomic.Int64 // unix ms before which no re-warm is scheduled

	// The learned googlevideo sn-<code> table (§6.4): ordered, capped,
	// persisted to CodeStore when it changes.
	codeMu    sync.Mutex
	codes     []string
	codeIndex map[string]bool
	codeDirty bool

	// The negative re-ask rate limiter (M8-y, ADR-0023): host → unix ms
	// of the last re-ask, bounded FIFO like the cache itself.
	reaskMu    sync.Mutex
	reask      map[string]int64
	reaskOrder []string

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
	if cfg.NegativeReask == 0 {
		cfg.NegativeReask = config.DNSCacheNegativeReask
	}
	if cfg.NegativeReask < 0 {
		cfg.NegativeReask = 0
	}
	if cfg.HedgeDelay == 0 {
		cfg.HedgeDelay = config.DoHHedgeDelay
	}
	if cfg.HedgeDelay < 0 {
		cfg.HedgeDelay = 0 // no hedging: the exact serial M8-y shape
	}
	if len(cfg.Servers) == 0 {
		cfg.Servers = config.Defaults()
	}
	r := &Resolver{cfg: cfg, cache: newCache(config.DNSCacheMaxEntries, cfg.Now),
		reask:      make(map[string]int64),
		fragPreset: cfg.FragmentPreset, fragConfig: cfg.FragmentConfig,
		codeIndex: make(map[string]bool)}
	// M8-z (ADR-0025 §6.1): ONE shared HTTP/2 transport. The x/net
	// http2.Transport dials once per server address and multiplexes
	// every query over that single connection — a burst of fresh
	// per-video names costs ONE fragmented TLS handshake, not one per
	// name. DialTLSContext runs the TCP dial through dialBootstrap (the
	// bootstrap IPs, netx, the protect hook — ADR-0011) and the TLS
	// handshake over the fragment.Conn wrapper, so the DoH ClientHello
	// stays fragmented exactly as before. ReadIdleTimeout/PingTimeout
	// keep the connection's health checked while it lives: a dead
	// connection is noticed and replaced instead of surfacing as slow
	// failures on the next burst. (The old http.Transport with
	// MaxIdleConnsPerHost=2 raced concurrent first queries into
	// separate handshakes and had no keep-alive pings.)
	transport := &http2.Transport{
		DialTLSContext:  r.dialTLSContext,
		ReadIdleTimeout: config.DoHReadIdleTimeout,
		PingTimeout:     config.DoHPingTimeout,
		// Explicitly no proxy fallback: DoH never loops through our own
		// proxy port (R5) — http2.Transport has no Proxy field to begin
		// with, which is the guarantee stated outright.
	}
	r.client = &http.Client{Transport: transport}
	r.hosts = make(map[string]int, len(cfg.Servers))
	for i, s := range cfg.Servers {
		if u, err := url.Parse(s.URL); err == nil && u.Hostname() != "" {
			r.hosts[u.Hostname()] = i
		}
	}
	r.loadCodes()
	return r
}

// FailureRecord is one recent per-server failure the report's DNS
// detail section renders (M8-z, ADR-0027 §8.3.3): which server number,
// which phase (tcp / tls / http / status / parse), the short reason,
// the latency. Bounded (failureLogCap); newest last.
type FailureRecord struct {
	Server int    // 1-based server number
	Phase  string // tcp | tls | http | status | parse
	Reason string
	MS     int64
}

// failureLogCap bounds the failure records kept for the report.
const failureLogCap = 32

// DoHDiag is the resolver's diagnostics snapshot for the report.
type DoHDiag struct {
	Queries       int64
	Dials         int64
	Reuse         int64 // queries served over an existing connection (queries − dials, floored)
	HedgeWins     int64
	HedgeByServer []int // per-server hedge wins, index-aligned with the server list
	Failures      []FailureRecord
	Codes         []string // the learned googlevideo sn-<code> list
	StickyServer  string
}

// Diagnostic returns the DoH counters and recent failures (report only).
func (r *Resolver) Diagnostic() DoHDiag {
	r.failMu.Lock()
	fails := append([]FailureRecord(nil), r.failRecs...)
	hedgeBy := append([]int(nil), r.hedgeByServer...)
	r.failMu.Unlock()
	if len(hedgeBy) < len(r.cfg.Servers) {
		grown := make([]int, len(r.cfg.Servers))
		copy(grown, hedgeBy)
		hedgeBy = grown
	}
	reuse := r.queries.Load() - r.dials.Load()
	if reuse < 0 {
		reuse = 0
	}
	return DoHDiag{
		Queries:       r.queries.Load(),
		Dials:         r.dials.Load(),
		Reuse:         reuse,
		HedgeWins:     r.hedgeWins.Load(),
		HedgeByServer: hedgeBy,
		Failures:      fails,
		Codes:         r.GvideoCodes(),
		StickyServer:  r.LastServerName(),
	}
}

// noteFailure records one per-server failure (bounded) for the report.
func (r *Resolver) noteFailure(server int, phase, reason string, ms int64) {
	r.failMu.Lock()
	r.failRecs = append(r.failRecs, FailureRecord{Server: server, Phase: phase, Reason: reason, MS: ms})
	if len(r.failRecs) > failureLogCap {
		r.failRecs = r.failRecs[len(r.failRecs)-failureLogCap:]
	}
	r.failMu.Unlock()
}

// phaseOf classifies a query failure into the report's phase vocabulary
// (tcp / tls / http / status / parse — ADR-0027 §8.3.3). The dial and
// handshake phases ride the error as a *phaseError; everything after
// the connection is classified by where queryOne caught it.
func phaseOf(err error) string {
	var pe *phaseError
	if errors.As(err, &pe) {
		return pe.phase
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	return "http"
}

// phaseError marks which leg of a DoH exchange failed: the TCP dial
// ("tcp") or the TLS handshake ("tls"). queryOne adds "status",
// "http" and "parse" shapes itself; phaseOf unwraps this marker so the
// report's DNS detail can name the failing phase (M8-z, ADR-0027
// §8.3.3: path interference shows as tls, a server problem as
// status/parse).
type phaseError struct {
	phase string
	err   error
}

func (e *phaseError) Error() string { return e.phase + ": " + e.err.Error() }
func (e *phaseError) Unwrap() error { return e.err }

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
		// M8-y (ADR-0023): a DNS_UNREACHABLE negative is evidence about
		// the RESOLVER'S OWN PATH (all servers unreachable / network /
		// bad status), not a verdict about the name — the debug.16
		// report measured the lockout it caused at the door: 50 dead
		// flows, the failure lines pairing at the same millisecond, one
		// DoH blip's tail amplified through the 15 s TTL. Such a hit
		// re-asks the servers (rate-limited below, singleflight-collapsed)
		// instead of failing instantly; server-verdict negatives keep the
		// instant fail — the servers SPOKE, re-asking a verdict is
		// pointless.
		if res.code == ErrUnreachable.Error() && r.reaskOK(host) {
			if r.cfg.Logger != nil {
				// The count of these lines in the next report IS the
				// evidence: lockouts the re-ask absorbed (M8-r's lesson:
				// the ring drops attrs, so the message carries it all).
				r.cfg.Logger.Warn("dns negative re-ask (path failure, not a name verdict)")
			}
			break // fall through to the singleflight ask below
		}
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
			// M8-z (ADR-0025 §6.4): learn the googlevideo cache-node code
			// so the background prewarm can resolve its rr1..rr5 siblings
			// before the next fresh per-video hostname needs them.
			if code := googleVideoCode(host); code != "" {
				r.noteGvideoCode(code)
			}
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
		// M8-y (ADR-0023): the walk that wrote a DNS_UNREACHABLE negative
		// IS this window's probe — a negative hit inside the window fails
		// fast instead of walking again seconds after a failure.
		if codeFor(err) == ErrUnreachable.Error() {
			r.noteReask(host)
		}
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

// ClearNegative drops host's negative-cache entry and nothing else
// (M8-w, ADR-0021's companion fix — the M8-r sweep lesson extended to
// the periodic checks). The debug.14 report showed the watchdog's dns
// step failing as DNS_UNREACHABLE with the network otherwise flowing:
// a dirty-window DoH blip had negative-cached the probe name (the very
// name app flows resolve), and the check read the 15 s memory instead
// of asking any server. A health probe's question is "what do the
// servers say NOW"; success entries stay (a good cached answer is
// still good — demoting it would only tax the tunnel path).
func (r *Resolver) ClearNegative(host string) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || net.ParseIP(host) != nil {
		return
	}
	r.cache.clearNegative(host)
}

// now is the resolver's clock (the injected test clock or the real one).
func (r *Resolver) now() time.Time {
	if r.cfg.Now != nil {
		return r.cfg.Now()
	}
	return time.Now()
}

// reaskOK reports whether host may re-ask the servers right now, and
// records the attempt when it may (M8-y, ADR-0023). The rate limit
// bounds probing to one WALK per NegativeReask window per host — the
// walk that WROTE the negative counts as the window's probe (noteReask
// at putNegative time), so every negative hit is either instant (the
// window is still warm — the flow fails fast and the app retries) or a
// genuinely fresh walk, never a redundant one seconds after a failure.
// The singleflight in Lookup coalesces a retry wave's concurrent
// re-asks into that one walk. The map is bounded FIFO, the same shape
// as the cache's own ordering.
func (r *Resolver) reaskOK(host string) bool {
	if r.cfg.NegativeReask <= 0 {
		return false
	}
	nowMS := r.now().UnixMilli()
	r.reaskMu.Lock()
	defer r.reaskMu.Unlock()
	if last, ok := r.reask[host]; ok && nowMS-last < r.cfg.NegativeReask.Milliseconds() {
		return false
	}
	r.noteReaskLocked(host, nowMS)
	return true
}

// noteReask records one probe time for host (M8-y, ADR-0023); the walk
// that writes a DNS_UNREACHABLE negative calls it so the window starts
// at the failure, not at the first negative hit.
func (r *Resolver) noteReask(host string) {
	r.reaskMu.Lock()
	defer r.reaskMu.Unlock()
	r.noteReaskLocked(host, r.now().UnixMilli())
}

func (r *Resolver) noteReaskLocked(host string, nowMS int64) {
	if _, ok := r.reask[host]; !ok {
		for len(r.reask) >= 1024 && len(r.reaskOrder) > 0 {
			oldest := r.reaskOrder[0]
			r.reaskOrder = r.reaskOrder[1:]
			delete(r.reask, oldest)
		}
		r.reaskOrder = append(r.reaskOrder, host)
	}
	r.reask[host] = nowMS
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

// askServers walks the servers HEDGED starting from the sticky one
// (M8-z, ADR-0025 §6.3): the sticky/first server is asked at t=0, the
// next at +HedgeDelay, the third at +2×HedgeDelay, and so on; the
// first VALID answer wins and cancels the rest, and a server that
// fails fast triggers the next launch immediately. The serial walk
// this replaces charged a full Timeout per silent server — the
// debug.16 report's cold lookups at 1055–1806 ms were exactly that
// serial front, paid at the worst moment (the first video's start, a
// burst of fresh names). HedgeDelay <= 0 pins the exact serial M8-y
// shape (the next server is asked only when the previous one FAILED).
func (r *Resolver) askServers(ctx context.Context, host string) ([]net.IP, int, time.Duration, error) {
	n := len(r.cfg.Servers)
	start := int(r.sticky.Load())
	if start < 0 || start >= n {
		start = 0
	}
	type walkResult struct {
		pos    int  // launch position in the walk (0 = the sticky/first ask)
		hedged bool // launched while an earlier server was still pending (the stagger or a fast failure — NOT the serial fallback)
		idx    int  // server index in cfg.Servers
		ips    []net.IP
		ttl    time.Duration
		err    error
		ms     int64
	}
	hctx, hcancel := context.WithCancel(ctx)
	defer hcancel()
	results := make(chan walkResult, n)
	failures := 0
	launched := 0
	launch := func(pos int) {
		idx := (start + pos) % n
		srv := r.cfg.Servers[idx]
		// A launch past position 0 is a HEDGE launch when an earlier
		// server is still pending (the stagger fired, or a fast failure
		// triggered it while others fly); the serial fallback — the next
		// server asked only after every earlier one FAILED — is not.
		hedged := pos > 0 && failures < launched
		t0 := time.Now()
		go func() {
			cctx, cancel := context.WithTimeout(hctx, r.cfg.Timeout)
			defer cancel()
			ips, ttl, err := r.queryOne(cctx, srv, host)
			results <- walkResult{pos: pos, hedged: hedged, idx: idx, ips: ips, ttl: ttl, err: err, ms: time.Since(t0).Milliseconds()}
		}()
	}
	var firstErr error
	var firstFailed string
	var firstFailedIdx int
	launch(0)
	launched = 1
	// The hedge clock: the next launch fires HedgeDelay after the last
	// one (a failure launches immediately instead and pushes the clock).
	hedge := r.cfg.HedgeDelay
	var timer *time.Timer
	var timerC <-chan time.Time
	armHedge := func() {
		if launched >= n || hedge <= 0 {
			return
		}
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(hedge)
		timerC = timer.C
	}
	armHedge()
	stopTimer := func() {
		if timer != nil {
			timer.Stop()
			timerC = nil
		}
	}
	defer stopTimer()
	for {
		select {
		case <-ctx.Done():
			stopTimer()
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			return nil, 0, 0, firstErr
		case <-timerC:
			// Hedge tick: the previous servers have not answered; ask the
			// next one NOW (first valid answer still wins).
			if launched < n {
				launch(launched)
				launched++
				armHedge()
			}
		case res := <-results:
			if res.err == nil {
				stopTimer()
				if firstFailed != "" && r.cfg.Logger != nil {
					r.cfg.Logger.Info(fmt.Sprintf("dns server failed; using the next one failed=#%d now=#%d", firstFailedIdx+1, res.idx+1),
						"failed", firstFailed, "nowUsing", r.cfg.Servers[res.idx].Name)
				}
				// A hedge win: the winner was launched while an earlier
				// server was still pending (the stagger or a fast failure —
				// the serial fallback after a settled failure is not one).
				if res.hedged {
					r.hedgeWins.Add(1)
					r.failMu.Lock()
					for len(r.hedgeByServer) <= res.idx {
						r.hedgeByServer = append(r.hedgeByServer, 0)
					}
					r.hedgeByServer[res.idx]++
					r.failMu.Unlock()
				}
				return res.ips, res.idx, res.ttl, nil
			}
			failures++
			r.noteFailure(res.idx+1, phaseOf(res.err), errKind(res.err), res.ms)
			if r.cfg.Logger != nil {
				// Server index, reason and PHASE ride in the message so the
				// diagnostic report's log ring (which drops attrs) carries the
				// evidence — the debug.10 report's three bare "dns server failed"
				// lines could not say timeout vs reset vs bad answer (M8-r),
				// and the debug.18 spec asks the phase outright (ADR-0027
				// §8.3.3: tcp/tls = path interference, status/parse = server
				// problem). The configured name stays an attr for the file log.
				r.cfg.Logger.Warn(fmt.Sprintf("dns server failed server=#%d reason=%s phase=%s", res.idx+1, errKind(res.err), phaseOf(res.err)),
					"server", r.cfg.Servers[res.idx].Name)
			}
			if firstErr == nil {
				firstErr = res.err
				firstFailed = r.cfg.Servers[res.idx].Name
				firstFailedIdx = res.idx
			}
			// Fast failure: launch the next server immediately (the hedge
			// clock is for SILENT servers, not failed ones).
			if launched < n {
				launch(launched)
				launched++
				armHedge()
			} else if failures >= launched {
				// Every launched server has failed and none are left.
				stopTimer()
				return nil, 0, 0, firstErr
			}
		}
	}
}

// queryOne performs the exact v1-style GET against one server. The
// request shape (the ?dns= GET, the type/ct parameters, the accept
// header), the cache-facing TTLs and the verdicts are unchanged from
// the serial walk; M8-z adds the failure PHASE classification (the
// report's DNS detail names which leg failed) and the query/dial
// counters, and schedules a re-warm when the failure looks like a
// connection-level problem (a dead shared connection, reopened in the
// background within 1 s — ADR-0025 §6.2).
func (r *Resolver) queryOne(ctx context.Context, srv config.DoHServer, host string) ([]net.IP, time.Duration, error) {
	r.queries.Add(1)
	wire, id, err := buildQuery(host, r.cfg.QueryAAAA)
	if err != nil {
		return nil, 0, &phaseError{"parse", err}
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
		return nil, 0, &phaseError{"parse", err}
	}
	req.Header.Set("accept", "application/dns-message")
	resp, err := r.client.Do(req)
	if err != nil {
		phase := phaseOf(err)
		if phase == "tcp" || phase == "tls" {
			r.scheduleRewarm()
		}
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

// dialServer is the TCP leg of the DoH transport's dial (the old
// http.Transport DialContext, kept verbatim): it ignores system DNS and
// dials the URL host's bootstrap IPs directly through netx, wrapping
// the connection so the DoH ClientHello itself is fragmented. M8-z:
// the http2.Transport calls it from dialTLSContext and counts the dial.
func (r *Resolver) dialServer(ctx context.Context, network, addr string) (net.Conn, error) {
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
		return nil, &phaseError{"tcp", err}
	}
	return fragment.NewConn(conn, r.cfg.FragmentPreset, r.cfg.FragmentConfig, r.cfg.RNG), nil
}

// dialTLSContext is the http2.Transport's dial (M8-z, ADR-0025 §6.1):
// the TCP leg goes through dialServer (the bootstrap IPs, netx, the
// protect hook — ADR-0011 — and the fragment.Conn wrapper, so the DoH
// ClientHello stays fragmented), then THIS function owns the TLS
// handshake with the server URL's host as ServerName (Go omits SNI for
// IP-literal names — the two no-SNI endpoints in the default list rely
// on exactly that, their certificates carry IP SANs) and ALPN h2. The
// transport pools one such connection per server address and shares it
// across every concurrent query — the burst-shape the debug.16 report
// measured (one fresh fragmented handshake per concurrent first query)
// cannot happen anymore.
func (r *Resolver) dialTLSContext(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
	// The transport's own TLSClientConfig is deliberately unused (nil):
	// the TLS shape — ServerName from the server URL's host, ALPN h2,
	// the test seam's trust pool — is owned HERE so it cannot drift
	// from the fragmented-dial path.
	r.dials.Add(1)
	frag, err := r.dialServer(ctx, network, addr)
	if err != nil {
		return nil, err // already phase=tcp
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}
	tlsCfg := &tls.Config{ServerName: host, NextProtos: []string{"h2"}}
	if base := r.cfg.TLS; base != nil {
		// The test seam: trust pool and overrides ride along, ALPN stays
		// h2 (the shared-connection guarantee is the point of the change).
		tlsCfg.RootCAs = base.RootCAs
		tlsCfg.InsecureSkipVerify = base.InsecureSkipVerify
		if base.ServerName != "" {
			tlsCfg.ServerName = base.ServerName
		}
	}
	tc := tls.Client(frag, tlsCfg)
	if dl, ok := ctx.Deadline(); ok {
		frag.SetDeadline(dl)
	}
	if err := tc.HandshakeContext(ctx); err != nil {
		frag.Close()
		return nil, &phaseError{"tls", err}
	}
	frag.SetDeadline(time.Time{})
	return tc, nil
}

// WarmProbe opens the shared DoH connection NOW instead of at the
// first query (M8-z, ADR-0025 §6.2): one background query at the
// sticky server through the transport — the HTTP/2 connection (with
// its health-check pings) exists before the app's first flow needs
// it, and the engine re-invokes this on its 5-minute prewarm cadence
// to reopen a connection the network dropped while idle. Failure is
// logged (the report's DNS detail carries it) and a re-warm is
// scheduled within 1 s when the failure was connection-shaped.
func (r *Resolver) WarmProbe() {
	go func() {
		if !r.warming.CompareAndSwap(false, true) {
			return
		}
		defer r.warming.Store(false)
		n := len(r.cfg.Servers)
		if n == 0 {
			return
		}
		idx := int(r.sticky.Load())
		if idx < 0 || idx >= n {
			idx = 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
		defer cancel()
		t0 := time.Now()
		_, _, err := r.queryOne(ctx, r.cfg.Servers[idx], "www.youtube.com")
		if r.cfg.Logger == nil {
			return
		}
		if err != nil {
			r.cfg.Logger.Warn(fmt.Sprintf("doh conn warm failed reason=%s phase=%s", errKind(err), phaseOf(err)))
			r.scheduleRewarm()
			return
		}
		r.cfg.Logger.Info(fmt.Sprintf("doh conn warm ok latency=%dms", time.Since(t0).Milliseconds()))
	}()
}

// scheduleRewarm reopens the shared connection in the background
// within 1 s of a connection-phase failure (ADR-0025 §6.2: "if it
// drops, reopen it in the background within 1 s"), debounced to at
// most one scheduled re-warm per 5 s — the hedge and the next query's
// own dial also cover the gap, this keeps the connection warm for the
// NEXT flow rather than healing the failed one.
func (r *Resolver) scheduleRewarm() {
	now := time.Now().UnixMilli()
	if r.rewarmAt.Load() > now {
		return
	}
	r.rewarmAt.Store(now + 5000)
	time.AfterFunc(time.Second, func() { r.WarmProbe() })
}

// googleVideoCode extracts the sn-<code> part of a googlevideo cache
// node name (M8-z, ADR-0025 §6.4): rr1---sn-abc123.googlevideo.com and
// r1---sn-abc123.googlevideo.com share the SAME sn-abc123 code — the
// per-video rr/r prefix varies, the node identity does not. Empty for
// every other name.
func googleVideoCode(host string) string {
	if !strings.HasSuffix(host, ".googlevideo.com") {
		return ""
	}
	name := strings.TrimSuffix(host, ".googlevideo.com")
	i := strings.Index(name, "---sn-")
	if i < 0 {
		return ""
	}
	code := "sn-" + name[i+len("---sn-"):]
	if len(code) <= len("sn-") {
		return ""
	}
	return code
}

// noteGvideoCode remembers one learned code (bounded, ordered) and
// persists the table to CodeStore when it grows (§6.4).
func (r *Resolver) noteGvideoCode(code string) {
	r.codeMu.Lock()
	defer r.codeMu.Unlock()
	if r.codeIndex[code] {
		return
	}
	r.codeIndex[code] = true
	r.codes = append(r.codes, code)
	if len(r.codes) > config.GoogleVideoCodeCap {
		// FIFO eviction: the oldest learned code goes.
		drop := r.codes[0]
		r.codes = r.codes[1:]
		delete(r.codeIndex, drop)
	}
	if r.cfg.CodeStore != "" {
		if err := os.WriteFile(r.cfg.CodeStore, []byte(strings.Join(r.codes, "\n")+"\n"), 0o600); err != nil && r.cfg.Logger != nil {
			r.cfg.Logger.Warn("doh code store save failed reason=" + errKind(err))
		}
	}
}

// GvideoCodes returns a copy of the learned googlevideo codes (the
// engine's prewarm loop resolves rr1..rr5 of each).
func (r *Resolver) GvideoCodes() []string {
	r.codeMu.Lock()
	defer r.codeMu.Unlock()
	return append([]string(nil), r.codes...)
}

// loadCodes reads the persisted code table at construction (§6.4).
func (r *Resolver) loadCodes() {
	if r.cfg.CodeStore == "" {
		return
	}
	data, err := os.ReadFile(r.cfg.CodeStore)
	if err != nil {
		return // missing or unreadable: start empty, learn fresh
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || r.codeIndex[line] || len(r.codes) >= config.GoogleVideoCodeCap {
			continue
		}
		r.codeIndex[line] = true
		r.codes = append(r.codes, line)
	}
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
