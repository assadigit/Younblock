package proxy

// The M8-af shapes (ADR-0043/0044/0045, spec W1-W6), pinned by test:
//
//  1. The per-flow evidence (W1/W2/W3): a googlevideo flow's done line
//     carries the winner class (win=orig), the kernel TCP pair
//     (tcp A[...] B[...]), the TLS record timeline (rec s2c=[...]
//     c2s=[...]) and the first-app-byte clock; a stalled flow's kill
//     line carries the tcp pair and the report's verdict counters tick
//     (loopback: the request is ACKed, the answer never comes ->
//     downstream-lost).
//
//  2. The breaker sensor fix (W5, the round's one PROVEN defect):
//     three PENDING stall kills on googlevideo flows arm the emission
//     breaker — the debug.23 report measured the opposite (13 stall
//     deaths, gvideoTerminals=0, cooldowns=0, the breaker watched the
//     wrong death class the whole session). The fourth flow fails
//     fast at the door with zero wire.
//
//  3. The experiment switches (W4): substitution off makes the
//     media-primed pool un-drawable (the all-corpse flow terminals,
//     zero attempts); breaker off makes two terminals arm nothing;
//     both back on restore the shapes.
//
//  4. The record tracker (W2): the 5-byte TLS record headers parse
//     out of arbitrary chunk boundaries, a non-TLS byte desyncs the
//     tracker honestly, and the first-app-byte rule finds the first
//     server record after the client's request record.
//
// Both-ways proof: with the watchStall feedGvideoBreaker call stashed,
// test 2 FAILS on cooldowns=0 (the kills happen, the ring never hears
// them); with the doneSuffix wiring stashed, test 1 FAILS on the
// missing win=/tcp/rec fields; with the goodSubstitutes switch gate
// stashed, test 3 FAILS on the substituted escape of an off-switch
// pool; with the tracker's desync guard stashed, test 4 FAILS on the
// garbage tail being rendered as records.

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/fragment"
)

