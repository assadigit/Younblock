// Package mobile is the gomobile facade over the engine (docs/android-port
// 04 §4, ADR-0004/0005; roadmap M3 task 4).
//
// gomobile's constraint is honored by design: the whole bridge is
// string-in/string-out, the JSON contracts byte-identical across Wails,
// webui and Android (the engine's jsonapi.go, verbatim). Nothing internal
// crosses the boundary: Kotlin sees Engine, SocketProtector and
// EventSink, and strings — plus the TUN lifecycle of tun.go
// (StartTun/StopTun/TunStats, roadmap M4): the service hands the
// established VpnService fd to the facade once the engine is serving.
//
// Threading contract (doc 04 §4, restated for the binding): gomobile
// calls arrive from Java threads attached to the VM; Go-side code runs
// them on the calling thread like any Go call. Engine events and notices
// arrive on the goroutine that produced them — Kotlin marshals to its
// main/service threads. A throwing Kotlin callback cannot take down the
// engine goroutine that emitted the event: the bridge wraps every call
// with a recover (the event is dropped, nothing else happens).
//
// The webui control (StartWebUI/StopWebUI) is the transport ADR-0005
// requires: the same in-process loopback server that powers the Linux web
// edition (internal/webui, D50), serving the existing Svelte bundle to
// the WebView. While it runs, engine events fan out to BOTH the SSE hub
// (the page) and this package's EventSink (the Kotlin shell: the
// notification, the service state) — webui.New would otherwise install
// its hub as the engine's only sink.
package mobile

import (
        "encoding/json"
        "fmt"
        "strings"
        "sync"

        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/platform/android"
        "github.com/assadigit/Younblock/internal/webui"
)

// SocketProtector protects one engine-upstream socket before it connects.
// The Kotlin implementation is VpnService.protect(fd); ADR-0011: every
// tunnel dial and every DoH bootstrap connection goes through the engine's
// one netx dialer, whose Control hook calls Protect. Returning false
// aborts the dial (fail safe: an unprotected socket would loop back into
// the TUN).
type SocketProtector interface {
        // Protect exempts one file descriptor from the VPN's routing.
        Protect(fd int32) bool
}

// EventSink receives engine events and notices. Event names today:
// "state" (full state JSON), "stats" (counters sample), "notice"
// ({"kind":"failure"|"recovery","text":"..."}).
type EventSink interface {
        OnEvent(name string, json string)
}

// Engine is the bound engine facade. All JSON methods forward to the
// engine's string API verbatim (internal/engine/jsonapi.go).
type Engine struct {
        mu sync.Mutex

        eng  *engine.Engine
        plat *android.Platform
        sink EventSink // nil means no bridge callbacks

        webUI    *webui.Server
        webUISrv chan struct{} // closed when the Serve goroutine returned

        tun *tunState // the live TUN stack (nil when none); see tun.go

        // shellVersion (M8-p, ADR-0015) is the APK's identity string as
        // Kotlin reports it from PackageManager ("1.6.6 (versionCode 8)"), so
        // a pasted diagnostic report names the exact build it came from —
        // versionName alone stays 1.6.6 across debug rounds. Guarded by mu.
        shellVersion string

        // deviceInfo / netInfo (M8-af, ADR-0043, spec W9): the phone's
        // identity and the radio's, as Kotlin reads them — the debug.23
        // consultation's §1.4 data gaps ("device model not recorded",
        // "ISP not recorded" — the reference projects' tuning tables
        // are per-ISP, and mobile networks and home lines filter
        // differently). Empty until Kotlin sets them.
        deviceInfo string
        netInfo    string
}

