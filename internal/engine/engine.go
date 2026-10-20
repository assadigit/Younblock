package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assadigit/Younblock/internal/health"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/doh"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/logx"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/proxy"
	"github.com/assadigit/Younblock/internal/stats"
	"github.com/assadigit/Younblock/internal/usage"
)

// Phase is the engine's coarse state (file 01, section 10).
type Phase string

const (
	PhaseOff        Phase = "off"
	PhaseConnecting Phase = "connecting"
	PhaseOn         Phase = "on"
	PhaseDegraded   Phase = "degraded"
	PhaseError      Phase = "error"
	PhaseStopping   Phase = "stopping"
)

// Notice is the "nothing changes silently" line (file 03, R13).
type Notice struct {
	Code string `json:"code"`
	Arg  string `json:"arg"`
}

// StateError carries a typed error code; wording lives in the UI.
type StateError struct {
	Code string `json:"code"`
}

// StateFail says which self-test step failed last and its code, so the UI
// can tell a secure-DNS problem from a YouTube-tunnel one (R13: nothing
// changes silently — not failures either; DECISIONS.md D44).
type StateFail struct {
	Step string `json:"step"`
	Code string `json:"code"`
}

// State is the full UI-facing state.
type State struct {
	Phase  Phase         `json:"phase"`
	Detail string        `json:"detail"`
	Since  time.Time     `json:"since"`
	Port   int           `json:"port"`
	Mode   config.Mode   `json:"mode"`
	Method config.Method `json:"method"`
	// LatencyMs is the last measured YouTube round-trip through the whole
	// chain (secure lookup + connect + request), refreshed by every health
	// check while on (user-requested "ping"; DECISIONS.md D41). Nil when no
	// measurement exists (off, or nothing succeeded yet).
	LatencyMs *int64      `json:"latencyMs,omitempty"`
	Fail      *StateFail  `json:"fail,omitempty"`
	Notice    *Notice     `json:"notice,omitempty"`
	Error     *StateError `json:"error,omitempty"`
	// Round-3 usage: the live session's relayed byte totals (or, once it
	// ended, the last session's — so the row does not blink to zero while
	// "stopping" settles). A host that cannot count would send -1; ours
	// always counts. No omitempty on lastSession: an explicit null is how
	// a cleared history makes the screen's Last-session line disappear.
	DownBytes   int64                `json:"downBytes"`
	UpBytes     int64                `json:"upBytes"`
	LastSession *usage.SessionRecord `json:"lastSession"`
}

// EventSink receives events for the UI (one JSON contract, file 01 §3 rule 4).
type EventSink interface {
	OnEvent(name string, json string)
}

// Options configures an engine.
type Options struct {
	// Platform is required (stub on non-Windows builds).
	Platform Platform
	// Version overrides config.AppVersion for GetAppInfo.
	Version string
	// LogDir overrides the log directory (tests); empty uses <config>/logs.
	LogDir string
	// AllowPrivateTargets is the test-only override (file 01, section 4).
	AllowPrivateTargets bool
	// PortOverride forces the listen port (cmd/core --port). 0 = setting.
	// It is not persisted.
	PortOverride int
	// MethodOverride forces the fragmentation preset (cmd/core --preset).
	// Empty = setting. It is not persisted.
	MethodOverride config.Method
	// Checker overrides the health probes (tests inject fake networks).
	Checker Checker
	// WatchdogInterval overrides the watchdog period (0 = config default).
	WatchdogInterval time.Duration
	// WatchdogFailLimit overrides the consecutive-failure trigger.
	WatchdogFailLimit int
	// HealCooldown overrides the automatic-sweep cooldown.
	HealCooldown time.Duration
	// StartTimeout overrides the total start budget (the 35 s cap).
	StartTimeout time.Duration
	// NetworkPollInterval overrides the interface polling period.
	NetworkPollInterval time.Duration
	// InterfaceLister overrides the interface address source (R19 tests).
	InterfaceLister func() ([]net.Addr, error)
	// HealthOverrides rewrites the health.Config used by the built-in
	// checks (tests point them at local fakes).
	HealthOverrides func(*health.Config)
	// ResolverTLS overrides the DoH client TLS config (tests inject a
	// trust pool for a local fake DoH server).
	ResolverTLS *tls.Config
	// Now is the clock (tests).
	Now func() time.Time
}

// Engine owns the whole app lifecycle.
type Engine struct {
	opts     Options
	platform Platform
	settings config.Settings
	logger   *logx.Logger
	stats    *stats.Counters
	// resolver is the live DoH resolver. It is an atomic pointer so a
	// changed secure-DNS list can swap it while the engine runs (D44).
	resolver atomic.Pointer[doh.Resolver]
	rng      *fragment.SafeRNG
	dialer   *netx.Dialer

	mu          sync.Mutex
	phase       Phase
	detail      string
	since       time.Time
	notice      *Notice
	noticeUntil time.Time
	errCode     string
	port        int
	server      *proxy.Server
	statsCtx    context.CancelFunc

	// round-3 usage session tracking (usage.go): one session per "on"
	// run, deltas of the relay counters, persisted to the usage store.
	usage        *usage.Store // nil when the history file could not be opened
	sessLive     bool
	sessStart    time.Time
	sessBaseDown int64
	sessBaseUp   int64
	lastSess     *usage.SessionRecord
	autosaveTk   int // stats-loop seconds since the last crash-recovery save

	checker        Checker
	runCancel      context.CancelFunc
	runAborted     bool
	runGen         uint64 // M7 (D82): identifies the live connect run; every cancel bumps it
	watchdogCancel context.CancelFunc
	wdFails        int
	netMonCancel   context.CancelFunc
	lastAutoSweep  time.Time
	lastNotifyAt   time.Time
	episodeStart   time.Time
	lastSteps      []health.Step
	latencyMs      int64
	failStep       string
	failCode       string

	events EventSink
	onQuit func()

	// reportExtra (M8-p, ADR-0015) is an optional embedder-supplied
	// section appended to the diagnostic report — the Android facade
	// adds its TUN counters and shell version through it. Nil on the
	// desktops (the report is unchanged there). Guarded by mu; the
	// hook itself runs OUTSIDE the lock (GetDiagnosticReport calls
	// GetState, which takes mu).
	reportExtra func() string

	// M8-z (ADR-0024/0025/0027): the per-host flow table behind the
	// report's Top-10 section; the googlevideo prewarm loop's cancel;
	// the timeline marks; and the dead-origin probe's once-per-run
	// latch. hostTable is immutable after New (nil-safe consumers).
	hostTable    *stats.HostTable
	gvideoCancel context.CancelFunc
	marks        []Mark      // the last timeline marks (bounded)
	markSnapshot *markWindow // counters snapshot at the last mark
	probeDone    bool        // the dead-origin probe ran this run
	probeLines   []string    // its lines, rendered once per report section

	// M8-ab (ADR-0032/0033): the origin-probe pass's round-robin
	// cursor over the learned googlevideo codes (owned by the gvideo
	// loop's goroutine), and the persisted good pool's file path —
	// the session-over-session memory the first video of the day
	// rides. goodPoolLast is the last snapshot written (the change
	// check); guarded by goodMu so the Stop-time save and the loop's
	// periodic save never interleave a write.
	probeCursor  int
	goodPoolPath string
	goodMu       sync.Mutex
	goodPoolLast string

	// M8-aa (ADR-0029): the effective lean stall budget this run's
	// proxy carries — Android's platform default (8 s), the
	// desktop's config.StallTimeout — printed beside the report's
	// tier counts so the next paste says which budget fired.
	stallLean time.Duration

	// M8-aa (ADR-0030): the auto-timeline heuristic's poller —
	// play-suspected (a routed burst after quiet) and first-data
	// (a down-burst on its heels) — plus its cancel.
	autoTL       *autoTimeline
	autoTLCancel context.CancelFunc

	measuredSleep time.Duration
	stopWait      sync.WaitGroup
}

