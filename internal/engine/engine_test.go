package engine_test

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// recordingSink collects events for assertions.
type recordingSink struct {
	mu     sync.Mutex
	events []struct{ name, payload string }
}

func (r *recordingSink) OnEvent(name string, payload string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, struct{ name, payload string }{name, payload})
}

func (r *recordingSink) all() []struct{ name, payload string } {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]struct{ name, payload string }{}, r.events...)
}

func (r *recordingSink) states() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		if e.name == "state" {
			out = append(out, e.payload)
		}
	}
	return out
}

// fakeNetwork is a programmable Checker: a fake network that can be
// configured to accept only specific methods (file 01, section 11).
type fakeNetwork struct {
	mu        sync.Mutex
	accept    map[config.Method]bool
	calls     int
	healCalls int
	delay     time.Duration
}

func (f *fakeNetwork) acceptAll(ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accept = map[config.Method]bool{
		config.MethodAuto: ok, config.MethodLight: ok, config.MethodBalanced: ok,
		config.MethodStrong: ok, config.MethodLegacy: ok,
	}
}

func (f *fakeNetwork) acceptsOnly(m config.Method) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accept = map[config.Method]bool{m: true}
}

func (f *fakeNetwork) result(m config.Method) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.accept[m]
}

func (f *fakeNetwork) healResult(m config.Method) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.healCalls++
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.accept[m]
}

func (f *fakeNetwork) QuickCheck(ctx context.Context, m config.Method) bool {
	return f.result(m)
}

func (f *fakeNetwork) HealCheck(ctx context.Context, m config.Method) bool {
	return f.healResult(m)
}

func (f *fakeNetwork) healCallsCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healCalls
}

func (f *fakeNetwork) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// newTestEngine builds an engine with a fake network that accepts everything.
func newTestEngine(t *testing.T) (*engine.Engine, *stub.Platform, *recordingSink, *fakeNetwork) {
	t.Helper()
	dir := t.TempDir()
	plat := stub.NewWithDir(dir)
	net := &fakeNetwork{}
	net.acceptAll(true)
	eng, err := engine.New(engine.Options{
		Platform: plat,
		LogDir:   dir + "/logs",
		Checker:  net,
	})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	sink := &recordingSink{}
	eng.SetEventSink(sink)
	t.Cleanup(eng.Shutdown)
	return eng, plat, sink, net
}

// waitForPhase polls until the engine reaches one of the wanted phases.
func waitForPhase(t *testing.T, eng *engine.Engine, want ...engine.Phase) engine.State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := eng.GetState()
		for _, w := range want {
			if st.Phase == w {
				return st
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("engine never reached %v, now %s", want, eng.GetState().Phase)
	return engine.State{}
}

func TestEngineStartStopCycle(t *testing.T) {
	eng, plat, sink, _ := newTestEngine(t)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForPhase(t, eng, engine.PhaseOn)
	if st.Port < 1 {
		t.Errorf("port = %d, want a real port", st.Port)
	}
	if st.Mode != config.ModeYouTube || st.Method != config.MethodAuto {
		t.Errorf("state mode/method = %s/%s", st.Mode, st.Method)
	}
	if got := plat.AttachCalls(); len(got) != 1 || got[0].Mode != config.ModeYouTube {
		t.Errorf("attach calls = %v, want one youtube attach", got)
	}
	if got := plat.AttachCalls()[0].Port != st.Port; got {
		t.Errorf("attach port mismatch")
	}
	// A real listener answers /health.
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(st.Port)), 2*time.Second)
	if err != nil {
		t.Fatalf("proxy not listening: %v", err)
	}
	conn.Close()

	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitForPhase(t, eng, engine.PhaseOff)
	if plat.DetachCalls() < 1 {
		t.Errorf("detach calls = %d, want >= 1", plat.DetachCalls())
	}
	states := sink.states()
	if len(states) < 3 {
		t.Errorf("state events = %d, want at least connecting/on/off: %v", len(states), states)
	}
	var first engine.State
	if err := json.Unmarshal([]byte(states[0]), &first); err != nil {
		t.Fatalf("state json: %v", err)
	}
	if first.Phase != engine.PhaseConnecting || first.Detail != "starting" {
		t.Errorf("first state = %s/%s, want connecting/starting", first.Phase, first.Detail)
	}
}

func TestEngineToggle(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Toggle(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	if err := eng.Toggle(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOff)
}

func TestEngineStartIsIdempotent(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	st := waitForPhase(t, eng, engine.PhaseOn)
	// M7 (D82): the second Start must arrive while ON for the
	// idempotency claim — a second Start while CONNECTING is the
	// documented cancel (file 01 §10.5). The old form of this test
	// (Start immediately followed by Start) passed only through the
	// spawn-race zombie: the cancelled-but-unregistered run completed
	// to "on" anyway. D82 makes the cancel honest and deterministic;
	// the gate suite covers that path.
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	// Both starts refer to the same single listener.
	if st.Port != eng.GetState().Port {
		t.Errorf("port changed between starts")
	}
	if err := eng.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOff)
	if err := eng.Stop(); err != nil {
		t.Errorf("double Stop must be safe: %v", err)
	}
}

