package engine_test

import (
        "encoding/json"
        "os"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/platform/stub"
        "github.com/assadigit/Younblock/internal/usage"
)

// TestUsageSessionLifecycle pins the round-3 rules end to end: a session
// is the engine's "on" run; its bytes are the relay-counter DELTAS (bytes
// that flowed before the session do not leak into it); the finished
// record lands in the history file; and the state carries both the live
// numbers and, after the disconnect, the last session.
func TestUsageSessionLifecycle(t *testing.T) {
        eng, plat, _, _ := newTestEngine(t)
        dir := plat.ConfigDir()

        // Bytes relayed by a PREVIOUS session must not count into this one.
        eng.Stats().AddBytesDown(1000)
        eng.Stats().AddBytesUp(500)

        if err := eng.Start(); err != nil {
                t.Fatalf("Start: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOn)

        st := eng.GetState()
        if st.DownBytes != 0 || st.UpBytes != 0 {
                t.Fatalf("live session bytes = %d/%d, want 0/0 (the baseline was taken at connect)", st.DownBytes, st.UpBytes)
        }
        if st.LastSession != nil {
                t.Fatalf("lastSession while the first session runs = %+v, want none", st.LastSession)
        }

        // The session relays data.
        eng.Stats().AddBytesDown(2_000_000_000)
        eng.Stats().AddBytesUp(60_000_000)

        st = eng.GetState()
        if st.DownBytes != 2_000_000_000 || st.UpBytes != 60_000_000 {
                t.Fatalf("live session bytes = %d/%d, want 2000000000/60000000", st.DownBytes, st.UpBytes)
        }

        // The usage triple the bridge reads agrees with the state.
        down, up, last := eng.Usage()
        if down != st.DownBytes || up != st.UpBytes || last != nil {
                t.Fatalf("Usage() = %d/%d/%+v, want the state's live numbers and no last", down, up, last)
        }

        if err := eng.Stop(); err != nil {
                t.Fatalf("Stop: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOff)

        st = eng.GetState()
        if st.LastSession == nil {
                t.Fatalf("no lastSession after the disconnect")
        }
        if st.LastSession.DownBytes != 2_000_000_000 || st.LastSession.UpBytes != 60_000_000 {
                t.Fatalf("lastSession = %+v, want the session's totals", st.LastSession)
        }
        if st.LastSession.EndMs <= st.LastSession.StartMs {
                t.Fatalf("lastSession times = %d..%d, want a real span", st.LastSession.StartMs, st.LastSession.EndMs)
        }

        // The record is on disk, exactly once, with no hostnames.
        b, err := os.ReadFile(usage.DefaultPath(dir))
        if err != nil {
                t.Fatalf("usage file: %v", err)
        }
        var doc struct {
                Sessions []usage.SessionRecord `json:"sessions"`
                Running  *usage.SessionRecord  `json:"running"`
        }
        if err := json.Unmarshal(b, &doc); err != nil {
                t.Fatalf("usage file json: %v", err)
        }
        if len(doc.Sessions) != 1 || doc.Running != nil {
                t.Fatalf("usage file = %d sessions, running=%v, want 1 and none", len(doc.Sessions), doc.Running)
        }
        if doc.Sessions[0].DownBytes != 2_000_000_000 {
                t.Fatalf("stored record = %+v", doc.Sessions[0])
        }

        // GetUsageJSON serves the summary the menu renders.
        var sum usage.Summary
        if err := json.Unmarshal([]byte(eng.GetUsageJSON()), &sum); err != nil {
                t.Fatalf("GetUsageJSON: %v", err)
        }
        if sum.SessionCount != 1 || sum.AllTimeBytes != 2_060_000_000 {
                t.Fatalf("summary = %+v, want 1 session / 2060000000 bytes", sum)
        }
}

// TestUsageSessionNotCreatedForCanceledConnect pins the round-3 rule "a
// cancelled or failed connection is not a session": pressing Connect and
// cancelling it while the quick check is still running records nothing.
func TestUsageSessionNotCreatedForCanceledConnect(t *testing.T) {
        eng, plat, _, net := newTestEngine(t)
        net.delay = 3 * time.Second // the quick check holds "connecting" open

        if err := eng.Start(); err != nil {
                t.Fatalf("Start: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseConnecting)
        if err := eng.Start(); err != nil { // a second press cancels the run
                t.Fatalf("cancel Start: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOff)

        if _, err := os.Stat(usage.DefaultPath(plat.ConfigDir())); !os.IsNotExist(err) {
                t.Fatalf("usage file exists after a cancelled connect (want none)")
        }
        st := eng.GetState()
        if st.LastSession != nil {
                t.Fatalf("lastSession after a cancelled connect = %+v, want none", st.LastSession)
        }
        var sum struct {
                SessionCount int `json:"sessionCount"`
        }
        if err := json.Unmarshal([]byte(eng.GetUsageJSON()), &sum); err != nil {
                t.Fatalf("GetUsageJSON: %v", err)
        }
        if sum.SessionCount != 0 {
                t.Fatalf("sessionCount = %d, want 0", sum.SessionCount)
        }
}

// TestUsageClearDropsLastSession pins the Clear contract: the history
// empties AND the state's lastSession turns into an explicit null (the
// event the screen needs to drop its "Last session:" line).
func TestUsageClearDropsLastSession(t *testing.T) {
        eng, _, _, _ := newTestEngine(t)

        if err := eng.Start(); err != nil {
                t.Fatalf("Start: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOn)
        eng.Stats().AddBytesDown(500)
        if err := eng.Stop(); err != nil {
                t.Fatalf("Stop: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOff)
        if eng.GetState().LastSession == nil {
                t.Fatalf("no lastSession before the clear")
        }

        if got := eng.ClearUsageJSON(); got != `{"error":null}` {
                t.Fatalf("ClearUsageJSON = %s", got)
        }
        st := eng.GetState()
        if st.LastSession != nil {
                t.Fatalf("lastSession survived the clear: %+v", st.LastSession)
        }
        var sum struct {
                SessionCount int `json:"sessionCount"`
        }
        if err := json.Unmarshal([]byte(eng.GetUsageJSON()), &sum); err != nil {
                t.Fatalf("GetUsageJSON: %v", err)
        }
        if sum.SessionCount != 0 {
                t.Fatalf("sessionCount after clear = %d, want 0", sum.SessionCount)
        }
}

// TestUsageSessionPersistsAcrossEngines pins the recovery promise: a new
// engine (the next app start) reads the previous one's history — the
// session survives the restart through the file, not memory.
func TestUsageSessionPersistsAcrossEngines(t *testing.T) {
        dir := t.TempDir()
        build := func() *engine.Engine {
                t.Helper()
                eng, _, _, _ := newTestEngineIn(t, dir)
                return eng
        }

        eng := build()
        if err := eng.Start(); err != nil {
                t.Fatalf("Start: %v", err)
        }
        waitForPhase(t, eng, engine.PhaseOn)
        eng.Stats().AddBytesDown(1234)
        eng.Shutdown() // quit while the engine owns it

        eng2 := build()
        st := eng2.GetState()
        if st.LastSession == nil || st.LastSession.DownBytes != 1234 {
                t.Fatalf("second engine's lastSession = %+v, want the persisted 1234-byte session", st.LastSession)
        }
}

// newTestEngineIn is newTestEngine with a shared config directory, so a
// second engine reads the first one's files.
func newTestEngineIn(t *testing.T, dir string) (*engine.Engine, *stub.Platform, *recordingSink, *fakeNetwork) {
        t.Helper()
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
