package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
)

// onOff renders a bool the report's experiment line reads.
func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// appInfo is the GetAppInfo payload (file 01, section 10).
type appInfo struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Platform     string       `json:"platform"`
	Capabilities Capabilities `json:"capabilities"`
	ConfigPath   string       `json:"configPath"`
	LogDir       string       `json:"logDir"`
}

// apiError wraps method results that can fail with a typed code.
type apiError struct {
	Error *string `json:"error"`
}

func errResult(code string) string {
	if code == "" {
		return `{"error":null}`
	}
	b, _ := json.Marshal(code)
	return fmt.Sprintf(`{"error":%s}`, b)
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// GetStateJSON returns the state as JSON (the UI-facing API shape).
func (e *Engine) GetStateJSON() string { return marshalJSON(e.GetState()) }

// StartJSON starts connecting and returns immediately.
func (e *Engine) StartJSON() string {
	if err := e.Start(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// StopJSON stops the engine.
func (e *Engine) StopJSON() string {
	if err := e.Stop(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// ToggleJSON toggles on/off.
func (e *Engine) ToggleJSON() string {
	if err := e.Toggle(); err != nil {
		return errResult(mapErrorCode(err))
	}
	return errResult("")
}

// GetSettingsJSON returns the normalized settings.
func (e *Engine) GetSettingsJSON() string {
	e.mu.Lock()
	s := e.settings
	e.mu.Unlock()
	return marshalJSON(s)
}

// SaveSettingsJSON applies a settings JSON payload, persists it, and applies
// mode/method changes immediately while On. Returns the normalized settings
// or a typed error.
func (e *Engine) SaveSettingsJSON(payload string) string {
	var incoming config.Settings
	if err := json.Unmarshal([]byte(payload), &incoming); err != nil {
		return errResult("UNKNOWN")
	}
	normalized := config.Normalize(incoming)

	e.mu.Lock()
	prev := e.settings
	e.settings = normalized
	running := e.server != nil
	e.mu.Unlock()

	if err := config.Save(e.platform.ConfigDir(), normalized); err != nil {
		e.logger.Error("saving settings failed", "reason", errKind(err))
	}

	// Apply immediately while On (file 01, section 10).
	if running {
		if normalized.Mode != prev.Mode {
			e.server.SetPACData(normalized.Mode, normalized.Advanced.Domains)
			if err := e.platform.Attach(e.port, normalized); err != nil {
				e.logger.Warn("re-attach after mode change failed", "reason", errKind(err))
			}
		}
		if normalized.Method != prev.Method || settingsFragmentChanged(prev, normalized) {
			e.server.SetFragment(e.currentPreset(), e.currentFragmentConfig())
			e.resolver.Load().SetFragment(e.currentPreset(), e.currentFragmentConfig())
		}
		// M8-af (ADR-0044, spec W4): the experiment switches apply LIVE —
		// the tester toggles substitution or the breaker in the
		// diagnostics screen and the next flow obeys, same sitting.
		if normalized.Advanced.ExperimentSubstitution != prev.Advanced.ExperimentSubstitution ||
			normalized.Advanced.ExperimentBreaker != prev.Advanced.ExperimentBreaker {
			e.server.SetExperimentFlags(normalized.Advanced.ExperimentSubstitution, normalized.Advanced.ExperimentBreaker)
			e.logger.Info("experiment switches changed",
				"substitution", normalized.Advanced.ExperimentSubstitution,
				"breaker", normalized.Advanced.ExperimentBreaker)
		}
	}
	// Autostart is applied whenever it flips, on or off.
	if normalized.StartWithWindows != prev.StartWithWindows {
		if err := e.platform.SetAutostart(normalized.StartWithWindows); err != nil {
			e.logger.Warn("autostart change failed", "reason", errKind(err))
		}
	}
	// A changed secure-DNS list swaps in a resolver bound to the new
	// servers (D44): the change used to only clear the cache while the
	// old servers kept being asked until a restart.
	if !sameDoHList(prev.Advanced.DoHServers, normalized.Advanced.DoHServers) {
		e.swapResolver(normalized.Advanced.DoHServers)
		e.logger.Info("secure-dns list changed; a new resolver is active")
	}
	e.emitState()
	return marshalJSON(normalized)
}

// sameDoHList reports whether two server lists are equal.
func sameDoHList(a, b []config.DoHServer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].URL != b[i].URL {
			return false
		}
		if len(a[i].Bootstrap) != len(b[i].Bootstrap) {
			return false
		}
		for j := range a[i].Bootstrap {
			if a[i].Bootstrap[j] != b[i].Bootstrap[j] {
				return false
			}
		}
	}
	return true
}

func settingsFragmentChanged(a, b config.Settings) bool {
	return a.Advanced.PieceSizeMin != b.Advanced.PieceSizeMin ||
		a.Advanced.PieceSizeMax != b.Advanced.PieceSizeMax ||
		a.Advanced.PauseMs != b.Advanced.PauseMs
}

// RestoreWindowsProxyJSON restores the system proxy state (Help button).
func (e *Engine) RestoreWindowsProxyJSON() string {
	if err := e.platform.RestoreProxy(); err != nil {
		e.logger.Error("restore proxy failed", "reason", errKind(err))
		return errResult("PROXY_SETTING_FAILED")
	}
	e.logger.Info("internet settings restored")
	return errResult("")
}

// GetAppInfoJSON returns app metadata.
func (e *Engine) GetAppInfoJSON() string {
	dir := e.platform.ConfigDir()
	info := appInfo{
		Name:         config.AppName,
		Version:      e.opts.Version,
		Platform:     e.platform.Name(),
		Capabilities: e.platform.Capabilities(),
		ConfigPath:   config.SettingsPath(dir),
		LogDir:       e.logDir(),
	}
	return marshalJSON(info)
}

// logDir returns the log directory path.
func (e *Engine) logDir() string {
	if e.opts.LogDir != "" {
		return e.opts.LogDir
	}
	return filepath.Join(e.platform.ConfigDir(), "logs")
}

// GetDiagnosticReport returns the plain-text, redacted diagnostic report
// (file 03, R20/R21).
func (e *Engine) GetDiagnosticReportJSON() string {
	return marshalJSON(map[string]string{"report": e.GetDiagnosticReport()})
}

// MarkJSON stamps the diagnostics timeline (M8-z, ADR-0027 §8.3.6):
// the facade's Mark(label) — the "I pressed play" / "First frame"
// buttons. Any label is accepted (it rides the report and the log ring
// verbatim, so the tester's own words land in the paste).
func (e *Engine) MarkJSON(label string) string {
	e.Mark(label)
	return errResult("")
}

// deadOriginProbe runs the one-shot SNI-vs-IP probe (M8-z, ADR-0027
// §8.3.5) when the engine is On and it has not run this run: for up to
// two IPs in the dead-IP penalty box, one ClientHello with SNI
// www.google.com and one with no SNI, each answered within 2 s or
// classified silent/reset. The verdicts answer whether the block is by
// SNI (alt name answers, real name silent) or by IP (both silent) —
// the penalty box's own justification. Only while On (never on a
// timer); at most once per run; the lines land in the log ring AND the
// report section.
func (e *Engine) deadOriginProbe() []string {
	e.mu.Lock()
	done := e.probeDone
	phase := e.phase
	srv := e.server
	e.mu.Unlock()
	if done {
		if lines := e.probeLinesSnapshot(); len(lines) > 0 {
			return lines
		}
		return []string{"dead-origin probe: already probed this run"}
	}
	if srv == nil || (phase != PhaseOn && phase != PhaseDegraded) {
		return []string{"dead-origin probe: skipped (the engine is not on)"}
	}
	e.mu.Lock()
	e.probeDone = true
	e.mu.Unlock()
	ips := srv.PenalizedIPs(2)
	if len(ips) == 0 {
		lines := []string{"dead-origin probe: no penalized origin IPs to probe"}
		e.setProbeLines(lines)
		return lines
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	type probeResult struct {
		sni  string
		kind string
	}
	results := make(chan probeResult, len(ips)*2)
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, sni := range []string{"www.google.com", ""} {
			go func(ip net.IP, sni string) {
				results <- probeResult{sni: sni, kind: srv.ProbeDeadOrigin(ctx, ip, sni)}
			}(ip, sni)
		}
	}
	var lines []string
	for range len(ips) * 2 {
		r := <-results
		label := "sni=" + r.sni
		if r.sni == "" {
			label = "sni=none"
		}
		line := fmt.Sprintf("probe dead-origin %s result=%s", label, r.kind)
		lines = append(lines, line)
		e.logger.Info(line)
	}
	out := append([]string{"dead-origin probe (one-shot, " + strconv.Itoa(len(ips)) + " penalized IPs):"}, lines...)
	e.setProbeLines(out)
	return out
}

func (e *Engine) setProbeLines(lines []string) {
	e.mu.Lock()
	e.probeLines = lines
	e.mu.Unlock()
}

func (e *Engine) probeLinesSnapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.probeLines...)
}

// GetUsageJSON returns the round-3 Data-usage summary the menu renders.
// A store that could not be opened answers a typed error — the sheet then
// shows its honest "Could not read the usage history" line and the rest
// of the menu keeps working.
func (e *Engine) GetUsageJSON() string {
	if e.usage == nil {
		return errResult("USAGE_UNAVAILABLE")
	}
	return marshalJSON(e.usage.Summary(e.opts.Now()))
}

// ClearUsageJSON empties the history and the Last-session line: the next
// state event carries an explicit lastSession:null, which is exactly how
// the screen knows to drop the line (the frontend maps an absent field to
// "keep", a present null to "gone").
func (e *Engine) ClearUsageJSON() string {
	if e.usage != nil {
		e.usage.Clear()
	}
	e.mu.Lock()
	e.lastSess = nil
	e.mu.Unlock()
	e.emitState()
	return errResult("")
}

// GetDiagnosticReport builds the report text. It contains only: app version,
// platform, settings summary (no custom lists), self-test status summary,
// counters, the embedder's extra section when installed (M8-p), measured
// sleep and the last 200 log lines.
//
// M8-z (ADR-0027 §8.3) adds the debug-build hostname notice, the new
// counter lines, the Top-10 slowest-or-failed hosts (redacted unless
// hostnames are logged), the plain-relay summary, the DoH detail
// (dials/reuse/hedge wins + the recent per-server failures with their
// phase), the one-shot dead-origin probe (only while On), and the
// timeline marks the facade's Mark() stamped.
func (e *Engine) GetDiagnosticReport() string {
	var b strings.Builder
	snapshot := e.settingsSnapshot()
	hostnames := snapshot.Advanced.LogDomains
	// §8.1: the debug TESTER build starts with hostnames on — the very
	// FIRST line of the report says so, so the tester knows what they
	// are pasting. A release build (or a build whose owner turned the
	// setting off) never prints it and never prints a host name.
	if config.IsDebugTesterBuild() && hostnames {
		fmt.Fprintf(&b, "DEBUG BUILD: hostnames included for diagnosis\n")
	}
	fmt.Fprintf(&b, "%s %s diagnostic report\n", config.AppName, e.opts.Version)
	fmt.Fprintf(&b, "platform: %s\n", e.platform.Name())
	fmt.Fprintf(&b, "generated: %s\n", e.opts.Now().Format(time.RFC3339))
	s := e.GetState()
	fmt.Fprintf(&b, "state: %s (detail %q)\n", s.Phase, s.Detail)
	fmt.Fprintf(&b, "mode: %s method: %s port: %d fragmentAllRouted=%v\n",
		s.Mode, s.Method, s.Port, snapshot.FragmentAllRouted)
	// M8-af (ADR-0044, spec W4): the experiment line — the three
	// switches the tester's sitting rides on, printed where the report
	// starts so every paste says which world its numbers came from.
	// substitution/breaker read the RUNNING server's live state (the
	// settings when off), method is the state's own field.
	substOn, breakerOn := snapshot.Advanced.ExperimentSubstitution, snapshot.Advanced.ExperimentBreaker
	if srv := e.RunningServer(); srv != nil {
		substOn, breakerOn = srv.ExperimentFlags()
	}
	fmt.Fprintf(&b, "experiment: method=%s substitution=%s breaker=%s\n",
		s.Method, onOff(substOn), onOff(breakerOn))
	if s.LatencyMs != nil {
		fmt.Fprintf(&b, "response time: %d ms\n", *s.LatencyMs)
	}
	if s.Fail != nil {
		fmt.Fprintf(&b, "last failed check: %s (%s)\n", s.Fail.Step, s.Fail.Code)
	}
	fmt.Fprintf(&b, "measured sleep for 1ms request: %d ms\n", e.measuredSleep.Milliseconds())
	d := e.stats.Diagnostic()
	fmt.Fprintf(&b, "counters: conns total %d, live %d, errors %d, panics %d\n",
		d.ConnsTotal, d.ConnsLive, d.ConnErrors, d.Panics)
	fmt.Fprintf(&b, "counters: dns ok %d, fail %d, up %s, down %s\n",
		d.DnsOK, d.DnsFail, config.FormatBytes(d.BytesUp), config.FormatBytes(d.BytesDown))
	// §8.4: the new counters — the plain net's kills, the dial RTT
	// classes, the slow first-bytes.
	fmt.Fprintf(&b, "counters: plainFirstByteTimeouts %d, dialNear %d, dialFar %d, slowFirstBytes %d\n",
		d.PlainFirstByteTo, d.DialNear, d.DialFar, d.FirstByteSlow)
	// M8-aa (ADR-0028/0029/0030): the tier split, the raced/salvage
	// saves, the routed-flow starts, and the origin-probe verdicts;
	// M8-ab adds the googlevideo terminal count the churn kick reads;
	// M8-ac adds the good-mark drop count (the pool's churn).
	fmt.Fprintf(&b, "counters: stallKillsLean %d, stallKillsRich %d, racedSaves %d, salvagedSaves %d, routedFlows %d, gvideoTerminals %d, goodDrops %d\n",
		d.StallKillLean, d.StallKillRich, d.RacedSaves, d.SalvagedSaves, d.RoutedFlows, d.GvideoTerminals, d.GoodDrops)
	// §8.3.3: the DoH transport's own counters (dials opened, reuse,
	// hedge wins by server number) ride the header too.
	dohd := e.resolver.Load().Diagnostic()
	hedgeBy := make([]string, len(dohd.HedgeByServer))
	for i, w := range dohd.HedgeByServer {
		hedgeBy[i] = fmt.Sprintf("#%d:%d", i+1, w)
	}
	fmt.Fprintf(&b, "doh: dials %d, reuse %d, queries %d, hedgeWins %d",
		dohd.Dials, dohd.Reuse, dohd.Queries, dohd.HedgeWins)
	if len(hedgeBy) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(hedgeBy, " "))
	}
	// M8-ac (ADR-0035): the DoH brownout periods — how many times the
	// resolver's connections churned hard enough to pause the
	// engine's background lanes (the connection-killer's fingerprint).
	fmt.Fprintf(&b, " brownouts=%d\n", d.DoHBrownouts)
	fmt.Fprintf(&b, "%s\n", e.lastSelfTestSummary())
	// §8.3.1: Top 10 slowest or failed hosts — per host (count,
	// failures, worst time to first byte, class); the host NAME rides
	// only when hostnames are logged (R21: a release report renders the
	// same rows redacted, the numbers stay comparable).
	fmt.Fprintf(&b, "top 10 slowest or failed hosts:\n")
	for _, row := range e.hostTable.TopSlowest(10) {
		host := row.Host
		if !hostnames {
			host = "<redacted>"
		}
		fmt.Fprintf(&b, "  host=%s class=%s flows=%d failures=%d plainKills=%d worstFirstByte=%dms\n",
			host, row.Class, row.Flows, row.Failures, row.PlainKills, row.WorstFirstByte)
	}
	// §8.3.2: the plain-relay summary.
	plain := e.hostTable.Plain()
	fmt.Fprintf(&b, "plain-relay: flows=%d firstByteTimeouts=%d worstFirstByte=%dms\n",
		plain.Flows, d.PlainFirstByteTo, plain.WorstFirstMS)
	// M8-aa (ADR-0029): the stall tier split with the budget that
	// fired — the next report's primary tuning evidence (the three
	// 15 s lean kills of the debug.18 paste become three 8 s ones).
	// M8-ac (ADR-0036) adds the cause split: PENDING kills died with
	// the client's request unanswered (the definite dead-origin
	// shape, and the demote's trigger), IDLE kills were quiet on both
	// ends (the ambiguous shape — the app's pooled connection OR a
	// mid-body death). Which one dominates says which world the
	// engine is in. M8-ad (ADR-0037) splits the rich tier too:
	// pending = the fast-budget kill that bounds the mid-video wedge
	// ("goes loading; refresh fixes it"), trickle = the per-connection
	// throttle the both-silent clock could never see (bytes kept
	// arriving, under the floor) — the two numbers that say whether
	// the long-lived-conn round is catching what the tester saw.
	fmt.Fprintf(&b, "stall kills: lean=%d (pending=%d idle=%d) rich=%d (pending=%d trickle=%d) leanBudget=%s\n",
		d.StallKillLean, d.StallLeanPending, d.StallLeanIdle, d.StallKillRich, d.StallRichPending, d.StallTrickle, e.stallLeanEffective())
	// M8-aa (ADR-0028): the origin-probe pass's own evidence — how
	// many passes ran, how many answer IPs the fragmented hello
	// verified, and the verdict split — plus the box's and the good
	// table's current shape (counts only, R21). The good count is the
	// dial lists' preferred front; penalized is the box's live size
	// and strikes its escalated entries.
	good, penalized, strikes := 0, 0, 0
	if srv := e.RunningServer(); srv != nil {
		good = srv.GoodIPCount()
		penalized, strikes = srv.DeadBoxStats()
	}
	fmt.Fprintf(&b, "origin probe: passes=%d ips=%d answered=%d silent=%d reset=%d good=%d penalized=%d strikes=%d rehabilitated=%d\n",
		d.ProbePasses, d.ProbeIPs, d.ProbeAnswered, d.ProbeSilent, d.ProbeReset, good, penalized, strikes, d.ProbeRehabilitated)
	// M8-ab (ADR-0031): the cross-origin substitution's own evidence —
	// how many substitute candidates launched, how many won their
	// round, and the pool they drew from (total verified-live origins,
	// gvideo-flagged among them). The one line that says whether the
	// escape worked in the field: attempts without wins means the
	// cross-cache bet failed and the next round needs a different
	// shape; wins with a shrinking auto-timeline means it worked.
	// M8-ai (ADR-0048) adds the size class: big= substituted flows that
	// carried real media (down at the PayloadVerifiedMinimum bar),
	// small= the fast small verdict answers — debug.23 and debug.26
	// both measured every win small, so big=0 beside small>0 says the
	// lane is the fast-verdict lane, not a media carrier. Counts only
	// (R21).
	poolTotal, poolGvideo := 0, 0
	if srv := e.RunningServer(); srv != nil {
		poolTotal, poolGvideo = srv.GoodIPStats()
	}
	fmt.Fprintf(&b, "substitution: attempts=%d wins=%d big=%d small=%d pool=%d (gvideo %d)\n",
		d.SubstAttempts, d.SubstWins, d.SubstBig, d.SubstSmall, poolTotal, poolGvideo)
	// M8-ah (ADR-0047): the dead-cluster memory and the payload-fed
	// pool — dead= how many sn-clusters carry a live dead mark,
	// fastFails= the instant door refusals that kept the app's
	// hostname rotation cycling (each one a saved 3 s first-byte
	// burn), probes= the periodic re-tests, rehabilitated= dead
	// marks an answered own-dial cleared, payloadVerified= origins
	// media-verified by delivered bytes (the pool's automatic food).
	// The debug.25 report read dead clusters nowhere and an empty
	// pool (gvideo 0, attempts 0) beside scout-carried 1.8 MB flows;
	// this line says which of those worlds the next report is in.
	// M8-ai (ADR-0048) adds verdicts= — dead-cluster flows the pool
	// SERVED at the door (the verdict lane's engagements): the
	// debug.26 report read 150 fast-fails beside a lane that had
	// starved on demoted pool marks; verdicts= read against
	// fastFails= says whether the lane held this time.
	fmt.Fprintf(&b, "gvideo clusters: dead=%d fastFails=%d verdicts=%d probes=%d rehabilitated=%d payloadVerified=%d\n",
		d.GvideoClusterDeads, d.GvideoClusterFastFail, d.GvideoClusterVerdicts, d.GvideoClusterProbes, d.GvideoClusterRehabs, d.GvideoPayloadFeeds)
	// M8-ae (ADR-0042): the wire-discipline evidence — the googlevideo
	// hellos the engine actually put on the wire this session (flow
	// candidates + substitutes + probe jobs) and the emission breaker's
	// arms. hellos read against routedFlows: the reference
	// implementations run ~1 per flow (plus zero background), debug.22
	// ran ~4 per flow plus bursts; cooldowns>0 means the breaker caught
	// a flag state that debug.22 rode into "never opens again". Counts
	// only (R21).
	// M8-af (ADR-0043, spec W9): the fragmenter's packet total rides
	// the discipline line — hello COUNT was measured since debug.23,
	// hello PACKET count never was (one legacy hello is up to 300 TCP
	// segments; the censor may key on packets per source, and the
	// network test prints its per-method deltas beside this total).
	// M8-ag (ADR-0046): the escapes ride it too — lifts (a googlevideo
	// first byte ended a cooldown early: the wire was never dead) and
	// scouts (the one flow per cooldown the door let probe). The
	// debug.24 report's failure read cooldowns=1 with the player dead
	// and attempts=0: these two numbers say which world the next
	// report landed in.
	fmt.Fprintf(&b, "gvideo discipline: hellos=%d cooldowns=%d lifts=%d scouts=%d fragPackets=%d\n",
		d.GvideoHellos, d.GvideoCooldowns, d.GvideoLifts, d.GvideoScouts, fragment.PiecesTotal())
	// M8-af (ADR-0043, spec W1): the kernel-TCP verdict split — every
	// stalled googlevideo flow classified from its A/B snapshots:
	// upstream-lost (the request never reached the server),
	// downstream-lost (it arrived, the answer is being dropped or
	// throttled), both-ok (the transport carried, the server/HTTP
	// layer is the suspect), unknown (no classifiable pair, never a
	// guess). The §7 decision table's second and third rows read this
	// line directly.
	fmt.Fprintf(&b, "tcp verdicts: upstream-lost=%d downstream-lost=%d both-ok=%d unknown=%d\n",
		d.TCPUpstreamLost, d.TCPDownstreamLost, d.TCPBothOK, d.TCPUnknown)
	// §8.3.3: the DNS detail — per recent failure, the server number,
	// the phase that failed (tcp, tls, http, status, parse) and the
	// latency.
	if len(dohd.Failures) == 0 {
		fmt.Fprintf(&b, "dns detail: no server failures recorded\n")
	} else {
		fmt.Fprintf(&b, "dns detail (last %d failures):\n", len(dohd.Failures))
		for _, f := range dohd.Failures {
			fmt.Fprintf(&b, "  server=#%d phase=%s reason=%s latency=%dms\n", f.Server, f.Phase, f.Reason, f.MS)
		}
	}
	// §8.3.5: the one-shot dead-origin probe — up to 2 penalized IPs,
	// a ClientHello with SNI www.google.com and one with no SNI, only
	// while the app is On, never on a timer. The verdict lines also
	// land in the log ring; the report section renders them once.
	for _, line := range e.deadOriginProbe() {
		fmt.Fprintf(&b, "%s\n", line)
	}
	// §8.3.6: the timeline marks — the elapsed time between the LAST
	// PAIR plus the failed/slow flows inside that window.
	marks := e.timelineMarks()
	if len(marks) == 0 {
		fmt.Fprintf(&b, "timeline: no marks (the \"I pressed play\" / \"First frame\" buttons stamp it)\n")
	} else if len(marks) == 1 {
		fmt.Fprintf(&b, "timeline: MARK %s (awaiting its pair)\n", marks[0].Label)
	} else {
		a, z := marks[len(marks)-2], marks[len(marks)-1]
		var failed, kills, slow int64
		if z.Window != nil {
			failed, kills, slow = z.Window.connErrors, z.Window.plainKills, z.Window.slowFirstByte
		}
		fmt.Fprintf(&b, "timeline: MARK %s -> MARK %s elapsed=%dms failedFlows=%d plainKills=%d slowFirstBytes=%d\n",
			a.Label, z.Label, z.At.Sub(a.At).Milliseconds(), failed, kills, slow)
	}
	// M8-aa (ADR-0030): the auto-timeline — the heuristic's answer to
	// the debug.18 report's "timeline: no marks". It measures play ->
	// data WITHOUT the buttons (a routed burst after quiet, then the
	// down-burst that answered it); the manual marks stay authoritative
	// (the heuristic cannot tell a feed load from a play).
	if pairs := e.autoTLPairs(); len(pairs) > 0 {
		last := pairs[len(pairs)-1]
		fmt.Fprintf(&b, "auto-timeline: play-suspected -> first-data elapsed=%dms pairs=%d (heuristic; the buttons stay authoritative)\n",
			last.data.Sub(last.play).Milliseconds(), len(pairs))
	} else {
		fmt.Fprintf(&b, "auto-timeline: no pair yet (heuristic; the buttons stay authoritative)\n")
	}
	// M8-af (ADR-0044, spec W7/W8): the network test — the last
	// completed run's table (ip × method -> verdict, the alt-SNI
	// sweep, the control, the per-method packet counts, the foreign
	// cluster rows), or the honest not-run line. The report build
	// itself never probes (a probe here would hang the report button
	// for minutes); the button is the only trigger.
	if lines := e.networkTestLines(); len(lines) > 0 {
		for _, line := range lines {
			fmt.Fprintf(&b, "%s\n", line)
		}
	} else {
		e.netTest.mu.Lock()
		running := e.netTest.running
		e.netTest.mu.Unlock()
		if running {
			fmt.Fprintf(&b, "network test: running (wait for it to finish, then refresh)\n")
		} else {
			fmt.Fprintf(&b, "network test: not run this session (press Run network test in Diagnostics)\n")
		}
	}
	// The embedder's section (M8-p, ADR-0015): snapshot under the lock,
	// run OUTSIDE it — the hook is free to touch engine state (GetState
	// below takes mu again, so holding it here would deadlock).
	e.mu.Lock()
	extra := e.reportExtra
	e.mu.Unlock()
	if extra != nil {
		if text := extra(); text != "" {
			fmt.Fprintf(&b, "%s\n", text)
		}
	}
	// M8-p: 200 lines (was 100) — a stalled video retries every few
	// seconds and each attempt writes its tunnel-up/first-byte/done
	// triple, so the doubled window keeps the stall's whole story in the
	// report the user copies afterwards.
	lines := e.logger.Ring.Lines()
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	fmt.Fprintf(&b, "last %d log lines:\n", len(lines))
	for _, l := range lines {
		fmt.Fprintf(&b, "  %s\n", l)
	}
	return b.String()
}

// OpenLogsFolderJSON returns the log folder path; opening a file browser is
// shell behavior (DECISIONS.md) so the UI layer handles it.
func (e *Engine) OpenLogsFolderJSON() string {
	return marshalJSON(map[string]string{"path": e.logDir()})
}

// QuitJSON quits the app.
func (e *Engine) QuitJSON() string {
	go func() {
		time.Sleep(50 * time.Millisecond) // let the reply reach the UI first
		e.Quit()
	}()
	return errResult("")
}
