package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/stats"
)

// rawTunnel opens a CONNECT tunnel through the proxy and returns the client
// connection after reading the 200 reply.
func rawTunnel(t *testing.T, addr, target string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n")); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		t.Fatalf("read connect reply: %v", err)
	}
	if !strings.Contains(line, "200") {
		conn.Close()
		rest, _ := io.ReadAll(br)
		t.Fatalf("connect reply = %q %q, want 200", line, rest)
	}
	// Drain the rest of the 200 reply headers.
	for {
		l, err := br.ReadString('\n')
		if err != nil || strings.TrimSpace(l) == "" {
			break
		}
	}
	conn.SetDeadline(time.Time{})
	return conn
}

// silentServer accepts connections and never sends anything.
func silentServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open, silently.
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

// echoServer echoes exactly what it receives back until the peer
// half-closes; then it closes.
func echoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
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
				buf := make([]byte, 4096)
				for {
					n, rerr := c.Read(buf)
					if n > 0 {
						if _, werr := c.Write(buf[:n]); werr != nil {
							return
						}
					}
					if rerr != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

// slowForeverServer writes one byte every 50 ms forever.
func slowForeverServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
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
				buf := make([]byte, 1)
				for {
					if _, err := c.Write(buf); err != nil {
						return
					}
					time.Sleep(50 * time.Millisecond)
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

func tlsLikeRecord(payloadLen int) []byte {
	out := make([]byte, 5+payloadLen)
	out[0], out[1], out[2] = 0x16, 0x03, 0x03
	out[3] = byte(payloadLen >> 8)
	out[4] = byte(payloadLen)
	for i := 5; i < len(out); i++ {
		out[i] = byte(i)
	}
	return out
}

func TestIdleTimeoutClosesSilentTunnel(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, _ := newTestServer(t, func(c *Config) {
		c.IdleTimeout = 400 * time.Millisecond
		c.FirstRecordTimeout = 400 * time.Millisecond
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	// Point the tunnel at the silent server's port.
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%s", upPort))
	defer conn.Close()
	// Send a complete first record so the tunnel reaches the copy phase,
	// then go silent: the idle timeout must close the tunnel.
	if _, err := conn.Write(tlsLikeRecord(64)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	for {
		if _, err := conn.Read(buf); err != nil {
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || isTimeoutErr(err) {
				return // tunnel closed: pass
			}
			t.Fatalf("unexpected read error: %v", err)
		}
	}
}

func TestFirstRecordTimeoutClosesClientThatNeverSpeaks(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, _ := newTestServer(t, func(c *Config) {
		c.FirstRecordTimeout = 300 * time.Millisecond
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%s", upPort))
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected the tunnel to close when the client never speaks")
	}
}

func TestConnectionCapReturns503(t *testing.T) {
	up := silentServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, _ := newTestServer(t, func(c *Config) {
		c.MaxConns = 2
		c.IdleTimeout = 30 * time.Second
		c.FirstRecordTimeout = 30 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	target := fmt.Sprintf("example.test:%s", upPort)
	// Two live tunnels fill the cap.
	c1 := rawTunnel(t, srv.Addr(), target)
	defer c1.Close()
	c2 := rawTunnel(t, srv.Addr(), target)
	defer c2.Close()

	// The third connection must get a clean 503.
	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read 503 line: %v", err)
	}
	if !strings.Contains(line, "503") {
		t.Fatalf("third connection reply = %q, want 503", line)
	}
}

func TestStopReturnsQuicklyWithOpenTunnels(t *testing.T) {
	up := slowForeverServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, _ := newTestServer(t, func(c *Config) {
		c.IdleTimeout = 60 * time.Second
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%s", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(32)); err != nil {
		t.Fatal(err)
	}
	// Let some traffic flow.
	time.Sleep(150 * time.Millisecond)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Stop(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
			t.Errorf("Stop took %v, want at most ~1.5s with a 1s budget", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return")
	}
	// The tunnel's client socket must be closed by the force-close; any
	// in-flight bytes drain first, so read until the socket errors.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 64)
	closed := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := conn.Read(buf); err != nil {
			closed = true
			break
		}
	}
	if !closed {
		t.Fatal("expected the open tunnel to be force-closed")
	}
}

func TestByteCountersMatchTransferredBytes(t *testing.T) {
	up := echoServer(t)
	upHost, upPort, _ := net.SplitHostPort(up)
	srv, st := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP(upHost)}}
	})
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%s", upPort))
	defer conn.Close()

	// 100 payload bytes: a 40-byte TLS-looking first record plus 60 raw.
	record := tlsLikeRecord(35) // 5 + 35 = 40 bytes
	payload := append(record, make([]byte, 60)...)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatal("echo mismatch")
	}
	closeWrite(conn)

	// Wait for the counters to settle.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st.Diagnostic().BytesUp >= int64(len(payload)) && st.Diagnostic().BytesDown >= int64(len(payload)) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	d := st.Diagnostic()
	if d.BytesUp != int64(len(payload)) {
		t.Errorf("bytesUp = %d, want exactly %d", d.BytesUp, len(payload))
	}
	if d.BytesDown != int64(len(payload)) {
		t.Errorf("bytesDown = %d, want exactly %d", d.BytesDown, len(payload))
	}
	if d.ConnsTotal < 1 {
		t.Errorf("connsTotal = %d, want >= 1", d.ConnsTotal)
	}
}

func TestProxyServesPACAndHealth(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	get := func(path string) (int, string) {
		conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\n\r\n", path, srv.Addr())
		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return resp.StatusCode, string(body)
	}
	code, body := get("/proxy.pac")
	if code != 200 || !strings.Contains(body, "FindProxyForURL") {
		t.Errorf("GET /proxy.pac -> %d %.80s", code, body)
	}
	code, body = get("/health")
	if code != 200 || body != "ok" {
		t.Errorf("GET /health -> %d %q", code, body)
	}
}

func TestProxyRedirectsPlainHTTP(t *testing.T) {
	// Routed hosts (the YouTube list) keep the http->https redirect.
	srv, _ := newTestServer(t, nil)
	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(conn, "GET http://www.youtube.com/page HTTP/1.1\r\nHost: www.youtube.com\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 302 {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://www.youtube.com/page" {
		t.Fatalf("Location = %q", loc)
	}
}

// httpOrigin is a plain-http origin that records the request it served.
type httpOrigin struct {
	addr string
	got  atomic.Value // *http.Request snapshot
}

func newHTTPOrigin(t *testing.T) *httpOrigin {
	t.Helper()
	o := &httpOrigin{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	o.addr = ln.Addr().String()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := *r
		o.got.Store(&snap)
		w.Header().Set("X-Origin", "yes")
		fmt.Fprint(w, "origin-response")
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return o
}

// TestProxyForwardsPlainHTTPForOtherHosts: in youtube mode the manual
// system proxy sends every host here, so a plain-http request to a host
// outside the routed list must be forwarded like a normal proxy (not
// redirected, not dropped) — plain-http sites keep working (D46).
func TestProxyForwardsPlainHTTPForOtherHosts(t *testing.T) {
	origin := newHTTPOrigin(t)
	srv, _ := newTestServer(t, nil) // youtube mode, static resolver -> 127.0.0.1
	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "GET http://plain.test:%s/path?q=1 HTTP/1.1\r\nHost: plain.test:%s\r\nAccept: text/plain\r\n\r\n",
		origin.addr[strings.Index(origin.addr, ":")+1:], origin.addr[strings.Index(origin.addr, ":")+1:])
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 || string(body) != "origin-response" {
		t.Fatalf("status %d body %q, want 200 origin-response", resp.StatusCode, body)
	}
	if resp.Header.Get("X-Origin") != "yes" {
		t.Fatal("response headers did not come from the origin")
	}
	req, ok := origin.got.Load().(*http.Request)
	if !ok {
		t.Fatal("origin saw no request")
	}
	// The forwarded request must be origin-form with the right host.
	if req.Host != "plain.test:"+origin.addr[strings.Index(origin.addr, ":")+1:] {
		t.Errorf("origin saw Host %q", req.Host)
	}
	if req.URL.String() != "/path?q=1" {
		t.Errorf("origin saw URL %q, want origin-form /path?q=1", req.URL.String())
	}
	if req.Method != "GET" || req.Header.Get("Accept") != "text/plain" {
		t.Errorf("origin saw %s %v", req.Method, req.Header)
	}
	if req.Header.Get("Connection") != "" && req.Header.Get("Connection") != "close" {
		t.Errorf("hop-by-hop Connection header leaked: %q", req.Header.Get("Connection"))
	}
	if req.Header.Get("Proxy-Connection") != "" {
		t.Error("Proxy-Connection header leaked to the origin")
	}
}

// TestYouTubeModeDoesNotFragmentUnroutedHosts: with the manual system proxy
// every host's traffic passes through, but only the routed hosts get the
// fragmented first TLS record; anything else relays untouched (D46).
func TestYouTubeModeDoesNotFragmentUnroutedHosts(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingListener{Listener: rawLn}
	upPort := rawLn.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	t.Cleanup(func() { rawLn.Close() })

	srv, _ := newTestServer(t, nil) // youtube mode; example.test is unrouted
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%d", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(400)); err != nil {
		t.Fatal(err)
	}
	// Wait for the record to arrive, then count the reads.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rc := rec.latest(); rc != nil {
			if reads, _ := rc.stats(); reads >= 1 {
				time.Sleep(150 * time.Millisecond) // let any further pieces land
				reads, _ = rc.stats()
				if reads != 1 {
					t.Fatalf("unrouted host's first record arrived in %d reads, want 1 (unfragmented)", reads)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("upstream saw no data")
}

// TestAllModeFragmentsEveryHost: route-all mode fragments the first TLS
// record for every host.
func TestAllModeFragmentsEveryHost(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingListener{Listener: rawLn}
	upPort := rawLn.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				io.Copy(io.Discard, c)
			}(c)
		}
	}()
	t.Cleanup(func() { rawLn.Close() })

	srv, _ := newTestServer(t, func(c *Config) {
		c.Mode = config.ModeAll
		c.FragmentConfig.Min = 80
		c.FragmentConfig.Max = 120
		c.FragmentConfig.Pause = 5 * time.Millisecond
	})
	conn := rawTunnel(t, srv.Addr(), fmt.Sprintf("example.test:%d", upPort))
	defer conn.Close()
	if _, err := conn.Write(tlsLikeRecord(400)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rc := rec.latest(); rc != nil {
			if reads, _ := rc.stats(); reads >= 2 {
				return // fragmented into several reads
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	rc := rec.latest()
	if rc == nil {
		t.Fatal("upstream saw no data")
	}
	reads, _ := rc.stats()
	t.Fatalf("routed host's first record arrived in %d reads, want >= 2 (fragmented)", reads)
}

func TestProxyAnswers405ForOtherMethods(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(conn, "DELETE http://example.com/x HTTP/1.1\r\nHost: example.com\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 405 {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestProxy502WhenDialFails(t *testing.T) {
	// A resolver that "resolves" to an unreachable TEST-NET address.
	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = staticResolver{ips: []net.IP{net.ParseIP("192.0.2.1")}}
		c.Dialer = newSlowDialer()
	})
	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "CONNECT down.test:443 HTTP/1.1\r\nHost: down.test\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 502 {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

func TestAutoPortPicksAnotherPort(t *testing.T) {
	// Occupy the configured port.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	srv, _ := newTestServer(t, func(c *Config) {
		c.Port = port
		c.AutoPort = true
	})
	if srv.Port() == port {
		t.Fatalf("proxy kept the busy port %d", port)
	}
	if srv.Port() < 1 {
		t.Fatal("no port chosen")
	}
}

func TestNoAutoPortReturnsPortBusy(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	st := newStats()
	cfg := Config{
		Host: "127.0.0.1", Port: port, AutoPort: false,
		Stats:  st,
		Dialer: newSlowDialer(),
	}
	_, err = Start(cfg)
	if !errors.Is(err, ErrPortBusy) {
		t.Fatalf("Start with busy port: %v, want ErrPortBusy", err)
	}
}

func TestStartStopRepeatedly(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	for i := 0; i < 3; i++ {
		if err := srv.StopWithTimeout(); err != nil {
			t.Fatalf("Stop %d: %v", i, err)
		}
		if err := srv.Start(); err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}
	}
	// Concurrent Stop calls must be safe.
	var errs atomic.Int32
	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go func() {
			if err := srv.StopWithTimeout(); err != nil {
				errs.Add(1)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	if errs.Load() != 0 {
		t.Fatalf("concurrent Stops errored")
	}
}

func isTimeoutErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func newSlowDialer() *netx.Dialer {
	return netx.New(500*time.Millisecond, 1500*time.Millisecond, nil)
}

func newStats() *stats.Counters { return stats.New() }
