package proxy

// The M8-j per-flow timing lines: every established tunnel logs its
// phases (dns / dial at setup, first-byte mid-flow, dur + byte totals at
// close) into the logger — which the engine feeds into the log ring the
// redacted diagnostics report carries. This is the instrumentation that
// turns a field report like "videos take 30-60s to start" into a
// measurement: the lines name the phase that eats the time. The tests
// pin the format and the privacy rule (no hostnames unless LogHostnames).

import (
        "bufio"
        "fmt"
        "io"
        "net"
        "strconv"
        "strings"
        "sync"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/fragment"
        "github.com/assadigit/Younblock/internal/netx"
        "github.com/assadigit/Younblock/internal/stats"
)

// recLogger captures Info/Warn/Error messages.
type recLogger struct {
        mu   sync.Mutex
        msgs []string
}

func (l *recLogger) Info(msg string, args ...any)  { l.add("INFO", msg, args) }
func (l *recLogger) Warn(msg string, args ...any)  { l.add("WARN", msg, args) }
func (l *recLogger) Error(msg string, args ...any) { l.add("ERROR", msg, args) }

func (l *recLogger) add(level, msg string, args []any) {
        l.mu.Lock()
        defer l.mu.Unlock()
        line := level + " " + msg
        for i := 0; i+1 < len(args); i += 2 {
                line += fmt.Sprintf(" %v=%v", args[i], args[i+1])
        }
        l.msgs = append(l.msgs, line)
}

func (l *recLogger) lines(prefix string) []string {
        l.mu.Lock()
        defer l.mu.Unlock()
        var out []string
        for _, m := range l.msgs {
                if strings.HasPrefix(m, prefix) {
                        out = append(out, m)
                }
        }
        return out
}

// echoPlain starts a plain TCP echo origin (no TLS — the timing lines
// fire before any handshake anyway).
func echoPlain(t *testing.T) int {
        t.Helper()
        ln, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil {
                t.Fatalf("echo listen: %v", err)
        }
        t.Cleanup(func() { ln.Close() })
        go func() {
                for {
                        c, err := ln.Accept()
                        if err != nil {
                                return
                        }
                        go func(c net.Conn) {
                                defer c.Close()
                                io.Copy(c, c)
                        }(c)
                }
        }()
        return ln.Addr().(*net.TCPAddr).Port
}

// dialConnect opens the HTTP door's CONNECT tunnel to host:port and
// returns the established client connection.
func dialConnect(t *testing.T, addr, host string, port int) net.Conn {
        t.Helper()
        c, err := net.DialTimeout("tcp", addr, 3*time.Second)
        if err != nil {
                t.Fatalf("dial door: %v", err)
        }
        req := fmt.Sprintf("CONNECT %s:%d HTTP/1.1\r\nHost: %s:%d\r\n\r\n", host, port, host, port)
        if _, err := c.Write([]byte(req)); err != nil {
                t.Fatalf("write CONNECT: %v", err)
        }
        br := bufio.NewReader(c)
        line, err := br.ReadString('\n')
        if err != nil || !strings.Contains(line, "200") {
                t.Fatalf("CONNECT reply: %q err=%v", line, err)
        }
        for {
                l, rerr := br.ReadString('\n')
                if rerr != nil || strings.TrimSpace(l) == "" {
                        break
                }
        }
        return c
}

func TestFlowTimingLines(t *testing.T) {
        port := echoPlain(t)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
        })

        // One complete exchange: CONNECT, one request up, the echo back,
        // then close so the flow ends and the done line fires.
        c := dialConnect(t, srv.Addr(), "example.test", port)
        if _, err := c.Write([]byte("hello timing")); err != nil {
                t.Fatalf("write: %v", err)
        }
        buf := make([]byte, 64)
        _ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(c, buf[:len("hello timing")]); err != nil {
                t.Fatalf("read echo: %v", err)
        }
        _ = c.Close()

        deadline := time.Now().Add(3 * time.Second)
        for time.Now().Before(deadline) {
                if len(log.lines("INFO tunnel done ")) > 0 {
                        break
                }
                time.Sleep(20 * time.Millisecond)
        }

        ups := log.lines("INFO tunnel up ")
        if len(ups) == 0 {
                t.Fatalf("no 'tunnel up' line logged: %+v", log.lines("INFO "))
        }
        if !strings.Contains(ups[0], "dns=") || !strings.Contains(ups[0], "dial=") {
                t.Fatalf("tunnel up line missing phases: %q", ups[0])
        }
        if strings.Contains(ups[0], "host=") {
                t.Fatalf("hostname leaked without LogHostnames: %q", ups[0])
        }

        fbs := log.lines("INFO tunnel first-byte ")
        if len(fbs) == 0 {
                t.Fatalf("no 'tunnel first-byte' line logged")
        }
        fb := strings.TrimSuffix(strings.TrimPrefix(fbs[0], "INFO tunnel first-byte "), "ms")
        if n, err := strconv.Atoi(fb); err != nil || n < 0 {
                t.Fatalf("first-byte not a non-negative ms count: %q", fbs[0])
        }

        dones := log.lines("INFO tunnel done ")
        if len(dones) == 0 {
                t.Fatalf("no 'tunnel done' line logged (flow close not observed)")
        }
        d := dones[0]
        for _, want := range []string{"dur=", "up=", "down="} {
                if !strings.Contains(d, want) {
                        t.Fatalf("tunnel done missing %s: %q", want, d)
                }
        }
        if !strings.Contains(d, "12 B") {
                t.Fatalf("tunnel done byte totals wrong (want 12 B — \"hello timing\" is 12 bytes): %q", d)
        }
}

func TestFlowTimingHostnamesGated(t *testing.T) {
        port := echoPlain(t)
        log := &recLogger{}
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = log
                c.LogHostnames = true
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
        })

        c := dialConnect(t, srv.Addr(), "host.example.test", port)
        _, _ = c.Write([]byte("x"))
        _ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
        _, _ = c.Read(make([]byte, 16))
        _ = c.Close()

        deadline := time.Now().Add(3 * time.Second)
        for time.Now().Before(deadline) {
                if len(log.lines("INFO tunnel up ")) > 0 {
                        break
                }
                time.Sleep(20 * time.Millisecond)
        }
        ups := log.lines("INFO tunnel up ")
        if len(ups) == 0 {
                t.Fatalf("no 'tunnel up' line logged")
        }
        if !strings.Contains(ups[0], "host=host.example.test") {
                t.Fatalf("hostname missing with LogHostnames on: %q", ups[0])
        }
}

// TestFlowTimingNoLoggerNilSafe pins the nil-Logger guard: the lines are
// best-effort instrumentation and must never break a flow.
func TestFlowTimingNoLoggerNilSafe(t *testing.T) {
        port := echoPlain(t)
        srv, _ := newTestServer(t, func(c *Config) {
                c.Logger = nil
                c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}
        })
        c := dialConnect(t, srv.Addr(), "example.test", port)
        if _, err := c.Write([]byte("nil logger")); err != nil {
                t.Fatalf("write: %v", err)
        }
        _ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
        if _, err := io.ReadFull(c, make([]byte, len("nil logger"))); err != nil {
                t.Fatalf("read echo: %v", err)
        }
        _ = c.Close()
}

// Compile-time guard that the unused-import set stays honest (the file
// deliberately mirrors newTestServer's dependency set).
var _ = stats.New
var _ = netx.New
var _ = fragment.Balanced
var _ = config.MaxHeaderBytes