// Mark is one timeline stamp (M8-z, ADR-0027 §8.3.6): the Go facade
// exposes Mark(label) — the Android diagnostics screen's "I pressed
// play" and "First frame" buttons — and the report prints the elapsed
// time between the LAST PAIR plus the failed/slow flows inside that
// window.
type Mark struct {
	Label  string
	At     time.Time
	Window *markWindow // the deltas captured when THIS mark landed (nil for the first)
}

// markWindow snapshots the flow counters at one mark so the next mark
// can report the window's failures/slowness as deltas.
type markWindow struct {
	connErrors    int64
	plainKills    int64
	slowFirstByte int64
}

// The auto-timeline heuristic's knobs (M8-aa, ADR-0030): the routed
// quiet that precedes a suspected play burst, the down-burst that
// pairs it with real data, the poll cadence, and the pair history's
// bound (the manual marks' own bound).
const (
	autoTLQuiet      = 5 * time.Second
	autoTLBurstBytes = 100 * 1024
	autoTLPollEvery  = 500 * time.Millisecond
	autoTLMaxPairs   = 8
)

// autoPair is one closed auto-timeline measurement (M8-aa, ADR-0030):
// a play-suspected stamp and the first-data stamp that answered it.
type autoPair struct {
	play, data time.Time
}

// autoTimeline is the engine's heuristic play detector (M8-aa,
// ADR-0030): the debug.18 report came back with "timeline: no marks"
// — the tester never pressed the buttons, and the one number that
// says whether the round succeeded (play -> first frame) was missing.
// The poller watches the counters the flows already feed: a routed
// flow burst after at least autoTLQuiet of routed quiet stamps
// "auto: play-suspected" (the app's player wave opening its
// connections); a down-burst of at least autoTLBurstBytes right after
// it stamps "auto: first-data" (video bytes actually flowing). The
// pairs land in the report beside the manual marks — which stay
// AUTHORITATIVE: the heuristic cannot tell a feed load from a play,
// only the buttons can — and the stamps ride the log ring as
// AUTO-MARK lines so the tester's paste carries them with timestamps.
type autoTimeline struct {
	lastRoutedAt time.Time  // the last poll that saw routed flow starts
	routedFlows  int64      // the routed flow counter at lastRoutedAt
	lastDown     int64      // the down byte counter at the last poll
	playAt       *time.Time // an unpaired play-suspected stamp
	pairs        []autoPair // the closed pairs (bounded)
}

