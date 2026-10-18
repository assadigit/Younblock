package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/doh"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/stats"

	"golang.org/x/net/dns/dnsmessage"
)

// fakeDoHServer answers DoH GET wire-format queries for the given names.
// It returns the base URL (https://127.0.0.1:<port>) served with the given cert.
func fakeDoHServer(t *testing.T, cert tls.Certificate, answers map[string]string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("dns")
		wire, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(q)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var p dnsmessage.Parser
		hdr, err := p.Start(wire)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		question, err := p.Question()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name := strings.TrimSuffix(question.Name.String(), ".")
		ip := answers[name]
		var ar dnsmessage.AResource
		if ip != "" {
			parsed := net.ParseIP(ip).To4()
			copy(ar.A[:], parsed)
		}
		b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
			ID: hdr.ID, Response: true, RCode: dnsmessage.RCodeSuccess,
		})
		b.StartQuestions()
		b.Question(question)
		b.StartAnswers()
		if ip != "" {
			b.AResource(dnsmessage.ResourceHeader{
				Name: question.Name, Type: dnsmessage.TypeA,
				Class: dnsmessage.ClassINET, TTL: 300,
			}, ar)
		}
		msg, err := b.Finish()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "application/dns-message")
		w.Write(msg)
	})
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn := tls.NewListener(rawLn, &tls.Config{Certificates: []tls.Certificate{cert}})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(tlsLn)
	t.Cleanup(func() {
		srv.Close()
		rawLn.Close()
	})
	return "https://" + tlsLn.Addr().String()
}

// echoTLSServer is a real TLS server that answers one HTTP request.
// Its listener records reads so tests can prove the ClientHello arrived
// in several pieces.
func echoTLSServer(t *testing.T, cert tls.Certificate) *recordingListener {
	t.Helper()
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingListener{Listener: rawLn}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	go func() {
		for {
			conn, err := rec.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				tlsConn := tls.Server(c, tlsCfg)
				tlsConn.SetDeadline(time.Now().Add(10 * time.Second))
				br := bufio.NewReader(tlsConn)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				io.Copy(io.Discard, req.Body)
				body := "pong"
				fmt.Fprintf(tlsConn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
				tlsConn.CloseWrite()
			}(conn)
		}
	}()
	t.Cleanup(func() { rawLn.Close() })
	return rec
}

// staticResolver is a fake resolver for behavior tests.
type staticResolver struct {
	ips []net.IP
	err error
}

func (s staticResolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	return s.ips, s.err
}

func newTestServer(t *testing.T, mutate func(*Config)) (*Server, *stats.Counters) {
	t.Helper()
	st := stats.New()
	cfg := Config{
		Host:               "127.0.0.1",
		Port:               0, // any free port
		AutoPort:           true,
		MaxConns:           64,
		RequestTimeout:     2 * time.Second,
		FirstRecordTimeout: 2 * time.Second,
		FirstByteTimeout:   2 * time.Second,
		IdleTimeout:        5 * time.Second,
		MaxHeaderBytes:     config.MaxHeaderBytes,
		Resolver:           staticResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}},
		Dialer:             netx.New(2*time.Second, 4*time.Second, nil),
		FragmentPreset:     fragment.Balanced,
		FragmentConfig:     fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:                fragment.NewDefaultRNG(),
		Stats:              st,
		Domains:            config.DefaultDomains(),
		Mode:               config.ModeYouTube,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	srv, err := Start(cfg)
	if err != nil {
		t.Fatalf("proxy.Start: %v", err)
	}
	t.Cleanup(func() { srv.StopWithTimeout() })
	return srv, st
}

// TestIntegrationHTTPSThroughProxyWithFragmentedClientHello is the Phase 1
// acceptance test: fake DoH + local TLS echo server; an http.Client through
// the proxy completes an HTTPS request to a ROUTED host (youtube mode);
// a recording TCP listener shows the ClientHello arrived in at least 2
// separate reads AND the handshake still succeeded.
func TestIntegrationHTTPSThroughProxyWithFragmentedClientHello(t *testing.T) {
	ca := newTestCA(t)
	// One leaf cert covers the DoH server (IP) and the echo server (name:
	// a routed YouTube host, so youtube mode fragments its ClientHello).
	leaf := ca.serverCert(t, []string{"www.youtube.com"}, []net.IP{net.ParseIP("127.0.0.1")})

	dohSrv := fakeDoHServer(t, leaf, map[string]string{"www.youtube.com": "127.0.0.1"})
	echoLn := echoTLSServer(t, leaf)
	echoPort := echoLn.Addr().(*net.TCPAddr).Port

	resolver := doh.New(doh.Config{
		Servers: []config.DoHServer{{
			Name:      "FakeDoH",
			URL:       dohSrv + "/dns-query?dns={dns}",
			Bootstrap: []string{"127.0.0.1"},
		}},
		Timeout:        3 * time.Second,
		BlockPrivate:   false, // test-only AllowPrivateTargets
		TLS:            &tls.Config{RootCAs: ca.pool},
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:            fragment.NewDefaultRNG(),
	})

	srv, _ := newTestServer(t, func(c *Config) {
		c.Resolver = resolver
		c.FragmentConfig = fragment.Config{Min: 80, Max: 200, Pause: 5 * time.Millisecond}
	})

	// Point the client's https://www.youtube.com at the local echo server:
	// the URL uses a routed host (so the first record is fragmented) and
	// the request goes through our proxy, whose DoH resolves it to
	// 127.0.0.1:<echoPort>.
	url := fmt.Sprintf("https://www.youtube.com:%d/hello", echoPort)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(mustURL(t, "http://"+srv.Addr())),
			TLSClientConfig: &tls.Config{RootCAs: ca.pool, ServerName: "www.youtube.com"},
		},
		Timeout: 15 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("HTTPS through proxy failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if resp.StatusCode != http.StatusOK || string(body) != "pong" {
		t.Fatalf("status %d body %q, want 200 pong", resp.StatusCode, body)
	}

	// The handshake succeeded; now prove the ClientHello arrived in pieces.
	// Wait for the echo connection to be accepted and its pre-write reads
	// to be recorded (they happen during the handshake, before the response).
	var preWrites int64
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rc := echoLn.latest(); rc != nil {
			_, preWrites = rc.stats()
			if preWrites > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if preWrites < 2 {
		t.Errorf("ClientHello arrived in %d separate reads, want >= 2", preWrites)
	}
	if rc := echoLn.latest(); rc != nil {
		if reads, _ := rc.stats(); reads < preWrites {
			t.Errorf("total reads %d < pre-write reads %d (impossible)", reads, preWrites)
		}
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url %q: %v", raw, err)
	}
	return u
}
