package proxy

import (
        "context"
        "errors"
        "fmt"
        "io"
        "math"
        "net"
        "strings"
        "sync"
        "sync/atomic"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/fragment"
)

// tunnelCloser closes both sockets of a tunnel exactly once (file 03, R2).
// setB retargets the upstream end: a salvage re-dial replaces the origin
// mid-flow (M8-t), and Stop's force-close sweep must hit the live one.
type tunnelCloser struct {
        once sync.Once
        mu   sync.Mutex
        a    net.Conn
        b    net.Conn
}

func (t *tunnelCloser) setB(b net.Conn) {
        t.mu.Lock()
        t.b = b
        t.mu.Unlock()
}

func (t *tunnelCloser) closeBoth() {
        t.mu.Lock()
        a, b := t.a, t.b
        t.mu.Unlock()
        t.once.Do(func() {
                if a != nil {
                        a.Close()
                }
                if b != nil {
                        b.Close()
                }
        })
}

// remoteB reports the tunnel's current origin IP ("" when unset) —
// the stall watch's demote target (M8-ac, ADR-0034).
func (t *tunnelCloser) remoteB() string {
        t.mu.Lock()
        b := t.b
        t.mu.Unlock()
        if b == nil {
                return ""
        }
        return remoteIP(b)
}

// bufferedConn is the client side after request parsing: it first serves the
// bytes bufio had already buffered, then the socket itself (file 01,
// section 7 step 1: keep using the same bufio.Reader afterwards).
type bufferedConn struct {
        net.Conn
        prefix []byte
}

func (c *bufferedConn) Read(b []byte) (int, error) {
        if len(c.prefix) > 0 {
                n := copy(b, c.prefix)
                c.prefix = c.prefix[n:]
                return n, nil
        }
        return c.Conn.Read(b)
}

// idleConn closes its connection after timeout of total silence; every read
// or write refreshes the deadline (file 03, R6).
type idleConn struct {
        net.Conn
        timeout time.Duration
}

func (c *idleConn) Read(b []byte) (int, error) {
        c.Conn.SetDeadline(time.Now().Add(c.timeout))
        n, err := c.Conn.Read(b)
        if n > 0 {
                c.Conn.SetDeadline(time.Now().Add(c.timeout))
        }
        return n, err
}

func (c *idleConn) Write(b []byte) (int, error) {
        c.Conn.SetDeadline(time.Now().Add(c.timeout))
        n, err := c.Conn.Write(b)
        if n > 0 {
                c.Conn.SetDeadline(time.Now().Add(c.timeout))
        }
        return n, err
}

// stampingReadConn stamps a per-direction activity clock on every read
// that returns bytes — the relay loops' only activity signal for the
// stall watch (M8-v, ADR-0020). It wraps the idle end (never the write
// side), so the idle semantics stay exactly the plain shape's.
type stampingReadConn struct {
        net.Conn
        stamp *atomic.Int64
}

func (c *stampingReadConn) Read(b []byte) (int, error) {
        n, err := c.Conn.Read(b)
        if n > 0 {
                c.stamp.Store(time.Now().UnixMilli())
        }
        return n, err
}

// watchStall closes the flow when BOTH relay directions have been
// silent for the whole effective stall budget (M8-v, ADR-0020; M8-x,
// ADR-0022 tiers it). The debug.13 report's stall cohort — first byte
// plus ~7 KB down, then total silence until the app's ~60 s patience —
// gets a bounded engine-side death instead: both ends closed (the
// app's read errors NOW, so its retry lands on a fresh origin with
// its own budget + salvage in seconds), the answer set demoted like
// every other dead-origin signal (the M8-o loop), the error counted,
// the stall line logged for the next report. One-direction silence
// never fires: a pure download (client silent) and a pure upload
// (origin silent) are healthy relay shapes, and only a whole-flow
// stall is dead.
//
// M8-x (ADR-0022): the LEAN tier keeps exactly those semantics, for
// flows that never proved themselves — first byte plus headers, never
// woke, silent to the budget. A flow that HAS proved itself —
// StallRichBytes or more delivered down, or one idle gap already
// survived and closed by fresh activity (the pacing proof: the player
// buffers, goes quiet, then asks for the next range on the SAME
// pooled connection) — escalates to the rich budget and, when that
// finally fires, dies QUIETLY: no demotion (an origin that just
// carried megabytes of payload is the opposite of suspect), no error
// count (the close is hygiene, not a dead-origin signal), tier=rich
// on the line. The debug.14 build's second report (versionCode 13,
// 2026-10-19T11:43Z) is the evidence: thirteen kills in ~70 s of
// log, every one a connection that had already carried real data —
// 2.1 MB, 1.6 MB, 723.6 KB, 633.7 KB, 162.6 KB down — murdered at
// exactly the 15 s lean budget mid-pacing, and with them the instant
// next/seek reuse the pre-watch builds had ("videos used to lag only
// at start; now even going next and back in the same video lags" —
// Ali, that report).
func (s *Server) watchStall(host, class string, tc *tunnelCloser, lastUp, lastDown, flowDown *atomic.Int64, stop chan struct{}) {
        tick := s.cfg.StallTimeout / 4
        if tick > 500*time.Millisecond {
                tick = 500 * time.Millisecond
        }
        if tick < 50*time.Millisecond {
                tick = 50 * time.Millisecond
        }
        leanLimit := s.cfg.StallTimeout.Milliseconds()
        // Escalation is armed only by a STRICTLY richer budget: zero,
        // negative (or merely equal) StallRichTimeout leaves every
        // flow on the exact M8-v semantics — lean budget, demotion,
        // error count.
        escalate := s.cfg.StallRichTimeout > s.cfg.StallTimeout
        richLimit := leanLimit
        if escalate {
                richLimit = s.cfg.StallRichTimeout.Milliseconds()
        }
        richBytes := s.cfg.StallRichBytes
        if richBytes < 0 {
                richBytes = math.MaxInt64 // negative: bytes never escalate — only a survived gap can
        }
        // The wake gap: an activity gap this wide that CLOSED — bytes
        // flowed again after it — proves the flow paces and escalates
        // the budget like real payload does. A fifth of the lean
        // budget (3 s at the default) sits past handshake chatter and
        // inside every real inter-request gap.
        wakeGap := leanLimit / 5
        if wakeGap < 50 {
                wakeGap = 50
        }
        woke := false
        prevNewest := lastUp.Load()
        if d := lastDown.Load(); d > prevNewest {
                prevNewest = d
        }
        t := time.NewTicker(tick)
        defer t.Stop()
        for {
                select {
                case <-stop:
                        return
                case <-t.C:
                        now := time.Now().UnixMilli()
                        up, downStamp := lastUp.Load(), lastDown.Load()
                        // The NEWEST stamp decides: the stall fires only when
                        // BOTH directions have been silent the whole budget —
                        // now minus the younger activity clock. (The min would
                        // fire on a pure download whose client never writes;
                        // exactly the shape the watch must spare.)
                        newest := up
                        if downStamp > newest {
                                newest = downStamp
                        }
                        if newest != prevNewest {
                                // Fresh activity since the last tick. When it
                                // closed a gap at least wakeGap wide, the flow
                                // has woken once — the pacing proof.
                                if newest-prevNewest >= wakeGap {
                                        woke = true
                                }
                                prevNewest = newest
                        }
                        idle := now - newest
                        rich := escalate && (woke || flowDown.Load() >= richBytes)
                        // The cause split (M8-ac, ADR-0036): the client speaking
                        // AFTER the origin's last byte is a request the origin
                        // never answered — the flow died with work outstanding
                        // (pending); a quiet client since the last origin byte is
                        // the ambiguous shape (pooled-healthy OR mid-body death —
                        // idle). Only the PENDING lean kill demotes the origin's
                        // good mark: definite dead-origin evidence, while an idle
                        // kill says nothing the rotation doesn't already handle.
                        pending := up > downStamp
                        if rich {
                                if idle < richLimit {
                                        continue
                                }
                                // Hygiene close, not a dead-origin signal: the
                                // flow proved itself (payload or pacing), so
                                // the answer set and the error counter stay
                                // untouched — the line carries the evidence.
                                // A PENDING rich kill still demotes (M8-ac,
                                // ADR-0034): a request the origin left
                                // unanswered for the whole rich budget is
                                // dead-origin evidence at any tier — only the
                                // noise stays quiet, the demote still lands.
                                tc.closeBoth()
                                if pending {
                                        if ip := tc.remoteB(); ip != "" {
                                                s.dropGoodIP(ip)
                                        }
                                }
                                s.logTunnelStallKilled(host, class, time.Duration(idle)*time.Millisecond, flowDown.Load(), true, pending)
                                return
                        }
                        if idle >= leanLimit {
                                tc.closeBoth()
                                if pending {
                                        if ip := tc.remoteB(); ip != "" {
                                                s.dropGoodIP(ip) // M8-ac (ADR-0034): dead-with-work-outstanding demotes, never boxes
                                        }
                                }
                                s.invalidateDNS(host)
                                s.countConnError()
                                s.logTunnelStallKilled(host, class, time.Duration(idle)*time.Millisecond, flowDown.Load(), false, pending)
                                return
                        }
                }
        }
}