// NewEngine builds the engine for the Android shell: the platform adapter
// is wired to the shell's two doors (protect hook, notice callback) and
// the engine loads/creates settings.json under configDir (the app's
// private storage; Kotlin passes getFilesDir()).
//
// protector may be nil (no VPN context yet: consent not granted, service
// not running) — the engine then dials unprotected exactly like the
// desktops, which is correct whenever no TUN exists to loop into.
// sink may be nil (callbacks dropped).
//
// It returns an error only when the engine cannot initialize (settings
// unreadable/logs unwritable under configDir).
//
// M5 (D77): the returned engine's start gate is ARMED CLOSED — no VPN
// consent, no connect. The shell opens it with SetVpnReady(true) once
// the consent is granted and the TUN interface is established, and
// closes it again on every unwind (onRevoke, service death). Until
// then a started engine honestly holds at "connecting": it never
// reports "on" while nothing can route.
func NewEngine(configDir string, protector SocketProtector, sink EventSink) (*Engine, error) {
        m, err := newEngineForTest(configDir, protector, sink, nil)
        if err != nil {
                return nil, err
        }
        m.plat.SetStartReady(false)
        return m, nil
}

// newEngineForTest is NewEngine's test seam: same construction path, with
// the engine Options open for the host tests to inject fakes (health
// overrides, resolver TLS pools). Unexported, so gomobile never binds it.
// The start gate is left OPEN (host tests drive it explicitly through
// m.plat / SetVpnReady); NewEngine is the only path that arms it closed.
func newEngineForTest(configDir string, protector SocketProtector, sink EventSink, mutate func(*engine.Options)) (*Engine, error) {
        var protect android.ProtectFunc
        if protector != nil {
                p := protector
                protect = p.Protect
        }
        var notify android.NotifyFunc
        if sink != nil {
                s := &protectedSink{target: sink}
                notify = func(kind engine.NoticeKind, text string) {
                        payload, err := json.Marshal(map[string]string{"kind": string(kind), "text": text})
                        if err != nil {
                                return
                        }
                        s.OnEvent("notice", string(payload))
                }
        }
        plat := android.New(android.Options{
                ConfigDir: configDir,
                Protect:   protect,
                Notify:    notify,
        })
        // M8-aa (debug.19, versionCode 18): the report header names the exact
        // debug round — versionName stays 1.6.6 across rounds, the -debug.NN
        // suffix is what a pasted report maps to its release.
        // M8-ad (debug.22, versionCode 21): the long-lived-connection round.
        // M8-af (debug.24, versionCode 23): the prove-before-change round.
        // M8-ag (debug.25, versionCode 24): the breaker-stops-starving round.
        // M8-ah (debug.26, versionCode 25): the dead-cluster memory round.
        // M8-ai (debug.27, versionCode 26): the pool keeps its food round.
        // M8-aj (debug.28, versionCode 27): the lane retires round.
        // M8-ak (debug.29, versionCode 28): the method-documentation round.
        // M8-am (debug.30, versionCode 29): the release-candidate polish
        // round — the owner's two pre-release asks: the real launcher
        // icon (his icon.ico mark) and the connect-time YouTube
        // hand-off (ADR-0051, D104/D105).
        // M8-an (debug.31, versionCode 30): the wait-until-ready round
        // — the hand-off now waits for the engine's warm gate
        // (ADR-0052) and the dead-cluster marks persist across
        // sessions (ADR-0053).
        opts := engine.Options{Platform: plat, Version: "1.6.6-debug.31"}
        if mutate != nil {
                mutate(&opts)
        }
        eng, err := engine.New(opts)
        if err != nil {
                return nil, fmt.Errorf("mobile: %w", err)
        }
        m := &Engine{eng: eng, plat: plat, sink: sink}
        if sink != nil {
                eng.SetEventSink(&protectedSink{target: sink})
        }
        // M8-p (ADR-0015): the facade owns evidence the engine cannot see —
        // the TUN counters and the shell version — so the diagnostic report
        // the WebView copies carries them too (engine.SetReportExtra).
        eng.SetReportExtra(m.diagnosticsExtra)
        return m, nil
}

// GetState returns the state JSON ({"phase":"off",...}).
func (m *Engine) GetState() string { return m.eng.GetStateJSON() }

// Start starts connecting; returns {"error":null} or a typed error code.
func (m *Engine) Start() string { return m.eng.StartJSON() }

// Stop stops the engine (idempotent-or-rejecting per the phase guards).
func (m *Engine) Stop() string { return m.eng.StopJSON() }

// Toggle toggles on/off.
func (m *Engine) Toggle() string { return m.eng.ToggleJSON() }