// New builds an engine: loads settings, opens logs, measures the real sleep
// length, and prepares the resolver. It does not start anything.
func New(opts Options) (*Engine, error) {
	if opts.Platform == nil {
		return nil, errors.New("engine: platform is required")
	}
	if opts.Version == "" {
		opts.Version = config.AppVersion
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	dir := opts.Platform.ConfigDir()
	// M8-z (ADR-0024): the loading platform's name reaches the settings
	// loader — fragmentAllRouted defaults true on Android, false on
	// every desktop (config.LoadFor); the debug-tester build's
	// LogHostnames default rides the same path (ADR-0027 §8.1).
	s, notice, err := config.LoadFor(dir, opts.Platform.Name())
	if err != nil {
		return nil, fmt.Errorf("engine: settings: %w", err)
	}
	// CLI overrides (not persisted): --port and --preset.
	if opts.PortOverride > 0 && opts.PortOverride < 65536 {
		s.Port = opts.PortOverride
		s.AutoPort = false
	}
	if opts.MethodOverride != "" && opts.MethodOverride.Valid() {
		s.Method = opts.MethodOverride
	}
	logDir := opts.LogDir
	if logDir == "" {
		logDir = filepath.Join(dir, "logs")
	}
	logger, err := logx.New(logDir, config.LogRingLines, config.LogFileSizeLimit, config.LogFileCount)
	if err != nil {
		return nil, fmt.Errorf("engine: logs: %w", err)
	}
	e := &Engine{
		opts:      opts,
		platform:  opts.Platform,
		settings:  s,
		logger:    logger,
		stats:     stats.New(),
		hostTable: stats.NewHostTable(),
		rng:       fragment.NewDefaultRNG(),
		phase:     PhaseOff,
		since:     opts.Now(),
		// M8-ab (ADR-0033): the persisted verified-live pool's file,
		// beside the codes store in the app's private directory.
		goodPoolPath: filepath.Join(opts.Platform.ConfigDir(), config.GoogleVideoGoodIPsFile),
	}
	// The round-3 usage history lives beside settings.json; a failed
	// open only disables the history (usageOpen logs it).
	e.usageOpen()
	// Seed the last finished session so a restarted app shows its
	// "Last session:" line immediately (the webui path reads it from
	// the state; the Wails bridge folds it into its first snapshot).
	if e.usage != nil {
		if last, ok := e.usage.Last(); ok {
			rec := last
			e.lastSess = &rec
		}
	}
	if notice != "" {
		logger.Warn(notice)
	}
	e.measuredSleep = fragment.MeasureSleep()
	logger.Info("engine ready",
		"platform", opts.Platform.Name(),
		"version", opts.Version,
		"measuredSleepMs", e.measuredSleep.Milliseconds(),
	)
	e.rebuildResolver()
	e.dialer = netx.New(config.DialTimeoutPerIP, config.DialTimeoutOverall, opts.Platform.DialControl())
	e.checker = opts.Checker
	if e.checker == nil {
		e.checker = &healthChecker{eng: e}
	}
	return e, nil
}

// resolverConfig builds the DoH resolver configuration for a server list.
// currentSettings snapshots the settings the list and flags come from.
func (e *Engine) resolverConfig(servers []config.DoHServer, s config.Settings) doh.Config {
	return doh.Config{
		Servers:        servers,
		Timeout:        config.DoHTimeout,
		BlockPrivate:   config.BlockPrivateTargets && !e.opts.AllowPrivateTargets,
		NegativeReask:  config.DNSCacheNegativeReask,
		Control:        e.platform.DialControl(),
		FragmentPreset: e.presetFor(s.Method),
		FragmentConfig: e.fragmentConfigFor(s),
		RNG:            e.rng,
		Logger:         e.logger.Logger,
		LogHostnames:   s.Advanced.LogDomains,
		Stats:          e.stats,
		Now:            e.opts.Now,
		TLS:            e.opts.ResolverTLS,
		// M8-z (ADR-0025 §6.3/§6.4): the hedged walk's stagger and the
		// googlevideo sn-<code> store's file (beside settings.json, in
		// the app's private directory).
		HedgeDelay: config.DoHHedgeDelay,
		CodeStore:  filepath.Join(e.platform.ConfigDir(), config.GoogleVideoCodesFile),
	}
}

// rebuildResolver (re)creates the DoH resolver from current settings.
func (e *Engine) rebuildResolver() {
	e.resolver.Store(doh.New(e.resolverConfig(e.settings.Advanced.DoHServers, e.settings)))
}

// swapResolver installs a resolver bound to the given server list and hands
// it to the running proxy for future tunnels. A changed secure-DNS list
// used to only clear the cache — the old servers kept being asked until a
// restart; now the change applies live (D44).
func (e *Engine) swapResolver(servers []config.DoHServer) {
	fresh := doh.New(e.resolverConfig(servers, e.settingsSnapshot()))
	e.resolver.Store(fresh)
	if srv := e.RunningServer(); srv != nil {
		srv.SetResolver(fresh)
	}
}

// presetFor maps a method setting to a fragmentation preset.
func (e *Engine) presetFor(m config.Method) fragment.Preset {
	if m == config.MethodAuto {
		return fragment.Balanced
	}
	return fragment.Preset(m)
}

// fragmentConfigFor merges preset defaults with the advanced overrides.
func (e *Engine) fragmentConfigFor(s config.Settings) fragment.Config {
	pc := fragment.PresetConfig(e.presetFor(s.Method))
	if s.Advanced.PieceSizeMin > 0 {
		pc.Min = s.Advanced.PieceSizeMin
	}
	if s.Advanced.PieceSizeMax > 0 && s.Advanced.PieceSizeMax >= pc.Min {
		pc.Max = s.Advanced.PieceSizeMax
	}
	if s.Advanced.PauseMs > 0 {
		pc.Pause = time.Duration(s.Advanced.PauseMs) * time.Millisecond
	}
	return pc
}

// currentPreset maps the current method setting to a fragmentation preset.
func (e *Engine) currentPreset() fragment.Preset {
	return e.presetFor(e.settingsSnapshot().Method)
}

// currentFragmentConfig merges the current preset with advanced overrides.
func (e *Engine) currentFragmentConfig() fragment.Config {
	return e.fragmentConfigFor(e.settingsSnapshot())
}

// SetEventSink registers the UI event sink.
func (e *Engine) SetEventSink(sink EventSink) {
	e.mu.Lock()
	e.events = sink
	e.mu.Unlock()
}

// SetOnQuit registers the shell's quit callback (window close, os.Exit).
func (e *Engine) SetOnQuit(fn func()) {
	e.mu.Lock()
	e.onQuit = fn
	e.mu.Unlock()
}

// Logger exposes the app logger for diagnostics.
func (e *Engine) Logger() *logx.Logger { return e.logger }

// SetReportExtra installs an embedder-supplied section that
// GetDiagnosticReport appends after the counters (M8-p, ADR-0015) — the
// Android facade uses it for its TUN counters and shell version. Passing
// nil removes the section again. The hook may be called concurrently with
// this swap; it must not call back into the engine's locked methods.
func (e *Engine) SetReportExtra(f func() string) {
	e.mu.Lock()
	e.reportExtra = f
	e.mu.Unlock()
}

// Stats exposes the counters for diagnostics.
func (e *Engine) Stats() *stats.Counters { return e.stats }

// Mark stamps the diagnostics timeline (M8-z, ADR-0027 §8.3.6): the
// facade exposes it as Mark(label) — the Android diagnostics screen's
// "I pressed play" and "First frame" buttons call it — and the report
// prints the elapsed time between the LAST PAIR plus the failed/slow
// flows inside that window. The line lands in the log ring as
// "MARK <label>" (timestamps ride the ring's own line prefix), and
// each mark snapshots the flow counters so the NEXT mark can report
// the window's deltas exactly.
func (e *Engine) Mark(label string) {
	now := e.opts.Now()
	d := e.stats.Diagnostic()
	cur := &markWindow{connErrors: d.ConnErrors, plainKills: d.PlainFirstByteTo, slowFirstByte: d.FirstByteSlow}
	e.mu.Lock()
	var win markWindow
	if prev := e.markSnapshot; prev != nil {
		win = markWindow{
			connErrors:    cur.connErrors - prev.connErrors,
			plainKills:    cur.plainKills - prev.plainKills,
			slowFirstByte: cur.slowFirstByte - prev.slowFirstByte,
		}
	}
	e.marks = append(e.marks, Mark{Label: label, At: now, Window: &win})
	if len(e.marks) > 8 {
		e.marks = e.marks[len(e.marks)-8:]
	}
	e.markSnapshot = cur
	e.mu.Unlock()
	e.logger.Info("MARK " + label)
}

// stallLeanEffective reports the lean stall budget this run's proxy
// carries (M8-aa, ADR-0029): the platform default when a run is live
// (Android's 8 s, the desktop's config.StallTimeout), the same default
// derived from the platform when no run has started yet. The report
// prints it beside the tier counts.
func (e *Engine) stallLeanEffective() time.Duration {
	e.mu.Lock()
	d := e.stallLean
	e.mu.Unlock()
	if d > 0 {
		return d
	}
	if e.platform.Name() == "android" {
		return config.StallLeanTimeoutAndroid
	}
	return config.StallTimeout
}

// timelineMarks returns the bounded mark history (the report reads the
// last pair).
func (e *Engine) timelineMarks() []Mark {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Mark(nil), e.marks...)
}