// bufPool pools the 32 KB copy buffers (file 01, section 7 step 5).
var bufPool = sync.Pool{
        New: func() any {
                b := make([]byte, 32*1024)
                return &b
        },
}

func getBuf() *[]byte  { return bufPool.Get().(*[]byte) }
func putBuf(b *[]byte) { bufPool.Put(b) }

// runBidirectionalCopy reads the first TLS record, fragments it upstream
// when this tunnel should be fragmented (in youtube mode only hosts on the
// routed list — everything else is a plain relay; D46), then copies both
// directions until both end (file 01, section 7 steps 5-6).
//
// M8-s (ADR-0018) adds the first-byte budget for processed hosts: after
// the first record went upstream, the origin's first byte must arrive
// within FirstByteTimeout or the flow fails fast (see
// copyFirstByteBudgeted).
// flowMeter counts one tunnel's own bytes so the close line can report
// the flow's totals alongside the global counters (M8-j).
type flowMeter struct {
        up, down atomic.Int64
}

// noteFlow/noteFirstByte/noteFailure/notePlainKill feed the per-host
// table behind the report's Top-10 section (M8-z, ADR-0027 §8.3.1).
// The HostTable methods are nil-safe: a nil table (the unit-test
// configs that do not assert on it) collects nothing.
func (s *Server) noteFlow(host, class string) { s.cfg.Hosts.NoteFlow(host, class) }
func (s *Server) noteFirstByte(host, class string, ms int64) {
        s.cfg.Hosts.NoteFirstByte(host, class, ms)
}
func (s *Server) noteFailure(host, class string) { s.cfg.Hosts.NoteFailure(host, class) }
func (s *Server) notePlainKill(host string)      { s.cfg.Hosts.NotePlainKill(host, "plain") }

