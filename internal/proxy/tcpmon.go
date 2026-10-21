// Package proxy — the debug.24 per-flow evidence holders (M8-af,
// ADR-0043, spec W1/W2/W3): the debug.23 consultation's §2.4 said it
// exactly — "we currently cannot distinguish 'handshake completed,
// zero application data' from 'small HTTP error response' in those
// 2.8 KB flows … a pure instrumentation gap". This file is the
// instrument:
//
//   - tcpEvidence (W1): two kernel TCP_INFO snapshots of the origin
//     socket — A right after the client's request bytes were first
//     written to the origin, B when the flow ends — plus the verdict
//     classification (upstream-lost / downstream-lost / both-ok /
//     unknown) the report aggregates. No packet capture: the kernel
//     already counted what was ACKed and what came back.
//   - recordTracker (W2): a light 5-byte TLS record header parser over
//     the RELAYED bytes (no decryption) — the first 12 server→client
//     and 8 client→server records as type:length@ms, and the
//     first-app-byte timing (the first server record after the
//     client's first post-handshake record).
//   - gvideoEvidence (W3): one googlevideo flow's combined evidence —
//     the tcp pair, both record trackers, the first-request stamp and
//     the winner class (orig|subst|race) — rendered onto the flow's
//     done/stall lines.
//
// Everything here is nil-safe: a non-googlevideo flow carries a nil
// *gvideoEvidence and every method degrades to a no-op, so desktop and
// non-gvideo behavior stays byte-for-byte what it was.
package proxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/assadigit/Younblock/internal/netx"
)

// tcpUnavailOnce latches the first TCP_INFO read failure into one log
// line (spec W1: "log tcpinfo=unavailable reason=… once and
// continue") — a device where the read fails prints it exactly once,
// then every flow's evidence simply stays absent (the report counts
// the flows as unknown, never a guess).
var tcpUnavailOnce sync.Once

// tcpEvidence is one flow's kernel-counter pair (spec W1). onFail
// (when non-nil) receives the first read failure's reason — the
// server turns it into the once-only device-level log line.
type tcpEvidence struct {
	mu     sync.Mutex
	origin net.Conn // the raw origin socket snapshots read from
	a, b   netx.TCPInfo
	haveA  bool
	haveB  bool
	onFail func(string)
}

func newTCPEvidence(onFail func(string)) *tcpEvidence {
	return &tcpEvidence{onFail: onFail}
}

// takeA records snapshot A — called at the exact moment the client's
// first post-handshake bytes were just written to the origin (the
// request write; its ACK has not landed yet, so A is the pre-request
// baseline). Once-only by construction; the first caller also installs
// the origin the B snapshot will read.
func (e *tcpEvidence) takeA(c net.Conn) {
	if e == nil {
		return
	}
	e.mu.Lock()
	if e.haveA {
		e.mu.Unlock()
		return
	}
	origin := e.origin
	e.mu.Unlock()
	if c != nil {
		origin = c
	}
	info, err := netx.SnapshotTCPInfo(origin)
	e.mu.Lock()
	if e.haveA { // a racing taker won
		e.mu.Unlock()
		return
	}
	if err != nil {
		fail := e.onFail
		e.mu.Unlock()
		if fail != nil {
			fail(err.Error()) // the once-only device line (spec W1)
		}
		return
	}
	e.origin = origin
	e.a = info
	e.haveA = true
	e.mu.Unlock()
}

// noteOrigin installs the flow's winning origin socket (the relay's
// entry) — the B snapshot reads from it when no earlier taker could.
func (e *tcpEvidence) noteOrigin(c net.Conn) {
	if e == nil || c == nil {
		return
	}
	e.mu.Lock()
	if e.origin == nil {
		e.origin = c
	}
	e.mu.Unlock()
}

// originIP reports the installed origin's IP ("" when none or not a
// TCP conn) — the payload-verified pool feed's subject (M8-ah,
// ADR-0047: the flow's DOWN bytes certify the origin that delivered
// them, substitute or primary alike). Safe after close: RemoteAddr
// reads the recorded address, no syscall.
func (e *tcpEvidence) originIP() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	c := e.origin
	e.mu.Unlock()
	if c == nil {
		return ""
	}
	return remoteIP(c)
}

// snapB records snapshot B — called at every flow end (relay unwind,
// stall kill, terminal) BEFORE the socket closes (Go guards a closed
// conn with "use of closed network connection", but the counters are
// gone with the fd — the kill paths must snapshot first). Once-only;
// a flow with no installed origin stays unclassifiable (unknown).
func (e *tcpEvidence) snapB() {
	if e == nil {
		return
	}
	e.mu.Lock()
	if e.haveB || e.origin == nil {
		e.mu.Unlock()
		return
	}
	origin := e.origin
	e.mu.Unlock()
	info, err := netx.SnapshotTCPInfo(origin)
	e.mu.Lock()
	if e.haveB {
		e.mu.Unlock()
		return
	}
	if err != nil {
		fail := e.onFail
		e.mu.Unlock()
		if fail != nil {
			fail(err.Error()) // the once-only device line (spec W1)
		}
		return
	}
	e.b = info
	e.haveB = true
	e.mu.Unlock()
}