// startGvideoLoop runs the DoH warm + the learned-googlevideo
// pre-resolution while the engine is On (M8-z, ADR-0025 §6.2/§6.4):
// the shared connection opens NOW (not at the first query), and every
// GoogleVideoPrewarmEvery the remembered sn-<code> table's rr1..rr5
// variants re-resolve in the background (at most
// GoogleVideoPrewarmInflight lookups in flight) so the next fresh
// per-video hostname often lands on a warm cache entry.
//
// M8-ab (ADR-0032/0033) adds two lanes, both Android-gated exactly
// like the probe pass itself (ADR-0028 decision 5 — the desktops keep
// the byte-for-byte M8-z loop): the FAST-START cadence, passes every
// ProbeFastStartEvery while the engine is young and the gvideo good
// pool is thin (the debug.19 session ran ONE pass in its whole 45 s
// cold start), and the CHURN KICK — a 1 s watch on the
// gvideoTerminals counter (a googlevideo flow whose raced round AND
// untried salvage all died) that kicks an immediate pass, debounced
// by ProbeChurnKickDebounce, so verified origins land inside the
// app's next retry wave instead of the next 5-minute cycle. The loop
// also flushes the persisted good pool on change
// (GoogleVideoGoodIPsFile, ADR-0033).
//
// M8-ac (ADR-0035) removes the 45 s fast-start lane the debug.20
// round added: its job — stocking the pool before the first video —
// is now owned by the PERSISTED seed (ADR-0033) and the flow-fed
// marks (ADR-0031), while under the flow-killing regime its verdicts
// mostly come back reset (the debug.20 session: the gvideo pool never
// crossed the floor, so the lane ran its whole 5-minute window
// sending scan-like bursts exactly while the network was hostile).
// The M8-ac lanes are all BROWNOUT-GATED: while the DoH resolver's
// connections are churning (the connection-killer at work — dials 59
// in the debug.20 session, 6 thirteen hours earlier), the prewarm,
// the probe passes, and the churn kicks pause until the lane recovers;
// the app's own flows keep the resolver busy enough.
func (e *Engine) startGvideoLoop() {
	e.resolver.Load().WarmProbe()
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	old := e.gvideoCancel
	e.gvideoCancel = cancel
	e.mu.Unlock()
	if old != nil {
		old()
	}
	go func() {
		e.prewarmGvideoCodes(ctx)
		e.probeOriginsPass(ctx)
		lastKick := time.Now()
		churnSeen := e.stats.Diagnostic().GvideoTerminals
		android := e.platform.Name() == "android"
		brownout := newDoHBrownout()
		slow := time.NewTicker(config.GoogleVideoPrewarmEvery)
		watch := time.NewTicker(time.Second)
		pool := time.NewTicker(config.GoodPoolSaveEvery)
		defer slow.Stop()
		defer watch.Stop()
		defer pool.Stop()
		for {
			select {
			case <-ctx.Done():
				e.saveGoodPoolIfChanged() // the Stop-time flush (ADR-0033)
				return
			case <-slow.C:
				if brownout.active() {
					continue // M8-ac (ADR-0035): the DoH lane is dying — the background stays quiet
				}
				e.prewarmGvideoCodes(ctx)
				e.probeOriginsPass(ctx)
			case <-watch.C:
				// The brownout tracker steps every second on the
				// DoH dial counter (a replacement storm = the
				// connection-killer working the lane).
				if brownout.step(e.resolver.Load().Diagnostic().Dials) {
					e.stats.DoHBrownout()
					e.logger.Warn("doh brownout: background lanes paused (resolver connections churning)")
				}
				// The churn lane: a fresh googlevideo terminal
				// (all-dead raced round + dead salvage) means the
				// answer sets just rolled black — verify new
				// origins NOW. The desktops never enter; neither
				// does a brownout.
				if !android || brownout.active() {
					continue
				}
				cur := e.stats.Diagnostic().GvideoTerminals
				if cur <= churnSeen {
					continue
				}
				churnSeen = cur
				if time.Since(lastKick) < config.ProbeChurnKickDebounce {
					continue // a recent pass is still landing
				}
				lastKick = time.Now()
				e.probeOriginsPass(ctx)
			case <-pool.C:
				e.saveGoodPoolIfChanged()
			}
		}
	}()
}

// dohBrownout tracks the DoH lane's connection churn (M8-ac,
// ADR-0035): a rolling window of fresh DoH dials, entered when
// BrownoutDialThreshold land inside BrownoutWindow, lifted after
// BrownoutQuiet passes without a new dial. step() takes the DoH
// transport's cumulative dial counter once a second and reports
// whether THIS step entered the brownout.
type dohBrownout struct {
	lastDials int64       // the cumulative dial count at the last step
	window    []time.Time // the dial timestamps inside the rolling window
	quiet     time.Time   // the last dial-free moment seen while active
	active_   bool
	now       func() time.Time // the clock (injected by the tests)
}

func newDoHBrownout() *dohBrownout {
	return &dohBrownout{now: time.Now}
}