// handShakeEchoListener accepts, reads whatever the fragmented
// ClientHello pieces bring in one read, echoes a fixed-size flight
// back (the origin SPOKE — first byte and a little data, the
// handshake-completing shape), then holds the conn open, silent
// forever (the 2.8 KB corpse's engine-side shape).
func handShakeEchoListenerAt(t *testing.T, ip string, port int) (int, *acceptTimes) {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	a := &acceptTimes{}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			a.mu.Lock()
			a.times = append(a.times, time.Now())
			a.mu.Unlock()
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				n, rerr := c.Read(buf) // the hello (or its first pieces)
				if n > 0 {
					flight := tlsLikeRecord(96)
					if _, werr := c.Write(flight); werr != nil {
						return
					}
				}
				if rerr != nil {
					return
				}
				for { // the corpse: hold open, never answer again
					if _, rerr := c.Read(buf); rerr != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, a
}

// pendingStallFlow drives one googlevideo flow into a PENDING lean
// stall kill: the origin answers the hello (handshake shape), the
// client sends its request after the answer (dead-with-work — the
// debug.23 corpse's exact shape), then everything goes quiet.
func pendingStallFlow(t *testing.T, srv *Server, host string, port int) {
	t.Helper()
	c := dialConnect(t, srv.Addr(), host, port)
	defer c.Close()
	if _, err := c.Write(tlsLikeRecord(64)); err != nil { // the hello
		t.Fatalf("write hello: %v", err)
	}
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	handshake := make([]byte, 5+96)
	if _, err := io.ReadFull(c, handshake); err != nil {
		t.Fatalf("read the origin's answer flight: %v", err)
	}
	// The request: client speaks AFTER the origin's last byte — with a
	// small real gap (loopback stamps share a millisecond otherwise and
	// the cause split cannot see the order). The gap must stay UNDER
	// the watch's wakeGap (leanLimit/5), or the pacing proof escalates
	// the flow to the rich tier and the lean kill never fires.
	time.Sleep(25 * time.Millisecond)
	// The request is LARGE on purpose: snapshot A is taken right after
	// the FIRST relay write of it, and on loopback a small request's
	// ACK lands within microseconds — A would already include the whole
	// request and the verdict would read unknown (a loopback artifact;
	// on a real network the ACK is an RTT behind the write). A request
	// far bigger than one copy buffer guarantees A cannot contain its
	// tail, so acked(B) > acked(A) is deterministic here.
	for i := 0; i < 200; i++ {
		if _, err := c.Write(tlsLikeRecord(1024)); err != nil {
			t.Fatalf("write request chunk %d: %v", i, err)
		}
	}
	// Wait out the lean budget; the kill closes our side (read errors).
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _ = c.Read(make([]byte, 128))
}

// TestGvideoFlowEvidenceOnDoneAndStallLines (M8-af shape 1): the
// per-flow evidence — the winner class, the kernel TCP pair, the
// record timeline and the first-app-byte ride the flow's lines, and
// the verdict counters classify the loopback corpse as
// downstream-lost (the request is ACKed by the local kernel; the
// answer never comes back).
func TestGvideoFlowEvidenceOnDoneAndStallLines(t *testing.T) {
	port, _ := handShakeEchoListenerAt(t, "127.0.0.1", 0)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr1---sn-af1.googlevideo.com": {net.ParseIP("127.0.0.1")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 500 * time.Millisecond
		c.StallTimeout = 300 * time.Millisecond
		// Serial on purpose: the fixture has ONE origin, and a raced
		// round's parallel candidates all dial it — which one speaks
		// first is a coin flip (win=orig|race both honest). The serial
		// path's winner is the primary, deterministically (the raced and
		// substituted winner classes have their own test below).
		c.FirstByteRaceDials = -1
	})
	pendingStallFlow(t, srv, "rr1---sn-af1.googlevideo.com", port)

	// The stall line: the tcp pair rides it (spec W1).
	killed := pollLines(log, "WARN tunnel stall killed ", 1, 4*time.Second)
	if len(killed) == 0 {
		t.Fatalf("the corpse flow must stall-kill")
	}
	if !strings.Contains(killed[0], "cause=pending") {
		t.Fatalf("the corpse's kill must be pending (request outstanding): %q", killed[0])
	}
	if !strings.Contains(killed[0], "tcp A[acked=") || !strings.Contains(killed[0], " B[acked=") {
		t.Fatalf("the stall line must carry the kernel TCP pair: %q", killed[0])
	}
	// The done line: winner class, tcp pair, record timeline,
	// first-app-byte (spec W1/W2/W3).
	done := pollLines(log, "INFO tunnel done ", 1, 4*time.Second)
	if len(done) == 0 {
		t.Fatalf("the corpse flow's done line must log")
	}
	for _, want := range []string{"win=orig", "tcp A[acked=", "rec s2c=[", "c2s=[", "first-app-byte="} {
		if !strings.Contains(done[0], want) {
			t.Fatalf("done line missing %q: %q", want, done[0])
		}
	}
	// The record timeline saw the origin's 96-byte flight (type 22,
	// the record header tlsLikeRecord built) and the client's request
	// record after it.
	if !strings.Contains(done[0], "22:96@") {
		t.Fatalf("the s2c timeline must carry the origin's 22:96 flight: %q", done[0])
	}
	if !strings.Contains(done[0], "22:1024@") {
		t.Fatalf("the c2s timeline must carry the request records: %q", done[0])
	}
	// The verdict: loopback ACKs the request (acked moves), the origin
	// stays silent after its flight (rcvd does not) -> downstream-lost.
	if d := st.Diagnostic(); d.TCPDownstreamLost != 1 {
		t.Fatalf("tcpDownstreamLost = %d, want 1 (loopback: request ACKed, answer never came); got %+v", d.TCPDownstreamLost, d)
	}
}

