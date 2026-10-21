package engine

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/health"
)

// Checker is the engine's health probe. The real implementation runs the
// self-test steps through the live proxy; tests inject fake networks that
// accept only specific methods (file 01, section 11, Phase 2).
type Checker interface {
	// QuickCheck runs self-test steps 1-3 (listener, dns, youtube) within
	// the context budget for the given method.
	QuickCheck(ctx context.Context, method config.Method) bool
	// HealCheck runs self-test steps 3-4 (youtube, video) within the
	// context budget for the given method.
	HealCheck(ctx context.Context, method config.Method) bool
}

// healthChecker is the production Checker built on internal/health.
type healthChecker struct {
	eng *Engine
}

func (c *healthChecker) config(budget time.Duration) health.Config {
	addr := ""
	if srv := c.eng.RunningServer(); srv != nil {
		addr = srv.Addr()
	}
	cfg := health.Config{
		ProxyAddr: addr,
		Resolver:  c.eng.resolver.Load(),
		Timeout:   budget,
	}
	if c.eng.opts.HealthOverrides != nil {
		c.eng.opts.HealthOverrides(&cfg)
	}
	return cfg
}

func (c *healthChecker) QuickCheck(ctx context.Context, method config.Method) bool {
	budget := quickCheckBudget(ctx)
	steps := health.RunSteps(ctx, c.config(budget), []string{"listener", "dns", "youtube"}, nil)
	for _, s := range steps {
		if !s.OK {
			c.eng.recordFail(s.ID, s.Code) // D44: say which part failed
			return false
		}
	}
	c.eng.recordLatency(steps) // the "ping" display (DECISIONS.md D41)
	return true
}

func (c *healthChecker) HealCheck(ctx context.Context, method config.Method) bool {
	budget := quickCheckBudget(ctx)
	steps := health.RunSteps(ctx, c.config(budget), []string{"youtube", "video"}, nil)
	for _, s := range steps {
		if !s.OK {
			c.eng.recordFail(s.ID, s.Code)
			return false
		}
	}
	c.eng.recordLatency(steps)
	return true
}

// recordLatency stores the duration of a successful "youtube" step as the
// live response-time metric. It looks at the last successful youtube step
// so both the watchdog's light check and the full self-test feed it.
func (e *Engine) recordLatency(steps []health.Step) {
	for _, s := range steps {
		if s.ID == "youtube" && s.OK && s.MS > 0 {
			e.setLatency(s.MS)
		}
	}
}

// recordFail stores a failing step and its code so the degraded state can
// say WHAT is broken (D44). setFail keeps the more root-cause failure when
// several steps fail in one cascade.
func (e *Engine) recordFail(step, code string) {
	e.setFail(step, code)
}

// quickCheckBudget derives a per-step budget from the context deadline.
func quickCheckBudget(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return config.DoHTimeout * 2
	}
	remaining := time.Until(deadline)
	if remaining < 500*time.Millisecond {
		remaining = 500 * time.Millisecond
	}
	return remaining
}

// Notification texts (file 01, section 9): only failure and recovery.
const (
	notifyTryingText = "YouTube stopped responding. Trying to fix it…"
	notifyFixedText  = "Fixed"
	notifyFailedText = "Couldn't fix it. Open Younblock."
)

// Timing accessors with test overrides.
func (e *Engine) startTimeout() time.Duration {
	if e.opts.StartTimeout > 0 {
		return e.opts.StartTimeout
	}
	return config.StartTotalTimeout
}

func (e *Engine) watchdogInterval() time.Duration {
	if e.opts.WatchdogInterval > 0 {
		return e.opts.WatchdogInterval
	}
	return config.WatchdogInterval
}

func (e *Engine) watchdogFailLimit() int {
	if e.opts.WatchdogFailLimit > 0 {
		return e.opts.WatchdogFailLimit
	}
	return config.WatchdogFailures
}

func (e *Engine) healCooldown() time.Duration {
	if e.opts.HealCooldown > 0 {
		return e.opts.HealCooldown
	}
	return config.HealSweepCooldown
}

func (e *Engine) netPollInterval() time.Duration {
	if e.opts.NetworkPollInterval > 0 {
		return e.opts.NetworkPollInterval
	}
	return 5 * time.Second
}