// GetSettings returns the normalized settings JSON.
func (m *Engine) GetSettings() string { return m.eng.GetSettingsJSON() }

// SaveSettings applies a settings payload; returns the normalized
// settings, or {"error":"UNKNOWN"} for unparseable input.
func (m *Engine) SaveSettings(payload string) string { return m.eng.SaveSettingsJSON(payload) }

// RunConnectionTest runs the four-step test; requires the engine to be on.
func (m *Engine) RunConnectionTest() string { return m.eng.RunConnectionTestJSON() }

// AutoFix runs the heal sweep on demand (the "Fix connection" button).
func (m *Engine) AutoFix() string { return m.eng.AutoFixJSON() }

// GetDiagnosticReport returns the redacted report JSON.
func (m *Engine) GetDiagnosticReport() string { return m.eng.GetDiagnosticReportJSON() }

// Mark stamps the diagnostics timeline (M8-z, ADR-0027 §8.3.6): the
// shell-side entry the WebView's diagnostics screen reaches through
// the webui route (the "I pressed play" / "First frame" buttons). The
// line lands in the log ring and the report prints the elapsed time
// between the last pair plus the failed/slow flows inside that window.
func (m *Engine) Mark(label string) { m.eng.Mark(label) }

// GetAppInfo returns app metadata JSON (version, platform, capabilities,
// config/log paths).
func (m *Engine) GetAppInfo() string { return m.eng.GetAppInfoJSON() }

// SetShellVersion records the APK's identity (M8-p, ADR-0015) — Kotlin
// passes it once at engine construction, built from PackageManager's
// versionName + versionCode (e.g. "1.6.6 (versionCode 8)"). It rides in
// the diagnostic report's android section so a pasted report names the
// exact debug round it came from. Empty until set; never fails.
func (m *Engine) SetShellVersion(v string) {
        m.mu.Lock()
        m.shellVersion = v
        m.mu.Unlock()
}

// SetDeviceInfo records the phone's identity (M8-af, ADR-0043, spec
// W9): Kotlin passes Build.MANUFACTURER, Build.MODEL and
// Build.VERSION.SDK_INT once at engine construction. It rides the
// report's header as "device: <manufacturer> <model> android <sdk>"
// — the consultation's unrecorded-phone-model data gap, closed. No
// unique identifiers (no serial, no IMEI): the model string only.
func (m *Engine) SetDeviceInfo(manufacturer, model string, sdkInt int32) {
        if manufacturer == "" && model == "" {
                return
        }
        m.mu.Lock()
        m.deviceInfo = fmt.Sprintf("%s %s android %d", strings.TrimSpace(manufacturer), strings.TrimSpace(model), sdkInt)
        m.mu.Unlock()
}

// SetNetworkInfo records the radio's identity (M8-af, ADR-0043, spec
// W9): Kotlin passes the network type guess ("wifi"/"cellular"/
// "unknown" — a heuristic the shell owns and documents), the
// operator's alphabetic name and the mccmnc code — both from
// TelephonyManager getters that need NO permission and read empty on
// devices that will not share them (the report prints unknown then).
// This is the consultation's unrecorded-ISP data gap: the reference
// tuning tables are per-ISP (MCI 80-250 fragments, Irancell 10-40),
// and no report has ever said which one the tester rode.
func (m *Engine) SetNetworkInfo(netType, operator, mccmnc string) {
        if netType == "" {
                netType = "unknown"
        }
        if operator == "" {
                operator = "unknown"
        }
        if mccmnc == "" {
                mccmnc = "unknown"
        }
        m.mu.Lock()
        m.netInfo = fmt.Sprintf("type=%s operator=%s mccmnc=%s", netType, operator, mccmnc)
        m.mu.Unlock()
}