// step advances the tracker with the cumulative dial count and the
// clock, returning true when the brownout is newly entered.
func (b *dohBrownout) step(dials int64) bool {
	now := b.now()
	if dials > b.lastDials {
		// New dials since the last step: stamp one per dial (the exact
		// count within the window is what matters, not the spacing).
		for i := b.lastDials; i < dials; i++ {
			b.window = append(b.window, now)
		}
		b.lastDials = dials
		b.quiet = now
	}
	// Prune the window.
	cutoff := now.Add(-config.BrownoutWindow)
	kept := b.window[:0]
	for _, t := range b.window {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	b.window = kept
	if !b.active_ {
		if len(b.window) >= config.BrownoutDialThreshold {
			b.active_ = true
			b.quiet = now
			return true
		}
		return false
	}
	// Active: lift only after a full quiet period without a dial.
	if now.Sub(b.quiet) >= config.BrownoutQuiet && len(b.window) == 0 {
		b.active_ = false
		return false
	}
	return false
}

func (b *dohBrownout) active() bool { return b.active_ }

// NewDoHBrownoutForTest, StepForTest, ActiveForTest, SetClockForTest —
// the engine tests' one reach into the brownout tracker (the engine
// package keeps its API surface otherwise untouched; the tracker is
// loop-internal logic).
func NewDoHBrownoutForTest() *dohBrownout { return newDoHBrownout() }

func (b *dohBrownout) StepForTest(dials int64) bool { return b.step(dials) }

func (b *dohBrownout) ActiveForTest() bool { return b.active() }

func (b *dohBrownout) SetClockForTest(now func() time.Time) { b.now = now }

// stopGvideoLoop halts the prewarm loop (Stop's path).
func (e *Engine) stopGvideoLoop() {
	e.mu.Lock()
	cancel := e.gvideoCancel
	e.gvideoCancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// loadGoodIPPool reads the persisted verified-live googlevideo pool
// (M8-ab, ADR-0033): one IP per line, the origins the LAST session's
// probe passes and flow first-bytes verified. Start seeds them back
// with the shorter GoodPoolSeedTTL; a seed that went bad overnight
// self-corrects through the demote-on-death guard. A missing or
// unreadable file is simply an empty pool (first run, or a fresh
// install) — never an error worth failing a start over.
func (e *Engine) loadGoodIPPool() []string {
	e.goodMu.Lock()
	path := e.goodPoolPath
	e.goodMu.Unlock()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if net.ParseIP(line) == nil {
			continue // a stale or corrupted line never poisons the pool
		}
		out = append(out, line)
		if len(out) >= goodIPTableMaxPersist {
			break
		}
	}
	return out
}

// goodIPTableMaxPersist bounds the persisted pool (M8-ab, ADR-0033):
// a session's verified gvideo origins, capped for the same reasons the
// in-memory table is — plenty of substitutes, trivially small file.
const goodIPTableMaxPersist = 32

// saveGoodPoolIfChanged writes the gvideo good pool to
// GoogleVideoGoodIPsFile when it changed since the last write (M8-ab,
// ADR-0033). Called from the gvideo loop's periodic tick and its
// Stop-time flush; goodMu serializes the change-check-and-write so the
// two never interleave. Write failures log and leave the file for the
// next change — the pool is an optimization, never a dependency.
func (e *Engine) saveGoodPoolIfChanged() {
	srv := e.RunningServer()
	if srv == nil {
		return
	}
	e.goodMu.Lock()
	path := e.goodPoolPath
	if path == "" {
		e.goodMu.Unlock()
		return
	}
	list := srv.GoodGvideoIPList()
	snapshot := strings.Join(list, "\n")
	if snapshot == e.goodPoolLast {
		e.goodMu.Unlock()
		return
	}
	e.goodPoolLast = snapshot
	e.goodMu.Unlock()
	data := ""
	if snapshot != "" {
		data = snapshot + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		e.logger.Warn("good pool save failed reason=" + errKind(err))
	}
}

// prewarmGvideoCodes resolves rr1..rr5 of every remembered code, at
// most GoogleVideoPrewarmInflight lookups in flight, each with its own
// small budget; a resolver swap mid-loop is tolerated (the resolver is
// read once per pass).
func (e *Engine) prewarmGvideoCodes(ctx context.Context) {
	r := e.resolver.Load()
	codes := r.GvideoCodes()
	if len(codes) == 0 {
		return
	}
	names := make([]string, 0, len(codes)*5)
	for _, code := range codes {
		for i := 1; i <= 5; i++ {
			names = append(names, fmt.Sprintf("rr%d---%s.googlevideo.com", i, code))
		}
	}
	sem := make(chan struct{}, config.GoogleVideoPrewarmInflight)
	var wg sync.WaitGroup
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(name string) {
			defer wg.Done()
			defer func() { <-sem }()
			qctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			r.Lookup(qctx, name) // cache fill; errors are the loop's own business
		}(name)
	}
	wg.Wait()
}

// probeOriginsPass runs one origin-probe pass through the running
// proxy (M8-aa, ADR-0028): the stable OriginProbeNames first — the
// player request's own API fronts — then a round-robin window of the
// learned googlevideo codes' rr names (M8-ab, ADR-0032: the debug.19
// pass pinned the freshest-2 codes — early in a session the local
// cluster twice, so the verified pool could not escape the blackhole
// even in principle; the rotation covers every remembered code,
// foreign clusters included, within a few passes). The pass resolves
// through the same resolver the flows use (the prewarm that just ran
// filled the cache), verifies each answer IP with the exact wire
// shape a real flow sends, and hands its verdicts to the box and the
// good table. Bounded by OriginProbePerPass and the loop's own ctx;
// the verdicts ride counters only (R21 — no names, no IPs anywhere).
func (e *Engine) probeOriginsPass(ctx context.Context) {
	// ADR-0028 decision 5: the pass runs on the phone's lane only —
	// the desktops keep the M8-z prewarm-resolution behavior
	// byte-for-byte (the cold-start spec's hard rule 5), and the stub
	// platform every host test builds on never dials a probe (the
	// test herd stays hermetic).
	if e.platform.Name() != "android" {
		return
	}
	srv := e.RunningServer()
	if srv == nil {
		return
	}
	names, next := probePassNames(config.OriginProbeNames, e.resolver.Load().GvideoCodes(), e.probeCursor, config.ProbeCodesPerPass)
	e.probeCursor = next
	srv.ProbeOrigins(ctx, names, 443, config.OriginProbePerPass)
}

