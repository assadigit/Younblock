// Package usage stores the per-session data-usage history (the round-3
// feature, feature-spec.md in Ali's mobile.zip): one JSON file per
// installation holding up to 500 finished sessions — times and byte
// totals only, never a website name — plus the last 15-second autosave of
// a running session for crash recovery. Nothing is ever sent anywhere;
// the file lives beside settings.json in the platform's config directory
// (%APPDATA%\Younblock on Windows, the app's private storage on Android).
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionRecord is one finished session. The JSON names are the round-3
// data contract and must match frontend/src/lib/types.ts exactly.
type SessionRecord struct {
	StartMs   int64 `json:"startMs"`
	EndMs     int64 `json:"endMs"`
	DownBytes int64 `json:"downBytes"`
	UpBytes   int64 `json:"upBytes"`
}

// Summary is the GetUsage payload the menu's Data-usage section renders.
type Summary struct {
	// Sessions is newest first, at most 10 entries.
	Sessions []SessionRecord `json:"sessions"`
	// SessionCount is how many records the history keeps (max 500).
	SessionCount int   `json:"sessionCount"`
	TodayBytes   int64 `json:"todayBytes"`
	MonthBytes   int64 `json:"monthBytes"`
	AllTimeBytes int64 `json:"allTimeBytes"`
}

// maxSessions bounds the history (the round-3 decision table: a small
// file, plenty of history).
const maxSessions = 500

// fileDoc is the on-disk document, version 1.
type fileDoc struct {
	Version  int             `json:"version"`
	Sessions []SessionRecord `json:"sessions"` // newest first
	// Running is the last autosave of a session that was still live when
	// the process died; Open recovers it into Sessions at its last saved
	// end time ("saved every 15 s and recovered on the next start").
	Running *SessionRecord `json:"running,omitempty"`
}

// Store is the usage history for one installation. Every method is safe
// for concurrent use; the embedder (the engine) serializes its own calls
// under its mutex, but the store defends itself anyway.
type Store struct {
	mu   sync.Mutex
	path string
	doc  fileDoc
}

// DefaultPath returns the usage-history path for a config directory —
// the single file this package owns.
func DefaultPath(configDir string) string {
	return filepath.Join(configDir, "usage.json")
}

// Open loads (or creates) the store. A running record left by a crashed
// or power-lost process is recovered into the history at its last
// autosave time. A missing file is an empty history, not an error; the
// first save creates it.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.doc = fileDoc{Version: 1}
			return s, nil
		}
		return nil, fmt.Errorf("usage: %w", err)
	}
	if err := json.Unmarshal(b, &s.doc); err != nil {
		return nil, fmt.Errorf("usage: %w", err)
	}
	if s.doc.Version != 1 {
		// An unknown future version is read best-effort as v1.
		s.doc.Version = 1
	}
	if s.doc.Running != nil {
		rec := *s.doc.Running
		s.doc.Running = nil
		if rec.EndMs > rec.StartMs {
			s.doc.Sessions = append([]SessionRecord{rec}, s.doc.Sessions...)
			s.trim()
		}
		// Persist the recovery so a crash before the next change cannot
		// resurrect the stale running record.
		_ = s.persist()
	}
	return s, nil
}

// End records a finished session (newest first) and clears any autosave.
// Called once per disconnect; a cancelled or failed connection never
// reaches here.
func (s *Store) End(rec SessionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.EndMs < rec.StartMs {
		rec.EndMs = rec.StartMs
	}
	s.doc.Running = nil
	s.doc.Sessions = append([]SessionRecord{rec}, s.doc.Sessions...)
	s.trim()
	_ = s.persist()
}

// SaveRunning snapshots a live session for crash recovery. The engine
// calls this every 15 seconds while connected.
func (s *Store) SaveRunning(rec SessionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := rec
	s.doc.Running = &r
	_ = s.persist()
}

// Clear empties the history and the recovery record — the menu's
// confirmed "Delete the whole history". After it, Last reports none and
// the screen's Last-session line disappears with the next state event.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Sessions = nil
	s.doc.Running = nil
	_ = s.persist()
}

// Last returns the most recently finished session.
func (s *Store) Last() (SessionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.doc.Sessions) == 0 {
		return SessionRecord{}, false
	}
	return s.doc.Sessions[0], true
}

// Summary folds the history into the menu's document. Today and This
// month count by session END time in the local calendar (the round-3
// decision table); a still-running session has no end time yet and is
// therefore not part of these totals — its live count is the main
// screen's job, fed from the engine's state.
func (s *Store) Summary(now time.Time) Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := Summary{Sessions: []SessionRecord{}}
	y, m, d := now.Date()
	startOfDay := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	for _, rec := range s.doc.Sessions {
		bytes := rec.DownBytes + rec.UpBytes
		sum.AllTimeBytes += bytes
		end := time.UnixMilli(rec.EndMs)
		if !end.Before(startOfDay) {
			sum.TodayBytes += bytes
		}
		if !end.Before(startOfMonth) {
			sum.MonthBytes += bytes
		}
	}
	n := len(s.doc.Sessions)
	if n > 10 {
		n = 10
	}
	if n > 0 {
		sum.Sessions = append(sum.Sessions, s.doc.Sessions[:n]...)
	}
	sum.SessionCount = len(s.doc.Sessions)
	return sum
}

// trim caps the history at maxSessions, oldest dropped; caller holds mu.
func (s *Store) trim() {
	if len(s.doc.Sessions) > maxSessions {
		s.doc.Sessions = s.doc.Sessions[:maxSessions]
	}
}

// persist writes the document atomically (tmp + rename, the config.Save
// pattern); caller holds mu. A failed write is returned, never panicked:
// the history is a convenience, not a correctness obligation.
func (s *Store) persist() error {
	b, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