func TestEnginePortBusyGivesErrorPhase(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	// Occupy a port and configure the engine to require it.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	settings := eng.GetSettingsJSON()
	var s config.Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		t.Fatal(err)
	}
	s.Port = port
	s.AutoPort = false
	out := eng.SaveSettingsJSON(mustJSON(s))
	var saved config.Settings
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatalf("SaveSettings output not settings JSON: %s", out)
	}
	if saved.Port != port {
		t.Fatalf("saved port = %d, want %d", saved.Port, port)
	}

	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	st := waitForPhase(t, eng, engine.PhaseError)
	if st.Error == nil || st.Error.Code != "PORT_BUSY" {
		t.Errorf("state error = %+v, want PORT_BUSY", st.Error)
	}
	// The engine can still be started again after fixing the port.
	s.AutoPort = true
	eng.SaveSettingsJSON(mustJSON(s))
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
}

func TestEngineSaveSettingsAppliesMethod(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)

	var s config.Settings
	if err := json.Unmarshal([]byte(eng.GetSettingsJSON()), &s); err != nil {
		t.Fatal(err)
	}
	s.Method = config.MethodStrong
	out := eng.SaveSettingsJSON(mustJSON(s))
	var got config.Settings
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("SaveSettings reply: %v", err)
	}
	if got.Method != config.MethodStrong {
		t.Errorf("method = %s, want strong", got.Method)
	}
	if st := eng.GetState(); st.Method != config.MethodStrong {
		t.Errorf("state method = %s, want strong (applies immediately)", st.Method)
	}
	// Settings persisted on disk.
	s2, _, err := config.Load(eng.ConfigDir())
	if err != nil || s2.Method != config.MethodStrong {
		t.Errorf("persisted method = %s err=%v, want strong", s2.Method, err)
	}
	eng.Stop()
}

func TestEngineSettingsNormalizationRoundTrip(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	// Nonsense values come back clamped.
	bad := `{"mode":"bogus","method":"bogus","port":99999,"version":1}`
	out := eng.SaveSettingsJSON(bad)
	var got config.Settings
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("reply not JSON: %s", out)
	}
	if got.Mode != config.ModeYouTube || got.Method != config.MethodAuto || got.Port != config.ListenPort {
		t.Errorf("normalized settings = %+v", got)
	}
}

func TestEngineAutostartFlipCallsPlatform(t *testing.T) {
	eng, plat, _, _ := newTestEngine(t)
	var s config.Settings
	if err := json.Unmarshal([]byte(eng.GetSettingsJSON()), &s); err != nil {
		t.Fatal(err)
	}
	s.StartWithWindows = true
	eng.SaveSettingsJSON(mustJSON(s))
	if got := plat.Autostarts(); len(got) != 1 || !got[0] {
		t.Errorf("autostart calls = %v, want [true]", got)
	}
}

func TestEngineGetAppInfo(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	var info struct {
		Name         string              `json:"name"`
		Version      string              `json:"version"`
		Platform     string              `json:"platform"`
		Capabilities engine.Capabilities `json:"capabilities"`
		ConfigPath   string              `json:"configPath"`
	}
	if err := json.Unmarshal([]byte(eng.GetAppInfoJSON()), &info); err != nil {
		t.Fatalf("GetAppInfo: %v", err)
	}
	if info.Name != "Younblock" || info.Version != config.AppVersion {
		t.Errorf("info = %+v", info)
	}
	if info.Platform != "stub" {
		t.Errorf("platform = %q, want stub", info.Platform)
	}
	if info.Capabilities.Tray || info.Capabilities.StartWithOS {
		t.Errorf("stub capabilities should all be false: %+v", info.Capabilities)
	}
	if !strings.HasSuffix(info.ConfigPath, "settings.json") {
		t.Errorf("configPath = %q", info.ConfigPath)
	}
}

func TestEngineDiagnosticReportRedacted(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	report := eng.GetDiagnosticReport()
	for _, want := range []string{config.AppVersion, "stub", "measured sleep", "counters"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	// R21: no hostnames or IPs in the report.
	for _, banned := range []string{"youtube.com", "www.youtube", "172.67", "127.0.0.1"} {
		if strings.Contains(report, banned) {
			t.Errorf("report contains %q (privacy leak):\n%s", banned, report)
		}
	}
	eng.Stop()
}

func TestEngineRingClearedOnStop(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	if eng.Logger().Ring.Len() == 0 {
		t.Fatal("ring should have lines while running")
	}
	eng.Stop()
	if eng.Logger().Ring.Len() != 0 {
		t.Errorf("ring not cleared on Stop: %d lines", eng.Logger().Ring.Len())
	}
}

func TestEngineQuitInvokesCallback(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	called := make(chan struct{}, 1)
	eng.SetOnQuit(func() { called <- struct{}{} })
	eng.QuitJSON()
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("quit callback never invoked")
	}
	waitForPhase(t, eng, engine.PhaseOff)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