// probePassNames builds one origin-probe pass's name list (M8-ab,
// ADR-0032): the stable names first (their answer sets are small and
// the first-play wave leans on them), then one rr name per code from
// a round-robin window of the learned code store — the rr index
// rotates with the cursor too, so a cluster's different rr variants
// (each a slightly different rotation of the same pool) get their
// turn across passes. Codes already covered by an earlier name in
// THIS list are skipped (the store is unique, but the wrap-around
// window may revisit). Returns the next cursor.
func probePassNames(stable []string, codes []string, cursor, perPass int) ([]string, int) {
	names := make([]string, 0, len(stable)+perPass)
	names = append(names, stable...)
	if len(codes) == 0 || perPass <= 0 {
		return names, cursor
	}
	seen := make(map[string]bool, perPass)
	for k := 0; k < perPass && k < len(codes); k++ {
		idx := (cursor + k) % len(codes)
		code := codes[idx]
		if seen[code] {
			continue
		}
		seen[code] = true
		rr := (cursor + k) % 5 // the rr variant rotates with the window
		names = append(names, fmt.Sprintf("rr%d---%s.googlevideo.com", rr+1, code))
	}
	next := cursor + perPass
	if next >= len(codes) {
		next = 0
	}
	return names, next
}

// ProbePassNamesForTest is the engine tests' one reach into the pass's
// pure name builder (the engine package keeps its API surface
// otherwise untouched — the function is unexported logic owned by the
// gvideo loop).
func ProbePassNamesForTest(stable, codes []string, cursor, perPass int) ([]string, int) {
	return probePassNames(stable, codes, cursor, perPass)
}

// startAutoTimeline runs the heuristic play detector while the engine
// is On (M8-aa, ADR-0030): a 500 ms poller over the counters the flows
// already feed. Idempotent across the degraded->on flap (the old
// poller is cancelled first, the state survives).
func (e *Engine) startAutoTimeline() {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	old := e.autoTLCancel
	e.autoTLCancel = cancel
	if e.autoTL == nil {
		e.autoTL = &autoTimeline{}
	}
	e.autoTL.routedFlows = e.stats.Diagnostic().RoutedFlows
	e.mu.Unlock()
	if old != nil {
		old()
	}
	go func() {
		ticker := time.NewTicker(autoTLPollEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.autoTLPoll()
			}
		}
	}()
}

