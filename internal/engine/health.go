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

// setRunCancel registers the current start run's cancel function.
func (e *Engine) setRunCancel(cancel context.CancelFunc) {
	e.mu.Lock()
	old := e.runCancel
	e.runCancel = cancel
	e.mu.Unlock()
	if old != nil {
		old()
	}
}

// cancelCurrentRun aborts a connecting start (Stop or button cancel).
func (e *Engine) cancelCurrentRun() {
	e.mu.Lock()
	cancel := e.runCancel
	e.runCancel = nil
	e.runAborted = true
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// connectSequence is the full Start sequence (file 01, section 10):
// starting -> checking -> (heal sweep) -> on | degraded, capped at the
// total timeout.
func (e *Engine) connectSequence() {
	ctx, cancel := context.WithTimeout(context.Background(), e.startTimeout())
	defer cancel()
	e.mu.Lock()
	e.runAborted = false
	e.mu.Unlock()
	e.setRunCancel(cancel)

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
	e.mu.Unlock()
	e.emitState()
	e.startStatsLoop()
	e.startWatchdog()
	e.startNetMon()
	e.prewarm()
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
	e.startNetMon()
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
