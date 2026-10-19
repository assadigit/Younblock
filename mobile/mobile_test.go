// mobile_test.go — host-runnable tests for the gomobile facade (roadmap
// M3 task 4), including the engine-level protect() call-counting proof
// ADR-0011 mandates. The exact every-dial invariant is enforced and
// exhaustively tested at the netx/adapter layer (android_test.go); here
// the proof is end-to-end: the facade's SocketProtector, handed over by
// NewEngine, is the hook the engine's real dials (DoH bootstrap, tunnel
// upstreams) call through during a real connect sequence and connection
// test — all against local fakes, no live network.
package mobile

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/health"

	"golang.org/x/net/dns/dnsmessage"
)

// ---- test doubles ----

// countingProtector is the ADR-0011 call counter: a SocketProtector whose
// Protect counts invocations (success or refusal alike — the adapter
// turns refusal into an aborted dial, tested in android_test.go).
type countingProtector struct {
	mu    sync.Mutex
	calls int
}

func (c *countingProtector) Protect(fd int32) bool {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return true
}

func (c *countingProtector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// recordingSink records every event name/payload it receives.
type recordingSink struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingSink) OnEvent(name string, json string) {
	r.mu.Lock()
	r.events = append(r.events, name+"|"+json)
	r.mu.Unlock()
}

func (r *recordingSink) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.events))
	for i, e := range r.events {
		out[i] = strings.SplitN(e, "|", 2)[0]
	}
	return out
}

func (r *recordingSink) reset() {
	r.mu.Lock()
	r.events = nil
	r.mu.Unlock()
}

func (r *recordingSink) has(name string) bool {
	for _, n := range r.names() {
		if n == name {
			return true
		}
	}
	return false
}

// newLeafCert makes one self-signed leaf trusted by a pool; SANs cover
// the fake DoH (IP) and the fake upstreams (DNS names the health checks
// use). Tests inject trust pools; production never disables verification.
func newLeafCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "younblock-mobile-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"www.youtube.com", "redirector.googlevideo.com", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})) {
		t.Fatal("pool rejected the test cert")
	}
	return cert, pool
}

// httpsServer serves handler over TLS with the given cert and returns its
// port on 127.0.0.1.
func httpsServer(t *testing.T, cert tls.Certificate, handler http.Handler) int {
	t.Helper()
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rawLn.Close() })
	tlsLn := tls.NewListener(rawLn, &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2"}})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(tlsLn)
	t.Cleanup(func() { srv.Close() })
	return rawLn.Addr().(*net.TCPAddr).Port
}

// fakeDoHHandler answers every A question with 127.0.0.1 (the same wire
// shape as the proxy package's fakeDoHServer).
func fakeDoHHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("dns")
		wire, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(q)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var p dnsmessage.Parser
		hdr, err := p.Start(wire)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		question, err := p.Question()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var ar dnsmessage.AResource
		copy(ar.A[:], net.ParseIP("127.0.0.1").To4())
		b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: hdr.ID, Response: true, RCode: dnsmessage.RCodeSuccess})
		b.StartQuestions()
		b.Question(question)
		b.StartAnswers()
		b.AResource(dnsmessage.ResourceHeader{
			Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 300,
		}, ar)
		msg, err := b.Finish()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "application/dns-message")
		w.Write(msg)
	})
}

// waitForPhase polls GetState until the phase matches or the deadline hit.
func waitForPhase(t *testing.T, m *Engine, want engine.Phase, timeout time.Duration) engine.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st := m.eng.GetState()
		if st.Phase == want {
			return st
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("engine never reached %q (now %q); state %+v", want, m.eng.GetState().Phase, m.eng.GetState())
	return engine.State{}
}

// ---- tests ----

func TestFacadePlatformAndJSONAPI(t *testing.T) {
	dir := t.TempDir()
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	// State: off, through the bound JSON surface.
	if got := m.GetState(); !strings.Contains(got, `"phase":"off"`) {
		t.Fatalf("GetState() = %s", got)
	}

	// App info: platform android, per-app routing capability, version from
	// the backend, paths under the config dir the shell passed.
	info := m.GetAppInfo()
	for _, want := range []string{`"platform":"android"`, `"perAppRouting":true`, config.AppVersion, dir} {
		if !strings.Contains(info, want) {
			t.Errorf("GetAppInfo() missing %q: %s", want, info)
		}
	}

	// Settings round-trip through the facade (Normalize on the other side).
	saved := m.SaveSettings(`{"mode":"all","method":"strong"}`)
	if !strings.Contains(saved, `"mode":"all"`) || !strings.Contains(saved, `"method":"strong"`) {
		t.Fatalf("SaveSettings = %s", saved)
	}
	if got := m.GetSettings(); !strings.Contains(got, `"mode":"all"`) {
		t.Fatalf("GetSettings = %s", got)
	}
	if got := m.SaveSettings(`{bad json`); got != `{"error":"UNKNOWN"}` {
		t.Fatalf("SaveSettings(bad) = %s, want UNKNOWN", got)
	}
	// The bad payload must not have clobbered the saved settings.
	if got := m.GetSettings(); !strings.Contains(got, `"mode":"all"`) {
		t.Fatalf("settings changed after a rejected payload: %s", got)
	}

	// A connection test while off is an honest failure, not a fake result.
	if got := m.RunConnectionTest(); !strings.Contains(got, `"error"`) {
		t.Fatalf("RunConnectionTest while off = %s, want an error object", got)
	}

	// Diagnostic report renders.
	if got := m.GetDiagnosticReport(); !strings.Contains(got, "diagnostic report") {
		t.Fatalf("GetDiagnosticReport = %.80s", got)
	}

	// Settings were persisted under the shell's config dir.
	if _, err := os.Stat(config.SettingsPath(dir)); err != nil {
		t.Fatalf("settings.json not persisted: %v", err)
	}
}

