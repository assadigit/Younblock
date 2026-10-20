// Package probe — the media-liveness probe (M8-af, ADR-0044, spec W7):
// the consultation's §2.3 open-matrix item 1, the one every prior
// round's "verified good" verdict rode on — every origin probe, every
// raced first-byte win, every substitution ever shipped certified a
// TLS HANDSHAKE only. Nothing in the engine had ever asked a
// googlevideo address for actual video bytes.
//
// The probe asks, deliberately badly: dial the candidate address,
// complete a TLS handshake with a REAL googlevideo SNI through the
// same fragmenter a flow uses, then send
//
//	GET /videoplayback HTTP/1.1
//	Host: <sni>
//	Connection: close
//
// and wait one answer budget for ANY HTTP status line. A healthy
// Google video edge answers a bad request with a small error (400/
// 404) — ANY status line proves the address can carry HTTP, and a
// googlevideo address that can carry HTTP can carry video. That is
// the "answered" class, the only one that certifies media capability
// (W6's substitution pool draws from exactly these verdicts).
//
// The classes (spec W7): answered / silent-after-handshake (the TLS
// handshake completed, the HTTP request got no reply — the debug.23
// corpse shape, now measured directly) / hello-silent (no TLS answer
// at all, including a dial timeout — the blackholed-IP class) / reset
// (actively refused or killed) / error (anything else, with the
// reason in Detail).
//
// The probe dials through the engine's one netx dialer (ADR-0011 loop
// safety: the Android protect hook applies), never on a timer — the
// tester presses the button (or the engine's network test runs it as
// one batch), never a lane.
package probe

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/assadigit/Younblock/internal/fragment"
	"github.com/assadigit/Younblock/internal/netx"
)

// MediaClass is one probe's verdict class (spec W7).
type MediaClass string

const (
	// MediaAnswered: an HTTP status line came back — the address can
	// carry HTTP, therefore media. The only certifying class.
	MediaAnswered MediaClass = "answered"
	// MediaSilentAfterHandshake: the handshake completed, the request
	// went out, no reply arrived inside the budget.
	MediaSilentAfterHandshake MediaClass = "silent-after-handshake"
	// MediaHelloSilent: not even a TLS answer — a dial timeout or a
	// handshake that never completes (the blackholed-address class).
	MediaHelloSilent MediaClass = "hello-silent"
	// MediaReset: the connection was actively refused or killed
	// (RST, or a close before any answer).
	MediaReset MediaClass = "reset"
	// MediaError: anything else — the reason rides in Detail.
	MediaError MediaClass = "error"
)

// MediaResult is one probe run's outcome.
type MediaResult struct {
	Class  MediaClass
	Status string // the status line's reason phrase when answered ("400 Bad Request")
	Detail string // the failure's short reason for reset/error
	MS     int64  // the whole probe's wall time
}

// MediaProbe runs media-liveness probes through one dialer.
type MediaProbe struct {
	// Dialer is the engine's one outbound door (protect applies).
	Dialer *netx.Dialer
	// RNG feeds the fragmentation plan (nil → the package default).
	RNG fragment.RNG
	// HandshakeBudget bounds the dial + TLS handshake (0 → 4 s).
	HandshakeBudget time.Duration
	// AnswerBudget bounds the wait for the HTTP status line
	// (0 → 3 s, the spec's number).
	AnswerBudget time.Duration
}

