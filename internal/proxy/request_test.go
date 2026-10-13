package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, raw string) *http.Request {
	t.Helper()
	req, err := parseRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("parseRequest(%q): %v", raw, err)
	}
	return req
}

func TestRouteConnect(t *testing.T) {
	req := mustParse(t, "CONNECT www.youtube.com:443 HTTP/1.1\r\nHost: www.youtube.com:443\r\n\r\n")
	rt := decide(req, 4525)
	if rt.kind != routeConnect {
		t.Fatalf("kind = %v, want routeConnect", rt.kind)
	}
	if rt.target != "www.youtube.com:443" {
		t.Errorf("target = %q, want www.youtube.com:443", rt.target)
	}
}

func TestRouteConnectDefaultsPort(t *testing.T) {
	req := mustParse(t, "CONNECT example.com HTTP/1.1\r\nHost: example.com\r\n\r\n")
	rt := decide(req, 4525)
	if rt.kind != routeConnect || rt.target != "example.com:443" {
		t.Errorf("route = %+v, want CONNECT example.com:443", rt)
	}
}

func TestRouteConnectIPv6Bracketed(t *testing.T) {
	req := mustParse(t, "CONNECT [2001:db8::1]:8443 HTTP/1.1\r\nHost: [2001:db8::1]:8443\r\n\r\n")
	rt := decide(req, 4525)
	if rt.kind != routeConnect {
		t.Fatalf("kind = %v, want routeConnect", rt.kind)
	}
	if rt.target != "[2001:db8::1]:8443" {
		t.Errorf("target = %q, want [2001:db8::1]:8443", rt.target)
	}
}

func TestRouteRedirectAbsoluteHTTP(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST"} {
		raw := method + " http://example.com/page?x=1 HTTP/1.1\r\nHost: example.com\r\n\r\n"
		req := mustParse(t, raw)
		rt := decide(req, 4525)
		if rt.kind != routeRedirect {
			t.Errorf("%s: kind = %v, want routeRedirect", method, rt.kind)
			continue
		}
		if rt.https != "https://example.com/page?x=1" {
			t.Errorf("%s: https = %q", method, rt.https)
		}
	}
}

func TestRoutePACAndHealth(t *testing.T) {
	cases := []struct {
		path string
		host string
		kind routeKind
	}{
		{"/proxy.pac", "127.0.0.1:4525", routePAC},
		{"/proxy.pac", "localhost:4525", routePAC},
		{"/health", "127.0.0.1:4525", routeHealth},
		{"/proxy.pac", "example.com:4525", routeNotFound},
		{"/proxy.pac", "127.0.0.1:9999", routeNotFound},
		{"/proxy.pac", "127.0.0.1", routeNotFound},
	}
	for _, tc := range cases {
		raw := "GET " + tc.path + " HTTP/1.1\r\nHost: " + tc.host + "\r\n\r\n"
		req := mustParse(t, raw)
		rt := decide(req, 4525)
		if rt.kind != tc.kind {
			t.Errorf("%s Host %s: kind = %v, want %v", tc.path, tc.host, rt.kind, tc.kind)
		}
	}
}

func TestRouteMethodNotAllowed(t *testing.T) {
	for _, raw := range []string{
		"PUT http://example.com/x HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"DELETE /path HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"GET /other HTTP/1.1\r\nHost: 127.0.0.1:4525\r\n\r\n",
		"GET https://example.com/x HTTP/1.1\r\nHost: example.com\r\n\r\n",
	} {
		req := mustParse(t, raw)
		if rt := decide(req, 4525); rt.kind != routeMethodNotAllowed {
			t.Errorf("%q -> kind %v, want routeMethodNotAllowed", strings.SplitN(raw, " ", 2)[0], rt.kind)
		}
	}
}

func TestParseRequestMalformed(t *testing.T) {
	for _, raw := range []string{
		"\r\n\r\n",
		"NONSENSE\r\n\r\n",
		"GET\r\n\r\n",
		"CONNECT\r\n\r\n",
	} {
		if _, err := parseRequest(bufio.NewReader(strings.NewReader(raw))); err == nil {
			t.Errorf("parseRequest(%q) succeeded, want error", raw)
		}
	}
}

func TestDrainBuffered(t *testing.T) {
	// The request plus 8 extra bytes the client pipelined after the headers.
	raw := "CONNECT example.com:443 HTTP/1.1\r\nHost: x\r\n\r\n12345678"
	br := bufio.NewReader(strings.NewReader(raw))
	if _, err := parseRequest(br); err != nil {
		t.Fatal(err)
	}
	extra := drainBuffered(br)
	if string(extra) != "12345678" {
		t.Errorf("drainBuffered = %q, want 12345678", extra)
	}
}

