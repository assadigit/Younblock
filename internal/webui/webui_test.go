package webui_test

import (
        "bufio"
        "context"
        "encoding/json"
        "io"
        "net/http"
        "strings"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/platform/stub"
        "github.com/assadigit/Younblock/internal/webui"
)

// fakeNetwork is the programmable Checker the engine tests use: it
// accepts every method so Start reaches "on" without any real network.
type fakeNetwork struct{}

func (f *fakeNetwork) QuickCheck(ctx context.Context, m config.Method) bool { return true }
func (f *fakeNetwork) HealCheck(ctx context.Context, m config.Method) bool  { return true }

// newServer builds a webui server around a real engine (stub platform,
// fake network) and starts it on an ephemeral loopback port.
func newServer(t *testing.T) (*webui.Server, *stub.Platform) {
        t.Helper()
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        eng, err := engine.New(engine.Options{
                Platform: plat,
                LogDir:   dir + "/logs",
                Checker:  &fakeNetwork{},
        })
        if err != nil {
                t.Fatalf("engine.New: %v", err)
        }
        srv := webui.New(eng, 0)
        addr, err := srv.Listen()
        if err != nil {
                t.Fatalf("Listen: %v", err)
        }
        go func() { _ = srv.Serve() }()
        t.Cleanup(func() {
                eng.Shutdown()
        })
        t.Logf("webui listening on %s", addr)
        return srv, plat
}

// getJSON fetches a URL and returns body + status.
func getJSON(t *testing.T, url string) (string, int) {
        t.Helper()
        res, err := http.Get(url)
        if err != nil {
                t.Fatalf("GET %s: %v", url, err)
        }
        defer res.Body.Close()
        body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
        if err != nil {
                t.Fatalf("read body: %v", err)
        }
        return string(body), res.StatusCode
}

// postJSON posts to a URL and returns body + status.
func postJSON(t *testing.T, url, body string) (string, int) {
        t.Helper()
        res, err := http.Post(url, "application/json", strings.NewReader(body))
        if err != nil {
                t.Fatalf("POST %s: %v", url, err)
        }
        defer res.Body.Close()
        out, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
        if err != nil {
                t.Fatalf("read body: %v", err)
        }
        return string(out), res.StatusCode
}

func TestStateAndAppInfo(t *testing.T) {
        srv, _ := newServer(t)
        base := "http://" + srv.Addr()

        body, code := getJSON(t, base+"/api/state")
        if code != http.StatusOK {
                t.Fatalf("state status = %d", code)
        }
        var st struct {
                Phase string `json:"phase"`
                Port  int    `json:"port"`
                Mode  string `json:"mode"`
        }
        if err := json.Unmarshal([]byte(body), &st); err != nil {
                t.Fatalf("state json: %v (%s)", err, body)
        }
        if st.Phase != "off" {
                t.Fatalf("initial phase = %q, want off", st.Phase)
        }
        if st.Mode != string(config.DefaultMode) {
                t.Fatalf("mode = %q, want %q", st.Mode, config.DefaultMode)
        }

        body, code = getJSON(t, base+"/api/appinfo")
        if code != http.StatusOK {
                t.Fatalf("appinfo status = %d", code)
        }
        var info struct {
                Name     string `json:"name"`
                Version  string `json:"version"`
                Platform string `json:"platform"`
        }
        if err := json.Unmarshal([]byte(body), &info); err != nil {
                t.Fatalf("appinfo json: %v (%s)", err, body)
        }
        if info.Name != config.AppName {
                t.Fatalf("name = %q", info.Name)
        }
        if info.Platform != "stub" {
                t.Fatalf("platform = %q, want stub", info.Platform)
        }
}

func TestToggleRoundTripAndStubAttach(t *testing.T) {
        srv, plat := newServer(t)
        base := "http://" + srv.Addr()

        if _, code := postJSON(t, base+"/api/toggle", ""); code != http.StatusOK {
                t.Fatalf("toggle status = %d", code)
        }
        // The fake network accepts everything, so the engine reaches "on".
        deadline := time.Now().Add(5 * time.Second)
        var phase string
        for time.Now().Before(deadline) {
                body, _ := getJSON(t, base+"/api/state")
                var st struct {
                        Phase string `json:"phase"`
                }
                if err := json.Unmarshal([]byte(body), &st); err != nil {
                        t.Fatalf("state json: %v", err)
                }
                phase = st.Phase
                if phase == "on" {
                        break
                }
                time.Sleep(25 * time.Millisecond)
        }
        if phase != "on" {
                t.Fatalf("phase after toggle = %q, want on", phase)
        }
        // The stub platform recorded the attach (the system-proxy call the
        // real Linux adapter answers with gsettings).
        if calls := plat.AttachCalls(); len(calls) == 0 {
                t.Fatalf("engine never attached the platform proxy")
        } else if calls[0].Port <= 0 {
                t.Fatalf("attach port = %d", calls[0].Port)
        }

        if _, code := postJSON(t, base+"/api/stop", ""); code != http.StatusOK {
                t.Fatalf("stop status = %d", code)
        }
        deadline = time.Now().Add(5 * time.Second)
        for time.Now().Before(deadline) {
                body, _ := getJSON(t, base+"/api/state")
                var st struct {
                        Phase string `json:"phase"`
                }
                json.Unmarshal([]byte(body), &st)
                if st.Phase == "off" {
                        phase = "off"
                        break
                }
                time.Sleep(25 * time.Millisecond)
        }
        if phase != "off" {
                t.Fatalf("phase after stop = %q, want off", phase)
        }
        if plat.DetachCalls() == 0 {
                t.Fatalf("engine never detached the platform proxy")
        }
}