// diagnosticsExtra is the report section the facade installs on the
// engine (ADR-0015): the shell version line, then the TUN counters —
// the one surface that names the "flow never reached the door" case
// (flowsOpened stuck at zero while the app buffers) and the QUIC suspect
// (udpDropped climbing while the YouTube app tries UDP). Counters only,
// in the report's style — no addresses, no hostnames (the R21 redaction
// applies to this section too).
func (m *Engine) diagnosticsExtra() string {
        var b strings.Builder
        m.mu.Lock()
        shell := m.shellVersion
        device := m.deviceInfo
        net := m.netInfo
        t := m.tun
        m.mu.Unlock()
        if shell != "" {
                fmt.Fprintf(&b, "android shell: %s\n", shell)
        }
        // M8-af (ADR-0043, spec W9): the environment lines the
        // consultation's §1.4 asked for — phone model and network
        // identity, per-ISP filtering made visible.
        if device != "" {
                fmt.Fprintf(&b, "device: %s\n", device)
        }
        if net != "" {
                fmt.Fprintf(&b, "net: %s\n", net)
        }
        if t == nil {
                b.WriteString("tun: inactive\n")
        } else {
                st := t.stack.Stats()
                // M8-z (ADR-0027 §8.4): v6Dropped rides the tun line — the
                // engine-side counters cannot see the stack's own discards.
                fmt.Fprintf(&b, "tun: active flows=%d rstUnmapped=%d overCap=%d doorFailures=%d dnsQueries=%d udpDropped=%d v6Dropped=%d\n",
                        st.FlowsOpened, st.FlowsRSTUnmapped, st.FlowsOverCap,
                        st.DoorFailures, st.DNSQueries, st.UDPDropped, st.V6Dropped)
        }
        return strings.TrimSuffix(b.String(), "\n")
}

// SetVpnReady is the M5 lifecycle door (D77): true once the VPN consent
// is granted AND the TUN interface is established — the engine's connect
// run may then leave the gate and serve; false the moment either is lost
// (onRevoke, service death, teardown), re-arming the gate for the next
// connect. While the gate is closed a started engine holds at
// "connecting" — never "on" without routing, never a misleading
// "Connected".
func (m *Engine) SetVpnReady(ready bool) { m.plat.SetStartReady(ready) }

// FailStart is the M7 platform-refusal door (D81): the shell discovered
// the connect cannot honestly proceed — today, PackageManager found none
// of the allow-list's apps installed (code "NO_TARGET_APPS") — and the
// gate-held connect run must fail through the engine's ordinary path:
// phase "error" with the typed code, the state emitted, the page showing
// "Can't Connect" with the honest reason, instead of a silent unwind or
// a parallel Kotlin error UI. The phase machine stays the single truth
// (ADR-0006).
//
// Returns {"error":null} when the refusal was delivered (or latched for
// a run that has not reached the gate yet), {"error":"NOT_HELD"} when
// the gate is open (an open gate holds no run to refuse — the shell's
// honest-stop path owns a serving engine; log it), or {"error":"BAD_CODE"}
// for any code outside the whitelisted vocabulary.
func (m *Engine) FailStart(code string) string {
        switch code {
        case "NO_TARGET_APPS":
        default:
                return `{"error":"BAD_CODE"}`
        }
        if !m.plat.RefuseStart(code) {
                return `{"error":"NOT_HELD"}`
        }
        return `{"error":null}`
}

// RoutedApps is the M6 settings surface for the Kotlin service: the
// per-app allow-list (schema v2) the VpnService bakes into its Builder
// at TUN establish, as {"apps":["com.android.chrome",...]} — the same
// normalized list GetSettings reports and the WebView's Apps-to-unblock
// row edits (one truth, three doors). The list is read at establish
// time: a change saved while the TUN is live applies on the NEXT
// connect (rebuilding an established TUN under live traffic is riskier
// than the delay, and the UI copy says so — no silent re-route).
//
// D86 put the YouTube app on the list (Ali's ruling after the debug.1
// device observation); D87 extends the default to the Chrome, Firefox,
// Opera and Brave browsers (his second ruling, same feedback round).
// The guard below still fails toward the EMPTY list (the
// NO_TARGET_APPS refusal), never toward the default: an engine state
// broken enough to lose its settings must refuse, not silently route
// anything.
func (m *Engine) RoutedApps() string {
        apps := m.eng.Settings().Apps
        if apps == nil {
                apps = []string{} // marshal [] — an explicit, honest empty list
        }
        out, err := json.Marshal(map[string][]string{"apps": apps})
        if err != nil {
                // Unreachable for a []string; if it ever fires, fail toward the
                // empty list — the NO_TARGET_APPS refusal owns what happens next,
                // never route-everything.
                out = []byte(`{"apps":[]}`)
        }
        return string(out)
}

