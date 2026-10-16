// Package webui serves the Younblock control UI and its JSON API over
// localhost HTTP — the transport of the single-executable Linux edition
// (DECISIONS.md D50). Every engine method the Wails shell binds is
// exposed as a small JSON endpoint with the same payload, and engine
// events are pushed to the page as Server-Sent Events, so the very same
// Svelte bundle runs unchanged: its http adapter (frontend/src/lib/
// adapters/http.ts) talks to these routes.
//
// The server binds the loopback interface only. There is no
// authentication by design: the port answers on this machine alone, for
// the user who launched the program (the same trust boundary as the
// Wails window and the local proxy itself).
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
)

// The built frontend is copied into dist/ by the build script before the
// Go build (go:embed cannot reach outside its own package). The
// committed placeholder keeps the embed valid in a source-only checkout;
// see dist/placeholder.txt and build.sh.
//
//go:embed all:dist
var distFS embed.FS

// settingsBodyLimit caps a POSTed settings payload (the file on disk is
// a few KB; anything larger is a mistake or abuse, not a setting).
const settingsBodyLimit = 1 << 20

// Server is the web control UI for one engine.
type Server struct {
	eng  *engine.Engine
	hub  *hub
	port int
	ln   net.Listener

	mu  sync.Mutex
	srv *http.Server // non-nil while Serve is running
}

// New creates the server for an engine and registers itself as the
// engine's event sink (the webui edition has no other sink; the Wails
// shell registers its own in the desktop build). Port 0 asks the
// operating system for a free port — Addr reports the one chosen.
func New(eng *engine.Engine, port int) *Server {
	s := &Server{eng: eng, hub: newHub(), port: port}
	eng.SetEventSink(s.hub)
	return s
}

// Listen binds the loopback interface and returns the bound address
// (host:port). Serve blocks on it afterwards.
func (s *Server) Listen() (string, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(config.ListenHost, strconv.Itoa(s.port)))
	if err != nil {
		return "", err
	}
	s.ln = ln
	return ln.Addr().String(), nil
}

// Serve runs the HTTP server until the listener is closed. One Serve
// per Server (the Listen → Serve → Close order the embedders use).
func (s *Server) Serve() error {
	srv := &http.Server{Handler: s.handler()}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()
	err := srv.Serve(s.ln)
	s.mu.Lock()
	s.srv = nil
	s.mu.Unlock()
	return err
}

// Addr reports the bound address (empty before Listen).
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// URL prints the address as the page the user opens.
func (s *Server) URL() string {
	return "http://" + s.Addr()
}

// Events returns the server's SSE hub as an engine.EventSink, for
// embedders that fan engine events out to more than one consumer (the
// Android facade serves the page AND forwards events across the bridge;
// added with the M3 mobile work, additive, D71).
func (s *Server) Events() engine.EventSink { return s.hub }

// Close closes the listener and, when Serve is running, also ends every
// established connection — closing the listener alone would leave
// keep-alive pages half-alive, which matters for the Android embedder's
// StopWebUI (added with the M3 mobile work, additive, D71). Safe to call
// repeatedly; Listen may be called again afterwards.
func (s *Server) Close() error {
	s.mu.Lock()
	ln, srv := s.ln, s.srv
	s.ln, s.srv = nil, nil
	s.mu.Unlock()
	var err error
	if ln != nil {
		err = ln.Close()
	}
	if srv != nil {
		_ = srv.Close()
	}
	return err
}

// handler wires every route.
func (s *Server) handler() http.Handler {
	m := http.NewServeMux()

	// The JSON API: same methods and payload shapes the Wails bindings
	// expose (app.go mirrors engine/jsonapi.go exactly). The method
	// guard answers 405 with an Allow header — a plain catch-all "/"
	// pattern for the embedded page would otherwise swallow method
	// mistakes as file-not-found.
	s.get(m, "/api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.GetStateJSON())
	})
	s.post(m, "/api/start", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.StartJSON())
	})
	s.post(m, "/api/stop", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.StopJSON())
	})
	s.post(m, "/api/toggle", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.ToggleJSON())
	})
	// Both settings verbs share one path, so they are registered once
	// with a two-method guard instead of twice (two single-method
	// registrations would conflict on the same plain pattern).
	m.HandleFunc("/api/settings", methodGuardMulti("GET, POST", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.eng.GetSettingsJSON())
		default: // POST
			s.handleSaveSettings(w, r)
		}
	}))
	s.get(m, "/api/appinfo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.GetAppInfoJSON())
	})
	s.post(m, "/api/test", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.RunConnectionTestJSON())
	})
	s.post(m, "/api/autofix", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.AutoFixJSON())
	})
	s.post(m, "/api/restore-proxy", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.RestoreWindowsProxyJSON())
	})
	s.get(m, "/api/report", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.eng.GetDiagnosticReportJSON())
	})
	s.get(m, "/api/logs", s.handleLogs)
	s.post(m, "/api/quit", func(w http.ResponseWriter, r *http.Request) {
		// QuitJSON stops the engine first and only then invokes the
		// shell's onQuit (cmd/core exits there), so the reply reaches
		// this client before the socket goes away.
		writeJSON(w, http.StatusOK, s.eng.QuitJSON())
	})
	s.get(m, "/api/events", s.handleEvents)

	// The embedded control page and its assets.
	m.Handle("/", s.staticHandler())
	return m
}

