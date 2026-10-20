// Package engine — the "Run network test" lane (M8-af, ADR-0044,
// spec W7/W8): the same-minute preset comparison the consultation's
// §2.3 open matrix item 3 demanded — preset vs hour-of-day are still
// confounded because every method comparison so far rode a DIFFERENT
// build at a different hour. One button press, one sitting: up to six
// CURRENT googlevideo addresses probed under ALL FOUR presets in the
// same minute, then the SNI discrimination (real name vs
// www.google.com vs no SNI), a known-good control front
// (i.ytimg.com), a per-method packet count (the censor may key on
// packets, not hellos — 300 fragments per hello is 300 TCP segments),
// and — when Ali's embedded file has entries — the foreign-cluster
// rows (W8).
//
// Only the "answered" class (an HTTP status line from a deliberately
// bad GET /videoplayback) certifies an address as media-capable, and
// those verdicts are what feed the W6 substitution pool
// (MarkMediaVerifiedIP). The lane is operator-pressed, never a timer;
// it dials directly through the engine's netx door (the Android
// protect hook applies) and deliberately runs OUTSIDE the emission
// breaker's door rules — a measurement must not change what it
// measures — but every googlevideo-SNI hello it emits IS counted in
// the discipline meter (the censor sees them; the report says so).
//
// The wall-time budget: each probe is bounded by ~4 s handshake + 3 s
// answer with a 3 s dial budget; a fully-silent network runs six
// addresses × four methods + two alt-SNI sweeps + controls + the
// foreign lane in a few minutes worst case, typically well under a
// minute — the tester's copy says "up to a few minutes".
package engine

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/probe"
)

// networkTestConcurrency bounds the probes in flight inside one
// method batch (the engine's own probe pass uses 3; the network test
// is a pressed measurement, so 4 keeps the same-minute promise while
// staying polite on a phone's radio).
const networkTestConcurrency = 4

// networkTestMaxAddresses is the candidate cap (spec W7: "up to 6
// current googlevideo IPs").
const networkTestMaxAddresses = 6

// networkTestForeignIPsPerHost bounds each foreign hostname's probed
// answer addresses (the W8 lane's volume guard).
const networkTestForeignIPsPerHost = 2

// networkTestState is the one-button lane's bookkeeping: the running
// flag, the timestamps, and the last completed run's report lines.
// The lines SURVIVE Stop deliberately (unlike the per-run probe
// latch): the test costs the tester a minute of their evening and its
// value is exactly in the report they paste afterwards, even if the
// engine was cycled between the run and the paste.
type networkTestState struct {
	mu        sync.Mutex
	running   bool
	startedAt time.Time
	lines     []string
	doneAt    time.Time
}

// NetworkTestStatusJSON is the lane's GET shape: {"running":…,
// "done":…, "lines":[…]} — the diagnostics screen polls it for the
// button's progress and renders the finished table.
func (e *Engine) NetworkTestStatusJSON() string {
	e.netTest.mu.Lock()
	running, started, lines, done := e.netTest.running, e.netTest.startedAt, e.netTest.lines, e.netTest.doneAt
	e.netTest.mu.Unlock()
	var b strings.Builder
	b.WriteString(`{"running":`)
	b.WriteString(strconv.FormatBool(running))
	b.WriteString(`,"done":`)
	b.WriteString(strconv.FormatBool(len(lines) > 0))
	if running {
		b.WriteString(`,"startedAt":`)
		writeJSONString(&b, started.Format(time.RFC3339))
	}
	if len(lines) > 0 {
		b.WriteString(`,"finishedAt":`)
		writeJSONString(&b, done.Format(time.RFC3339))
		b.WriteString(`,"lines":[`)
		for i, l := range lines {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONString(&b, l)
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

// writeJSONString appends s as a JSON string literal.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// RunNetworkTestJSON starts the network test asynchronously (the
// button's POST): {"error":null} when started, {"error":"NOT_ON"}
// when the engine is not serving (the lane dials through the engine's
// own door — protect, DoH, fragment — so it needs the run), and
// {"error":"RUNNING"} while one is already in flight. The completion
// event ("networktest") carries the status payload.
func (e *Engine) RunNetworkTestJSON() string {
	st := e.GetState()
	if st.Phase != PhaseOn && st.Phase != PhaseDegraded {
		return `{"error":"NOT_ON"}`
	}
	e.netTest.mu.Lock()
	if e.netTest.running {
		e.netTest.mu.Unlock()
		return `{"error":"RUNNING"}`
	}
	e.netTest.running = true
	e.netTest.startedAt = e.opts.Now()
	e.netTest.mu.Unlock()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.logger.Error("network test panicked", "panic", fmt.Sprint(r))
				e.netTest.mu.Lock()
				e.netTest.running = false
				e.netTest.mu.Unlock()
			}
		}()
		lines := e.runNetworkTest()
		e.netTest.mu.Lock()
		e.netTest.running = false
		e.netTest.lines = lines
		e.netTest.doneAt = e.opts.Now()
		e.netTest.mu.Unlock()
		e.emit("networktest", e.NetworkTestStatusJSON())
	}()
	return `{"error":null}`
}