// logTunnelUp records one established flow's setup phases: the secure
// DNS lookup and the origin dial — with the flow's class and the
// dial's RTT class in the message (M8-z, ADR-0027 §8.2/§8.3.4:
// class=routed for a budgeted list host, class=plain otherwise —
// plain-but-fragmented included; rtt-class=near under 200 ms, far at
// or above, plus the near/far counters). It lands in the engine's log
// ring and therefore in the redacted diagnostics report (the ring
// handler drops attrs, so the values ride in the message itself).
func (s *Server) logTunnelUp(host, class string, dnsMS, dialMS int64) {
        s.noteFlow(host, class)
        if class == "routed" {
                s.stats.RoutedFlowStart() // the auto-timeline heuristic's burst signal (M8-aa, ADR-0030)
        }
        if dialMS < 200 {
                s.stats.DialNear()
        } else {
                s.stats.DialFar()
        }
        if s.cfg.Logger == nil {
                return
        }
        rtt := "far"
        if dialMS < 200 {
                rtt = "near"
        }
        msg := fmt.Sprintf("tunnel up class=%s rtt-class=%s dns=%dms dial=%dms", class, rtt, dnsMS, dialMS)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelFirstByte records how long the first origin→client byte took
// after the tunnel was established (the TLS handshake and the start of
// the origin's response, both through the fragment path when routed).
func (s *Server) logTunnelFirstByte(host, class string, d time.Duration) {
        s.noteFirstByte(host, class, d.Milliseconds())
        if d > time.Second {
                s.stats.FirstByteSlow() // the timeline window's "slow flows" metric (ADR-0027 §8.3.6)
        }
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte class=%s %dms", class, d.Milliseconds())
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelFirstByteFailed records a budgeted tunnel whose origin never
// sent its first byte (M8-s, ADR-0018): the reason rides in the message
// so the diagnostics report's log ring (which drops attrs — the M8-r
// lesson) carries the failure kind. M8-t: this is the TERMINAL failure
// only — attempts absorbed by a salvage log logTunnelFirstByteRetry.
func (s *Server) logTunnelFirstByteFailed(host, class, reason string) {
        s.noteFailure(host, class)
        if s.cfg.Logger == nil {
                return
        }
        msg := "tunnel first-byte failed class=" + class + " reason=" + reason
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// logPlainFirstByteFailed records the plain-relay safety net's close
// (M8-z, ADR-0024 work item B): a not-budgeted TLS flow whose origin
// never sent a byte within PlainFirstByteTimeout of the ClientHello.
// The flow closes without a dead-IP mark (the plain cohort never
// feeds the penalty box), without a connError (its own counter,
// plainFirstByteTimeouts, owns the count), and the line rides the
// ring with class=plain so the next report's Top-10 section can pair
// it with the host's other rows.
func (s *Server) logPlainFirstByteFailed(host, reason string) {
        s.notePlainKill(host)
        s.stats.PlainFirstByteTimeout()
        if s.cfg.Logger == nil {
                return
        }
        msg := "tunnel plain first-byte failed class=plain reason=" + reason
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// logTunnelFirstByteRetry records one salvage re-dial (M8-t, ADR-0019):
// the attempt that just died, absorbed inside the flow — the app never
// sees it. Warn, so the report's log ring keeps it distinct from the
// terminal failure line.
func (s *Server) logTunnelFirstByteRetry(host, class, reason string, attempt int) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte retry class=%s reason=%s attempt=%d", class, reason, attempt)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// logTunnelFirstByteSalvaged records a flow whose origin answered only
// after re-dials (M8-t): the evidence a salvage saved a video start the
// app would otherwise have retried itself.
func (s *Server) logTunnelFirstByteSalvaged(host, class string, dials int) {
        s.stats.SalvagedSave() // one flow a salvage round saved (M8-aa, ADR-0030's counter)
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte salvaged class=%s dials=%d", class, dials)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelFirstByteRaced records a raced flow whose PRIMARY lost the
// first-dial race and an extra candidate delivered the first byte
// (M8-w, ADR-0021): the evidence that racing from the start saved the
// flow the whole attempt-0 budget + salvage chain — the primary was
// dirty, the flow never showed the app a retry, and the first byte
// still landed at the fastest live origin. A clean primary stays quiet
// (nothing happened the pre-race shape would report).
func (s *Server) logTunnelFirstByteRaced(host, class string, dials int) {
        s.stats.RacedSave() // one flow the race saved (M8-aa, ADR-0030's counter)
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte raced class=%s dials=%d", class, dials)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelFirstByteSubstituted records a flow the cross-origin
// substitute saved (M8-ab, ADR-0031): the host's own answer set lost
// the raced round and a VERIFIED-LIVE pool origin delivered the first
// byte instead — the debug.19 report's 45 s escape (five retry waves
// gambling rotated unknowns until the app's own fallback landed a
// foreign cache) collapsed into one round. dials is the pool list the
// candidate walked.
func (s *Server) logTunnelFirstByteSubstituted(host, class string, dials int) {
        s.stats.SubstWin() // one flow the substitute saved (M8-ab, ADR-0031's counter)
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte substituted class=%s dials=%d", class, dials)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelStallKilled records the stall kill (M8-v, ADR-0020; M8-x,
// ADR-0022; M8-ac adds the cause split, ADR-0036): a budgeted flow
// whose relay went silent in BOTH directions for the whole effective
// budget — the debug.13 report's dead cohort (first byte plus ~7 KB,
// then silence to the app's ~60 s patience) on the lean tier, a
// proven flow's hygiene close on the rich tier. pending says whether
// the CLIENT had spoken after the origin's last byte (a request the
// origin never answered — the flow died with work outstanding) or
// had been quiet since (idle — EITHER the app's pooled connection
// between requests OR a response that stopped mid-body; the split is
// the next report's evidence for which world we are in, and it
// weights the demote: a PENDING lean kill drops the origin's good
// mark (definite dead-origin evidence), an idle one never does
// (ambiguous — the origin may be perfectly healthy, and the rotation
// already keeps the dial lists spread). The line rides in the
// report's ring like every other tunnel line; the done line that
// follows carries the flow's totals, and the tier marks which budget
// fired (the next report's primary tuning evidence).
func (s *Server) logTunnelStallKilled(host, class string, idle time.Duration, down int64, rich, pending bool) {
        s.noteFailure(host, class)
        if rich {
                s.stats.StallKillRich() // the tier split the report reads (M8-aa, ADR-0029)
        } else {
                s.stats.StallKillLean() // the lean total (M8-aa) plus the cause split (M8-ac, ADR-0036)
                if pending {
                        s.stats.StallLeanPending()
                } else {
                        s.stats.StallLeanIdle()
                }
        }
        if s.cfg.Logger == nil {
                return
        }
        tier := "lean"
        if rich {
                tier = "rich"
        }
        cause := "idle"
        if pending {
                cause = "pending"
        }
        msg := fmt.Sprintf("tunnel stall killed class=%s idle=%dms down=%s tier=%s cause=%s",
                class, idle.Milliseconds(), config.FormatBytes(down), tier, cause)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// firstByteReason reduces a first-read error to a short, hostless reason
// (the report's ring keeps only the message). timeout / closed / reset
// cover the observed silent-origin shapes; the OS reset string carries
// socket addresses before its first colon, so it is matched directly.
func firstByteReason(err error) string {
        if err == nil {
                return ""
        }
        if isTimeout(err) {
                return "timeout"
        }
        if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
                return "closed"
        }
        msg := err.Error()
        if strings.Contains(msg, "connection reset") {
                return "reset"
        }
        if i := indexByte(msg, ':'); i > 0 {
                return msg[:i]
        }
        return msg
}

// logTunnelDone records the flow's lifetime and its own byte totals —
// with first-byte it separates "slow to start" from "slow to transfer"
// on the very same line pair.
func (s *Server) logTunnelDone(host, class string, d time.Duration, m *flowMeter) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel done class=%s dur=%dms up=%s down=%s",
                class, d.Milliseconds(), config.FormatBytes(m.up.Load()), config.FormatBytes(m.down.Load()))
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

func (s *Server) runBidirectionalCopy(client *bufferedConn, upstream net.Conn, host string, port int, fragmentTLS, budgeted bool) {
        tstart := time.Now()
        flow := &flowMeter{}
        // The flow's class (M8-z, ADR-0027 §8.2): routed when the
        // budgeted machinery owns it, plain otherwise — including the
        // plain-but-fragmented flows FragmentAllRouted creates.
        class := "plain"
        if budgeted {
                class = "routed"
        }
        var firstOnce sync.Once
        logFirst := func() {
                firstOnce.Do(func() { s.logTunnelFirstByte(host, class, time.Since(tstart)) })
        }

        tcloser := &tunnelCloser{a: client.Conn, b: upstream}
        s.register(tcloser)
        defer func() {
                tcloser.closeBoth()
                s.unregister(tcloser)
        }()

        // Read the first client message with its own deadline.
        client.Conn.SetReadDeadline(time.Now().Add(s.cfg.FirstRecordTimeout))
        first, isTLS, firstErr := readFirstRecord(client)
        client.Conn.SetReadDeadline(time.Time{})
        if firstErr != nil {
                if !errors.Is(firstErr, io.EOF) || len(first) > 0 {
                        s.countConnError()
                }
                return
        }

        // Copy phase. A budgeted host with a first-byte budget runs the
        // budgeted shape (ADR-0018/0019) — which owns the first record's
        // write, because a salvage re-dial has to replay it; every other
        // tunnel keeps the plain two-goroutine relay, byte-for-byte the
        // pre-M8-s behavior. M8-z (ADR-0024): the gate is budgetedHost,
        // no longer fragmentTLS — FragmentAllRouted fragments the plain
        // cohort too, without dragging it into the expensive machinery
        // (the budget, salvage, race, dead-IP box and stall watch stay
        // scoped to the list hosts exactly as before).
        if budgeted && s.cfg.FirstByteTimeout > 0 {
                s.copyFirstByteBudgeted(client, upstream, host, port, flow, tstart, logFirst, tcloser, first, isTLS, class)
                return
        }
        var up io.Writer = upstream
        if isTLS && fragmentTLS {
                preset, fcfg := s.fragmentData()
                fconn := fragment.NewConn(upstream, preset, fcfg, s.cfg.RNG)
                up = fconn
        }
        // The plain-relay safety net (M8-z, ADR-0024 work item B): a TLS
        // flow the budgeted machinery does not own still gets ONE
        // deadline — the origin's first byte must arrive within
        // PlainFirstByteTimeout of the first record going upstream, or
        // both sides close and the kill counts. A TLS server always
        // answers a ClientHello quickly; silence means dead. Non-TLS
        // flows and post-first-byte silence keep the plain idle
        // semantics (the idleConn wrapper owns those, and the guard
        // disarms itself the moment a byte lands).
        origin := upstream
        if isTLS && !budgeted && s.cfg.PlainFirstByteTimeout > 0 {
                guard := &plainGuardConn{Conn: upstream, budget: s.cfg.PlainFirstByteTimeout}
                guard.onTimeout = func() {
                        tcloser.closeBoth()
                        s.logPlainFirstByteFailed(host, "timeout")
                }
                origin = guard
        }
        if len(first) > 0 {
                if _, err := up.Write(first); err != nil {
                        s.countConnError()
                        return
                }
                s.stats.AddBytesUp(int64(len(first)))
                flow.up.Add(int64(len(first)))
        }
        s.copyPlain(client, origin, host, class, flow, tstart, logFirst)
}

// plainGuardConn arms a first-byte deadline on the origin's READ side
// until the first byte lands (M8-z, ADR-0024 work item B): every Read
// before the first byte re-arms a PlainFirstByteTimeout read deadline
// (the idleConn wrapper re-arms its own longer deadline first — the
// guard's earlier one wins because it is set last), and the first byte
// disarm hands the connection back to the plain idle semantics. A read
// that times out while still armed fires onTimeout exactly once — the
// safety net's close-both-and-count.
type plainGuardConn struct {
        net.Conn
        budget    time.Duration
        once      sync.Once
        seen      atomic.Bool
        onTimeout func()
}

func (c *plainGuardConn) Read(b []byte) (int, error) {
        if !c.seen.Load() {
                // Arm (or re-arm) the first-byte budget for this read.
                c.Conn.SetReadDeadline(time.Now().Add(c.budget))
                n, err := c.Conn.Read(b)
                if n > 0 {
                        c.seen.Store(true)
                        // The plain idle semantics own the connection from
                        // here; the next idleConn Read re-arms its own
                        // deadline, so nothing needs clearing.
                        return n, err
                }
                if err != nil && isTimeout(err) && !c.seen.Load() {
                        c.once.Do(c.onTimeout)
                }
                return n, err
        }
        return c.Conn.Read(b)
}

// CloseWrite forwards the relay's half-close to the wrapped origin.
// The guard embeds the net.Conn INTERFACE, so without this method its
// method set would hide the *net.TCPConn CloseWrite the plain relay's
// closeWrite call depends on — the client-side half-close would be
// LOST, the origin would never see EOF, and every guarded flow would
// hang to the idle timeout instead of unwinding at close (the soak
// test caught exactly this shape). The fallback mirrors tunstack's
// closeWrite for origins that cannot half-close.
func (c *plainGuardConn) CloseWrite() error {
        if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
                return cw.CloseWrite()
        }
        return c.Conn.Close()
}

// copyPlain is the original relay shape: both directions copy concurrently
// from the start; the first downstream read fires onFirst (the first-byte
// line). Every tunnel without a first-byte budget runs this — unrouted
// relays and budget-disabled runs keep the pre-M8-s behavior exactly.
func (s *Server) copyPlain(client *bufferedConn, upstream net.Conn, host, class string, flow *flowMeter, tstart time.Time, onFirst func()) {
        // Idle-wrapped ends for the copy phase. The upstream end may be
        // the plain safety net's guard (M8-z): its own first-byte
        // deadline applies until the first byte, the idle timeout after.
        cIdle := &idleConn{Conn: client, timeout: s.cfg.IdleTimeout}
        uIdle := &idleConn{Conn: upstream, timeout: s.cfg.IdleTimeout}

        done := make(chan struct{}, 2)
        go func() {
                s.copyDir(cIdle, uIdle, true, flow, nil) // client -> upstream counts as up
                closeWrite(upstream)
                done <- struct{}{}
        }()
        go func() {
                s.copyDir(uIdle, cIdle, false, flow, onFirst) // upstream -> client counts as down
                closeWrite(client.Conn)
                done <- struct{}{}
        }()
        <-done
        <-done
        s.logTunnelDone(host, class, time.Since(tstart), flow)
}

// firstByteReplayCap bounds the client bytes remembered for a salvage
// replay. A ClientHello plus an early-data flight is a few KB on Ali's
// network (the reports show ~1.7–1.8 KB); far beyond that a replay
// stops being worth its memory — the flow keeps its live relay and
// loses salvage eligibility instead.
const firstByteReplayCap = 64 * 1024

// errOriginSwapped tells the client pump its write raced a salvage
// swap: the bytes are already safe in the replay buffer (the tee runs
// before the write), so the pump unwinds quietly and the re-dial's
// replay re-sends them.
var errOriginSwapped = errors.New("origin swapped")

// replayBuf remembers the client bytes a salvage may have to send
// again: the first record (always — the budgeted shape owns its write)
// plus everything the pump reads during the first-byte wait. Appends
// stop at the cap; a truncated buffer never replays (a partial replay
// would corrupt the fresh origin's stream).
type replayBuf struct {
        mu        sync.Mutex
        b         []byte
        truncated bool
}

func (r *replayBuf) append(p []byte) {
        r.mu.Lock()
        defer r.mu.Unlock()
        if r.truncated || len(r.b) >= firstByteReplayCap {
                r.truncated = true
                return
        }
        room := firstByteReplayCap - len(r.b)
        if len(p) > room {
                p = p[:room]
                r.truncated = true
        }
        r.b = append(r.b, p...)
}

func (r *replayBuf) bytes() []byte {
        r.mu.Lock()
        defer r.mu.Unlock()
        return r.b
}

func (r *replayBuf) fits() bool {
        r.mu.Lock()
        defer r.mu.Unlock()
        return !r.truncated
}

// originLink is the client pump's swap-able write end. A salvage
// closes the dead origin and installs the next under the mutex, so a
// write racing the swap fails fast instead of blocking on a dead
// socket; the phase-A write deadline bounds even a live one.
type originLink struct {
        mu           sync.Mutex
        dst          net.Conn
        writeTimeout time.Duration
}

func (l *originLink) Write(b []byte) (int, error) {
        l.mu.Lock()
        dst := l.dst
        l.mu.Unlock()
        if dst == nil {
                return 0, errOriginSwapped
        }
        dst.SetWriteDeadline(time.Now().Add(l.writeTimeout))
        return dst.Write(b)
}

// swap installs next as the write end, closing the previous origin
// (nil = close only — teardown).
func (l *originLink) swap(next net.Conn) {
        l.mu.Lock()
        old := l.dst
        l.dst = next
        l.mu.Unlock()
        if old != nil {
                old.Close()
        }
}

func (l *originLink) current() net.Conn {
        l.mu.Lock()
        defer l.mu.Unlock()
        return l.dst
}

// remoteIP reports the peer address of a dialed origin ("" when not a
// TCP conn) — the salvage excludes exactly the IPs whose origins
// already died, so a multi-answer host does not re-dial its own corpse.
func remoteIP(c net.Conn) string {
        if ta, ok := c.RemoteAddr().(*net.TCPAddr); ok {
                return ta.IP.String()
        }
        return ""
}

// excludeDead drops the dead IPs from the fresh answer set; if that
// would empty it (a single-answer host), the full set stands — the
// re-dial takes its chances rather than refusing.
func excludeDead(ips []net.IP, dead map[string]bool) []net.IP {
        var out []net.IP
        for _, ip := range ips {
                if !dead[ip.String()] {
                        out = append(out, ip)
                }
        }
        if len(out) == 0 {
                return ips
        }
        return out
}

// dedupeIPs drops duplicate addresses, preserving order (M8-v: a
// salvage race wants its candidates on DISTINCT origins whenever the
// answer set has them).
func dedupeIPs(ips []net.IP) []net.IP {
        if len(ips) < 2 {
                return ips
        }
        seen := make(map[string]bool, len(ips))
        out := make([]net.IP, 0, len(ips))
        for _, ip := range ips {
                k := ip.String()
                if !seen[k] {
                        seen[k] = true
                        out = append(out, ip)
                }
        }
        return out
}

// rotateIPs returns ips rotated by k places (M8-v): the salvage race's
// candidate k starts at ips[k], and DialHost still walks the whole
// rotated list with its stagger — every candidate keeps the serial
// re-dial's failover while the candidates together cover the live set
// in one budget's wall time.
func rotateIPs(ips []net.IP, k int) []net.IP {
        if len(ips) < 2 {
                return ips
        }
        r := k % len(ips)
        if r == 0 {
                return ips
        }
        out := make([]net.IP, 0, len(ips))
        out = append(out, ips[r:]...)
        out = append(out, ips[:r]...)
        return out
}

// pumpClient is the budgeted flow's ONE client reader while it runs:
// it tees phase-A client bytes into the replay buffer and writes
// everything live to the current origin through the link — a chatty
// client (TLS early data) is never blocked behind the budget, exactly
// like the plain shape's upstream copy. stop interrupts it through the
// client's read deadline; every read is fully processed (tee + write +
// count) before the stop is honored, so no byte is ever lost between
// pump generations.
func (s *Server) pumpClient(client *bufferedConn, link *originLink, replay *replayBuf, flow *flowMeter, stop chan struct{}, phaseA bool, done chan<- struct{}) {
        defer close(done)
        buf := getBuf()
        defer putBuf(buf)
        for {
                select {
                case <-stop:
                        return
                default:
                }
                // Rolling idle deadline — the plain shape's cIdle semantics.
                client.Conn.SetReadDeadline(time.Now().Add(s.cfg.IdleTimeout))
                n, rerr := client.Read(*buf)
                if n > 0 {
                        if phaseA {
                                replay.append((*buf)[:n])
                        }
                        if _, werr := link.Write((*buf)[:n]); werr != nil {
                                // Phase-A write failures are the salvage's
                                // business (the bytes are in replay); in
                                // phase B they are real connection errors.
                                if !phaseA && !isEOF(rerr) {
                                        s.countConnError()
                                }
                                return
                        }
                        s.stats.AddBytesUp(int64(n))
                        flow.up.Add(int64(n))
                }
                if rerr != nil {
                        select {
                        case <-stop:
                                return // the deliberate deadline interrupt
                        default:
                        }
                        if !isEOF(rerr) {
                                s.countConnError()
                        }
                        if isEOF(rerr) {
                                // The client half-closed: tell the origin,
                                // like the plain shape's closeWrite(upstream).
                                if cw, ok := link.current().(closeWriter); ok {
                                        _ = cw.CloseWrite()
                                }
                        }
                        return
                }
        }
}

// stopClientPump stops the pump and waits for its exit. The read
// deadline interrupt is re-armed until the pump observes the stop (the
// tiny window between its stop-check and its next Read re-arms the
// idle deadline), so the wait is bounded no matter where the first
// interrupt lands. Returns false only pathologically; callers then
// fail the flow rather than risk a second reader.
func stopClientPump(client *bufferedConn, stop chan struct{}, done chan struct{}) bool {
        close(stop)
        deadline := time.Now().Add(3 * time.Second)
        for time.Now().Before(deadline) {
                client.Conn.SetReadDeadline(time.Now())
                select {
                case <-done:
                        client.Conn.SetReadDeadline(time.Time{})
                        return true
                case <-time.After(150 * time.Millisecond):
                }
        }
        client.Conn.SetReadDeadline(time.Time{})
        return false
}

// writeReplay writes the buffered client bytes to an origin: the first
// record through a fresh fragment conn (its first Write is the one the
// preset splits — the wire shape is identical to the original attempt),
// the captured extras right after, unfragmented.
func (s *Server) writeReplay(origin net.Conn, replay []byte, firstLen int, isTLS bool, preset fragment.Preset, fcfg fragment.Config) error {
        var w io.Writer = origin
        if isTLS && firstLen > 0 {
                w = fragment.NewConn(origin, preset, fcfg, s.cfg.RNG)
        }
        if firstLen > 0 {
                if _, err := w.Write(replay[:firstLen]); err != nil {
                        return err
                }
        }
        if len(replay) > firstLen {
                if _, err := w.Write(replay[firstLen:]); err != nil {
                        return err
                }
        }
        return nil
}

// racedCandidate is one first-dial race candidate's outcome (M8-w,
// ADR-0021). conn is a speaking origin (nil on death) with its first
// downstream bytes; canceled marks a death the round's own teardown
// caused — the dial ctx aborted by the round's end, or a conn closed
// under the candidate (the swap-closed primary) — never a dirty-origin
// signal, so it never demotes. A death at the candidate's OWN budget
// stays natural even after a sibling won: the origin was silent the
// whole budget, exactly the M8-o evidence.
type racedCandidate struct {
        conn     net.Conn
        first    []byte
        death    string
        canceled bool
        // deadIP is the candidate's origin IP when its death is the
        // classified first-byte shape (the origin connected, or reset at
        // the hello write, and never spoke) — the penalty-box evidence
        // (M8-y, ADR-0023). Empty for dial deaths (no single-IP
        // evidence) and for every death (canceled or not) the
        // candidate's own conn does not witness.
        deadIP string
        // subst marks the cross-origin substitute candidate (M8-ab,
        // ADR-0031): its conn dialed a VERIFIED-LIVE pool origin, not
        // this host's answer set — a win is the escape the substitution
        // line counts, and its death demotes the POOL origin (never the
        // host's answer set).
        subst bool
}

// raceFirstByte runs one raced first-byte round (M8-w, ADR-0021): the
// primary origin (already dialed — its natural head start; nil = no
// primary, the salvage round's shape) plus raceN extra candidates
// compete for the origin's first byte, each replaying the client's
// buffered bytes byte-exact through a fresh fragment conn onto its own
// rotated start of the caller's answer set (DialHost still walks the
// whole list with its stagger, so a blackholed front address fails over
// inside the candidate). The first origin to speak wins; the losers are
// closed and reaped (a late natural death still demotes the answer set
// — one per dead candidate, the M8-o loop — while a death the teardown
// itself caused never does). The client stays frozen for the round
// exactly like the M8-v salvage race freezes it: bytes beyond the first
// record wait in the socket and flow to the winner in order through the
// plain relay. Returns the winner with its first bytes, or a nil conn
// with the last natural death's reason when every candidate died, plus
// the origin IPs every candidate actually dialed — the M8-aa salvage
// round's untried computation (a dial failure reports no IP; the
// primary's IP rides its own result).
//
// M8-ab (ADR-0031): when subst is non-empty (a *.googlevideo.com
// flow's escape hatch), ONE extra candidate slot becomes the
// substitute — it dials the good pool's verified-live origins instead
// of an answer-set rotation, replaying the identical fragmented hello
// with the SNI unchanged (Google's video edge serves any
// *.googlevideo.com name). The dial count stays flat: the substitute
// replaces one own-set rotation. A substitute win is reported through
// the final return so the caller logs the escape; a substitute death
// demotes the POOL origin (box + good-mark drop) and never the host's
// answer set.
//
// The debug.14 report's evidence this shape answers: attempt 0 still
// walked its budget ALONE before the M8-v race launched (six timeout
// terminals at 5320–5576 ms = the two-budget chain, salvage saves only
// at 738–848 ms) while CONCURRENT flows to the same hosts answered at
// 72–162 ms in the same seconds — the dirt is per-origin, not
// per-moment, so spending the dial budget in parallel from t=0 lands a
// mixed-window flow at its fastest live origin and terminals an
// all-dead flow at ~1 budget instead of 2.
func (s *Server) raceFirstByte(primary net.Conn, host string, port int, ips []net.IP, snapshot []byte, firstLen int, isTLS bool, preset fragment.Preset, fcfg fragment.Config, raceN int, subst []net.IP) (net.Conn, []byte, string, []string, bool) {
        substN := 0
        if s.cfg.GoodIPSubstitution > 0 && len(subst) > 0 && raceN > 0 {
                substN = 1
                raceN-- // the substitute replaces one own-set rotation — flat dial count
                if s.cfg.Stats != nil {
                        s.cfg.Stats.SubstAttempt()
                }
        }
        total := raceN
        if primary != nil {
                total++
        }
        total += substN
        if total == 0 {
                return nil, nil, "", nil, false
        }
        results := make(chan racedCandidate, total)
        stopRound := make(chan struct{})
        var stopOnce sync.Once
        endRound := func() { stopOnce.Do(func() { close(stopRound) }) }
        defer endRound() // both exits — winner and all-dead — reap the rest
        roundStopped := func() bool {
                select {
                case <-stopRound:
                        return true
                default:
                        return false
                }
        }

        // launch starts one candidate. pre != nil is the primary (already
        // dialed; its dial time is the head start the extras must beat).
        // substCandidate marks the cross-origin substitute (M8-ab): its
        // list dials the good pool, its result carries the flag. A
        // deathCanceled classifies one death: teardown-caused only when the
        // round ended AND the error is the teardown's own fingerprint (the
        // aborted dial ctx or a conn closed under the candidate) — a budget
        // timeout or an origin reset is natural dirt whenever it happens.
        deathCanceled := func(err error) bool {
                if !roundStopped() {
                        return false
                }
                return errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)
        }
        launch := func(pre net.Conn, list []net.IP, substCandidate bool) {
                go func(pre net.Conn, list []net.IP, substCandidate bool) {
                        conn := pre
                        if conn == nil {
                                ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
                                defer cancel()
                                // The round's teardown aborts a dial still in
                                // flight instead of waiting out its budget.
                                go func() {
                                        <-stopRound
                                        cancel()
                                }()
                                var derr error
                                conn, derr = s.cfg.Dialer.DialHost(ctx, "tcp", host, port, list)
                                if derr != nil {
                                        results <- racedCandidate{death: "dial", canceled: deathCanceled(derr), subst: substCandidate}
                                        return
                                }
                        }
                        if werr := s.writeReplay(conn, snapshot, firstLen, isTLS, preset, fcfg); werr != nil {
                                deadIP := remoteIP(conn)
                                conn.Close()
                                results <- racedCandidate{death: firstByteReason(werr), canceled: deathCanceled(werr), deadIP: deadIP, subst: substCandidate}
                                return
                        }
                        cbuf := getBuf()
                        conn.SetReadDeadline(time.Now().Add(s.cfg.FirstByteTimeout))
                        cn, cerr := conn.Read(*cbuf)
                        conn.SetReadDeadline(time.Time{})
                        if cn > 0 {
                                firstDown := append([]byte(nil), (*cbuf)[:cn]...)
                                putBuf(cbuf)
                                results <- racedCandidate{conn: conn, first: firstDown, subst: substCandidate}
                                return
                        }
                        putBuf(cbuf)
                        deadIP := remoteIP(conn)
                        conn.Close()
                        results <- racedCandidate{death: firstByteReason(cerr), canceled: deathCanceled(cerr), deadIP: deadIP, subst: substCandidate}
                }(pre, list, substCandidate)
        }

        dialed := make([]string, 0, total)
        substWin := false
        if primary != nil {
                launch(primary, nil, false)
                for i := 0; i < raceN; i++ {
                        launch(nil, rotateIPs(ips, i+1), false)
                }
                if substN > 0 {
                        launch(nil, subst, true)
                }
        } else {
                // The salvage round's shape: no primary, the rotations start at 0.
                for i := 0; i < raceN; i++ {
                        launch(nil, rotateIPs(ips, i), false)
                }
                if substN > 0 {
                        launch(nil, subst, true)
                }
        }

        // Collect: the first speaking origin wins; every natural death
        // demotes (one per dead candidate, the M8-o loop). A substitute's
        // death demotes the POOL origin only — the host's answer set was
        // never dialed by it (M8-ab).
        var winner net.Conn
        var winnerFirst []byte
        lastDeath := ""
        received := 0
        for received < total && winner == nil {
                res := <-results
                received++
                if res.canceled {
                        continue // cannot precede a winner; defensive
                }
                if res.conn != nil {
                        winner, winnerFirst, substWin = res.conn, res.first, res.subst
                        if ip := remoteIP(res.conn); ip != "" {
                                dialed = append(dialed, ip)
                        }
                        break
                }
                lastDeath = res.death
                if res.deadIP != "" {
                        dialed = append(dialed, res.deadIP)
                        // M8-ac (ADR-0034): the death's own class weights the mark —
                        // a reset is a temporal signal (a brief skip, no demote), a
                        // full-budget silence keeps the strong verdict.
                        s.markDeadIPClass(res.deadIP, res.death)
                }
                if !res.subst {
                        s.invalidateDNS(host)
                }
        }
        if winner == nil {
                return nil, nil, lastDeath, dialed, false
        }
        // The round ended with a winner: the still-pending candidates are
        // reaped in the background — a late speaker is closed (it lost),
        // a late death demotes only when the teardown did not cause it.
        endRound()
        pending := total - received
        go func(pending int) {
                for i := 0; i < pending; i++ {
                        res := <-results
                        if res.conn != nil {
                                res.conn.Close()
                                continue
                        }
                        if !res.canceled {
                                if res.deadIP != "" {
                                        // M8-ac: class-weighted like every death mark.
                                        s.markDeadIPClass(res.deadIP, res.death)
                                }
                                if !res.subst {
                                        s.invalidateDNS(host)
                                }
                        }
                }
        }(pending)
        return winner, winnerFirst, "", dialed, substWin
}

// racedSalvage is one raced salvage round's outcome: the winning origin
// with its first downstream bytes, or the last natural death's reason
// when every candidate died.
type racedSalvage struct {
        conn  net.Conn
        first []byte
        death string
}

// raceIPList races count candidates over the answer set (M8-aa,
// ADR-0028 — the M8-v salvage race's candidate core, extracted so the
// raced path's salvage round runs the identical machinery the serial
// path always had): candidate i dials its rotated start of ips
// (DialHost still walks the whole list with its stagger, so a
// blackholed front address fails over inside the candidate), replays
// the buffered client bytes byte-exact through a fresh fragment conn,
// and waits one budget for the origin's first byte — the first origin
// to speak wins, the losers are closed and reaped, every natural death
// marks its origin IP for the box (M8-y) and demotes the answer set
// (the M8-o loop).
func (s *Server) raceIPList(host string, port int, ips []net.IP, count int, snapshot []byte, firstLen int, isTLS bool, preset fragment.Preset, fcfg fragment.Config) racedSalvage {
        var out racedSalvage
        if count <= 0 || len(ips) == 0 {
                return out
        }
        results := make(chan racedCandidate, count)
        for i := 0; i < count; i++ {
                list := rotateIPs(ips, i)
                go func(list []net.IP) {
                        ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
                        defer cancel()
                        conn, derr := s.cfg.Dialer.DialHost(ctx, "tcp", host, port, list)
                        if derr != nil {
                                results <- racedCandidate{death: "dial"}
                                return
                        }
                        if werr := s.writeReplay(conn, snapshot, firstLen, isTLS, preset, fcfg); werr != nil {
                                deadIP := remoteIP(conn)
                                conn.Close()
                                results <- racedCandidate{death: firstByteReason(werr), deadIP: deadIP}
                                return
                        }
                        cbuf := getBuf()
                        conn.SetReadDeadline(time.Now().Add(s.cfg.FirstByteTimeout))
                        cn, cerr := conn.Read(*cbuf)
                        conn.SetReadDeadline(time.Time{})
                        if cn > 0 {
                                firstDown := append([]byte(nil), (*cbuf)[:cn]...)
                                putBuf(cbuf)
                                results <- racedCandidate{conn: conn, first: firstDown}
                                return
                        }
                        putBuf(cbuf)
                        deadIP := remoteIP(conn)
                        conn.Close()
                        results <- racedCandidate{death: firstByteReason(cerr), deadIP: deadIP}
                }(list)
        }

        // Collect: the first speaking origin wins; the still-pending
        // candidates are reaped in the background (their conns closed, their
        // late deaths still demoted — one per dead candidate, the M8-o loop).
        // A winner SPOKE on the flow's own wire shape — the good-pool
        // evidence the substitute candidates draw from (M8-ab, ADR-0031).
        deaths := 0
        for {
                res := <-results
                if res.conn != nil {
                        out.conn, out.first = res.conn, res.first
                        if ip := remoteIP(res.conn); ip != "" {
                                s.markGoodIP(ip, isGvideoHost(host))
                        }
                        break
                }
                deaths++
                out.death = res.death
                if res.deadIP != "" {
                        // M8-ac (ADR-0034): class-weighted like every death mark.
                        s.markDeadIPClass(res.deadIP, res.death)
                }
                s.invalidateDNS(host)
                if deaths == count {
                        return out
                }
        }
        go func(pending int) {
                for i := 0; i < pending; i++ {
                        res := <-results
                        if res.conn != nil {
                                res.conn.Close()
                        } else {
                                if res.deadIP != "" {
                                        // M8-ac: class-weighted like every death mark.
                                        s.markDeadIPClass(res.deadIP, res.death)
                                }
                                s.invalidateDNS(host)
                        }
                }
        }(count - deaths - 1)
        return out
}

// untriedIPs computes the raced-path salvage round's candidates (M8-aa,
// ADR-0028): the host's CURRENT answer set (a fresh resolve — the
// round's deaths demoted the old one, the M8-o loop) minus every IP the
// flow already dialed (the round's candidates, live or dead) minus
// everything the box penalizes right now, with verified-live origins
// first — only origins this flow has NOT gambled on and the engine does
// not already know dead. An exhausted or fully boxed set returns nil and
// the flow keeps the fast M8-w terminal: a hopeless host must not burn
// a second budget.
func (s *Server) untriedIPs(host string, dialed []string) []net.IP {
        ips, err := s.resolve(host)
        if err != nil {
                return nil
        }
        ips = dedupeIPs(ips)
        dialedSet := make(map[string]bool, len(dialed))
        for _, ip := range dialed {
                dialedSet[ip] = true
        }
        now := time.Now()
        s.deadMu.Lock()
        kept := make([]net.IP, 0, len(ips))
        for _, ip := range ips {
                key := ip.String()
                if dialedSet[key] {
                        continue
                }
                if m, ok := s.dead[key]; ok && now.Before(m.until) {
                        continue // the box knows this one dead
                }
                kept = append(kept, ip)
        }
        s.deadMu.Unlock()
        if len(kept) == 0 {
                return nil
        }
        s.goodMu.Lock()
        var live, neutral []net.IP
        for _, ip := range kept {
                if m, ok := s.good[ip.String()]; ok && now.Before(m.until) {
                        live = append(live, ip)
                } else {
                        neutral = append(neutral, ip)
                }
        }
        s.goodMu.Unlock()
        if len(live) == 0 {
                return kept
        }
        return append(live, neutral...)
}

// copyFirstByteBudgeted is the processed-host relay shape (M8-s,
// ADR-0018; M8-t, ADR-0019; M8-v, ADR-0020; M8-w, ADR-0021; M8-x,
// ADR-0022): after the
// first record went upstream, the origin's first byte must arrive within
// FirstByteTimeout. A dead origin — the pasted reports' signature: a
// dial that completes, ~1.7 KB up, then a ~0.5 s reset or silence to
// the budget — is RETRIED in-flow: the engine closes only the dead
// origin, demotes the answer set (the M8-o loop), re-dials the same
// host with the same preset and replays the buffered client bytes, so
// the app never sees the failure and its own serial retry waves
// collapse into bounded in-engine attempts. M8-v races the remaining
// re-dials in parallel (the debug.13 report's terminal failures were
// the serial chain's cost: 21 timeout dones at 8014-8227 ms = three
// 2.5 s budgets walked one after another) and adds the stall watch to
// the relay phase: both directions silent for StallTimeout closes the
// flow (the report's five 57-64 s stalls died only at the app's own
// patience); M8-x tiers that watch — a flow that proved itself
// (payload delivered, or one idle gap survived) is closed only at
// StallRichTimeout, quietly: no demotion, no error count.
// M8-w spends the whole dial budget racing from the flow's
// start (FirstByteRaceDials extra candidates against the primary, the
// first speaker winning — the debug.14 report showed attempt 0 still
// walking its budget alone before the race launched, so a dead primary
// cost two budgets and a mixed window recovered only at salvage time
// while concurrent flows answered in the same seconds). When the
// retries run out, the flow fails FAST exactly as at M8-s: both ends
// closed, the error counted, "tunnel first-byte failed reason=..." in
// the report, the answer set demoted. Absorbed attempts log "tunnel
// first-byte retry reason=... attempt=N"; a flow saved by them logs
// "tunnel first-byte salvaged dials=N"; a flow the first-dial race
// saved logs "tunnel first-byte raced dials=N". Unrouted relays never
// enter this shape (the D46 scoping).
//
// Shape: the pump (pumpClient) reads the client and writes live to the
// current origin through the swap-able link, so a chatty client is
// never blocked behind the budget; the main goroutine reads the
// origin's first byte under the deadline. Once the origin speaks, the
// pump is stopped (its reads are all delivered) and the plain
// two-goroutine relay takes over byte-for-byte — under the stall
// watch, which is the only M8-v addition to the relay itself. The
// raced round (M8-w) freezes the client for its duration instead —
// the M8-v salvage race's own proven semantics: bytes beyond the first
// record wait in the socket and flow to the winner in order.
func (s *Server) copyFirstByteBudgeted(client *bufferedConn, upstream net.Conn, host string, port int, flow *flowMeter, tstart time.Time, onFirst func(), tc *tunnelCloser, first []byte, isTLS bool, class string) {
        link := &originLink{dst: upstream, writeTimeout: s.cfg.FirstByteTimeout}
        defer link.swap(nil) // the final origin closes on every exit path
        replay := &replayBuf{}
        replay.append(first) // the budgeted shape owns the first record's write
        deadIPs := make(map[string]bool, 2)
        // The flow's preset is captured once: every attempt of this flow,
        // original or salvage, puts the identical shape on the wire (never
        // a per-flow method variation — D102 is Ali's ruling, not ours).
        preset, fcfg := s.fragmentData()
        maxRedial := s.cfg.FirstByteSalvageDials
        if maxRedial < 0 {
                maxRedial = 0
        }
        raceN := s.cfg.FirstByteRaceDials
        if raceN < 0 {
                raceN = 0
        }

        var pumpStop chan struct{}
        var pumpDone chan struct{}
        pumpRunning := false
        startPump := func() {
                pumpStop = make(chan struct{})
                pumpDone = make(chan struct{})
                go s.pumpClient(client, link, replay, flow, pumpStop, true, pumpDone)
                pumpRunning = true
        }
        // failFlow is the M8-s terminal: both ends closed, the error
        // counted once, the reason line, the done line with final totals.
        // A googlevideo terminal is the churn signal the engine's gvideo
        // loop kicks an origin-probe pass on (M8-ab, ADR-0032) — the
        // flow's raced round AND its untried salvage all died.
        failFlow := func(reason string) {
                link.swap(nil) // unblocks a pump stuck writing before anything else
                if pumpRunning {
                        stopClientPump(client, pumpStop, pumpDone)
                        pumpRunning = false
                }
                tc.closeBoth()
                s.countConnError()
                if isGvideoHost(host) {
                        s.stats.GvideoTerminal()
                }
                s.logTunnelFirstByteFailed(host, class, reason)
                s.logTunnelDone(host, class, time.Since(tstart), flow)
        }

        // relayEntry runs the post-first-byte relay under the stall watch
        // (M8-v, ADR-0020) — the plain two-goroutine shape byte-for-byte,
        // plus the both-directions-silence clock. The stamps live on the
        // READ side of each direction (the copy loops' only activity
        // signal); the idle ends keep the plain shape's IdleTimeout.
        relayEntry := func(origin net.Conn) {
                cIdle := &idleConn{Conn: client, timeout: s.cfg.IdleTimeout}
                uIdle := &idleConn{Conn: origin, timeout: s.cfg.IdleTimeout}
                nowMS := time.Now().UnixMilli()
                lastUp := &atomic.Int64{}
                lastDown := &atomic.Int64{}
                lastUp.Store(nowMS)
                lastDown.Store(nowMS)
                upSrc := &stampingReadConn{Conn: cIdle, stamp: lastUp}
                downSrc := &stampingReadConn{Conn: uIdle, stamp: lastDown}
                stallStop := make(chan struct{})
                if s.cfg.StallTimeout > 0 {
                        go s.watchStall(host, class, tc, lastUp, lastDown, &flow.down, stallStop)
                }
                done := make(chan struct{}, 2)
                go func() {
                        s.copyDir(upSrc, uIdle, true, flow, nil) // client -> origin counts as up
                        closeWrite(origin)
                        done <- struct{}{}
                }()
                go func() {
                        s.copyDir(downSrc, cIdle, false, flow, nil) // origin -> client counts as down
                        closeWrite(client.Conn)
                        done <- struct{}{}
                }()
                <-done
                <-done
                close(stallStop)
                s.logTunnelDone(host, class, time.Since(tstart), flow)
        }

        // M8-w (ADR-0021): the first-dial race. With extra candidates
        // configured (the default), the whole dial budget is spent in
        // parallel from the flow's start — the primary plus the extras
        // compete for the first byte, the first speaker wins, and an
        // all-dead round terminals at ~1 budget (the M8-v shape spent
        // the first budget serially before its race even launched; the
        // debug.14 report's terminals at 5320-5576 ms were exactly that
        // two-budget chain). The client stays frozen for the round (the
        // salvage race's own proven semantics); the pump never starts on
        // this path, so the serial loop below is the racing-disabled
        // shape only.
        if raceN > 0 && replay.fits() {
                // The extras dial the current answer set (cached fresh — the
                // handleTunnel resolve landed milliseconds ago; a sibling flow's
                // demotion in between makes this the fresher set, exactly the
                // M8-o intent) through the box's preference and the verified-live
                // front (M8-aa). A resolve failure degenerates the round to the
                // primary alone rather than failing a flow that already has a
                // connected origin.
                raceIPs := []net.IP(nil)
                if resolved, err := s.resolve(host); err == nil {
                        raceIPs = s.liveIPs(dedupeIPs(resolved))
                } else {
                        raceN = 0
                }
                // M8-ab (ADR-0031): the cross-origin escape hatch. A googlevideo
                // flow whose answer set is all blackholed cache nodes gets ONE
                // substitute candidate — the good pool's verified-live origins,
                // not in this answer set, not boxed, freshest verdict first.
                // The substitute replays the identical fragmented ClientHello
                // (SNI unchanged — the client's own hello): Google's video edge
                // serves any *.googlevideo.com name, so the flow rides an
                // origin the engine has SEEN speak instead of re-gambling the
                // rotation. Non-gvideo hosts and empty pools keep the exact
                // debug.19 round.
                subst := []net.IP(nil)
                if isGvideoHost(host) {
                        subst = s.goodSubstitutes(raceIPs, config.SubstitutionPoolDials)
                }
                winner, winnerFirst, lastDeath, dialedIPs, substWin := s.raceFirstByte(upstream, host, port, raceIPs, replay.bytes(), len(first), isTLS, preset, fcfg, raceN, subst)
                // The first record went upstream on every candidate; it
                // counts once (the winner's copy is the flow's).
                if len(first) > 0 {
                        s.stats.AddBytesUp(int64(len(first)))
                        flow.up.Add(int64(len(first)))
                }
                // The winner SPOKE with the exact wire shape a real flow
                // sends — stronger evidence than the probe's synthetic hello,
                // and the pool the substitute candidates draw from (M8-ab,
                // ADR-0031: the session's own foreign-cache win arms every
                // later flow).
                if winner != nil {
                        if ip := remoteIP(winner); ip != "" {
                                s.markGoodIP(ip, isGvideoHost(host))
                        }
                }
                if winner == nil {
                        // M8-aa (ADR-0028): the raced round died on every candidate
                        // it dialed — one budget spent, the app's own retry wave the
                        // next cost (its backoff, a fresh resolve, a fresh gamble:
                        // the debug.18 report's top failing host ran 13 flows for 8
                        // failures). Before the terminal, ONE more raced round over
                        // exactly the answer set's UNTRIED, unboxed origins — the
                        // rotations covered only the round's slice of a mixed answer
                        // set (live and blackholed cache nodes together, the report's
                        // whole remaining cold-start cost). The replay stays
                        // byte-exact, the client stays frozen exactly as the round
                        // kept it, and the app never sees the failure. A set with
                        // nothing untried keeps the fast M8-w terminal — a hopeless
                        // host must not burn a second budget.
                        untried := s.untriedIPs(host, dialedIPs)
                        if len(untried) > 0 && replay.fits() {
                                s.logTunnelFirstByteRetry(host, class, lastDeath, 1)
                                link.swap(nil) // the round already closed the primary's corpse
                                salv := s.raceIPList(host, port, untried, len(untried), replay.bytes(), len(first), isTLS, preset, fcfg)
                                if salv.conn != nil {
                                        totalDials := raceN + 1 + len(untried)
                                        tc.setB(salv.conn)   // Stop's sweep must hit the live origin
                                        link.swap(salv.conn)      // the salvage winner owns the flow now
                                        if err := writeFull(client, salv.first, s.cfg.IdleTimeout); err != nil {
                                                // The origin spoke but the client is gone: a
                                                // client-side death, not a suspect-origin
                                                // signal (the M8-s branch — no demotion).
                                                link.swap(nil)
                                                tc.closeBoth()
                                                s.countConnError()
                                                s.logTunnelDone(host, class, time.Since(tstart), flow)
                                                return
                                        }
                                        s.stats.AddBytesDown(int64(len(salv.first)))
                                        flow.down.Add(int64(len(salv.first)))
                                        onFirst()
                                        s.logTunnelFirstByteSalvaged(host, class, totalDials)
                                        relayEntry(salv.conn)
                                        return
                                }
                                if salv.death != "" {
                                        lastDeath = salv.death
                                }
                        }
                        s.invalidateDNS(host) // terminal: the set connects but never speaks (M8-s)
                        failFlow(lastDeath)
                        return
                }
                if winner != upstream {
                        tc.setB(winner)   // Stop's sweep must hit the live origin
                        link.swap(winner) // closes the primary — it lost
                }
                if err := writeFull(client, winnerFirst, s.cfg.IdleTimeout); err != nil {
                        // The origin spoke but the client is gone: a
                        // client-side death, not a suspect-origin signal
                        // (the M8-s branch — no demotion).
                        link.swap(nil)
                        tc.closeBoth()
                        s.countConnError()
                        s.logTunnelDone(host, class, time.Since(tstart), flow)
                        return
                }
                s.stats.AddBytesDown(int64(len(winnerFirst)))
                flow.down.Add(int64(len(winnerFirst)))
                onFirst()
                if substWin {
                        s.logTunnelFirstByteSubstituted(host, class, len(subst))
                } else if winner != upstream {
                        s.logTunnelFirstByteRaced(host, class, raceN)
                }
                relayEntry(winner)
                return
        }

        origin := upstream
        countedFirst := false
        redials := 0
        for {
                // (Re)write the buffered client bytes upstream — attempt 0
                // writes the first record; a salvage replays it (and any
                // early data captured since) onto the fresh origin,
                // re-fragmented identically.
                if err := s.writeReplay(origin, replay.bytes(), len(first), isTLS, preset, fcfg); err != nil {
                        failFlow(firstByteReason(err))
                        return
                }
                if !countedFirst {
                        countedFirst = true
                        if len(first) > 0 {
                                s.stats.AddBytesUp(int64(len(first)))
                                flow.up.Add(int64(len(first)))
                        }
                }
                if !pumpRunning {
                        startPump()
                }

                buf := getBuf()
                origin.SetReadDeadline(time.Now().Add(s.cfg.FirstByteTimeout))
                n, rerr := origin.Read(*buf)
                origin.SetReadDeadline(time.Time{})
                if n > 0 {
                        // The origin spoke. Deliver the first bytes, then hand
                        // the client reads to the plain two-goroutine relay
                        // (the pump's reads are all delivered; the shape from
                        // here is byte-for-byte the plain copy's). The spoke
                        // origin is good-pool evidence (M8-ab, ADR-0031).
                        if ip := remoteIP(origin); ip != "" {
                                s.markGoodIP(ip, isGvideoHost(host))
                        }
                        if err := writeFull(client, (*buf)[:n], s.cfg.IdleTimeout); err != nil {
                                putBuf(buf)
                                // The origin spoke but the client is gone: a
                                // client-side death, not a suspect-origin
                                // signal (the M8-s branch — no demotion).
                                link.swap(nil)
                                if pumpRunning {
                                        stopClientPump(client, pumpStop, pumpDone)
                                        pumpRunning = false
                                }
                                tc.closeBoth()
                                s.countConnError()
                                s.logTunnelDone(host, class, time.Since(tstart), flow)
                                return
                        }
                        s.stats.AddBytesDown(int64(n))
                        flow.down.Add(int64(n))
                        putBuf(buf)
                        onFirst()
                        if redials > 0 {
                                s.logTunnelFirstByteSalvaged(host, class, redials)
                        }
                        if !stopClientPump(client, pumpStop, pumpDone) {
                                // Pathological pump stop: never risk a second
                                // reader — the flow dies here, honestly.
                                tc.closeBoth()
                                s.countConnError()
                                s.logTunnelDone(host, class, time.Since(tstart), flow)
                                return
                        }
                        pumpRunning = false
                        relayEntry(origin)
                        return
                }
                putBuf(buf)

                // Dead origin (n == 0, rerr != nil): reset, closed, or the
                // budget's timeout. Salvage while attempts and replay remain.
                reason := firstByteReason(rerr)
                if ip := remoteIP(origin); ip != "" {
                        deadIPs[ip] = true
                        // M8-ac (ADR-0034): the death's class weights the mark — the
                        // knowledge outlives the flow either way, but a reset is now a
                        // temporal signal, not an IP verdict.
                        s.markDeadIPClass(ip, reason)
                }
                if redials >= maxRedial || !replay.fits() {
                        s.invalidateDNS(host) // terminal: the set connects but never speaks (M8-s)
                        failFlow(reason)
                        return
                }
                // M8-v (ADR-0020): the remaining salvage re-dials are RACED
                // in parallel, not walked serially. The debug.13 report's
                // terminal failures were the serial chain's cost — 21
                // timeout dones at 8014-8227 ms, three 2.5 s budgets plus
                // dials, every one an app retry wave away from a fresh
                // origin. Raced, the same attempts cover the live answer set
                // in ONE budget's wall time: candidates dial distinct
                // origins immediately (the flow's dead IPs excluded, the
                // lists rotated so every candidate starts elsewhere), the
                // first origin to speak wins, the losers are closed and
                // reaped, and a silent-host terminal costs ~2x the budget
                // (attempt 0 + the race), never 3x.
                raced := maxRedial - redials
                redials = maxRedial
                s.invalidateDNS(host) // the set that went silent is suspect (the M8-s loop)
                for attempt := 1; attempt <= raced; attempt++ {
                        s.logTunnelFirstByteRetry(host, class, reason, attempt)
                }
                // Close the dead origin first (unblocks a pump write stuck
                // on it), then stop the pump — the client's bytes during the
                // race wait in the socket buffer and flow to the winner in
                // order, right after the replay it already holds.
                link.swap(nil)
                if !stopClientPump(client, pumpStop, pumpDone) {
                        failFlow(reason)
                        return
                }
                pumpRunning = false
                origin = nil
                ips, rerr2 := s.resolve(host)
                if rerr2 != nil {
                        s.invalidateDNS(host) // the re-dial's set failed to resolve too (the M8-o rule)
                        failFlow("dial")
                        return
                }
                ips = s.liveIPs(dedupeIPs(excludeDead(ips, deadIPs))) // the box adds its cross-flow memory (M8-y, ADR-0023)
                snapshot := replay.bytes()

                // The candidates. Each dials its rotated start of the live
                // set (DialHost walks the whole list with its stagger, so a
                // blackholed address still fails over inside the candidate),
                // replays the buffered client bytes byte-exact through a
                // fresh fragment conn, and waits one budget for the first
                // byte. First speaker wins. (M8-aa, ADR-0028: the candidate
                // core now lives in raceIPList — the raced path's salvage
                // round runs the identical machinery.)
                salv := s.raceIPList(host, port, ips, raced, snapshot, len(first), isTLS, preset, fcfg)
                if salv.conn == nil {
                        death := salv.death
                        if death == "" {
                                death = reason
                        }
                        failFlow(death)
                        return
                }
                winner, winnerFirst := salv.conn, salv.first
                origin = winner
                tc.setB(winner)
                link.swap(winner)
                // The winner spoke: deliver its first bytes, log the save,
                // and enter the relay (the pump is already stopped — the
                // race stopped it — so the client's buffered bytes flow to
                // the winner through the plain copy).
                if err := writeFull(client, winnerFirst, s.cfg.IdleTimeout); err != nil {
                        // The origin spoke but the client is gone: a
                        // client-side death, not a suspect-origin signal
                        // (the M8-s branch — no demotion).
                        link.swap(nil)
                        tc.closeBoth()
                        s.countConnError()
                        s.logTunnelDone(host, class, time.Since(tstart), flow)
                        return
                }
                s.stats.AddBytesDown(int64(len(winnerFirst)))
                flow.down.Add(int64(len(winnerFirst)))
                onFirst()
                s.logTunnelFirstByteSalvaged(host, class, redials)
                relayEntry(winner)
                return
        }
}

// copyDir moves bytes one way, counting them globally and on the flow
// meter; onFirst (when non-nil) fires once, on the first downstream read.
func (s *Server) copyDir(src io.Reader, dst io.Writer, isUp bool, m *flowMeter, onFirst func()) {
        buf := getBuf()
        defer putBuf(buf)
        firstDone := false
        for {
                n, rerr := src.Read(*buf)
                if n > 0 {
                        if !isUp && onFirst != nil && !firstDone {
                                firstDone = true
                                onFirst()
                        }
                        if _, werr := dst.Write((*buf)[:n]); werr != nil {
                                if !isEOF(rerr) {
                                        s.countConnError()
                                }
                                return
                        }
                        if isUp {
                                s.stats.AddBytesUp(int64(n))
                                m.up.Add(int64(n))
                        } else {
                                s.stats.AddBytesDown(int64(n))
                                m.down.Add(int64(n))
                        }
                }
                if rerr != nil {
                        if !isEOF(rerr) && rerr != nil {
                                s.countConnError()
                        }
                        return
                }
        }
}

func isEOF(err error) bool {
        return err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}

// readFirstRecord reads the first client-to-server message as one unit when
// it is a TLS handshake record: the 5-byte header (0x16, 0x03, version,
// 2-byte length) plus exactly that many bytes, capped at 16 KB + 5
// (file 01, section 5). Anything else passes through untouched.
func readFirstRecord(r io.Reader) ([]byte, bool, error) {
        head := make([]byte, 5)
        n, err := io.ReadFull(r, head)
        if err != nil {
                if errors.Is(err, io.ErrUnexpectedEOF) && n == 0 {
                        return nil, false, io.EOF
                }
                return head[:n], false, err
        }
        if head[0] != 0x16 || head[1] != 0x03 {
                // Not a TLS handshake: pass these bytes through untouched.
                return head, false, nil
        }
        length := int(head[3])<<8 | int(head[4])
        if length > config.MaxTLSRecord-5 {
                length = config.MaxTLSRecord - 5 // cap the unit; the rest flows normally
        }
        body := make([]byte, length)
        if _, err := io.ReadFull(r, body); err != nil {
                return append(head, body...), true, err
        }
        return append(head, body...), true, nil
}

func splitHostPort(target string) (string, string, error) {
        return net.SplitHostPort(target)
}

// closeWriter is implemented by *net.TCPConn; half-closing tells the other
// side no more data will come from us (file 01, section 7 step 6).
type closeWriter interface{ CloseWrite() error }

func closeWrite(c net.Conn) {
        if cw, ok := c.(closeWriter); ok {
                cw.CloseWrite()
        }
}