// cancelCurrentRun aborts a connecting start (Stop or button cancel).
// M7 (D82): bumping runGen makes a connect goroutine that was spawned
// but not yet scheduled STALE — it exits quietly at its first
// instruction instead of registering a fresh context nothing can cancel
// (the spawn-race window the late-refusal test exposed: such a zombie
// run could resurrect a phase after its own Stop — the exact thing
// D77's quiet-cancellation rule exists to prevent).
func (e *Engine) cancelCurrentRun() {
	e.mu.Lock()
	cancel := e.runCancel
	e.runCancel = nil
	e.runAborted = true
	e.runGen++
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// connectSequence is the full Start sequence (file 01, section 10):
// starting -> checking -> (heal sweep) -> on | degraded, capped at the
// total timeout. The M5 platform start gate (D77, optional) sits BEFORE
// the listener: the run holds in "connecting" while the platform is not
// ready, outside the start budget — a consent dialog is not a network
// failure — and a cancellation exits quietly (the canceller owns the
// phase).
func (e *Engine) connectSequence(gen uint64) {
	// runCtx is the run's lifetime: cancelled by Stop / a second Start /
	// Shutdown through the runCancel registration below. The start budget
	// is derived from it only AFTER the gate, so waiting at the gate never
	// burns the timeout.
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	// D82: the stale-check and the registration are one critical section.
	// A goroutine whose generation was superseded (its Stop/second-Start/
	// Shutdown landed between spawn and first instruction) exits quietly
	// HERE — it never resets runAborted, never registers a context, never
	// touches the phase. A fresh run registers atomically, so every future
	// cancel reaches it through runCancel.
	e.mu.Lock()
	if gen != e.runGen {
		e.mu.Unlock()
		return
	}
	e.runAborted = false
	old := e.runCancel
	e.runCancel = runCancel
	e.mu.Unlock()
	if old != nil {
		old()
	}

	if g, ok := e.platform.(StartGater); ok {
		if err := g.WaitStartReady(runCtx); err != nil {
			if runCtx.Err() != nil {
				// Cancelled (Stop / a second Start / Shutdown): the
				// canceller owns the phase transition — quiet exit (D77).
				e.logger.Info("connect run released at the platform gate",
					"reason", errKind(err))
				return
			}
			// A platform refusal (D81): the connect cannot honestly
			// proceed (e.g. none of the allow-list's apps is installed).
			// The ordinary failure path owns the rest — phase "error"
			// with the typed code, the state emitted, the notice path
			// unchanged. failStart's stopListener is a no-op here (the
			// gate sits before the listener).
			e.failStart(err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(runCtx, e.startTimeout())
	defer cancel()

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

	// Quick check: self-test steps 1-3, max 8 s.
	e.setPhaseDetail(PhaseConnecting, "checking")
	e.emitState()
	if e.checker.QuickCheck(ctx, e.currentMethod()) {
		e.goOn()
		return
	}
	if ctx.Err() != nil {
		e.abandonCanceledStart()
		return
	}

	// One automatic heal sweep per Start press (R13).
	e.healSweep(ctx, false)
}

// healSweep tries every method in order; the first that passes becomes the
// working method with a METHOD_SWITCHED notice. Returns the winning method
// or "" when nothing worked (then the engine goes degraded).
func (e *Engine) healSweep(ctx context.Context, userInvoked bool) string {
	if !userInvoked {
		if !e.healCooldownPassed() {
			e.logger.Warn("automatic heal sweep skipped (cooldown active)")
			e.goDegraded(notifyFailedText)
			return ""
		}
		e.markAutoSweep()
	}
	e.logger.Info("heal sweep started", "userInvoked", userInvoked)
	// M8-r (Ali's debug.10 diagnostic report): the checks that triggered this
	// sweep may have just negative-cached the sweep's own probe name. The
	// QuickCheck's dns failure caches www.youtube.com for the negative TTL
	// (15 s), every method's youtube step then fails instantly on that entry
	// (ErrRecentlyFailed — no server is ever asked again), and a sweep that
	// starts inside the window cannot succeed even on a fully recovered
	// network — the report showed exactly that: "heal sweep found no working
	// method" while the sweep's own video handshakes were completing. A sweep
	// is a suspected-network-change event: reset the resolver exactly like
	// the network monitor does on a change (R19) and probe from zero.
	e.resolver.Load().Reset()
	prev := e.currentMethod()
	for _, m := range config.HealSweepOrder {
		if ctx.Err() != nil {
			e.abandonCanceledStart()
			return ""
		}
		e.setPhaseDetail(PhaseConnecting, "trying:"+string(m))
		e.emitState()
		e.applyMethod(m)
		if e.checker.HealCheck(ctx, m) {
			e.saveMethod(m)
			if m == prev {
				e.setNotice("RECOVERED", "")
			} else {
				e.setNotice("METHOD_SWITCHED", string(m))
			}
			e.logger.Info("heal sweep found a working method", "method", string(m))
			e.goOn()
			return string(m)
		}
	}
	e.logger.Warn("heal sweep found no working method")
	// A failed sweep must not leave the last-tried method live: applyMethod
	// mutates the setting as the sweep probes, and the debug.10 report showed
	// method=legacy after a failed sweep dragged it off the user's saved
	// choice — a silent downgrade delivered by the very sweep that found
	// nothing. Nothing was found, so restore what ran before.
	e.applyMethod(prev)
	e.goDegraded(notifyFailedText)
	return ""
}

// currentMethod returns the effective method setting.
func (e *Engine) currentMethod() config.Method {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings.Method
}

// applyMethod switches the fragmentation method live.
func (e *Engine) applyMethod(m config.Method) {
	e.mu.Lock()
	e.settings.Method = m
	e.mu.Unlock()
	preset, cfg := e.currentPreset(), e.currentFragmentConfig()
	if srv := e.RunningServer(); srv != nil {
		srv.SetFragment(preset, cfg)
	}
	e.resolver.Load().SetFragment(preset, cfg)
}

// saveMethod persists the working method (file 01 section 10: "save it as
// the working method").
func (e *Engine) saveMethod(m config.Method) {
	e.mu.Lock()
	e.settings.Method = m
	e.mu.Unlock()
	if err := config.Save(e.platform.ConfigDir(), e.settingsSnapshot()); err != nil {
		e.logger.Warn("saving the working method failed", "reason", errKind(err))
	}
	e.emitState()
}

// settingsSnapshot returns a copy of the current settings.
func (e *Engine) settingsSnapshot() config.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

// goOn transitions to on (guarded: only from connecting or degraded) and
// starts the background loops.
func (e *Engine) goOn() {
	e.mu.Lock()
	if e.phase != PhaseConnecting && e.phase != PhaseDegraded {
		e.mu.Unlock()
		return
	}
	e.errCode = ""
	e.clearFailLocked() // it works again; forget the recorded failure
	e.setPhaseLocked(PhaseOn, "")
	// Round-3: the engine reaching "on" is what begins a usage session
	// (a heal's degraded→on flap keeps the original start).
	e.startUsageSessionLocked()
	e.mu.Unlock()
	e.emitState()
	e.startStatsLoop()
	e.startWatchdog()
	e.startWarmGate() // M8-an (ADR-0052): the hand-off's readiness gate
	e.startNetMon()
	e.prewarm()
	e.startGvideoLoop()   // M8-z (ADR-0025 §6.2/§6.4): warm DoH conn + googlevideo prewarm
	e.startAutoTimeline() // M8-aa (ADR-0030): the heuristic play detector
}

// goDegraded transitions to degraded with the SELFTEST_FAILED error
// (guarded: only from connecting or on) and notifies once, rate-limited.
func (e *Engine) goDegraded(notifyText string) {
	e.mu.Lock()
	if e.phase != PhaseConnecting && e.phase != PhaseOn {
		e.mu.Unlock()
		return
	}
	e.errCode = "SELFTEST_FAILED"
	e.setPhaseLocked(PhaseDegraded, "")
	e.mu.Unlock()
	e.emitState()
	// The proxy is live and serving the browser even when the checks fail,
	// so the timer and usage keep ticking (an honest "on"), and the
	// watchdog keeps re-checking while degraded (R14) so a later success
	// recovers on its own instead of sitting dead until the next button
	// press (D44).
	e.startStatsLoop()
	e.startWatchdog()
	e.startWarmGate() // M8-an (ADR-0052): degraded serves — the gate runs there too
	e.startNetMon()
	e.startGvideoLoop()   // degraded still serves traffic (an honest "on"): keep the DoH conn warm
	e.startAutoTimeline() // M8-aa: the heuristic runs in degraded too — traffic still flows
	if notifyText != "" {
		e.notifyLimited(NoticeFailure, notifyText)
	}
}

// abandonCanceledStart ends a connect sequence whose context ended. A user
// cancel (Stop or button press) leaves quietly; running out of the total
// budget gives the final degraded answer (the 35 s cap).
func (e *Engine) abandonCanceledStart() {
	e.mu.Lock()
	aborted := e.runAborted
	phase := e.phase
	e.mu.Unlock()
	if phase != PhaseConnecting || aborted {
		return // Stop or cancel already moved the state on
	}
	e.logger.Warn("start sequence hit the total time cap")
	e.goDegraded(notifyFailedText)
}

// setPhaseDetail updates detail while staying in connecting.
func (e *Engine) setPhaseDetail(p Phase, detail string) {
	e.mu.Lock()
	e.setPhaseLocked(p, detail)
	e.mu.Unlock()
}

// setNotice shows a notice line for NoticeDuration (file 02, section 4.1).
func (e *Engine) setNotice(code, arg string) {
	e.mu.Lock()
	e.notice = &Notice{Code: code, Arg: arg}
	e.noticeUntil = e.opts.Now().Add(config.NoticeDuration)
	e.mu.Unlock()
	e.emitState()
}

// healCooldownPassed reports whether an automatic sweep may run now.
func (e *Engine) healCooldownPassed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.opts.Now().Before(e.lastAutoSweep.Add(e.healCooldown()))
}

// markAutoSweep records the time of the last automatic sweep.
func (e *Engine) markAutoSweep() {
	e.mu.Lock()
	e.lastAutoSweep = e.opts.Now()
	e.mu.Unlock()
}

// notifyEpisodeGrace is how long after a notification its outcome may
// still be delivered without starting a new rate-limited episode.
const notifyEpisodeGrace = 60 * time.Second

// notifyLimited sends a platform notification. The 10-minute rate limit
// throttles notification EPISODES (file 01, section 9): "YouTube stopped
// responding. Trying to fix it…" is followed by its outcome ("Fixed" or
// "Couldn't fix it. Open Younblock."), and both halves of that pair are
// delivered; only a brand-new episode is suppressed inside the window.
func (e *Engine) notifyLimited(kind NoticeKind, text string) {
	now := e.opts.Now()
	e.mu.Lock()
	if now.Sub(e.lastNotifyAt) > notifyEpisodeGrace {
		// Starting a new episode: apply the 10-minute limit.
		if now.Before(e.episodeStart.Add(config.NotificationRateLimit)) {
			e.mu.Unlock()
			return
		}
		e.episodeStart = now
	}
	e.lastNotifyAt = now
	e.mu.Unlock()
	e.platform.Notify(kind, text)
	e.logger.Info("notification sent", "kind", string(kind))
}

// startWatchdog runs the light check every watchdogInterval while the
// engine is on or degraded (R14).
func (e *Engine) startWatchdog() {
	e.mu.Lock()
	if e.watchdogCancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.watchdogCancel = cancel
	e.wdFails = 0
	e.mu.Unlock()
	go func() {
		ticker := time.NewTicker(e.watchdogInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.watchdogTick()
			}
		}
	}()
}

// stopWatchdog halts the watchdog.
func (e *Engine) stopWatchdog() {
	e.mu.Lock()
	cancel := e.watchdogCancel
	e.watchdogCancel = nil
	e.wdFails = 0
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// watchdogTick runs one light check and the failure/recovery transitions.
func (e *Engine) watchdogTick() {
	e.mu.Lock()
	phase := e.phase
	e.mu.Unlock()
	if phase != PhaseOn && phase != PhaseDegraded {
		return
	}
	e.ensureProxyAttached()
	if e.checker.QuickCheck(context.Background(), e.currentMethod()) {
		e.mu.Lock()
		wasDegraded := e.phase == PhaseDegraded
		e.wdFails = 0
		e.mu.Unlock()
		if wasDegraded {
			// One later success returns to on (R14).
			e.setNotice("RECOVERED", "")
			e.goOn()
			e.notifyLimited(NoticeRecovery, notifyFixedText)
		}
		return
	}
	e.mu.Lock()
	e.wdFails++
	fails := e.wdFails
	e.mu.Unlock()
	if phase == PhaseOn && fails >= e.watchdogFailLimit() {
		e.logger.Warn("watchdog failures reached the limit", "failures", fails)
		e.notifyLimited(NoticeFailure, notifyTryingText)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), e.startTimeout())
			defer cancel()
			e.healSweep(ctx, false)
		}()
	}
}