// verdict applies the spec W1 rule (netx.ClassifyTCPVerdict); a flow
// without both snapshots is unknown, never a guess.
func (e *tcpEvidence) verdict() string {
	if e == nil {
		return "unknown"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.haveA || !e.haveB {
		return "unknown"
	}
	return netx.ClassifyTCPVerdict(e.a, e.b)
}

// line renders the done/stall-line suffix (spec W1's exact shape):
// " tcp A[acked=… sent=… retrans=… unacked=… rtt=…ms mss=…]
// B[acked=… rcvd=… retrans=… unacked=… rtt=… lastRecv=…ms
// lastAck=…ms]". Empty when nothing was captured.
func (e *tcpEvidence) line() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.haveA {
		return ""
	}
	a := fmt.Sprintf(" tcp A[acked=%d sent=%d retrans=%d unacked=%d rtt=%dms mss=%d]",
		e.a.Acked, e.a.Sent, e.a.Retrans, e.a.Unacked, e.a.RttMS, e.a.MSS)
	if !e.haveB {
		return a
	}
	b := fmt.Sprintf(" B[acked=%d rcvd=%d retrans=%d unacked=%d rtt=%dms lastRecv=%dms lastAck=%dms]",
		e.b.Acked, e.b.Rcvd, e.b.Retrans, e.b.Unacked, e.b.RttMS, e.b.LastRecvMS, e.b.LastAckMS)
	return a + b
}

// recNote is one observed TLS record header (spec W2): content type,
// length, and the arrival time in milliseconds since the flow started.
type recNote struct {
	typ    byte
	length int
	atMS   int64
}

// recordTracker parses the 5-byte TLS record headers out of one
// direction's relayed byte stream (spec W2): the framing survives
// arbitrary read chunking (a partial header spans feeds), the body
// bytes are skipped (never decrypted), and the first `cap` records are
// kept. A byte stream that stops looking like TLS records marks the
// tracker desynced — a non-TLS or mid-stream garbage tail, recorded
// honestly by stopping, never by guessing.
type recordTracker struct {
	mu     sync.Mutex
	start  time.Time
	hdr    [5]byte
	n      int // bytes of the current header already seen
	skip   int // bytes of the current record's body still to skip
	desync bool
	recs   []recNote
	cap    int
}

func newRecordTracker(start time.Time, cap int) *recordTracker {
	return &recordTracker{start: start, cap: cap}
}

// feed offers one read chunk to the parser (at = the read's wall time).
func (t *recordTracker) feed(p []byte, at time.Time) {
	if t == nil || len(p) == 0 {
		return
	}
	atMS := at.Sub(t.start).Milliseconds()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.desync || len(t.recs) >= t.cap {
		return
	}
	for len(p) > 0 {
		if t.skip > 0 {
			n := t.skip
			if len(p) < n {
				n = len(p)
			}
			t.skip -= n
			p = p[n:]
			continue
		}
		if t.n < 5 {
			need := 5 - t.n
			if len(p) < need {
				need = len(p)
			}
			copy(t.hdr[t.n:], p[:need])
			t.n += need
			p = p[need:]
			if t.n < 5 {
				return // header spans the next feed
			}
		}
		typ, ver, length := t.hdr[0], t.hdr[1], int(t.hdr[3])<<8|int(t.hdr[4])
		if typ < 20 || typ > 24 || ver != 3 {
			t.desync = true // not a TLS record boundary — stop, honestly
			return
		}
		if len(t.recs) < t.cap {
			t.recs = append(t.recs, recNote{typ: typ, length: length, atMS: atMS})
		}
		t.n = 0
		t.skip = length
	}
}

// notes returns the captured records (a copy).
func (t *recordTracker) notes() []recNote {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]recNote(nil), t.recs...)
}

// render prints the captured records in the spec W2 shape:
// "22:122@600 20:1@600 23:2650@640" (empty string when none).
func (t *recordTracker) render() string {
	if t == nil {
		return ""
	}
	notes := t.notes()
	if len(notes) == 0 {
		return ""
	}
	out := ""
	for i, r := range notes {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%d:%d@%d", r.typ, r.length, r.atMS)
	}
	return out
}

// gvideoEvidence is one googlevideo flow's combined evidence (spec
// W1+W2+W3): the tcp pair, both record trackers, the client's
// first-request stamp (the first post-handshake client record — the
// first-app-byte clock's zero), and the winner class.
type gvideoEvidence struct {
	start time.Time
	tcp   *tcpEvidence
	s2c   *recordTracker
	c2s   *recordTracker

	mu           sync.Mutex
	c2sFirstAtMS int64 // -1 until the client's first post-hello record
	winner       string
}

func newGvideoEvidence(start time.Time, onFail func(string)) *gvideoEvidence {
	return &gvideoEvidence{
		start:        start,
		tcp:          newTCPEvidence(onFail),
		s2c:          newRecordTracker(start, 12), // spec W2: first 12 server records
		c2s:          newRecordTracker(start, 8),  // spec W2: first 8 client records
		c2sFirstAtMS: -1,
		winner:       "orig",
	}
}