// TestStallPendingFeedsBreaker (M8-af shape 2, spec W5 — the round's
// one PROVEN defect): three PENDING googlevideo stall kills inside
// the window ARM the emission breaker; the next googlevideo flow gets
// the door's 502 with zero wire. Pre-fix (the debug.23 shape) the same
// three kills leave cooldowns=0 — the breaker's sensor watched only
// first-byte terminals while the flows died mid-stream.
func TestStallPendingFeedsBreaker(t *testing.T) {
	port, echo := handShakeEchoListenerAt(t, "127.0.0.1", 0)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr2---sn-af2.googlevideo.com": {net.ParseIP("127.0.0.1")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 500 * time.Millisecond
		c.StallTimeout = 250 * time.Millisecond
		c.FirstByteRaceDials = -1 // serial: the stall shape, not the race's
		c.GvideoBreakerTerminals = 3
		c.GvideoBreakerWindow = 10 * time.Second
		c.GvideoBreakerCooldown = 500 * time.Millisecond
	})
	for i := 0; i < 3; i++ {
		pendingStallFlow(t, srv, "rr2---sn-af2.googlevideo.com", port)
	}
	if armed := pollLines(log, "WARN gvideo emission breaker armed", 1, 4*time.Second); len(armed) == 0 {
		t.Fatalf("the third pending kill must ARM the breaker (W5)")
	}
	d := st.Diagnostic()
	if d.GvideoCooldowns != 1 {
		t.Fatalf("gvideoCooldowns = %d, want 1 (the sensor extension armed it)", d.GvideoCooldowns)
	}
	if d.GvideoTerminals != 3 {
		t.Fatalf("gvideoTerminals = %d, want 3 (the ring heard every pending kill)", d.GvideoTerminals)
	}
	wire := echo.count()
	// M8-ag (ADR-0046) shape: the cooldown's first googlevideo flow
	// is the SCOUT — it dials the live origin, the origin's first
	// byte falsifies the arm's premise on the spot, and the breaker
	// LIFTS (the debug.24 field report's breaker sat blind behind
	// its own door while the wire was this healthy).
	sc := dialConnect(t, srv.Addr(), "rr2---sn-af2.googlevideo.com", port)
	_, _ = sc.Write(tlsLikeRecord(64))
	_ = sc.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = sc.Read(make([]byte, 128))
	_ = sc.Close()
	if lifted := pollLines(log, "INFO gvideo emission breaker lifted", 1, 2*time.Second); len(lifted) == 0 {
		t.Fatalf("the scout's first byte must LIFT the cooldown")
	}
	d = st.Diagnostic()
	if d.GvideoScouts != 1 || d.GvideoLifts != 1 {
		t.Fatalf("scouts=%d lifts=%d, want 1/1 (the scout dialed, the wire answered)", d.GvideoScouts, d.GvideoLifts)
	}
	if probed := echo.count(); probed != wire+1 {
		t.Fatalf("the scout must dial: %d -> %d connections", wire, probed)
	}

	// The LIFT (not the lapse) restored service: the very next
	// googlevideo flow passes the door too.
	c, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial door: %v", err)
	}
	_, _ = c.Write([]byte(fmt.Sprintf("CONNECT rr2---sn-af2.googlevideo.com:%d HTTP/1.1\r\nHost: rr2---sn-af2.googlevideo.com:%d\r\n\r\n", port, port)))
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, rerr := c.Read(buf)
	_ = c.Close()
	if rerr != nil || !strings.Contains(string(buf[:n]), "200") {
		t.Fatalf("door reply = %q err=%v, want the lifted door's 200", string(buf[:n]), rerr)
	}
	if served := echo.count(); served != wire+2 {
		t.Fatalf("the lifted cooldown must serve the next flow: %d -> %d connections", wire+1, served)
	}
}