// warmGateSettle returns the settle window (M8-an, ADR-0052; the
// opts override pattern beside every other timing knob).
func (e *Engine) warmGateSettle() time.Duration {
	if e.opts.WarmGateSettle > 0 {
		return e.opts.WarmGateSettle
	}
	return config.WarmGateSettle
}

// warmGateRetry returns the re-check cadence while the light check
// keeps failing (M8-an, ADR-0052).
func (e *Engine) warmGateRetry() time.Duration {
	if e.opts.WarmGateRetry > 0 {
		return e.opts.WarmGateRetry
	}
	return config.WarmGateRetry
}

// warmGateCap returns the never-trap bound (M8-an, ADR-0052).
func (e *Engine) warmGateCap() time.Duration {
	if e.opts.WarmGateCap > 0 {
		return e.opts.WarmGateCap
	}
	return config.WarmGateCap
}

// startWarmGate arms the serving run's readiness gate (M8-an,
// ADR-0052 — the tester's own prescription: "make the user wait and
// see connecting, only you're sure that even initial video is smooth
// and fast"). The debug.30 field report showed the connect-time
// hand-off firing at second zero — the earliest instant the stack
// exists — so a cold-started YouTube app burst into a chain still
// paying its own per-flow establishment, with the dead-cluster memory
// empty (per-session), and the felt result was the old 1–2 minute
// first-video open back. The gate holds the hand-off closed until the
// serving phase HELD for the settle window and one light check
// (listener, dns, youtube — the watchdog's own blessed emission
// classes, never googlevideo) re-proved the chain through the door;
// a failing check retries on its cadence, and the cap opens the gate
// regardless — a late hand-off is recoverable (the user taps
// YouTube), a trapped one is not (the M8-am refusal-path rule). Once
// per run: warm stays true through on↔degraded flaps (a heal sweep
// is the same run), and the phase machine re-arms the gate when the
// run ends (setPhaseLocked).
func (e *Engine) startWarmGate() {
	e.mu.Lock()
	if e.warm || e.warmCancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.warmCancel = cancel
	e.mu.Unlock()
	go func() {
		settle := time.NewTimer(e.warmGateSettle())
		defer settle.Stop()
		capT := time.NewTimer(e.warmGateCap())
		defer capT.Stop()
		for {
			select {
			case <-ctx.Done():
				return // the run ended; setPhaseLocked reset warm and cleared the cancel
			case <-capT.C:
				e.logger.Warn("warm gate: the cap reached — opening without a proven re-check (the hand-off is recoverable)")
				e.fireWarm()
				return
			case <-settle.C:
				// The re-check: one QuickCheck through the door — the same
				// call the watchdog makes on every tick, the same emission
				// classes the connect sequence already paid (ADR-0041's
				// discipline is untouched: no googlevideo handshake).
				if e.checker.QuickCheck(ctx, e.currentMethod()) {
					e.fireWarm()
					return
				}
				// Failed — the chain flapped (a radio blink, a heal sweep
				// mid-flight). Retry on the cadence; the phase may be
				// anything between while the run lives.
				settle.Reset(e.warmGateRetry())
			}
		}
	}()
}