// stopAutoTimeline halts the heuristic poller (Stop's path).
func (e *Engine) stopAutoTimeline() {
	e.mu.Lock()
	cancel := e.autoTLCancel
	e.autoTLCancel = nil
	e.autoTL = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// autoTLPoll is one heuristic tick (M8-aa, ADR-0030): a routed flow
// burst after at least autoTLQuiet of routed quiet stamps an unpaired
// play-suspected; a down-burst of at least autoTLBurstBytes closes the
// pair. Both stamps ride the log ring (AUTO-MARK lines — the paste
// carries their timestamps even when the report's last-pair line has
// since been replaced by a newer measurement).
func (e *Engine) autoTLPoll() {
	d := e.stats.Diagnostic()
	now := e.opts.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.autoTL
	if a == nil {
		return
	}
	if d.RoutedFlows > a.routedFlows {
		if now.Sub(a.lastRoutedAt) >= autoTLQuiet && a.playAt == nil {
			play := now
			a.playAt = &play
			e.logger.Info("AUTO-MARK play-suspected (routed burst after quiet)")
		}
		a.routedFlows = d.RoutedFlows
		a.lastRoutedAt = now
	}
	if a.playAt != nil && d.BytesDown-a.lastDown >= autoTLBurstBytes {
		a.pairs = append(a.pairs, autoPair{play: *a.playAt, data: now})
		if len(a.pairs) > autoTLMaxPairs {
			a.pairs = a.pairs[len(a.pairs)-autoTLMaxPairs:]
		}
		a.playAt = nil
		e.logger.Info("AUTO-MARK first-data (down burst)")
	}
	a.lastDown = d.BytesDown
}

// autoTLPairs returns the heuristic's closed pairs (the report reads
// the last one and the count).
func (e *Engine) autoTLPairs() []autoPair {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.autoTL == nil {
		return nil
	}
	return append([]autoPair(nil), e.autoTL.pairs...)
}

// Resolver exposes the DoH resolver (for the self-test and diagnostics).
func (e *Engine) Resolver() *doh.Resolver { return e.resolver.Load() }

// MeasuredSleep reports the real 1 ms sleep length measured at startup.
func (e *Engine) MeasuredSleep() time.Duration { return e.measuredSleep }

// Settings returns a copy of the current, normalized settings. The value
// is always normalized (Load and SaveSettingsJSON both run Normalize),
// so callers may trust it as the engine's enforced view — the M6 facade
// reads the per-app allow-list through here. A copy: the caller can
// never race a concurrent save.
func (e *Engine) Settings() config.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

// Start begins the connect sequence asynchronously (phase 1 skeleton:
// listener + attach; the quick check and heal sweep arrive in phase 2).
// A second call while connecting cancels the first cleanly (file 01 §10.5).
func (e *Engine) Start() error {
	e.mu.Lock()
	switch e.phase {
	case PhaseConnecting:
		e.mu.Unlock()
		e.cancelStart()
		return nil
	case PhaseOn, PhaseDegraded:
		e.mu.Unlock()
		return nil
	case PhaseStopping:
		e.mu.Unlock()
		return errors.New("busy")
	}
	e.setPhaseLocked(PhaseConnecting, "starting")
	e.runGen++
	gen := e.runGen
	e.mu.Unlock()
	e.emitState()

	go e.connectSequence(gen)
	return nil
}

// startListener binds the proxy and records the port.
func (e *Engine) startListener() error {
	idle := config.IdleTimeout
	if e.settings.Advanced.IdleTimeoutSec > 0 {
		idle = time.Duration(e.settings.Advanced.IdleTimeoutSec) * time.Second
	}
	// M8-aa (ADR-0029): Android's lean stall budget is 8 s — the
	// debug.18 report's three mid-response corpses each held the
	// app's retry hostage for the whole 15 s desktop budget. The
	// platform gate lives HERE (the engine's listener build); the
	// proxy's stall semantics are untouched and every desktop
	// build keeps config.StallTimeout byte-for-byte.
	stall := config.StallTimeout
	if e.platform.Name() == "android" {
		stall = config.StallLeanTimeoutAndroid
	}
	srv, err := proxy.Start(proxy.Config{
		Host:                  config.ListenHost,
		Port:                  e.settings.Port,
		AutoPort:              e.settings.AutoPort,
		MaxConns:              config.MaxConcurrentConnections,
		RequestTimeout:        config.RequestReadTimeout,
		FirstRecordTimeout:    config.FirstRecordReadTimeout,
		FirstByteTimeout:      config.FirstByteTimeout,
		FirstByteSalvageDials: config.FirstByteSalvageDials,
		FirstByteRaceDials:    config.FirstByteRaceDials,
		StallTimeout:          stall,
		StallRichTimeout:      config.StallRichTimeout,
		StallRichBytes:        config.StallRichBytes,
		DeadIPPenalty:         config.DeadIPPenalty,
		// M8-ab (ADR-0031): the cross-origin escape hatch — a
		// googlevideo flow whose own answer set keeps dying dials
		// the good pool's verified-live origins instead.
		GoodIPSubstitution: config.GoodIPSubstitution,
		IdleTimeout:        idle,
		MaxHeaderBytes:     config.MaxHeaderBytes,
		// M8-z (ADR-0024): the platform-aware setting the doors consult
		// through fragmentHost, the plain-relay safety net's budget, and
		// the report's per-host table.
		FragmentAllRouted:     e.settings.FragmentAllRouted,
		PlainFirstByteTimeout: config.PlainFirstByteTimeout,
		Hosts:                 e.hostTable,
		Resolver:              e.resolver.Load(),
		Dialer:                e.dialer,
		DialTimeoutOverall:    config.DialTimeoutOverall,
		FragmentPreset:        e.currentPreset(),
		FragmentConfig:        e.currentFragmentConfig(),
		RNG:                   e.rng,
		Stats:                 e.stats,
		Logger:                e.logger.Logger,
		LogHostnames:          e.settings.Advanced.LogDomains,
		Domains:               e.settings.Advanced.Domains,
		Mode:                  e.settings.Mode,
	})
	if err != nil {
		return err
	}
	// M8-ab (ADR-0033): seed the persisted verified-live pool — the
	// origins the LAST session verified, gvideo-flagged with the
	// shorter seed TTL, so the first video of this session rides them
	// instead of re-paying the whole discovery cost. A stale seed
	// self-corrects: a flow death drops its mark and boxes the IP.
	if seeded := e.loadGoodIPPool(); len(seeded) > 0 {
		srv.SeedGoodIPs(seeded)
	}
	e.mu.Lock()
	e.server = srv
	e.port = srv.Port()
	e.stallLean = stall
	e.mu.Unlock()
	e.logger.Info("proxy started", "port", e.port)
	return nil
}

// stopListener stops the proxy server if it is running.
func (e *Engine) stopListener() {
	e.mu.Lock()
	srv := e.server
	e.server = nil
	e.mu.Unlock()
	if srv != nil {
		srv.StopWithTimeout()
	}
}

// failStart records a start failure with its typed code.
func (e *Engine) failStart(err error) {
	code := mapErrorCode(err)
	e.logger.Error("start failed", "reason", errKind(err), "code", code)
	e.stopListener()
	e.mu.Lock()
	e.errCode = code
	e.setPhaseLocked(PhaseError, "")
	e.mu.Unlock()
	e.emitState()
}

// cancelStart cleanly abandons a connecting start (button press while
// connecting): undo what was done and go back to off.
func (e *Engine) cancelStart() {
	e.logger.Info("connect canceled by the user")
	e.cancelCurrentRun()
	e.stopListener()
	if err := e.platform.Detach(); err != nil {
		e.logger.Warn("detach after cancel failed", "reason", errKind(err))
	}
	e.mu.Lock()
	e.latencyMs = 0
	e.clearFailLocked()
	e.setPhaseLocked(PhaseOff, "")
	e.mu.Unlock()
	e.emitState()
}

// Stop turns everything off and restores the platform state.
func (e *Engine) Stop() error {
	e.mu.Lock()
	switch e.phase {
	case PhaseOff, PhaseError:
		e.mu.Unlock()
		return nil
	case PhaseStopping:
		e.mu.Unlock()
		return nil
	}
	e.setPhaseLocked(PhaseStopping, "")
	e.mu.Unlock()
	e.emitState()

	e.cancelCurrentRun()
	e.stopStatsLoop()
	e.stopWatchdog()
	e.stopNetMon()
	e.stopGvideoLoop()   // M8-z: the prewarm loop dies with the run
	e.stopAutoTimeline() // M8-aa (ADR-0030): the heuristic poller too
	e.stopListener()
	if err := e.platform.Detach(); err != nil {
		e.logger.Warn("detach failed", "reason", errKind(err))
	}
	// R21: the in-memory ring is cleared on Stop — and the timeline
	// marks and the dead-origin probe's once-per-run latch with it (the
	// next run's report starts clean; M8-z). M8-aa: the effective lean
	// budget resets too (the next run re-derives it from its platform).
	e.logger.Ring.Clear()
	e.mu.Lock()
	e.marks = nil
	e.markSnapshot = nil
	e.probeDone = false
	e.probeLines = nil
	e.stallLean = 0
	e.mu.Unlock()
	e.mu.Lock()
	e.latencyMs = 0 // a live metric; it stops with the proxy
	e.clearFailLocked()
	e.setPhaseLocked(PhaseOff, "")
	e.mu.Unlock()
	e.emitState()
	e.stopWait.Wait()
	return nil
}

// Toggle switches based on the current phase.
func (e *Engine) Toggle() error {
	e.mu.Lock()
	phase := e.phase
	e.mu.Unlock()
	switch phase {
	case PhaseOn, PhaseDegraded:
		return e.Stop()
	default:
		return e.Start()
	}
}

// startStatsLoop emits stats once per second while on, and every
// usageAutosaveEvery seconds snapshots the live usage session into the
// store for crash recovery (round-3).
func (e *Engine) startStatsLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	old := e.statsCtx
	e.statsCtx = cancel
	e.autosaveTk = 0
	e.mu.Unlock()
	if old != nil {
		old()
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.mu.Lock()
				phase := e.phase
				e.mu.Unlock()
				if phase != PhaseOn && phase != PhaseDegraded {
					continue
				}
				e.emit("stats", marshalJSON(e.stats.Sample()))
				e.mu.Lock()
				e.autosaveTk++
				due := time.Duration(e.autosaveTk)*time.Second >= usageAutosaveEvery
				if due {
					e.autosaveTk = 0
				}
				e.mu.Unlock()
				if due {
					e.autosaveUsage()
				}
			}
		}
	}()
}

