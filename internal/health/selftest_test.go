package health

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/doh"
	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
	"github.com/assadigit/Younblock/internal/proxy"
	"github.com/assadigit/Younblock/internal/stats"

	"golang.org/x/net/dns/dnsmessage"
)

// testEnv assembles fake DoH + TLS web server + proxy for the self-test.
type testEnv struct {
	ca      *x509.CertPool
	tlsCert tls.Certificate
	dohURL  string
	webPort int
	proxy   *proxy.Server
}

func newTestEnv(t *testing.T, answers map[string]string, statusFor func(path string) int) *testEnv {
	t.Helper()
	// CA + one leaf covering everything.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "health test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "health test leaf"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		DNSNames:    []string{"yt.test", "video.test"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, leafKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf := tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}

	// Fake DoH over TLS.
	dohLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dohTLS := tls.NewListener(dohLn, &tls.Config{Certificates: []tls.Certificate{leaf}})
	dohMux := http.NewServeMux()
	dohMux.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
		wire, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(r.URL.Query().Get("dns"))
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
		b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: hdr.ID, Response: true, RCode: dnsmessage.RCodeSuccess})
		b.StartQuestions()
		b.Question(question)
		b.StartAnswers()
		if ip != "" {
			v4 := net.ParseIP(ip).To4()
			b.AResource(dnsmessage.ResourceHeader{
				Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 300,
			}, dnsmessage.AResource{A: [4]byte{v4[0], v4[1], v4[2], v4[3]}})
		}
		msg, _ := b.Finish()
		w.Header().Set("content-type", "application/dns-message")
		w.Write(msg)
	})
	dohSrv := &http.Server{Handler: dohMux}
	go dohSrv.Serve(dohTLS)
	t.Cleanup(func() { dohSrv.Close(); dohLn.Close() })

	// TLS web server answering per-path statuses.
	webLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	webPort := webLn.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := webLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{leaf}})
				tc.SetDeadline(time.Now().Add(10 * time.Second))
				br := bufio.NewReader(tc)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				status := statusFor(req.URL.Path)
				if status == 0 {
					status = http.StatusOK
				}
				fmt.Fprintf(tc, "HTTP/1.1 %d X\r\nContent-Length: 0\r\n\r\n", status)
				tc.CloseWrite()
			}(c)
		}
	}()
	t.Cleanup(func() { webLn.Close() })

	// Resolver + proxy.
	resolver := doh.New(doh.Config{
		Servers: []config.DoHServer{{
			Name: "FakeDoH", URL: "https://" + dohTLS.Addr().String() + "/dns-query?dns={dns}",
			Bootstrap: []string{"127.0.0.1"},
		}},
		Timeout:        3 * time.Second,
		BlockPrivate:   false,
		TLS:            &tls.Config{RootCAs: caPool},
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:            fragment.NewDefaultRNG(),
	})
	srv, err := proxy.Start(proxy.Config{
		Host: "127.0.0.1", Port: 0, AutoPort: true, MaxConns: 32,
		RequestTimeout: 2 * time.Second, FirstRecordTimeout: 2 * time.Second, IdleTimeout: 5 * time.Second,
		MaxHeaderBytes: config.MaxHeaderBytes, Resolver: resolver,
		Dialer:         netx.New(2*time.Second, 4*time.Second, nil),
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:            fragment.NewDefaultRNG(), Stats: stats.New(),
		Domains: config.DefaultDomains(), Mode: config.ModeYouTube,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.StopWithTimeout() })

	return &testEnv{ca: caPool, tlsCert: leaf, dohURL: "https://" + dohTLS.Addr().String(), webPort: webPort, proxy: srv}
}

func TestSelfTestAllStepsPass(t *testing.T) {
	env := newTestEnv(t,
		map[string]string{"yt.test": "127.0.0.1", "video.test": "127.0.0.1"},
		func(path string) int {
			if path == "/generate_204" {
				return http.StatusNoContent
			}
			return http.StatusOK
		})
	var progressed []Step
	steps := RunSelfTest(context.Background(), Config{
		ProxyAddr:   env.proxy.Addr(),
		Resolver:    env.resolver(t),
		PrimaryURL:  fmt.Sprintf("https://yt.test:%d/generate_204", env.webPort),
		FallbackURL: fmt.Sprintf("https://yt.test:%d/robots.txt", env.webPort),
		VideoHost:   fmt.Sprintf("video.test:%d", env.webPort),
		DNSName:     "yt.test",
		VideoTLS:    &tls.Config{RootCAs: env.ca},
		WebTLS:      &tls.Config{RootCAs: env.ca},
		Timeout:     5 * time.Second,
	}, func(s Step) { progressed = append(progressed, s) })
	if len(progressed) != 4 {
		t.Fatalf("progress events = %d, want 4", len(progressed))
	}
	for _, s := range steps {
		if !s.OK {
			t.Errorf("step %s failed: code=%q detail=%q", s.ID, s.Code, s.Detail)
		}
	}
	if steps[1].Detail != "FakeDoH answered." {
		t.Errorf("dns detail = %q, want the server name", steps[1].Detail)
	}
}