// TestExperimentSwitches (M8-af shape 3, spec W4): the toggles gate
// live — substitution off makes even a media-primed pool un-drawable;
// breaker off makes two terminals arm nothing; both back on restore
// the shapes.
func TestExperimentSwitches(t *testing.T) {
	_, _, port := substFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = &hostResolver{hosts: map[string][]net.IP{
			"rr3---sn-aaa.googlevideo.com": {
				net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
			},
			"rr1---sn-bbb.googlevideo.com": {net.ParseIP("127.0.0.4")},
		}}
		c.FirstByteTimeout = 200 * time.Millisecond
		// Racing stays ON (the default): the substitute candidate is a
		// raced-round slot — with racing off the serial path has no
		// substitution at all and the switch would test nothing.
		c.FirstByteSalvageDials = -1 // the raced round's terminal is the witness
		c.GvideoBreakerTerminals = 2
		c.GvideoBreakerWindow = 5 * time.Second
		c.GvideoBreakerCooldown = 300 * time.Millisecond
	})
	// The media-armed pool (the W7 evidence class) exists — but the
	// switch is off: the all-corpse flow terminals with ZERO attempts.
	srv.MarkMediaVerifiedIP("127.0.0.4")
	srv.SetExperimentFlags(false, true)
	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	_, _ = c.Write(tlsLikeRecord(64))
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = c.Read(make([]byte, 128))
	_ = c.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second); len(failed) == 0 {
		t.Fatalf("the off-switch pool must leave the all-corpse flow to its terminal")
	}
	if d := st.Diagnostic(); d.SubstAttempts != 0 {
		t.Fatalf("substAttempts = %d with the switch off, want 0", d.SubstAttempts)
	}

	// Breaker off: two more terminals (the arming count) arm nothing.
	srv.SetExperimentFlags(false, false)
	for i := 0; i < 2; i++ {
		c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
		_, _ = c.Write(tlsLikeRecord(64))
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _ = c.Read(make([]byte, 128))
		_ = c.Close()
	}
	if d := st.Diagnostic(); d.GvideoCooldowns != 0 {
		t.Fatalf("gvideoCooldowns = %d with the breaker off, want 0 (the debug.22 shape)", d.GvideoCooldowns)
	}

	// Both back on: the pool saves the very next flow (the switch is
	// live — the next flow obeys).
	srv.SetExperimentFlags(true, true)
	c2 := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	rec := tlsLikeRecord(64)
	_, _ = c2.Write(rec)
	c2.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	if _, err := io.ReadFull(c2, got); err != nil {
		t.Fatalf("the re-enabled pool must save the flow: %v", err)
	}
	_ = c2.Close()
	if saved := pollLines(log, "INFO tunnel first-byte substituted ", 1, 2*time.Second); len(saved) == 0 {
		t.Fatalf("the re-enabled substitution must log its escape")
	}
}

// TestRecordTrackerParsesAcrossChunks (M8-af shape 4, spec W2): the
// 5-byte TLS record headers parse out of arbitrary chunk boundaries,
// the body bytes are skipped, and a non-TLS tail desyncs the tracker
// honestly (it stops, never guesses).
func TestRecordTrackerParsesAcrossChunks(t *testing.T) {
	start := time.Now()
	tr := newRecordTracker(start, 12)
	rec := func(typ byte, length int) []byte {
		out := make([]byte, 5+length)
		out[0], out[1], out[2] = typ, 0x03, 0x03
		out[3] = byte(length >> 8)
		out[4] = byte(length)
		return out
	}
	stream := append(append(append([]byte{}, rec(22, 122)...), rec(20, 1)...), rec(23, 64)...)
	// Feed in awkward chunks: 1, 2, 3, then the rest minus 1, then 1.
	i := 0
	for _, step := range []int{1, 2, 3} {
		tr.feed(stream[i:i+step], time.Now())
		i += step
	}
	tr.feed(stream[i:len(stream)-1], time.Now())
	tr.feed(stream[len(stream)-1:], time.Now())
	notes := tr.notes()
	if len(notes) != 3 {
		t.Fatalf("parsed %d records, want 3 (chunked across every header boundary)", len(notes))
	}
	want := []struct {
		typ byte
		len int
	}{{22, 122}, {20, 1}, {23, 64}}
	for i, w := range want {
		if notes[i].typ != w.typ || notes[i].length != w.len {
			t.Fatalf("record %d = %d:%d, want %d:%d", i, notes[i].typ, notes[i].length, w.typ, w.len)
		}
	}
	// A garbage tail desyncs — and stays desynced.
	tr.feed([]byte{0x07, 0x01, 0x02, 0x03, 0x04, 0x05}, time.Now())
	if len(tr.notes()) != 3 {
		t.Fatalf("a desynced tracker must stop, not parse garbage: %d notes", len(tr.notes()))
	}
}