// networkTestLines returns the last completed run's lines (the report
// renders them; nil = never run).
func (e *Engine) networkTestLines() []string {
	e.netTest.mu.Lock()
	defer e.netTest.mu.Unlock()
	return append([]string(nil), e.netTest.lines...)
}

// netAddr is one probed address with its per-method verdicts and the
// SNI-variant verdicts (the variants run under the CURRENT method).
type netAddr struct {
	ip        net.IP
	label     string // "addrN" (release) or the IP text (hostname-logged builds)
	methods   map[config.Method]probe.MediaResult
	altGoogle probe.MediaResult
	altNone   probe.MediaResult
}

// netForeignRow is one W8 hostname's verdicts (its own real SNI).
type netForeignRow struct {
	host    string
	label   string // the hostname when hostnames are logged, else <redacted>
	ips     []net.IP
	methods map[config.Method]probe.MediaResult
}

// runNetworkTest executes the whole test and returns the report
// lines. Structure per spec W7: (1) the four-method sweep over the
// current cluster's addresses with the REAL SNI (with per-method
// fragment-packet deltas), (2) the alt-SNI sweep under the current
// method, (3) the control front per method, (4) the foreign lane
// (W8), (5) the media-verified marking (answered under the REAL SNI
// only — the only class that certifies).
func (e *Engine) runNetworkTest() []string {
	now := e.opts.Now()
	snapshot := e.settingsSnapshot()
	hostnames := snapshot.Advanced.LogDomains
	srv := e.RunningServer()
	if srv == nil {
		return []string{"network test: skipped (the engine is not serving)"}
	}
	resolver := e.resolver.Load()

	// The candidates (spec W7: "up to 6 current googlevideo IPs") and
	// the REAL SNI: the freshest learned code's rr names, resolved
	// NOW through the normal DoH path — the same cluster a fresh
	// video's flow would dial. A session with no learned codes (and no
	// persisted code file) falls back to the media redirector, the one
	// googlevideo front every session knows.
	candidates := []net.IP{}
	sni := "redirector.googlevideo.com"
	source := "redirector fallback"
	if codes := resolver.GvideoCodes(); len(codes) > 0 {
		freshest := codes[len(codes)-1]
		sni = fmt.Sprintf("rr1---%s.googlevideo.com", freshest)
		source = "learned codes"
		seen := make(map[string]bool)
		for i := 1; i <= 5 && len(candidates) < networkTestMaxAddresses; i++ {
			name := fmt.Sprintf("rr%d---%s.googlevideo.com", i, freshest)
			ips, err := resolver.Lookup(context.Background(), name)
			if err != nil {
				continue
			}
			for _, ip := range ips {
				key := ip.String()
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, ip)
				}
				if len(candidates) >= networkTestMaxAddresses {
					break
				}
			}
		}
	} else if ips, err := resolver.Lookup(context.Background(), sni); err == nil {
		candidates = dedupeNetIPs(ips)
		if len(candidates) > networkTestMaxAddresses {
			candidates = candidates[:networkTestMaxAddresses]
		}
	}
	if len(candidates) == 0 {
		line := "network test: no googlevideo addresses could be resolved (play a video first, then press again)"
		e.logger.Info(line)
		return []string{line}
	}

	// The dedicated probe door: the engine's one netx dialer's control
	// hook (Android protect applies) with the test's own tighter dial
	// budgets, so a fully-silent network still finishes in minutes.
	pr := &probe.MediaProbe{
		Dialer: &netx.Dialer{
			PerIPTimeout:   3 * time.Second,
			OverallTimeout: 4 * time.Second,
			Control:        e.dialer.Control,
			FallbackDelay:  netx.DefaultFallbackDelay,
		},
		RNG:             e.rng,
		HandshakeBudget: 4 * time.Second,
		AnswerBudget:    3 * time.Second,
	}
	methods := config.HealSweepOrder // legacy -> strong -> balanced -> light
	currentMethod := snapshot.Method
	addrs := make([]*netAddr, len(candidates))
	for i, ip := range candidates {
		label := "addr" + strconv.Itoa(i+1)
		if hostnames {
			label = ip.String()
		}
		addrs[i] = &netAddr{ip: ip, label: label, methods: make(map[config.Method]probe.MediaResult)}
	}

	// (1) The four-method sweep, same minute, same addresses — with
	// the per-method fragment-packet delta (the censor may key on
	// packets, not hellos).
	packets := make(map[config.Method]int64)
	for _, m := range methods {
		preset := e.presetFor(m)
		fcfg := fragment.PresetConfig(preset) // the preset's OWN shape — the comparison is preset vs preset
		before := fragment.PiecesTotal()
		runBatch(addrs, func(a *netAddr) probe.MediaResult {
			e.countProbeHello(sni)
			return pr.Run(context.Background(), a.ip, 443, sni, "/videoplayback", preset, fcfg)
		}, func(a *netAddr, r probe.MediaResult) { a.methods[m] = r })
		packets[m] = fragment.PiecesTotal() - before
	}

	// (2) The alt-SNI sweep under the CURRENT method: does the block
	// key on the media SNI? (www.google.com answers / none at all.)
	altPreset, altCfg := e.currentPreset(), fragment.PresetConfig(e.currentPreset())
	googleSNI := "www.google.com"
	runBatch(addrs, func(a *netAddr) probe.MediaResult {
		e.countProbeHello(googleSNI)
		return pr.Run(context.Background(), a.ip, 443, googleSNI, "/videoplayback", altPreset, altCfg)
	}, func(a *netAddr, r probe.MediaResult) { a.altGoogle = r })
	runBatch(addrs, func(a *netAddr) probe.MediaResult {
		return pr.Run(context.Background(), a.ip, 443, "", "/videoplayback", altPreset, altCfg)
	}, func(a *netAddr, r probe.MediaResult) { a.altNone = r })

	// (3) The control front per method: the test's own sanity row — a
	// failing control means the table itself is broken (the §7 last
	// row: "fix the test before drawing any conclusion").
	controlResults := make(map[config.Method]probe.MediaResult)
	controlOK := false
	if controlIPs, err := resolver.Lookup(context.Background(), "i.ytimg.com"); err == nil && len(controlIPs) > 0 {
		controlOK = true
		controlIP := controlIPs[0]
		for _, m := range methods {
			preset := e.presetFor(m)
			fcfg := fragment.PresetConfig(preset)
			controlResults[m] = pr.Run(context.Background(), controlIP, 443, "i.ytimg.com", "/", preset, fcfg)
		}
	}

	// (4) The foreign lane (W8): Ali's embedded hostnames, resolved
	// on-device through the normal DoH path, probed with each line's
	// own real SNI under every method. All of a host's probed
	// addresses must answer for the row to say answered under a
	// method (a partial verdict is the first failure).
	var foreign []netForeignRow
	for _, host := range config.ExperimentNodes() {
		ips, rerr := resolver.Lookup(context.Background(), host)
		if rerr != nil || len(ips) == 0 {
			continue
		}
		ips = dedupeNetIPs(ips)
		if len(ips) > networkTestForeignIPsPerHost {
			ips = ips[:networkTestForeignIPsPerHost]
		}
		label := "<redacted>"
		if hostnames {
			label = host
		}
		row := netForeignRow{host: host, label: label, ips: ips, methods: make(map[config.Method]probe.MediaResult)}
		for _, m := range methods {
			preset := e.presetFor(m)
			fcfg := fragment.PresetConfig(preset)
			row.methods[m] = probe.MediaResult{Class: probe.MediaAnswered}
			for _, ip := range ips {
				e.countProbeHello(host)
				if r := pr.Run(context.Background(), ip, 443, host, "/videoplayback", preset, fcfg); r.Class != probe.MediaAnswered {
					row.methods[m] = r
					break
				}
			}
		}
		foreign = append(foreign, row)
	}

	// (5) The media-verified marking: ONLY real-SNI answered verdicts
	// certify (an alt-SNI answered says the block is SNI-keyed, not
	// that the address carries media under its real name).
	marked := 0
	for _, a := range addrs {
		for _, m := range methods {
			if a.methods[m].Class == probe.MediaAnswered {
				srv.MarkMediaVerifiedIP(a.ip.String())
				marked++
				break
			}
		}
	}
	for _, row := range foreign {
		for _, m := range methods {
			if row.methods[m].Class == probe.MediaAnswered {
				for _, ip := range row.ips {
					srv.MarkMediaVerifiedIP(ip.String())
				}
				break
			}
		}
	}

	lines := e.renderNetworkTest(now, sni, source, currentMethod, methods, addrs, controlOK, controlResults, foreign, packets, marked, hostnames)
	// One counts-only ring line (R21): the table itself rides the
	// report section, where the LogHostnames redaction already shaped
	// every address label.
	answered := 0
	for _, a := range addrs {
		if a.methods[methods[0]].Class == probe.MediaAnswered {
			answered++
		}
	}
	e.logger.Info(fmt.Sprintf("network test: done addresses=%d answered-under-%s=%d marked=%d foreign=%d control=%v",
		len(addrs), methods[0], answered, marked, len(foreign), controlOK))
	return lines
}

