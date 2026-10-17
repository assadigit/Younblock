package webui_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
	"github.com/assadigit/Younblock/internal/webui"
)

// newServerWithEngine is newServer (webui_test.go) with the engine handed
// back too, so a test can drive a real session through the same engine
// the routes serve.
func newServerWithEngine(t *testing.T) (*webui.Server, *engine.Engine, *stub.Platform) {
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
	return srv, eng, plat
}

// TestUsageRoutes pins the round-3 HTTP surface the Android WebView and
// the Linux web edition drive: GET /api/usage serves the summary, the
// state carries the session fields, and POST /api/usage/clear empties
// the history and turns lastSession into an explicit null (the screen's
// Last-session line depends on the difference between null and absent).
func TestUsageRoutes(t *testing.T) {
	srv, eng, _ := newServerWithEngine(t)
	base := "http://" + srv.Addr()

	// Nothing yet: an empty summary, no lastSession in the state.
	body, code := getJSON(t, base+"/api/usage")
	if code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if !strings.Contains(body, `"sessionCount":0`) {
		t.Fatalf("fresh /api/usage = %s, want sessionCount 0", body)
	}
	body, _ = getJSON(t, base+"/api/state")
	if !strings.Contains(body, `"lastSession":null`) {
		t.Fatalf("fresh state = %s, want an explicit lastSession null", body)
	}

	// Run one real session with bytes through the SAME engine the
	// server serves.
	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitEnginePhase(t, eng, engine.PhaseOn)
	eng.Stats().AddBytesDown(1_500_000)
	eng.Stats().AddBytesUp(50_000)
	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitEnginePhase(t, eng, engine.PhaseOff)

	body, _ = getJSON(t, base+"/api/usage")
	if !strings.Contains(body, `"sessionCount":1`) {
		t.Fatalf("usage after a session = %s, want sessionCount 1", body)
	}
	if !strings.Contains(body, `"downBytes":1500000`) {
		t.Fatalf("usage after a session = %s, want the session's downBytes", body)
	}

	body, _ = getJSON(t, base+"/api/state")
	if !strings.Contains(body, `"lastSession":{"startMs"`) {
		t.Fatalf("state after a session = %s, want an embedded lastSession", body)
	}

	body, _ = postJSON(t, base+"/api/usage/clear", "")
	if !strings.Contains(body, `"error":null`) {
		t.Fatalf("clear reply = %s, want error null", body)
	}
	body, _ = getJSON(t, base+"/api/state")
	if !strings.Contains(body, `"lastSession":null`) {
		t.Fatalf("state after clear = %s, want an explicit lastSession null", body)
	}
	body, _ = getJSON(t, base+"/api/usage")
	if !strings.Contains(body, `"sessionCount":0`) {
		t.Fatalf("usage after clear = %s, want sessionCount 0", body)
	}
}

func waitEnginePhase(t *testing.T, eng *engine.Engine, want engine.Phase) engine.State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := eng.GetState()
		if st.Phase == want {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("engine never reached %s", want)
	return engine.State{}
}