// takeA / noteOrigin / snapB / verdict delegate to the tcp pair —
// every one nil-safe (a non-gvideo flow carries a nil holder and all
// of them degrade to no-ops: behavior unchanged).
func (e *gvideoEvidence) takeA(c net.Conn) {
	if e == nil {
		return
	}
	e.tcp.takeA(c)
}

func (e *gvideoEvidence) noteOrigin(c net.Conn) {
	if e == nil {
		return
	}
	e.tcp.noteOrigin(c)
}

// originIP reports the flow's winning origin's IP (M8-ah, ADR-0047):
// "" when no evidence holder or no origin installed.
func (e *gvideoEvidence) originIP() string {
	if e == nil {
		return ""
	}
	return e.tcp.originIP()
}

func (e *gvideoEvidence) snapB() {
	if e == nil {
		return
	}
	e.tcp.snapB()
}

func (e *gvideoEvidence) verdict() string {
	if e == nil {
		return "unknown"
	}
	return e.tcp.verdict()
}

// setWinner records which candidate carried the flow (spec W3):
// orig (the primary/serial/salvage dial), subst (the cross-origin
// pool candidate), race (an own-set rotation).
func (e *gvideoEvidence) setWinner(w string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.winner = w
	e.mu.Unlock()
}

// winnerClass reports the winner ("" when no evidence holder at all).
func (e *gvideoEvidence) winnerClass() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.winner
}

// feedS2C offers one origin→client chunk to the record timeline.
func (e *gvideoEvidence) feedS2C(p []byte, at time.Time) {
	if e == nil {
		return
	}
	e.s2c.feed(p, at)
}

// feedC2S offers one client→origin chunk and latches the client's
// first post-handshake record — the request stamp the first-app-byte
// clock measures from.
func (e *gvideoEvidence) feedC2S(p []byte, at time.Time) {
	if e == nil || len(p) == 0 {
		return
	}
	e.c2s.feed(p, at)
	e.mu.Lock()
	if e.c2sFirstAtMS < 0 {
		e.c2sFirstAtMS = at.Sub(e.start).Milliseconds()
	}
	e.mu.Unlock()
}

// firstAppByte renders the spec W2 metric: the time from the client's
// request (its first post-handshake record) to the first server record
// that arrived AFTER it — the handshake flight ends and real answer
// bytes begin. "no-request" when the client never sent a post-hello
// record; "none" when a request was sent but no server record ever
// followed it (the 2.8 KB corpse's exact shape, now measured).
func (e *gvideoEvidence) firstAppByte() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	reqAt := e.c2sFirstAtMS
	e.mu.Unlock()
	if reqAt < 0 {
		return "no-request"
	}
	for _, r := range e.s2c.notes() {
		if r.atMS >= reqAt {
			return fmt.Sprintf("%dms", r.atMS-reqAt)
		}
	}
	return "none"
}

// stallSuffix is the stall-killed line's evidence: the tcp pair only
// (spec W1 puts both snapshots on the stall line; the record timeline
// and winner ride the done line).
func (e *gvideoEvidence) stallSuffix() string {
	if e == nil {
		return ""
	}
	return e.tcp.line()
}

// doneSuffix is the done line's evidence (spec W1+W2+W3): the winner
// class, the tcp pair, the record timeline and the first-app-byte.
func (e *gvideoEvidence) doneSuffix() string {
	if e == nil {
		return ""
	}
	out := " win=" + e.winnerClass()
	if tcp := e.tcp.line(); tcp != "" {
		out += tcp
	}
	rec := "rec s2c=[" + e.s2c.render() + "] c2s=[" + e.c2s.render() + "] first-app-byte=" + e.firstAppByte()
	return out + " " + rec
}

// noteTCPUnavail is the once-only device-level failure line (spec W1).
func (s *Server) noteTCPUnavail(reason string) {
	tcpUnavailOnce.Do(func() {
		if s.cfg.Logger != nil {
			s.cfg.Logger.Warn("tcpinfo=unavailable reason=" + reason)
		}
	})
}

// evidenceReadConn wraps a relay read side so every chunk also feeds
// the flow's record tracker (spec W2) — the byte stream is untouched,
// only observed.
type evidenceReadConn struct {
	net.Conn
	ev *gvideoEvidence
	up bool // true: client→origin (c2s), false: origin→client (s2c)
}

func (c *evidenceReadConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		if c.up {
			c.ev.feedC2S(b[:n], time.Now())
		} else {
			c.ev.feedS2C(b[:n], time.Now())
		}
	}
	return n, err
}

// evidenceWriteConn wraps the relay's client→origin write side so the
// FIRST request write takes snapshot A at the exact spec W1 moment —
// right after the bytes went out, before their ACK can land.
type evidenceWriteConn struct {
	net.Conn
	ev     *gvideoEvidence
	origin net.Conn // the raw origin socket (snapshot A's subject)
	once   sync.Once
}

func (c *evidenceWriteConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		c.once.Do(func() { c.ev.takeA(c.origin) })
	}
	return n, err
}