// renderNetworkTest builds the report section's lines (the table the
// §7 decision table reads). Addresses appear as addrN labels unless
// hostnames are logged (the debug tester build); the SNI and foreign
// hostnames follow the same rule (R21).
func (e *Engine) renderNetworkTest(now time.Time, sni, source string, currentMethod config.Method, methods []config.Method, addrs []*netAddr, controlOK bool, controlResults map[config.Method]probe.MediaResult, foreign []netForeignRow, packets map[config.Method]int64, marked int, hostnames bool) []string {
	sniLabel := "<redacted>"
	if hostnames {
		sniLabel = sni
	}
	lines := []string{}
	lines = append(lines, fmt.Sprintf("network test (%s, method=%s, sni=%s):",
		now.Format(time.RFC3339), currentMethod, sniLabel))
	lines = append(lines, fmt.Sprintf("  addresses: %d (source: %s)", len(addrs), source))
	for _, a := range addrs {
		var sb strings.Builder
		fmt.Fprintf(&sb, "  %s:", a.label)
		for _, m := range methods {
			fmt.Fprintf(&sb, " %s=%s", m, renderMediaResult(a.methods[m]))
		}
		fmt.Fprintf(&sb, " | alt-sni: google.com=%s none=%s", renderMediaResult(a.altGoogle), renderMediaResult(a.altNone))
		lines = append(lines, sb.String())
	}
	if controlOK {
		var sb strings.Builder
		sb.WriteString("  control (i.ytimg.com):")
		for _, m := range methods {
			fmt.Fprintf(&sb, " %s=%s", m, renderMediaResult(controlResults[m]))
		}
		lines = append(lines, sb.String())
	} else {
		lines = append(lines, "  control (i.ytimg.com): unresolved — the table above is not trustworthy (fix the test first)")
	}
	var sb strings.Builder
	sb.WriteString("  packets (fragmenter TCP segments):")
	for _, m := range methods {
		fmt.Fprintf(&sb, " %s=%d", m, packets[m])
	}
	lines = append(lines, sb.String())
	if len(foreign) > 0 {
		for _, row := range foreign {
			var fb strings.Builder
			fmt.Fprintf(&fb, "  foreign-cluster %s:", row.label)
			for _, m := range methods {
				fmt.Fprintf(&fb, " %s=%s", m, renderMediaResult(row.methods[m]))
			}
			lines = append(lines, fb.String())
		}
	} else {
		lines = append(lines, "  foreign-cluster: none (the experiment file is empty)")
	}
	lines = append(lines, fmt.Sprintf("  media-verified: %d of %d addresses marked (answered under the real SNI — the substitution pool's only food now)",
		marked, len(addrs)))
	return lines
}