// Run probes one address: TCP dial → fragmented TLS handshake with
// sni ("" sends none — Go omits the extension) and ALPN http/1.1 →
// the deliberately bad GET path → one answer budget for a status
// line. path is the request target ("/videoplayback" for media
// addresses, "/" for the control front).
func (p *MediaProbe) Run(ctx context.Context, ip net.IP, port int, sni, path string, preset fragment.Preset, fcfg fragment.Config) MediaResult {
	start := time.Now()
	hsBudget := p.HandshakeBudget
	if hsBudget <= 0 {
		hsBudget = 4 * time.Second
	}
	ansBudget := p.AnswerBudget
	if ansBudget <= 0 {
		ansBudget = 3 * time.Second
	}
	dialer := p.Dialer
	if dialer == nil {
		dialer = netx.New(5*time.Second, 12*time.Second, nil)
	}

	hsCtx, hsCancel := context.WithTimeout(ctx, hsBudget)
	defer hsCancel()
	conn, err := dialer.DialHost(hsCtx, "tcp", ip.String(), port, []net.IP{ip})
	if err != nil {
		return MediaResult{Class: classifyNetError(err), Detail: shortReason(err), MS: msSince(start)}
	}
	defer conn.Close()

	// The ClientHello rides the same fragmenter a flow uses — the wire
	// shape under test is the flow's own.
	fconn := fragment.NewConn(conn, preset, fcfg, p.RNG)
	tlsCfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true, NextProtos: []string{"http/1.1"}}
	tc := tls.Client(fconn, tlsCfg)
	if err := tc.HandshakeContext(hsCtx); err != nil {
		if isTimeout(err) {
			return MediaResult{Class: MediaHelloSilent, Detail: "handshake", MS: msSince(start)}
		}
		if strings.Contains(err.Error(), "connection reset") {
			return MediaResult{Class: MediaReset, Detail: "handshake", MS: msSince(start)}
		}
		// A server that SPOKE but broke the handshake (an alert after
		// the ServerHello, a bad certificate) is a transport-level
		// anomaly the table should not hide.
		return MediaResult{Class: MediaError, Detail: shortReason(err), MS: msSince(start)}
	}

	// The deliberately bad request. Any status line — even a 400 —
	// proves the address carries HTTP (spec W7).
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, sni)
	if err := tc.SetDeadline(time.Now().Add(ansBudget)); err != nil {
		return MediaResult{Class: MediaError, Detail: "deadline", MS: msSince(start)}
	}
	if _, err := tc.Write([]byte(req)); err != nil {
		if isTimeout(err) {
			return MediaResult{Class: MediaSilentAfterHandshake, Detail: "write", MS: msSince(start)}
		}
		return MediaResult{Class: MediaReset, Detail: "request-write", MS: msSince(start)}
	}
	br := bufio.NewReader(tc)
	line, err := br.ReadString('\n')
	if err != nil {
		if isTimeout(err) {
			return MediaResult{Class: MediaSilentAfterHandshake, Detail: "no-reply", MS: msSince(start)}
		}
		return MediaResult{Class: MediaReset, Detail: "closed-before-reply", MS: msSince(start)}
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) >= 2 && (fields[0] == "HTTP/1.1" || fields[0] == "HTTP/1.0" || fields[0] == "HTTP/2.0") {
		status := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))
		return MediaResult{Class: MediaAnswered, Status: status, MS: msSince(start)}
	}
	return MediaResult{Class: MediaError, Detail: "non-http reply", MS: msSince(start)}
}

// classifyNetError maps a dial-phase failure onto the class set: a
// timeout is the blackholed-address shape (hello-silent — the hello
// never got an answer at any layer), an active refusal is reset,
// anything else is error with its reason.
func classifyNetError(err error) MediaClass {
	if isTimeout(err) {
		return MediaHelloSilent
	}
	if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connection reset") {
		return MediaReset
	}
	return MediaError
}

func shortReason(err error) string {
	msg := err.Error()
	if len(msg) > 60 {
		msg = msg[:60]
	}
	return msg
}

func msSince(start time.Time) int64 { return time.Since(start).Milliseconds() }

func isTimeout(err error) bool {
	type timeoutErr interface{ Timeout() bool }
	if te, ok := err.(timeoutErr); ok && te.Timeout() {
		return true
	}
	return err == context.DeadlineExceeded || strings.Contains(err.Error(), "i/o timeout")
}