func TestSelfTestListenerDown(t *testing.T) {
	steps := RunSelfTest(context.Background(), Config{
		ProxyAddr: "127.0.0.1:1", // nothing there
		Resolver:  nil,
		Timeout:   2 * time.Second,
	}, nil)
	if steps[0].OK || steps[0].Code != "LISTENER_DOWN" {
		t.Errorf("listener step = %+v, want LISTENER_DOWN", steps[0])
	}
	// The later steps also fail when nothing is listening.
	if steps[2].OK {
		t.Errorf("youtube step should fail with the listener down: %+v", steps[2])
	}
	if steps[3].OK {
		t.Errorf("video step should fail with the listener down: %+v", steps[3])
	}
}

func TestSelfTestDNSFailureHint(t *testing.T) {
	// A DoH server that always 500s gives a DNS_UNREACHABLE hint.
	env := newTestEnv(t, map[string]string{}, func(path string) int { return http.StatusNoContent })
	// Replace the DoH handler with a failing one: point the resolver at a
	// closed port instead.
	badResolver := doh.New(doh.Config{
		Servers: []config.DoHServer{{
			Name: "BadDoH", URL: "https://127.0.0.1:9/dns-query?dns={dns}", Bootstrap: []string{"127.0.0.1"},
		}},
		Timeout:        500 * time.Millisecond,
		TLS:            &tls.Config{RootCAs: env.ca, InsecureSkipVerify: true},
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Pause: 0},
		RNG:            fragment.NewDefaultRNG(),
	})
	steps := RunSelfTest(context.Background(), Config{
		ProxyAddr:   env.proxy.Addr(),
		Resolver:    badResolver,
		PrimaryURL:  "https://yt.test:1/x",
		FallbackURL: "https://yt.test:1/y",
		VideoHost:   "video.test:1",
		VideoTLS:    &tls.Config{RootCAs: env.ca},
		Timeout:     2 * time.Second,
	}, nil)
	if steps[0].OK != true {
		t.Errorf("listener step should pass: %+v", steps[0])
	}
	if steps[1].OK || steps[1].Code != "DNS_UNREACHABLE" {
		t.Errorf("dns step = %+v, want DNS_UNREACHABLE", steps[1])
	}
}

func TestSelfTestYouTubeWrongStatus(t *testing.T) {
	env := newTestEnv(t,
		map[string]string{"yt.test": "127.0.0.1"},
		func(path string) int { return http.StatusInternalServerError })
	steps := RunSelfTest(context.Background(), Config{
		ProxyAddr:   env.proxy.Addr(),
		Resolver:    env.resolver(t),
		PrimaryURL:  fmt.Sprintf("https://yt.test:%d/generate_204", env.webPort),
		FallbackURL: fmt.Sprintf("https://yt.test:%d/robots.txt", env.webPort),
		VideoTLS:    &tls.Config{RootCAs: env.ca},
		WebTLS:      &tls.Config{RootCAs: env.ca},
		Timeout:     5 * time.Second,
	}, nil)
	if steps[2].OK {
		t.Fatalf("youtube step should fail on 500: %+v", steps[2])
	}
	if steps[2].Code != "HTTP_500" {
		t.Errorf("youtube code = %q, want HTTP_500", steps[2].Code)
	}
}

func TestSelfTestVideoTLSFailure(t *testing.T) {
	env := newTestEnv(t,
		map[string]string{"yt.test": "127.0.0.1", "video.test": "127.0.0.1"},
		func(path string) int { return http.StatusNoContent })
	// VideoTLS with a bogus pool: the handshake must fail with TLS_ERROR.
	steps := RunSelfTest(context.Background(), Config{
		ProxyAddr:   env.proxy.Addr(),
		Resolver:    env.resolver(t),
		PrimaryURL:  fmt.Sprintf("https://yt.test:%d/generate_204", env.webPort),
		FallbackURL: fmt.Sprintf("https://yt.test:%d/robots.txt", env.webPort),
		VideoHost:   fmt.Sprintf("video.test:%d", env.webPort),
		VideoTLS:    &tls.Config{RootCAs: x509.NewCertPool()},
		WebTLS:      &tls.Config{RootCAs: env.ca},
		Timeout:     5 * time.Second,
	}, nil)
	if steps[3].OK || steps[3].Code != "TLS_ERROR" {
		t.Errorf("video step = %+v, want TLS_ERROR", steps[3])
	}
}

// resolver returns a resolver pointed at the env's fake DoH server.
func (e *testEnv) resolver(t *testing.T) *doh.Resolver {
	t.Helper()
	return doh.New(doh.Config{
		Servers: []config.DoHServer{{
			Name: "FakeDoH", URL: e.dohURL + "/dns-query?dns={dns}", Bootstrap: []string{"127.0.0.1"},
		}},
		Timeout:        3 * time.Second,
		BlockPrivate:   false,
		TLS:            &tls.Config{RootCAs: e.ca},
		FragmentPreset: fragment.Balanced,
		FragmentConfig: fragment.Config{Min: 80, Max: 200, Pause: 0},
		RNG:            fragment.NewDefaultRNG(),
	})
}