// fireWarm opens the gate exactly once per run (M8-an, ADR-0052): the
// state event this emits is the hand-off's trigger (the Kotlin shell
// fires kill+launch when it reads serving AND warm AND tunBound) and
// the connect screen's cue to swap "getting the first video ready"
// for "Connected".
func (e *Engine) fireWarm() {
	e.mu.Lock()
	if e.warm {
		e.mu.Unlock()
		return
	}
	e.warm = true
	e.mu.Unlock()
	e.logger.Info("warm gate: the video lane is ready (the chain held and re-proved)")
	e.emitState()
}

// ensureProxyAttached re-asserts the system proxy when another program
// overwrote it (R13: the rewrite is logged and surfaced as a notice, not
// silent). The watchdog calls this on every tick while on (D46).
func (e *Engine) ensureProxyAttached() {
	e.mu.Lock()
	port := e.port
	settings := e.settings
	e.mu.Unlock()
	changed, err := e.platform.EnsureAttached(port, settings)
	if err != nil {
		e.logger.Warn("re-asserting the windows proxy failed", "reason", errKind(err))
		return
	}
	if changed {
		e.logger.Warn("another program changed the windows proxy; ours was re-applied")
		e.setNotice("PROXY_REAPPLIED", "")
		e.emitState()
	}
}