// AutoOpenYouTube is the M8-am settings surface for the Kotlin service
// (ADR-0051, D105): the connect-time hand-off switch, as
// {"enabled":<bool>} — the same normalized truth GetSettings reports
// (the `autoOpenYouTube` field, schema v8) and the WebView's Settings
// row edits. The shell reads it fresh at the moment the engine turns
// on (a change saved while connected applies on the NEXT connect, the
// same as the allow-list), so the three doors can never disagree about
// whether the hand-off fires.
//
// The default is true on Android (the owner's checked-by-default
// ruling; LoadFor applies it to fresh installs and pre-v8 files) — the
// facade reports whatever the engine's normalized settings hold.
func (m *Engine) AutoOpenYouTube() string {
        out, err := json.Marshal(map[string]bool{"enabled": m.eng.Settings().AutoOpenYouTube})
        if err != nil {
                // Unreachable for a bool; if it ever fires, fail toward
                // DISABLED — an unasked-for app launch is worse than a
                // missed one, never the reverse.
                return `{"enabled":false}`
        }
        return string(out)
}

// StartWebUI starts the in-process loopback control server (ADR-0005) on
// the given port (0 = a free port) and returns the URL the WebView loads.
// While it runs, engine events fan out to the SSE hub AND the EventSink.
// Idempotent: returns the existing URL if already running.
func (m *Engine) StartWebUI(port int32) (string, error) {
        m.mu.Lock()
        defer m.mu.Unlock()
        if m.webUI != nil {
                return m.webUI.URL(), nil
        }
        srv := webui.New(m.eng, int(port))
        // webui.New just installed its hub as the engine's only sink; put the
        // bridge back in alongside it (hub first: the page, then the shell).
        if m.sink != nil {
                m.eng.SetEventSink(&fanoutSink{hub: srv.Events(), bridge: &protectedSink{target: m.sink}})
        }
        if _, err := srv.Listen(); err != nil {
                return "", fmt.Errorf("mobile: webui: %w", err)
        }
        m.webUI = srv
        m.webUISrv = make(chan struct{})
        go func() {
                defer close(m.webUISrv)
                _ = srv.Serve()
        }()
        return srv.URL(), nil
}

// StopWebUI closes the control server (idempotent). The EventSink keeps
// receiving engine events afterwards.
func (m *Engine) StopWebUI() {
        m.mu.Lock()
        srv := m.webUI
        srvDone := m.webUISrv
        m.webUI = nil
        m.webUISrv = nil
        m.mu.Unlock()
        if srv == nil {
                return
        }
        _ = srv.Close()
        if srvDone != nil {
                <-srvDone
        }
        if m.sink != nil {
                m.eng.SetEventSink(&protectedSink{target: m.sink})
        }
}

// Shutdown stops the TUN stack (if any), the webui and the engine,
// idempotently, bounding resource teardown. No callbacks are delivered
// after it returns; the engine's sink contract (no events after
// Shutdown) is inherited as-is.
func (m *Engine) Shutdown() {
        m.StopTun()
        m.StopWebUI()
        m.eng.Shutdown()
}

// protectedSink guards the bridge: a throwing Kotlin callback must not
// kill the engine goroutine that emitted the event. The event is dropped
// in that case (nothing else is safe to do from here).
type protectedSink struct {
        target EventSink
}

func (p *protectedSink) OnEvent(name string, payload string) {
        defer func() { _ = recover() }()
        p.target.OnEvent(name, payload)
}

// fanoutSink delivers one event to both consumers: the SSE hub (the
// WebView page) first, the bridge (the Kotlin shell) second.
type fanoutSink struct {
        hub    engine.EventSink
        bridge EventSink
}

func (f *fanoutSink) OnEvent(name string, payload string) {
        if f.hub != nil {
                f.hub.OnEvent(name, payload)
        }
        if f.bridge != nil {
                f.bridge.OnEvent(name, payload)
        }
}
