// Package health runs the built-in connection test (file 01, section 10;
// file 03, R12): four steps with plain-language results. It is also what
// `younblock-core.exe --selftest` runs.
package health

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/doh"
)

// Step is one self-test result (JSON shape from file 01, section 10).
type Step struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	MS     int64  `json:"ms"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// Resolver is what the self-test needs from the DoH resolver.
type Resolver interface {
	Lookup(ctx context.Context, host string) ([]net.IP, error)
	LastServerName() string
}

// Config wires the test to its targets; zero values fall back to the
// configuration block, and tests inject local fakes.
type Config struct {
	ProxyAddr   string // host:port of our own proxy
	Resolver    Resolver
	PrimaryURL  string
	FallbackURL string
	VideoHost   string // host:port for the TLS-handshake-only check
	DNSName     string
	// VideoTLS overrides the client TLS config for the video handshake
	// (tests inject a trust pool; nil uses system defaults).
	VideoTLS *tls.Config
	// WebTLS overrides the client TLS config for the YouTube fetch
	// (tests inject a trust pool; nil uses system defaults).
	WebTLS *tls.Config
	// Timeout bounds each network step.
	Timeout time.Duration
}

// RunSelfTest runs the four steps in order, reporting each through onStep
// as it completes (nil onStep is allowed). The returned slice is the final
// result, also used by the diagnostic report.
func RunSelfTest(ctx context.Context, cfg Config, onStep func(Step)) []Step {
	return RunSteps(ctx, cfg, nil, onStep)
}

// RunSteps runs only the requested step ids (in canonical order); nil ids
// means all four. The engine uses it for the quick check (steps 1-3) and
// the heal check (steps 3-4).
func RunSteps(ctx context.Context, cfg Config, ids []string, onStep func(Step)) []Step {
	if cfg.PrimaryURL == "" {
		cfg.PrimaryURL = config.SelfTestURL
	}
	if cfg.FallbackURL == "" {
		cfg.FallbackURL = config.SelfTestFallbackURL
	}
	if cfg.VideoHost == "" {
		cfg.VideoHost = config.SelfTestVideoHost
	}
	if cfg.DNSName == "" {
		cfg.DNSName = config.SelfTestDNSName
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Second
	}
	all := []struct {
		id  string
		run func() Step
	}{
		{"listener", func() Step { return testListener(cfg) }},
		{"dns", func() Step { return testDNS(ctx, cfg) }},
		{"youtube", func() Step { return testYouTube(ctx, cfg) }},
		{"video", func() Step { return testVideo(ctx, cfg) }},
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var steps []Step
	for _, s := range all {
		if ids != nil && !want[s.id] {
			continue
		}
		step := s.run()
		steps = append(steps, step)
		if onStep != nil {
			onStep(step)
		}
	}
	return steps
}

// testListener checks that the local proxy accepts a connection and answers
// /health.
func testListener(cfg Config) Step {
	start := time.Now()
	s := Step{ID: "listener"}
	conn, err := net.DialTimeout("tcp", cfg.ProxyAddr, cfg.Timeout)
	if err != nil {
		s.Code, s.Detail = "LISTENER_DOWN", "The app is not listening."
		s.MS = msSince(start)
		return s
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(cfg.Timeout))
	fmt.Fprintf(conn, "GET /health HTTP/1.1\r\nHost: %s\r\n\r\n", cfg.ProxyAddr)
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil || !strings.Contains(line, "200") {
		s.Code, s.Detail = "LISTENER_DOWN", "The app answered oddly."
		s.MS = msSince(start)
		return s
	}
	s.OK = true
	port := ""
	if _, p, err := net.SplitHostPort(cfg.ProxyAddr); err == nil {
		port = p
	}
	s.Detail = fmt.Sprintf("Listening on port %s.", port)
	s.MS = msSince(start)
	return s
}

// testDNS resolves the YouTube name through the secure-DNS servers.
func testDNS(ctx context.Context, cfg Config) Step {
	start := time.Now()
	s := Step{ID: "dns"}
	if cfg.Resolver == nil {
		s.Code, s.Detail = "DNS_UNREACHABLE", "No resolver configured."
		s.MS = msSince(start)
		return s
	}
	cctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	ips, err := cfg.Resolver.Lookup(cctx, cfg.DNSName)
	if err != nil || len(ips) == 0 {
		switch {
		case errors.Is(err, doh.ErrBlockedAnswer):
			s.Code = "DNS_BLOCKED_ANSWER"
		case errors.Is(err, doh.ErrNoAnswer):
			s.Code = "DNS_NO_ANSWER"
		default:
			s.Code = "DNS_UNREACHABLE"
		}
		s.Detail = "The secure lookup failed."
		s.MS = msSince(start)
		return s
	}
	s.OK = true
	s.Detail = fmt.Sprintf("%s answered.", cfg.Resolver.LastServerName())
	s.MS = msSince(start)
	return s
}

// testYouTube fetches the self-test URL through our own proxy.
func testYouTube(ctx context.Context, cfg Config) Step {
	start := time.Now()
	s := Step{ID: "youtube"}
	proxyURL, err := url.Parse("http://" + cfg.ProxyAddr)
	if err != nil {
		s.Code, s.Detail = "NET_ERROR", "Bad proxy address."
		s.MS = msSince(start)
		return s
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	if cfg.WebTLS != nil {
		transport.TLSClientConfig = cfg.WebTLS.Clone()
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
	// Primary URL expects 204; the fallback expects 200.
	if code, detail := fetchExpect(client, cfg.PrimaryURL, http.StatusNoContent); code == "" {
		s.OK = true
		s.Detail = "YouTube answered."
		s.MS = msSince(start)
		return s
	} else if fcode, _ := fetchExpect(client, cfg.FallbackURL, http.StatusOK); fcode == "" {
		s.OK = true
		s.Detail = "YouTube answered (fallback URL)."
		s.MS = msSince(start)
		return s
	} else {
		// Both failed: report the primary's failure.
		pcode, pdetail := fetchExpect(client, cfg.PrimaryURL, http.StatusNoContent)
		if pcode == "" {
			// The primary recovered between attempts.
			s.OK = true
			s.Detail = "YouTube answered."
			s.MS = msSince(start)
			return s
		}
		s.Code, s.Detail = pcode, pdetail
		_ = detail
		s.MS = msSince(start)
		return s
	}
}

// fetchExpect GETs a URL and returns ("", "") when the status matches.
func fetchExpect(client *http.Client, rawURL string, want int) (code, detail string) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return "NET_ERROR", "The request through the app failed."
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
	if resp.StatusCode != want {
		return fmt.Sprintf("HTTP_%d", resp.StatusCode), fmt.Sprintf("YouTube returned %d.", resp.StatusCode)
	}
	return "", ""
}

// testVideo performs a TLS handshake only, through the proxy, to the video
// host; a completed handshake passes.
func testVideo(ctx context.Context, cfg Config) Step {
	start := time.Now()
	s := Step{ID: "video"}
	host, _, err := net.SplitHostPort(cfg.VideoHost)
	if err != nil {
		host = cfg.VideoHost
	}
	conn, err := net.DialTimeout("tcp", cfg.ProxyAddr, cfg.Timeout)
	if err != nil {
		s.Code, s.Detail = "NET_ERROR", "The app is not listening."
		s.MS = msSince(start)
		return s
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(cfg.Timeout))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", cfg.VideoHost, cfg.VideoHost)
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil || !strings.Contains(line, "200") {
		s.Code, s.Detail = "NET_ERROR", "The tunnel to the video server failed."
		s.MS = msSince(start)
		return s
	}
	for {
		l, rerr := br.ReadString('\n')
		if rerr != nil || strings.TrimSpace(l) == "" {
			break
		}
	}
	tlsCfg := cfg.VideoTLS
	if tlsCfg == nil {
		tlsCfg = &tls.Config{}
	} else {
		tlsCfg = tlsCfg.Clone()
	}
	tlsCfg.ServerName = host
	tlsConn := tls.Client(conn, tlsCfg)
	hsCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	_ = tlsConn.SetDeadline(time.Now().Add(cfg.Timeout))
	if err := tlsConn.HandshakeContext(hsCtx); err != nil {
		s.Code, s.Detail = "TLS_ERROR", "The video server handshake failed."
		s.MS = msSince(start)
		return s
	}
	s.OK = true
	s.Detail = "Video server handshake completed."
	s.MS = msSince(start)
	return s
}

// AllOK reports whether every step passed.
func AllOK(steps []Step) bool {
	for _, s := range steps {
		if !s.OK {
			return false
		}
	}
	return true
}

func msSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
