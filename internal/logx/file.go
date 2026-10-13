package logx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log/slog"
)

// fileWriter appends to <dir>/app.log and rotates when the file grows past
// maxBytes: app.log -> app.log.1 -> app.log.2 ... keeping (files-1) backups.
type fileWriter struct {
	mu       sync.Mutex
	dir      string
	path     string
	maxBytes int64
	files    int
	f        *os.File
	size     int64
}

func newFileWriter(dir string, maxBytes int64, files int) (*fileWriter, error) {
	if files < 1 {
		files = 1
	}
	w := &fileWriter{
		dir:      dir,
		path:     filepath.Join(dir, "app.log"),
		maxBytes: maxBytes,
		files:    files,
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	w.f = f
	w.size = st.Size()
	return w, nil
}

func (w *fileWriter) rotateLocked() {
	if w.f != nil {
		w.f.Close()
		w.f = nil
	}
	// Shift backups: .2 -> deleted, .1 -> .2, app.log -> .1 (for files=3).
	for i := w.files - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", w.path, i-1)
		to := fmt.Sprintf("%s.%d", w.path, i)
		if i-1 == 0 {
			from = w.path
		}
		if _, err := os.Stat(from); err == nil {
			os.Rename(from, to)
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	w.f = f
	w.size = 0
}

func (w *fileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		// Reopen (rotation may have failed earlier); if that fails too,
		// drop the line rather than crash the app over logging.
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return len(p), nil
		}
		w.f = f
		w.size = 0
	}
	if w.size+int64(len(p)) > w.maxBytes {
		w.rotateLocked()
	}
	if w.f == nil {
		return len(p), nil
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (w *fileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// textHandler is a minimal slog.Handler writing compact single lines:
// 2006-01-02T15:04:05.000Z INFO message key=value
type textHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	level  slog.Level
	attrs  []slog.Attr
	groups string
}

func (h *textHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *textHandler) Handle(_ context.Context, r slog.Record) error {
	var buf []byte
	buf = r.Time.AppendFormat(buf, "2006-01-02T15:04:05.000Z07:00")
	buf = append(buf, ' ')
	buf = append(buf, r.Level.String()...)
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)
	for _, a := range h.attrs {
		buf = appendAttr(buf, h.groups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		buf = appendAttr(buf, h.groups, a)
		return true
	})
	buf = append(buf, '\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func appendAttr(buf []byte, prefix string, a slog.Attr) []byte {
	buf = append(buf, ' ')
	if prefix != "" {
		buf = append(buf, prefix...)
		buf = append(buf, '.')
	}
	buf = append(buf, a.Key...)
	buf = append(buf, '=')
	return append(buf, fmt.Sprintf("%v", a.Value.Any())...)
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	if clone.groups == "" {
		clone.groups = name
	} else {
		clone.groups = clone.groups + "." + name
	}
	return &clone
}

// multiHandler fans records out to several handlers.
type multiHandler struct {
	hs []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m.hs {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.hs {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{hs: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{hs: hs}
}

// ringHandler appends formatted lines to the ring.
type ringHandler struct {
	ring  *Ring
	level slog.Level
}

func (h *ringHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *ringHandler) Handle(_ context.Context, r slog.Record) error {
	var buf []byte
	buf = r.Time.AppendFormat(buf, "2006-01-02T15:04:05.000")
	buf = append(buf, ' ')
	buf = append(buf, r.Level.String()...)
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)
	buf = append(buf, '\n')
	h.ring.Append(string(buf))
	return nil
}

func (h *ringHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *ringHandler) WithGroup(string) slog.Handler      { return h }

// Logger bundles the ring and the rotating file behind one *slog.Logger.
type Logger struct {
	*slog.Logger
	Ring *Ring
	file *fileWriter
}

// New creates the app logger: level Info, an in-memory ring of ringLines
// lines and a rotating app.log inside logDir. logDir may be empty to keep
// only the ring (used in tests).
func New(logDir string, ringLines, maxBytes, files int) (*Logger, error) {
	ring := NewRing(ringLines)
	handlers := []slog.Handler{&ringHandler{ring: ring, level: slog.LevelInfo}}
	var fw *fileWriter
	if logDir != "" {
		var err error
		fw, err = newFileWriter(logDir, int64(maxBytes), files)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, &textHandler{
			mu:    &sync.Mutex{},
			w:     fw,
			level: slog.LevelInfo,
		})
	}
	return &Logger{
		Logger: slog.New(&multiHandler{hs: handlers}),
		Ring:   ring,
		file:   fw,
	}, nil
}

// Close closes the log file (the ring stays readable).
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

// LogFiles returns the log file paths that currently exist.
func (l *Logger) LogFiles() []string {
	if l.file == nil {
		return nil
	}
	var out []string
	for i := 0; i < l.file.files; i++ {
		p := l.file.path
		if i > 0 {
			p = fmt.Sprintf("%s.%d", l.file.path, i)
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// Now is the clock used by tests; production uses time.Now.
var Now = time.Now