// startNetMon polls interface addresses for network changes (R19).
func (e *Engine) startNetMon() {
	e.mu.Lock()
	if e.netMonCancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.netMonCancel = cancel
	e.mu.Unlock()
	lister := e.opts.InterfaceLister
	if lister == nil {
		lister = net.InterfaceAddrs
	}
	go func() {
		last := addrSet(lister())
		ticker := time.NewTicker(e.netPollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cur := addrSet(lister())
				if !sameAddrSet(last, cur) {
					last = cur
					e.logger.Info("network change detected; resetting DNS and re-checking")
					e.resolver.Load().Reset()
					e.watchdogTick()
				}
			}
		}
	}()
}

// stopNetMon halts the network monitor.
func (e *Engine) stopNetMon() {
	e.mu.Lock()
	cancel := e.netMonCancel
	e.netMonCancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func addrSet(addrs []net.Addr, err error) map[string]bool {
	// err is deliberately ignored: a transient failure must not look like
	// a network change.
	out := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		out[a.String()] = true
	}
	return out
}

func sameAddrSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// RunConnectionTestJSON runs the full four-step test, emitting
// test:progress after each step, and returns the steps JSON.
func (e *Engine) RunConnectionTestJSON() string {
	srv := e.RunningServer()
	if srv == nil {
		return errResult("SELFTEST_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg := health.Config{
		ProxyAddr: srv.Addr(),
		Resolver:  e.resolver.Load(),
		Timeout:   8 * time.Second,
	}
	if e.opts.HealthOverrides != nil {
		e.opts.HealthOverrides(&cfg)
	}
	steps := health.RunSelfTest(ctx, cfg, func(s health.Step) {
		e.emit("test:progress", marshalJSON(s))
	})
	e.mu.Lock()
	e.lastSteps = steps
	e.mu.Unlock()
	e.recordLatency(steps)
	for _, s := range steps {
		if !s.OK {
			// The full test feeds the degraded reason too (D44).
			e.recordFail(s.ID, s.Code)
			break
		}
	}
	return marshalJSON(steps)
}

// AutoFixJSON runs the heal sweep on demand (the "Fix connection" button).
func (e *Engine) AutoFixJSON() string {
	e.mu.Lock()
	phase := e.phase
	e.mu.Unlock()
	switch phase {
	case PhaseOff, PhaseError:
		// From off, a plain Start already contains the check-and-heal.
		if err := e.Start(); err != nil {
			return errResult(mapErrorCode(err))
		}
		return `{"method":null}`
	case PhaseStopping:
		return errResult("UNKNOWN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.startTimeout())
	defer cancel()
	worked := e.healSweep(ctx, true)
	if worked == "" {
		return errResult("SELFTEST_FAILED")
	}
	return marshalJSON(map[string]string{"method": worked})
}

// lastSelfTestSummary renders the stored self-test steps for the report.
func (e *Engine) lastSelfTestSummary() string {
	e.mu.Lock()
	steps := e.lastSteps
	e.mu.Unlock()
	if len(steps) == 0 {
		return "self-test: not run in this session"
	}
	var b strings.Builder
	b.WriteString("self-test:")
	for _, s := range steps {
		status := "ok"
		if !s.OK {
			status = "fail(" + s.Code + ")"
		}
		b.WriteString(" " + s.ID + "=" + status)
	}
	return b.String()
}