func TestFacadeStartStopCycle(t *testing.T) {
	dir := t.TempDir()
	sink := &recordingSink{}
	m, err := NewEngine(dir, nil, sink)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}
	// The production facade arms the start gate closed (D77, M5): the
	// Kotlin service opens it once consent is granted and the TUN is
	// established. This test has no service — it opens the door directly,
	// because the Start/Stop CYCLE is under test here; the consent gating
	// itself has its own lifecycle suite.
	m.SetVpnReady(true)
	// With no fakes injected this host run hits the real self-test; the
	// phase must still settle into an honest terminal state (on, degraded
	// or error — never a lie), and the sink must have seen state events.
	waitForAnyPhase(t, m, 40*time.Second)
	if !sink.has("state") {
		t.Fatal("the bridge sink never received a state event")
	}
	if got := m.Stop(); got != `{"error":null}` {
		t.Fatalf("Stop = %s", got)
	}
	st := waitForAnyPhase(t, m, 10*time.Second)
	if st.Phase != engine.PhaseOff {
		t.Fatalf("after Stop: phase %q, want off", st.Phase)
	}
	// Idempotent second stop.
	if got := m.Stop(); got != `{"error":null}` {
		t.Fatalf("second Stop = %s", got)
	}
}

func waitForAnyPhase(t *testing.T, m *Engine, timeout time.Duration) engine.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	terminal := map[engine.Phase]bool{
		engine.PhaseOn: true, engine.PhaseDegraded: true, engine.PhaseError: true, engine.PhaseOff: true,
	}
	for time.Now().Before(deadline) {
		st := m.eng.GetState()
		if terminal[st.Phase] {
			return st
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("engine never reached a terminal phase; state %+v", m.eng.GetState())
	return engine.State{}
}

// TestProtectCallCountingThroughEngine is the ADR-0011 engine-level
// proof: the SocketProtector passed to NewEngine is called for the
// engine's real dials — the DoH bootstrap connection and the tunnel
// upstream dials — during a genuine connect sequence and connection test,
// with all checks served by local fakes. Lower bounds, not exact counts:
// the exact number depends on HTTP connection reuse and the prewarm
// list; the every-dial invariant itself is proven exactly at the netx
// layer (internal/platform/android tests).
func TestProtectCallCountingThroughEngine(t *testing.T) {
	dir := t.TempDir()
	cert, pool := newLeafCert(t)

	// Fake DoH server (the engine's resolver will point here via settings).
	dohPort := httpsServer(t, cert, fakeDoHHandler())

	// Fake upstream for the youtube/video checks: 204 on /generate_204,
	// anything else 200 (the video step only needs the TLS handshake).
	upMux := http.NewServeMux()
	upMux.HandleFunc("/generate_204", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	upMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	upPort := httpsServer(t, cert, upMux)

	// Settings that point the resolver at the fake DoH (persisted before
	// NewEngine, so the engine loads them at construction).
	settings := config.Settings{
		Version: 1,
		Mode:    config.ModeYouTube,
		Advanced: config.Advanced{
			DoHServers: []config.DoHServer{{
				Name:      "FakeDoH",
				URL:       fmt.Sprintf("https://127.0.0.1:%d/dns-query?dns={dns}", dohPort),
				Bootstrap: []string{"127.0.0.1"},
			}},
			Domains: config.DefaultDomains(),
		},
	}
	if err := config.Save(dir, settings); err != nil {
		t.Fatal(err)
	}

	protector := &countingProtector{}
	sink := &recordingSink{}
	newTLS := func() *tls.Config { return &tls.Config{RootCAs: pool} }
	m, err := newEngineForTest(dir, protector, sink, func(o *engine.Options) {
		o.AllowPrivateTargets = true // tests only: the fakes live on 127.0.0.1
		o.ResolverTLS = newTLS()
		o.WatchdogInterval = time.Hour // keep the test quiet
		o.HealthOverrides = func(c *health.Config) {
			c.PrimaryURL = fmt.Sprintf("https://www.youtube.com:%d/generate_204", upPort)
			c.FallbackURL = fmt.Sprintf("https://www.youtube.com:%d/fallback", upPort)
			c.VideoHost = fmt.Sprintf("redirector.googlevideo.com:%d", upPort)
			c.DNSName = "www.youtube.com"
			c.VideoTLS = newTLS()
			c.WebTLS = newTLS()
		}
	})
	if err != nil {
		t.Fatalf("newEngineForTest: %v", err)
	}
	defer m.Shutdown()

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}
	st := waitForPhase(t, m, engine.PhaseOn, 40*time.Second)
	if st.LatencyMs == nil {
		t.Fatalf("phase on without a real latency measurement: %+v", st)
	}

	// The connect sequence alone dials: the DoH bootstrap (dns step) and
	// the tunnel upstream (youtube step). That is the lower bound while
	// connecting; prewarm may add more DoH dials concurrently.
	if got := protector.count(); got < 2 {
		t.Fatalf("after a full connect sequence, protect called %d times, want >= 2 (DoH bootstrap + tunnel dial)", got)
	}

	// The four-step connection test adds at least one more tunnel dial
	// (video step) through a fresh upstream connection.
	if got := m.RunConnectionTest(); strings.Contains(got, `"error"`) {
		t.Fatalf("RunConnectionTest = %s", got)
	}
	if got := protector.count(); got < 3 {
		t.Fatalf("after the connection test, protect called %d times, want >= 3", got)
	}

	if got := m.Stop(); got != `{"error":null}` {
		t.Fatalf("Stop = %s", got)
	}
	waitForPhase(t, m, engine.PhaseOff, 10*time.Second)
	if !sink.has("state") {
		t.Fatal("bridge sink never saw a state event")
	}
}

// TestNoticeForwarding: the platform adapter's Notify (engine failure and
// recovery notices) crosses the bridge as a "notice" event.
func TestNoticeForwarding(t *testing.T) {
	dir := t.TempDir()
	sink := &recordingSink{}
	m, err := NewEngine(dir, nil, sink)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	m.plat.Notify(engine.NoticeFailure, "secure dns is unreachable")
	deadline := time.Now().Add(2 * time.Second)
	for !sink.has("notice") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !sink.has("notice") {
		t.Fatal("no notice event crossed the bridge")
	}
	found := false
	for _, raw := range eventsRaw(sink) {
		if strings.HasPrefix(raw, "notice|") {
			body := strings.TrimPrefix(raw, "notice|")
			if strings.Contains(body, `"kind":"failure"`) && strings.Contains(body, "secure dns is unreachable") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("notice payload wrong: %v", eventsRaw(sink))
	}
}

func eventsRaw(r *recordingSink) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// TestWebUILifecycle: the in-process loopback control server (ADR-0005)
// starts, serves the JSON API and the static page over real HTTP, keeps
// the bridge sink alive while it runs, stops, and restarts.
func TestWebUILifecycle(t *testing.T) {
	dir := t.TempDir()
	sink := &recordingSink{}
	m, err := NewEngine(dir, nil, sink)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	url, err := m.StartWebUI(0)
	if err != nil {
		t.Fatalf("StartWebUI: %v", err)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("webui URL = %q, want a loopback http URL", url)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// The JSON API through the loopback server (the WebView's transport).
	resp, err := client.Get(url + "/api/state")
	if err != nil {
		t.Fatalf("GET /api/state: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"phase":"off"`) {
		t.Fatalf("/api/state: %d %s", resp.StatusCode, body)
	}

	// The static page (the bundle webui embeds).
	resp, err = client.Get(url + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	index, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(index), "<html") {
		t.Fatalf("/ : %d %.60s", resp.StatusCode, index)
	}

	// webui.New installs its own sink; the bridge must survive that (the
	// fanout): a settings save emits a state event, and the bridge gets it.
	sink.reset()
	if got := m.SaveSettings(`{"mode":"all"}`); !strings.Contains(got, `"mode":"all"`) {
		t.Fatalf("SaveSettings = %s", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !sink.has("state") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !sink.has("state") {
		t.Fatal("the bridge sink was disconnected by the webui (fanout broken)")
	}

	// Stop: the port closes; Start again binds a new one.
	m.StopWebUI()
	_, err = client.Get(url + "/api/state")
	if err == nil {
		t.Fatalf("webui still serving on %s after StopWebUI", url)
	}
	url2, err := m.StartWebUI(0)
	if err != nil {
		t.Fatalf("second StartWebUI: %v", err)
	}
	resp, err = client.Get(url2 + "/api/state")
	if err != nil {
		t.Fatalf("GET /api/state after restart: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("restart: %d", resp.StatusCode)
	}
	m.StopWebUI()
}