func TestSettingsRoundTrip(t *testing.T) {
        srv, _ := newServer(t)
        base := "http://" + srv.Addr()

        body, _ := getJSON(t, base+"/api/settings")
        var settings config.Settings
        if err := json.Unmarshal([]byte(body), &settings); err != nil {
                t.Fatalf("settings json: %v", err)
        }
        if settings.Port != config.ListenPort {
                t.Fatalf("default port = %d, want %d", settings.Port, config.ListenPort)
        }

        // Change one thing; the normalizer must echo the whole settings back.
        settings.ConnectOnLaunch = true
        payload, err := json.Marshal(settings)
        if err != nil {
                t.Fatalf("marshal: %v", err)
        }
        out, code := postJSON(t, base+"/api/settings", string(payload))
        if code != http.StatusOK {
                t.Fatalf("save status = %d", code)
        }
        var saved config.Settings
        if err := json.Unmarshal([]byte(out), &saved); err != nil {
                t.Fatalf("saved json: %v (%s)", err, out)
        }
        if !saved.ConnectOnLaunch {
                t.Fatalf("connectOnLaunch not saved")
        }

        // Garbage is rejected as a typed error, not a crash.
        _, code = postJSON(t, base+"/api/settings", "{not json")
        if code != http.StatusBadRequest {
                t.Fatalf("garbage status = %d, want 400", code)
        }
}

func TestSSEStreamCarriesEngineEvents(t *testing.T) {
        srv, _ := newServer(t)
        base := "http://" + srv.Addr()

        res, err := http.Get(base + "/api/events")
        if err != nil {
                t.Fatalf("GET events: %v", err)
        }
        defer res.Body.Close()
        if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
                t.Fatalf("content-type = %q", ct)
        }

        // Read events line by line while toggling the engine on.
        reader := bufio.NewReader(res.Body)
        events := make(chan string, 32)
        go func() {
                for {
                        line, err := reader.ReadString('\n')
                        if err != nil {
                                close(events)
                                return
                        }
                        if strings.HasPrefix(line, "data:") {
                                events <- strings.TrimSpace(strings.TrimPrefix(line, "data:"))
                        }
                }
        }()

        // The first frame is the current state (sent on connect).
        select {
        case data := <-events:
                if !strings.Contains(data, `"phase":"off"`) && !strings.Contains(data, `"phase": "off"`) {
                        t.Fatalf("first SSE frame is not the off state: %s", data)
                }
        case <-time.After(2 * time.Second):
                t.Fatalf("no initial state frame within 2s")
        }

        if _, code := postJSON(t, base+"/api/toggle", ""); code != http.StatusOK {
                t.Fatalf("toggle status = %d", code)
        }
        // Subsequent frames mirror the state machine the engine emits.
        deadline := time.Now().Add(5 * time.Second)
        sawConnecting, sawOn := false, false
        for time.Now().Before(deadline) {
                select {
                case data, ok := <-events:
                        if !ok {
                                t.Fatalf("event stream closed early")
                        }
                        if strings.Contains(data, `"connecting"`) {
                                sawConnecting = true
                        }
                        if strings.Contains(data, `"on"`) {
                                sawOn = true
                        }
                        if sawConnecting && sawOn {
                                return // the stream carries live engine events
                        }
                case <-time.After(100 * time.Millisecond):
                }
        }
        t.Fatalf("stream never carried connecting+on (connecting=%v on=%v)", sawConnecting, sawOn)
}

func TestStaticPlaceholderServes(t *testing.T) {
        srv, _ := newServer(t)
        base := "http://" + srv.Addr()

        // In a source-only checkout the embedded bundle is the committed
        // placeholder note; after a real frontend build (build.sh copies
        // frontend/dist in) it is the app shell. Either way the route must
        // answer 200 from the embedded FS, and the index document must not
        // be cached.
        res, err := http.Get(base + "/")
        if err != nil {
                t.Fatalf("GET /: %v", err)
        }
        defer res.Body.Close()
        if res.StatusCode != http.StatusOK {
                t.Fatalf("GET / status = %d", res.StatusCode)
        }
        body, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
        served := string(body)
        if !strings.Contains(served, "placeholder") && !strings.Contains(served, `<div id="app">`) {
                t.Fatalf("index content = %q, want the placeholder note or the app shell", served)
        }
        if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
                t.Fatalf("index cache-control = %q, want no-store", cc)
        }
}

func TestMethodChecks(t *testing.T) {
        srv, _ := newServer(t)
        base := "http://" + srv.Addr()

        // GET on a POST-only route must be rejected, not silently accepted.
        res, err := http.Get(base + "/api/toggle")
        if err != nil {
                t.Fatalf("GET toggle: %v", err)
        }
        res.Body.Close()
        if res.StatusCode != http.StatusMethodNotAllowed {
                t.Fatalf("GET toggle status = %d, want 405", res.StatusCode)
        }

        // POST on a GET-only route likewise.
        _, code := postJSON(t, base+"/api/state", "")
        if code != http.StatusMethodNotAllowed {
                t.Fatalf("POST state status = %d, want 405", code)
        }
}

func TestLoopbackOnly(t *testing.T) {
        srv, _ := newServer(t)
        addr := srv.Addr()
        if !strings.HasPrefix(addr, "127.0.0.1:") {
                t.Fatalf("bound address %q is not loopback", addr)
        }
}