func TestReadFirstRecordNonTLS(t *testing.T) {
	data := "GET / HTTP/1.1"
	r := strings.NewReader(data)
	first, isTLS, err := readFirstRecord(r)
	if err != nil || isTLS {
		t.Fatalf("isTLS=%v err=%v, want passthrough", isTLS, err)
	}
	// Only the peeked 5 bytes come back; the rest must remain readable for
	// the copy loop on the same connection.
	rest, _ := io.ReadAll(r)
	if string(first)+string(rest) != data {
		t.Errorf("passthrough = %q + %q, want the original %q", first, rest, data)
	}
}

func TestReadFirstRecordTLS(t *testing.T) {
	// A fake record: 0x16 0x03 0x03, length 10, 10 body bytes.
	var buf bytes.Buffer
	buf.Write([]byte{0x16, 0x03, 0x03, 0x00, 0x0A})
	buf.Write([]byte("0123456789"))
	first, isTLS, err := readFirstRecord(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !isTLS {
		t.Fatal("isTLS = false, want true")
	}
	if len(first) != 15 {
		t.Fatalf("len = %d, want 15", len(first))
	}
	if string(first[5:]) != "0123456789" {
		t.Errorf("body = %q", first[5:])
	}
}

func TestReadFirstRecordTLSSplitAcrossReads(t *testing.T) {
	// The same record delivered as two chunks: still one unit.
	var buf bytes.Buffer
	buf.Write([]byte{0x16, 0x03, 0x03, 0x00, 0x0A})
	buf.Write([]byte("01234"))
	buf.Write([]byte("56789"))
	first, isTLS, err := readFirstRecord(&slowReader{r: bytes.NewReader(buf.Bytes()), chunk: 7})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !isTLS || len(first) != 15 {
		t.Fatalf("isTLS=%v len=%d", isTLS, len(first))
	}
}

func TestReadFirstRecordCap(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x16, 0x03, 0x03, 0xFF, 0xFF}) // claims 65535 bytes
	buf.Write(bytes.Repeat([]byte{7}, 32*1024))
	first, isTLS, err := readFirstRecord(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !isTLS {
		t.Fatal("isTLS = false, want true")
	}
	if len(first) != 5+16*1024 {
		t.Fatalf("len = %d, want the 16 KB + 5 cap", len(first))
	}
}

func TestReadFirstRecordEmptyAndPartial(t *testing.T) {
	if _, _, err := readFirstRecord(strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Errorf("empty read err = %v, want io.EOF", err)
	}
	first, _, err := readFirstRecord(strings.NewReader("ab"))
	if err == nil {
		t.Errorf("partial read should error, got %q", first)
	}
}

// slowReader delivers at most chunk bytes per Read, simulating a TCP stream
// that arrives in pieces.
type slowReader struct {
	r     io.Reader
	chunk int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(p) > s.chunk {
		p = p[:s.chunk]
	}
	return s.r.Read(p)
}

func FuzzParseRequest(f *testing.F) {
	seeds := []string{
		"CONNECT www.youtube.com:443 HTTP/1.1\r\nHost: www.youtube.com:443\r\n\r\n",
		"CONNECT example.com HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"CONNECT [2001:db8::1]:8443 HTTP/1.1\r\nHost: [2001:db8::1]:8443\r\n\r\n",
		"GET http://example.com/page HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"POST http://example.com/submit HTTP/1.1\r\nHost: example.com\r\nContent-Length: 3\r\n\r\nabc",
		"HEAD http://example.com/ HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"GET /proxy.pac HTTP/1.1\r\nHost: 127.0.0.1:4525\r\n\r\n",
		"GET /health HTTP/1.1\r\nHost: localhost:4525\r\n\r\n",
		"PUT http://example.com/x HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"OPTIONS * HTTP/1.1\r\nHost: x\r\n\r\n",
		"\r\n\r\n",
		"NONSENSE\r\n\r\n",
		"CONNECT\r\n\r\n",
		"GET\r\n\r\n",
		"CONNECT :443 HTTP/1.1\r\nHost: :443\r\n\r\n",
		"CONNECT host:99999999999999999999 HTTP/1.1\r\nHost: x\r\n\r\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		br := bufio.NewReader(bytes.NewReader(data))
		req, err := parseRequest(br)
		if err != nil {
			return // any clean error is fine; panics fail the fuzz run
		}
		// Exercise the router with whatever parsed.
		_ = decide(req, 4525)
		// Draining must not panic either.
		_ = drainBuffered(br)
	})
}

// The fuzz target must also survive a timeout-free world: ReadRequest on a
// fixed byte slice never blocks, so a 5s watchdog guards against regressions.
func TestFuzzGuardNoHang(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		br := bufio.NewReader(strings.NewReader(strings.Repeat("CONNECT x:443 HTTP/1.1\r\n", 100)))
		parseRequest(br)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("parseRequest hung")
	}
}