// renderMediaResult prints one verdict: answered(400 Bad Request),
// silent-after-handshake, hello-silent, reset(detail), error(detail).
func renderMediaResult(r probe.MediaResult) string {
	switch r.Class {
	case probe.MediaAnswered:
		return "answered(" + strings.TrimSpace(r.Status) + ")"
	case probe.MediaReset, probe.MediaError:
		if r.Detail != "" {
			return string(r.Class) + "(" + r.Detail + ")"
		}
		return string(r.Class)
	default:
		return string(r.Class)
	}
}

// runBatch runs one probe over every address with the concurrency
// cap. work runs the probe (and counts its own hellos); collect
// stores the verdict.
func runBatch(addrs []*netAddr, work func(*netAddr) probe.MediaResult, collect func(*netAddr, probe.MediaResult)) {
	sem := make(chan struct{}, networkTestConcurrency)
	var wg sync.WaitGroup
	for _, a := range addrs {
		wg.Add(1)
		sem <- struct{}{}
		go func(a *netAddr) {
			defer wg.Done()
			defer func() { <-sem }()
			collect(a, work(a))
		}(a)
	}
	wg.Wait()
}

// countProbeHello counts one probe's hello into the discipline meter
// when its SNI is a googlevideo name — the censor sees it, the
// report says so (ADR-0042's rule, kept honest for the test's own
// deliberate burst).
func (e *Engine) countProbeHello(sni string) {
	if strings.HasSuffix(sni, ".googlevideo.com") {
		e.stats.GvideoHello()
	}
}

func dedupeNetIPs(ips []net.IP) []net.IP {
	seen := make(map[string]bool)
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		key := ip.String()
		if !seen[key] {
			seen[key] = true
			out = append(out, ip)
		}
	}
	return out
}