// TestFirstAppByteRule (spec W2's simple rule): the first server
// record that arrives after the client's first post-handshake record
// — measured from the c2s request stamp, not from the handshake
// flight.
func TestFirstAppByteRule(t *testing.T) {
	ev := newGvideoEvidence(time.Now(), nil)
	t0 := ev.start
	// The server's handshake flight (records before the request).
	ev.feedS2C(tlsLikeRecord(96), t0.Add(10*time.Millisecond))
	// The client's request record.
	ev.feedC2S(tlsLikeRecord(48), t0.Add(50*time.Millisecond))
	// Nothing after: no answer — the corpse's exact shape.
	if got := ev.firstAppByte(); got != "none" {
		t.Fatalf("firstAppByte = %q, want none (no server record after the request)", got)
	}
	// An answer arrives 120 ms after the request.
	ev.feedS2C(tlsLikeRecord(256), t0.Add(170*time.Millisecond))
	if got := ev.firstAppByte(); got != "120ms" {
		t.Fatalf("firstAppByte = %q, want 120ms", got)
	}
	// A flow whose client never sent a post-hello record.
	ev2 := newGvideoEvidence(time.Now(), nil)
	ev2.feedS2C(tlsLikeRecord(96), time.Now())
	if got := ev2.firstAppByte(); got != "no-request" {
		t.Fatalf("firstAppByte = %q, want no-request", got)
	}
}

// TestWinnerClassRidesTheDoneLine (spec W3): the substituted flow's
// done line says win=subst, the clean primary's says win=orig.
func TestWinnerClassRidesTheDoneLine(t *testing.T) {
	resolver, _, port := substFixture(t)
	log := &recLogger{}
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	primePool(t, srv, port)

	// The substituted flow.
	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	rec := tlsLikeRecord(64)
	_, _ = c.Write(rec)
	c.SetReadDeadline(time.Now().Add(4 * time.Second))
	got := make([]byte, len(rec))
	_, _ = io.ReadFull(c, got)
	_ = c.Close()
	// The clean primary flow (the echo origin under its own name).
	c2 := dialConnect(t, srv.Addr(), "rr1---sn-bbb.googlevideo.com", port)
	_, _ = c2.Write(rec)
	c2.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = io.ReadFull(c2, make([]byte, len(rec)))
	_ = c2.Close()

	dones := pollLines(log, "INFO tunnel done ", 2, 3*time.Second)
	var subst, orig bool
	for _, d := range dones {
		if strings.Contains(d, "win=subst") {
			subst = true
		}
		if strings.Contains(d, "win=orig") {
			orig = true
		}
	}
	if !subst {
		t.Fatalf("the substituted flow's done line must say win=subst: %v", dones)
	}
	if !orig {
		t.Fatalf("the clean primary's done line must say win=orig: %v", dones)
	}
}

// TestFragmentPiecesCounted (spec W9): every piece the fragmenter
// writes lands in the process-wide segment counter — the packet
// accounting the censor may actually key on (300 fragments per legacy
// hello is 300 TCP segments).
func TestFragmentPiecesCounted(t *testing.T) {
	before := fragment.PiecesTotal()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var wgRead sync.WaitGroup
	wgRead.Add(1)
	go func() {
		defer wgRead.Done()
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 8192)
		for {
			if _, rerr := c.Read(buf); rerr != nil {
				return
			}
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fc := fragment.NewConn(conn, fragment.Legacy, fragment.PresetConfig(fragment.Legacy), nil)
	// A 1500-byte hello: legacy cuts at up to 299 random points — the
	// counter must grow by exactly the plan's piece count.
	hello := tlsLikeRecord(1495)
	if _, err := fc.Write(hello); err != nil {
		t.Fatal(err)
	}
	pieces := len(fragment.Plan(len(hello), fragment.Legacy, nil, fragment.PresetConfig(fragment.Legacy)))
	if pieces < 100 {
		t.Fatalf("the legacy plan for 1500 bytes cut %d pieces, want the fragmented shape", pieces)
	}
	after := fragment.PiecesTotal()
	if got := after - before; got != int64(pieces) {
		t.Fatalf("pieces counter delta = %d, want %d (one count per written piece)", got, pieces)
	}
}