// get registers a GET-only JSON route.
func (s *Server) get(m *http.ServeMux, path string, h http.HandlerFunc) {
	m.HandleFunc(path, methodGuard(http.MethodGet, h))
}

// post registers a POST-only JSON route.
func (s *Server) post(m *http.ServeMux, path string, h http.HandlerFunc) {
	m.HandleFunc(path, methodGuard(http.MethodPost, h))
}

// methodGuard answers 405 (with Allow) for any other method.
func methodGuard(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeJSON(w, http.StatusMethodNotAllowed, `{"error":"UNKNOWN"}`)
			return
		}
		h(w, r)
	}
}

// methodGuardMulti answers 405 (with a comma-separated Allow) for any
// method outside the allowed list; the handler dispatches itself.
func methodGuardMulti(allow string, h http.HandlerFunc) http.HandlerFunc {
	allowed := make(map[string]bool)
	for _, m := range strings.Split(allow, ",") {
		allowed[strings.TrimSpace(m)] = true
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Method] {
			w.Header().Set("Allow", allow)
			writeJSON(w, http.StatusMethodNotAllowed, `{"error":"UNKNOWN"}`)
			return
		}
		h(w, r)
	}
}

// handleSaveSettings applies a POSTed settings JSON payload.
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, settingsBodyLimit))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, `{"error":"UNKNOWN"}`)
		return
	}
	if !json.Valid(body) {
		writeJSON(w, http.StatusBadRequest, `{"error":"UNKNOWN"}`)
		return
	}
	writeJSON(w, http.StatusOK, s.eng.SaveSettingsJSON(string(body)))
}

// handleLogs returns the log directory path and, when a desktop file
// opener exists, opens it (the Wails shell opens Explorer; xdg-open is
// the freedesktop counterpart — a harmless no-op failure on headless
// boxes, where the path alone is the answer).
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	out := s.eng.OpenLogsFolderJSON()
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err == nil && payload.Path != "" {
		go func(path string) {
			if err := exec.Command("xdg-open", path).Run(); err != nil {
				s.eng.Logger().Warn("opening the logs folder failed", "reason", err.Error())
			}
		}(payload.Path)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleEvents streams engine events to the page as Server-Sent Events
// (the browser side is a plain EventSource). The current state is sent
// first so a reconnecting page re-syncs immediately; slow consumers drop
// events instead of stalling the engine.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	writeSSE(w, "state", s.eng.GetStateJSON())
	flusher.Flush()

	id, ch := s.hub.subscribe()
	defer s.hub.unsubscribe(id)
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			writeSSE(w, ev.name, ev.data)
			flusher.Flush()
		}
	}
}

// staticHandler serves the embedded frontend. The index document is
// never cached, so a replaced binary serves its own fresh bundle instead
// of a previously cached one pointing at assets that no longer exist
// (the asset names are content-hashed; index.html is not).
func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Unreachable: the embed above guarantees the directory exists.
		panic(fmt.Sprintf("webui: embedded dist: %v", err))
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-store")
		}
		fileServer.ServeHTTP(w, r)
	})
}

// writeJSON emits one engine JSON payload verbatim.
func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

// writeSSE frames one event per the SSE wire format. JSON payloads are
// single-line, but a newline inside data would silently split an event,
// so it is guarded anyway (each continuation line gets its own "data:"
// prefix, which the browser re-joins with a newline).
func writeSSE(w io.Writer, name, data string) {
	data = strings.ReplaceAll(data, "\n", "\ndata: ")
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
}

// event is one engine event on its way to a browser.
type event struct {
	name string
	data string
}

// hub fans engine events out to every connected page. It is the engine's
// EventSink (registered by New).
type hub struct {
	mu   sync.Mutex
	next int64
	subs map[int64]chan event
}

func newHub() *hub {
	return &hub{subs: make(map[int64]chan event)}
}

// OnEvent implements engine.EventSink.
func (h *hub) OnEvent(name string, payload string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- event{name: name, data: payload}:
		default: // a slow consumer drops events; the initial state on reconnect recovers
		}
	}
}

// subscribe registers a stream subscriber.
func (h *hub) subscribe() (int64, <-chan event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.next
	h.next++
	ch := make(chan event, 16)
	h.subs[id] = ch
	return id, ch
}

// unsubscribe removes a stream subscriber.
func (h *hub) unsubscribe(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, id)
}
