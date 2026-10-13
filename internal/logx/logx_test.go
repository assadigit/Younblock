package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"log/slog"
)

func TestRingKeepsLastN(t *testing.T) {
	r := NewRing(3)
	for i := 0; i < 10; i++ {
		r.Append(string(rune('a' + i)))
	}
	got := strings.Join(r.Lines(), ",")
	if got != "h,i,j" {
		t.Errorf("Lines() = %q, want h,i,j", got)
	}
	r.Clear()
	if r.Len() != 0 || len(r.Lines()) != 0 {
		t.Errorf("Clear() left %d lines", r.Len())
	}
}

func TestLoggerWritesRingAndFile(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir, 10, 1024*1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Info("starting up", "port", 4525)
	l.Warn("something odd")
	if len(l.Ring.Lines()) != 2 {
		t.Errorf("ring has %d lines, want 2", len(l.Ring.Lines()))
	}
	data, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "INFO starting up port=4525") {
		t.Errorf("file line missing: %q", text)
	}
	if !strings.Contains(text, "WARN something odd") {
		t.Errorf("file line missing: %q", text)
	}
}

func TestLogFileRotates(t *testing.T) {
	dir := t.TempDir()
	// Tiny limit so three lines rotate.
	l, err := New(dir, 10, 60, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		l.Info("line with some padding to fill the file", "i", i)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	files := l.LogFiles()
	if len(files) < 2 {
		t.Fatalf("expected rotated files, got %v", files)
	}
	// app.log.1 must exist and be non-empty.
	b1, err := os.ReadFile(filepath.Join(dir, "app.log.1"))
	if err != nil || len(b1) == 0 {
		t.Errorf("app.log.1 missing or empty: %v", err)
	}
	// No more than 3 log files total.
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app.log") {
			count++
		}
	}
	if count > 3 {
		t.Errorf("log file count = %d, want <= 3", count)
	}
}

func TestLoggerWithoutDir(t *testing.T) {
	l, err := New("", 5, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	l.Error("boom")
	if len(l.Ring.Lines()) != 1 {
		t.Fatal("ring missing line")
	}
	if l.LogFiles() != nil {
		t.Errorf("LogFiles should be nil without a dir, got %v", l.LogFiles())
	}
}

func TestRecordCloneIsolation(t *testing.T) {
	// The multi handler must not let one handler's consumption break another.
	dir := t.TempDir()
	l, err := New(dir, 5, 1024*1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	child := l.With("k", "v")
	child.Info("hello")
	if len(l.Ring.Lines()) != 1 {
		t.Fatalf("ring lines = %d, want 1", len(l.Ring.Lines()))
	}
	data, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	if !strings.Contains(string(data), "hello k=v") {
		t.Errorf("attrs lost: %q", string(data))
	}
	_ = slog.LevelInfo
	_ = time.Now
}
