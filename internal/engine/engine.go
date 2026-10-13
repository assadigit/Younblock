package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/doh"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/logx"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/proxy"
	"github.com/assadigit/Younblock/internal/stats"
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

// State is the full UI-facing state.
type State struct {
	Phase  Phase         `json:"phase"`
	Detail string        `json:"detail"`
	Since  time.Time     `json:"since"`
	Port   int           `json:"port"`
	Mode   config.Mode   `json:"mode"`
	Method config.Method `json:"method"`
	Notice *Notice       `json:"notice,omitempty"`
	Error  *StateError   `json:"error,omitempty"`
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
	resolver *doh.Resolver
	rng      *fragment.SafeRNG
	dialer   *netx.Dialer

	mu       sync.Mutex
	phase    Phase
	detail   string
	since    time.Time
	notice   *Notice
	errCode  string
	port     int
	server   *proxy.Server
	statsCtx context.CancelFunc

	events EventSink
	onQuit func()

	measuredSleep time.Duration
	stopWait      sync.WaitGroup
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
	s, notice, err := config.Load(dir)
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
		opts:     opts,
		platform: opts.Platform,
		settings: s,
		logger:   logger,
		stats:    stats.New(),
		rng:      fragment.NewDefaultRNG(),
		phase:    PhaseOff,
		since:    opts.Now(),
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
	return e, nil
}

// rebuildResolver (re)creates the DoH resolver from current settings.
func (e *Engine) rebuildResolver() {
	cfg := doh.Config{
		Servers:        e.settings.Advanced.DoHServers,
		Timeout:        config.DoHTimeout,
		BlockPrivate:   config.BlockPrivateTargets && !e.opts.AllowPrivateTargets,
		Control:        e.platform.DialControl(),
		FragmentPreset: e.currentPreset(),
		FragmentConfig: e.currentFragmentConfig(),
		RNG:            e.rng,
		Logger:         e.logger.Logger,
		LogHostnames:   e.settings.Advanced.LogDomains,
		Stats:          e.stats,
		Now:            e.opts.Now,
	}
	e.resolver = doh.New(cfg)
}

// currentPreset maps the method setting to a fragmentation preset.
func (e *Engine) currentPreset() fragment.Preset {
	m := e.settings.Method
	if m == config.MethodAuto {
		return fragment.Balanced
	}
	return fragment.Preset(m)
}

// currentFragmentConfig merges preset defaults with the advanced overrides.
func (e *Engine) currentFragmentConfig() fragment.Config {
	pc := fragment.PresetConfig(e.currentPreset())
	adv := e.settings.Advanced
	if adv.PieceSizeMin > 0 {
		pc.Min = adv.PieceSizeMin
	}
	if adv.PieceSizeMax > 0 && adv.PieceSizeMax >= pc.Min {
		pc.Max = adv.PieceSizeMax
	}
	if adv.PauseMs > 0 {
		pc.Pause = time.Duration(adv.PauseMs) * time.Millisecond
	}
	return pc
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

// Stats exposes the counters for diagnostics.
func (e *Engine) Stats() *stats.Counters { return e.stats }

// Resolver exposes the DoH resolver (for the self-test and diagnostics).
func (e *Engine) Resolver() *doh.Resolver { return e.resolver }

// MeasuredSleep reports the real 1 ms sleep length measured at startup.
func (e *Engine) MeasuredSleep() time.Duration { return e.measuredSleep }

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
	e.mu.Unlock()
	e.emitState()

	go func() {
		if err := e.startListener(); err != nil {
			e.failStart(err)
			return
		}
		if err := e.platform.Attach(e.port, e.settings); err != nil {
			e.logger.Error("platform attach failed", "reason", errKind(err))
			e.stopListener()
			e.mu.Lock()
			e.errCode = "PROXY_SETTING_FAILED"
			e.setPhaseLocked(PhaseError, "")
			e.mu.Unlock()
			e.emitState()
			return
		}
		e.mu.Lock()
		e.setPhaseLocked(PhaseOn, "")
		e.mu.Unlock()
		e.emitState()
		e.startStatsLoop()
		e.prewarm()
	}()
	return nil
}

// startListener binds the proxy and records the port.
func (e *Engine) startListener() error {
	idle := config.IdleTimeout
	if e.settings.Advanced.IdleTimeoutSec > 0 {
		idle = time.Duration(e.settings.Advanced.IdleTimeoutSec) * time.Second
	}
	srv, err := proxy.Start(proxy.Config{
		Host:               config.ListenHost,
		Port:               e.settings.Port,
		AutoPort:           e.settings.AutoPort,
		MaxConns:           config.MaxConcurrentConnections,
		RequestTimeout:     config.RequestReadTimeout,
		FirstRecordTimeout: config.FirstRecordReadTimeout,
		IdleTimeout:        idle,
		MaxHeaderBytes:     config.MaxHeaderBytes,
		Resolver:           e.resolver,
		Dialer:             e.dialer,
		DialTimeoutOverall: config.DialTimeoutOverall,
		FragmentPreset:     e.currentPreset(),
		FragmentConfig:     e.currentFragmentConfig(),
		RNG:                e.rng,
		Stats:              e.stats,
		Logger:             e.logger.Logger,
		LogHostnames:       e.settings.Advanced.LogDomains,
		Domains:            e.settings.Advanced.Domains,
		Mode:               e.settings.Mode,
	})
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.server = srv
	e.port = srv.Port()
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
	e.stopListener()
	if err := e.platform.Detach(); err != nil {
		e.logger.Warn("detach after cancel failed", "reason", errKind(err))
	}
	e.mu.Lock()
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

	e.stopStatsLoop()
	e.stopListener()
	if err := e.platform.Detach(); err != nil {
		e.logger.Warn("detach failed", "reason", errKind(err))
	}
	// R21: the in-memory ring is cleared on Stop.
	e.logger.Ring.Clear()
	e.mu.Lock()
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

// startStatsLoop emits stats once per second while on.
func (e *Engine) startStatsLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	old := e.statsCtx
	e.statsCtx = cancel
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
	go func() {
		for _, n := range names {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e.resolver.Lookup(ctx, n)
			cancel()
		}
	}()
}

// setPhaseLocked updates phase bookkeeping; caller holds e.mu.
func (e *Engine) setPhaseLocked(p Phase, detail string) {
	e.phase = p
	e.detail = detail
	e.since = e.opts.Now()
	if p != PhaseConnecting {
		e.detail = detail
	}
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
	if e.notice != nil {
		n := *e.notice
		st.Notice = &n
	}
	if e.errCode != "" && e.phase == PhaseError {
		st.Error = &StateError{Code: e.errCode}
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
