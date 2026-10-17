package usage_test

import (
        "encoding/json"
        "os"
        "path/filepath"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/usage"
)

func open(t *testing.T, path string) *usage.Store {
        t.Helper()
        s, err := usage.Open(path)
        if err != nil {
                t.Fatalf("Open: %v", err)
        }
        return s
}

func TestStoreMissingFileIsEmpty(t *testing.T) {
        s := open(t, filepath.Join(t.TempDir(), "usage.json"))
        sum := s.Summary(time.Now())
        if sum.SessionCount != 0 || len(sum.Sessions) != 0 || sum.AllTimeBytes != 0 {
                t.Fatalf("fresh store summary = %+v, want empty", sum)
        }
        if _, ok := s.Last(); ok {
                t.Fatalf("Last on a fresh store reported a session")
        }
}

func TestStoreEndClearsRunningAndKeepsNewestFirst(t *testing.T) {
        path := filepath.Join(t.TempDir(), "usage.json")
        s := open(t, path)
        base := time.Now().Add(-time.Hour).UnixMilli()

        s.SaveRunning(usage.SessionRecord{StartMs: base, EndMs: base + 1000, DownBytes: 5, UpBytes: 1}) // an autosave in flight
        s.End(usage.SessionRecord{StartMs: base, EndMs: base + 2000, DownBytes: 100, UpBytes: 10})      // the real end
        s.End(usage.SessionRecord{StartMs: base + 3000, EndMs: base + 4000, DownBytes: 7, UpBytes: 3})

        last, ok := s.Last()
        if !ok || last.DownBytes != 7 {
                t.Fatalf("Last = %+v ok=%v, want the newest (7 bytes down)", last, ok)
        }
        // The finished record must have cleared the running autosave: a
        // reload recovers nothing extra.
        s2 := open(t, path)
        if sum := s2.Summary(time.Now()); sum.SessionCount != 2 {
                t.Fatalf("after reload sessionCount = %d, want 2 (no running resurrected)", sum.SessionCount)
        }
}

func TestStoreCrashRecovery(t *testing.T) {
        path := filepath.Join(t.TempDir(), "usage.json")
        s := open(t, path)
        // A process died mid-session with one autosave on disk.
        start := time.Now().Add(-30 * time.Minute).UnixMilli()
        s.SaveRunning(usage.SessionRecord{StartMs: start, EndMs: time.Now().Add(-15 * time.Second).UnixMilli(), DownBytes: 2_600_000_000, UpBytes: 60_000_000})

        s2 := open(t, path) // the next start recovers it
        last, ok := s2.Last()
        if !ok {
                t.Fatalf("recovered store has no last session")
        }
        if last.DownBytes != 2_600_000_000 || last.EndMs <= last.StartMs {
                t.Fatalf("recovered record = %+v, want the autosave's bytes and a sane end", last)
        }
        b, err := os.ReadFile(path)
        if err != nil {
                t.Fatalf("read file: %v", err)
        }
        var doc struct {
                Running *usage.SessionRecord `json:"running"`
        }
        if err := json.Unmarshal(b, &doc); err != nil {
                t.Fatalf("unmarshal: %v", err)
        }
        if doc.Running != nil {
                t.Fatalf("running record survived the recovery (would double-count on the next start)")
        }
}

func TestStoreSummaryBucketsByEndTime(t *testing.T) {
        s := open(t, filepath.Join(t.TempDir(), "usage.json"))
        now := time.Now()
        mk := func(daysAgo int, down, up int64) usage.SessionRecord {
                end := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
                return usage.SessionRecord{StartMs: end.UnixMilli() - 3_600_000, EndMs: end.UnixMilli(), DownBytes: down, UpBytes: up}
        }
        // Ended oldest-first so the store's newest-first stack matches reality.
        s.End(mk(40, 100_000, 10_000)) // an older month
        s.End(mk(2, 10_000, 1_000))    // this month (unless the test straddles a month boundary: then only all-time — accepted)
        s.End(mk(0, 1_000, 100))       // today, this month

        sum := s.Summary(now)
        if sum.AllTimeBytes != 122_100 {
                t.Fatalf("allTimeBytes = %d, want 122100", sum.AllTimeBytes)
        }
        if sum.TodayBytes != 1_100 {
                t.Fatalf("todayBytes = %d, want 1100", sum.TodayBytes)
        }
        if len(sum.Sessions) != 3 || sum.Sessions[0].DownBytes != 1_000 {
                t.Fatalf("sessions = %+v, want 3 newest-first", sum.Sessions)
        }
        if sum.SessionCount != 3 {
                t.Fatalf("sessionCount = %d, want 3", sum.SessionCount)
        }
}

func TestStoreClear(t *testing.T) {
        path := filepath.Join(t.TempDir(), "usage.json")
        s := open(t, path)
        s.End(usage.SessionRecord{StartMs: 1, EndMs: 2, DownBytes: 3, UpBytes: 4})
        s.Clear()
        if _, ok := s.Last(); ok {
                t.Fatalf("Last after Clear reported a session")
        }
        sum := s.Summary(time.Now())
        if sum.SessionCount != 0 || sum.AllTimeBytes != 0 {
                t.Fatalf("summary after Clear = %+v, want empty", sum)
        }
        // The clear must survive a reload (the menu asks on every open).
        s2 := open(t, path)
        if sum := s2.Summary(time.Now()); sum.SessionCount != 0 {
                t.Fatalf("reloaded summary = %+v, want empty", sum)
        }
}

func TestStoreCapsAt500(t *testing.T) {
        s := open(t, filepath.Join(t.TempDir(), "usage.json"))
        for i := 0; i < 520; i++ {
                s.End(usage.SessionRecord{StartMs: int64(i), EndMs: int64(i) + 1, DownBytes: int64(i), UpBytes: 0})
        }
        sum := s.Summary(time.Now())
        if sum.SessionCount != 500 {
                t.Fatalf("sessionCount = %d, want the 500 cap", sum.SessionCount)
        }
        // Oldest dropped, newest kept.
        if sum.Sessions[0].StartMs != 519 {
                t.Fatalf("newest session startMs = %d, want 519", sum.Sessions[0].StartMs)
        }
        // The kept records are i = 20..519 (the oldest 20 dropped).
        var want int64
        for i := 20; i <= 519; i++ {
                want += int64(i)
        }
        if sum.AllTimeBytes != want {
                t.Fatalf("allTimeBytes = %d, want %d", sum.AllTimeBytes, want)
        }
}

func TestStoreFileHoldsNoHostnames(t *testing.T) {
        path := filepath.Join(t.TempDir(), "usage.json")
        s := open(t, path)
        s.End(usage.SessionRecord{StartMs: 1, EndMs: 2, DownBytes: 3, UpBytes: 4})
        b, err := os.ReadFile(path)
        if err != nil {
                t.Fatalf("read: %v", err)
        }
        var doc map[string]any
        if err := json.Unmarshal(b, &doc); err != nil {
                t.Fatalf("unmarshal: %v", err)
        }
        for _, rec := range doc["sessions"].([]any) {
                m := rec.(map[string]any)
                for key := range m {
                        switch key {
                        case "startMs", "endMs", "downBytes", "upBytes":
                        default:
                                t.Fatalf("session record carries field %q — the history must hold times and byte totals only", key)
                        }
                }
        }
}
