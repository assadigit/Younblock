package probe

// The media-liveness probe's class contract (M8-af, ADR-0044, spec
// W7), pinned by test against local fake origins — the classes the
// network test's table reads:
//
//   1. answered: a TLS server that completes the handshake and
//      answers the deliberately bad GET with an HTTP status line
//      (400 here) — the only certifying class; ANY status line proves
//      the address carries HTTP.
//   2. silent-after-handshake: the handshake completes, the request
//      goes out, no reply inside the budget — the debug.23 corpse
//      shape, now measured directly.
//   3. reset: the connection is refused or killed.
//   4. hello-silent / reset for a dead address: a dial failure never
//      comes back answered (the timeout arm is the blackhole class).
//
// Both-ways proof: with the status-line classifier stashed, test 1
// FAILS (a 400 reply classified as closed-before-reply); with the
// handshake-timeout branch stashed, test 2 FAILS (hello-silent, not
// silent-after-handshake); with the dial classification stashed, test
// 4 FAILS (error, not the dead-address classes).

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/fragment"
)

// testLeafCert issues one throwaway self-signed leaf (no CA chain —
// the probe never verifies; the server just needs A certificate).
func testLeafCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Younblock Probe Test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// probeServerMode selects the fake origin's behavior.
type probeServerMode int

const (
	modeAnswer400            probeServerMode = iota // handshake + "HTTP/1.1 400 Bad Request"
	modeSilentAfterHandshake                        // handshake, then never a reply
	modeResetAfterAccept                            // accept, then RST on close
)

// startProbeServer binds one fake origin in the given mode and
// returns its host:port.
func startProbeServer(t *testing.T, mode probeServerMode) string {
	t.Helper()
	cert := testLeafCert(t)
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}}
	// A RAW listener: the handler wraps each accepted conn in tls.Server
	// itself (tls.Listen would hand back already-wrapped conns and the
	// handler's second wrap would speak TLS-over-TLS — the probe would
	// rightly fail and the test would pin a fantasy).
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
				if mode == modeResetAfterAccept {
					if tc, ok := c.(*net.TCPConn); ok {
						_ = tc.SetLinger(0) // RST on close
					}
					return
				}
				tc := tls.Server(c, tlsCfg)
				if err := tc.Handshake(); err != nil {
					return
				}
				if mode == modeAnswer400 {
					br := bufio.NewReader(tc)
					if _, err := br.ReadString('\n'); err != nil {
						return // the GET never came
					}
					_, _ = tc.Write([]byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n"))
					return
				}
				// modeSilentAfterHandshake: hold the conn open, never reply.
				buf := make([]byte, 512)
				for {
					if _, rerr := tc.Read(buf); rerr != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

// probeAt runs the probe against addr with tight test budgets and the
// balanced preset (the shape does not matter to the classifier).
func probeAt(t *testing.T, addr, sni, path string) MediaResult {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	p := &MediaProbe{
		HandshakeBudget: 2 * time.Second,
		AnswerBudget:    500 * time.Millisecond,
	}
	return p.Run(context.Background(), net.ParseIP("127.0.0.1"), port, sni, path, fragment.Balanced, fragment.PresetConfig(fragment.Balanced))
}

func TestProbeAnsweredOnAnyStatusLine(t *testing.T) {
	addr := startProbeServer(t, modeAnswer400)
	r := probeAt(t, addr, "rr1---sn-test.googlevideo.com", "/videoplayback")
	if r.Class != MediaAnswered {
		t.Fatalf("class = %s (%s), want answered — ANY status line certifies", r.Class, r.Detail)
	}
	if r.Status != "400 Bad Request" {
		t.Fatalf("status = %q, want the 400 Bad Request line", r.Status)
	}
}

func TestProbeSilentAfterHandshake(t *testing.T) {
	addr := startProbeServer(t, modeSilentAfterHandshake)
	r := probeAt(t, addr, "rr1---sn-test.googlevideo.com", "/videoplayback")
	if r.Class != MediaSilentAfterHandshake {
		t.Fatalf("class = %s (%s), want silent-after-handshake — the corpse shape, measured", r.Class, r.Detail)
	}
}

func TestProbeResetOnRefusal(t *testing.T) {
	addr := startProbeServer(t, modeResetAfterAccept)
	r := probeAt(t, addr, "rr1---sn-test.googlevideo.com", "/videoplayback")
	if r.Class != MediaReset {
		t.Fatalf("class = %s (%s), want reset", r.Class, r.Detail)
	}
}

func TestProbeDeadAddressNeverAnswers(t *testing.T) {
	// A bound-then-closed port: nothing listens. The dial either times
	// out (hello-silent — the blackhole class) or is refused (reset);
	// both are honest dead-address verdicts, and answered is
	// impossible.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	ln.Close()
	p := &MediaProbe{HandshakeBudget: 1 * time.Second, AnswerBudget: 500 * time.Millisecond}
	r := p.Run(context.Background(), net.ParseIP("127.0.0.1"), port, "rr1---sn-test.googlevideo.com", "/videoplayback", fragment.Balanced, fragment.PresetConfig(fragment.Balanced))
	if r.Class == MediaAnswered {
		t.Fatalf("class = answered against a dead port — impossible")
	}
	switch r.Class {
	case MediaHelloSilent, MediaReset:
		// both honest dead-address verdicts
	default:
		t.Fatalf("class = %s (%s), want hello-silent or reset for a dead address", r.Class, r.Detail)
	}
}

func TestProbeClassifyNetError(t *testing.T) {
	// The dial-phase classifier's pure pin: timeout -> hello-silent,
	// refusal -> reset, anything else -> error.
	if got := classifyNetError(fmt.Errorf("dial tcp 1.2.3.4:443: i/o timeout")); got != MediaHelloSilent {
		t.Fatalf("timeout dial = %s, want hello-silent", got)
	}
	if got := classifyNetError(fmt.Errorf("dial tcp 1.2.3.4:443: connect: connection refused")); got != MediaReset {
		t.Fatalf("refused dial = %s, want reset", got)
	}
	if got := classifyNetError(fmt.Errorf("dial tcp: weird")); got != MediaError {
		t.Fatalf("weird dial = %s, want error", got)
	}
}