// stopStatsLoop halts the stats ticker.
func (e *Engine) stopStatsLoop() {
	e.mu.Lock()
	cancel := e.statsCtx
	e.statsCtx = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// prewarm resolves the video names ahead of the first click (optional,
// file 01 section 6).
func (e *Engine) prewarm() {
	names := config.PrewarmNames
	r := e.resolver.Load()
	go func() {
		for _, n := range names {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			r.Lookup(ctx, n)
			cancel()
		}
	}()
}

// setPhaseLocked updates phase bookkeeping; caller holds e.mu.
func (e *Engine) setPhaseLocked(p Phase, detail string) {
	wasOn := e.phase == PhaseOn || e.phase == PhaseDegraded
	e.phase = p
	e.detail = detail
	e.since = e.opts.Now()
	if p != PhaseConnecting {
		e.detail = detail
	}
	// Round-3: anything that leaves on/degraded — a user Stop, a quit,
	// an engine error — closes the usage session (a cancelled or
	// failed CONNECT never opened one).
	if wasOn && p != PhaseOn && p != PhaseDegraded {
		e.endUsageSessionLocked()
	}
}

// noticeVisibleLocked applies the notice expiry; caller holds e.mu.
func (e *Engine) noticeVisibleLocked() *Notice {
	if e.notice == nil {
		return nil
	}
	if !e.opts.Now().Before(e.noticeUntil) {
		e.notice = nil
		return nil
	}
	n := *e.notice
	return &n
}

// setLatency records the last successful YouTube check duration (ms).
func (e *Engine) setLatency(ms int64) {
	if ms <= 0 {
		return
	}
	e.mu.Lock()
	e.latencyMs = ms
	e.mu.Unlock()
}

// failPriority ranks self-test steps by how root-cause they are: when the
// secure lookup fails, the youtube/video steps fail after it as a cascade,
// so a dns failure must not be overwritten by the youtube failure that
// follows it in the heal sweep (D44).
var failPriority = map[string]int{"dns": 3, "listener": 2, "youtube": 1, "video": 0}

// setFail records the failing self-test step and its code, keeping the
// more root-cause failure when one is already recorded (D44).
func (e *Engine) setFail(step, code string) {
	if step == "" {
		return
	}
	p, ok := failPriority[step]
	if !ok {
		p = 0
	}
	e.mu.Lock()
	if e.failStep != "" {
		if fp, ok := failPriority[e.failStep]; !ok || fp >= p {
			e.mu.Unlock()
			return // an equal or more root-cause failure stands
		}
	}
	e.failStep = step
	e.failCode = code
	e.mu.Unlock()
}

// clearFailLocked forgets the recorded failure; caller holds e.mu.
func (e *Engine) clearFailLocked() {
	e.failStep = ""
	e.failCode = ""
}

// GetState returns the current state.
func (e *Engine) GetState() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	method := e.settings.Method
	mode := e.settings.Mode
	port := e.port
	st := State{
		Phase:  e.phase,
		Detail: e.detail,
		Since:  e.since,
		Port:   port,
		Mode:   mode,
		Method: method,
	}
	if (e.phase == PhaseOn || e.phase == PhaseDegraded) && e.latencyMs > 0 {
		ms := e.latencyMs
		st.LatencyMs = &ms
	}
	if e.phase == PhaseDegraded && e.failStep != "" {
		st.Fail = &StateFail{Step: e.failStep, Code: e.failCode}
	}
	if n := e.noticeVisibleLocked(); n != nil {
		st.Notice = n
	}
	if e.errCode != "" && (e.phase == PhaseError || e.phase == PhaseDegraded) {
		st.Error = &StateError{Code: e.errCode}
	}
	// Round-3 usage fields (usage.go owns the rules): the live
	// session's deltas while connected, the last finished session's
	// totals afterwards, and the record itself for the screen's
	// Last-session line.
	if e.sessLive {
		down, up := e.stats.Totals()
		st.DownBytes = down - e.sessBaseDown
		st.UpBytes = up - e.sessBaseUp
		if st.DownBytes < 0 {
			st.DownBytes = 0
		}
		if st.UpBytes < 0 {
			st.UpBytes = 0
		}
	} else if e.lastSess != nil {
		st.DownBytes = e.lastSess.DownBytes
		st.UpBytes = e.lastSess.UpBytes
	}
	if e.lastSess != nil {
		rec := *e.lastSess
		st.LastSession = &rec
	}
	return st
}

// RunningServer reports the live proxy server (nil when off).
func (e *Engine) RunningServer() *proxy.Server {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.server
}

// emitState sends the state event.
func (e *Engine) emitState() {
	e.emit("state", marshalJSON(e.GetState()))
}

// emit sends any event to the sink.
func (e *Engine) emit(name, payload string) {
	e.mu.Lock()
	sink := e.events
	e.mu.Unlock()
	if sink != nil {
		sink.OnEvent(name, payload)
	}
}

// Shutdown stops everything (used by Quit).
func (e *Engine) Shutdown() {
	_ = e.Stop()
	e.logger.Close()
}

// Quit stops the engine and invokes the shell callback.
func (e *Engine) Quit() {
	e.Shutdown()
	e.mu.Lock()
	fn := e.onQuit
	e.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ConfigDir returns the platform config directory.
func (e *Engine) ConfigDir() string { return e.platform.ConfigDir() }

// mapErrorCode converts lower-level errors to UI error codes.
func mapErrorCode(err error) string {
	var refusal *PlatformRefusalError
	if errors.As(err, &refusal) {
		// The facade whitelists the vocabulary at the bridge; mapping here
		// re-checks it so an unknown code can never invent a UI error.
		switch refusal.Code {
		case "NO_TARGET_APPS":
			return refusal.Code
		default:
			return "UNKNOWN"
		}
	}
	switch {
	case errors.Is(err, proxy.ErrPortBusy):
		return "PORT_BUSY"
	case errors.Is(err, doh.ErrUnreachable):
		return "DNS_UNREACHABLE"
	case errors.Is(err, doh.ErrBlockedAnswer):
		return "DNS_BLOCKED_ANSWER"
	default:
		return "UNKNOWN"
	}
}

// errKind reduces an error to a short reason without hostnames or IPs.
func errKind(err error) string {
	if err == nil {
		return ""
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
