package proxy

import (
        "context"
        "errors"
        "fmt"
        "io"
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
// silent for the whole StallTimeout (M8-v, ADR-0020). The debug.13
// report's stall cohort — first byte plus ~7 KB down, then total
// silence until the app's ~60 s patience — gets a bounded engine-side
// death instead: both ends closed (the app's read errors NOW, so its
// retry lands on a fresh origin with its own budget + salvage in
// seconds), the answer set demoted like every other dead-origin signal
// (the M8-o loop), the error counted, the stall line logged for the
// next report. One-direction silence never fires: a pure download
// (client silent) and a pure upload (origin silent) are healthy relay
// shapes, and only a whole-flow stall is dead.
func (s *Server) watchStall(host string, tc *tunnelCloser, lastUp, lastDown *atomic.Int64, stop chan struct{}) {
        tick := s.cfg.StallTimeout / 4
        if tick > 500*time.Millisecond {
                tick = 500 * time.Millisecond
        }
        if tick < 50*time.Millisecond {
                tick = 50 * time.Millisecond
        }
        limit := s.cfg.StallTimeout.Milliseconds()
        t := time.NewTicker(tick)
        defer t.Stop()
        for {
                select {
                case <-stop:
                        return
                case <-t.C:
                        now := time.Now().UnixMilli()
                        up, down := lastUp.Load(), lastDown.Load()
                        // The NEWEST stamp decides: the stall fires only when
                        // BOTH directions have been silent the whole budget —
                        // now minus the younger activity clock. (The min would
                        // fire on a pure download whose client never writes;
                        // exactly the shape the watch must spare.)
                        newest := up
                        if down > newest {
                                newest = down
                        }
                        idle := now - newest
                        if idle >= limit {
                                tc.closeBoth()
                                s.invalidateDNS(host)
                                s.countConnError()
                                s.logTunnelStallKilled(host, time.Duration(idle)*time.Millisecond)
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

// logTunnelUp records one established flow's setup phases: the secure
// DNS lookup and the origin dial. It lands in the engine's log ring and
// therefore in the redacted diagnostics report (the ring handler drops
// attrs, so the values ride in the message itself).
func (s *Server) logTunnelUp(host string, dnsMS, dialMS int64) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel up dns=%dms dial=%dms", dnsMS, dialMS)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelFirstByte records how long the first origin→client byte took
// after the tunnel was established (the TLS handshake and the start of
// the origin's response, both through the fragment path when routed).
func (s *Server) logTunnelFirstByte(host string, d time.Duration) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte %dms", d.Milliseconds())
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
func (s *Server) logTunnelFirstByteFailed(host, reason string) {
        if s.cfg.Logger == nil {
                return
        }
        msg := "tunnel first-byte failed reason=" + reason
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// logTunnelFirstByteRetry records one salvage re-dial (M8-t, ADR-0019):
// the attempt that just died, absorbed inside the flow — the app never
// sees it. Warn, so the report's log ring keeps it distinct from the
// terminal failure line.
func (s *Server) logTunnelFirstByteRetry(host, reason string, attempt int) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte retry reason=%s attempt=%d", reason, attempt)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Warn(msg)
}

// logTunnelFirstByteSalvaged records a flow whose origin answered only
// after re-dials (M8-t): the evidence a salvage saved a video start the
// app would otherwise have retried itself.
func (s *Server) logTunnelFirstByteSalvaged(host string, dials int) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel first-byte salvaged dials=%d", dials)
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

// logTunnelStallKilled records the M8-v stall kill (ADR-0020): a
// budgeted flow whose relay went silent in BOTH directions for the
// whole StallTimeout — the debug.13 report's dominant start-lag shape
// (five video-start flows stalled mid-stream at ~7 KB down until the
// app's own ~60 s patience killed them). The line rides in the
// report's ring like every other tunnel line; the done line that
// follows carries the flow's totals.
func (s *Server) logTunnelStallKilled(host string, idle time.Duration) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel stall killed idle=%dms", idle.Milliseconds())
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
func (s *Server) logTunnelDone(host string, d time.Duration, m *flowMeter) {
        if s.cfg.Logger == nil {
                return
        }
        msg := fmt.Sprintf("tunnel done dur=%dms up=%s down=%s",
                d.Milliseconds(), config.FormatBytes(m.up.Load()), config.FormatBytes(m.down.Load()))
        if s.cfg.LogHostnames {
                msg += " host=" + host
        }
        s.cfg.Logger.Info(msg)
}

func (s *Server) runBidirectionalCopy(client *bufferedConn, upstream net.Conn, host string, port int, fragmentTLS bool) {
        tstart := time.Now()
        flow := &flowMeter{}
        var firstOnce sync.Once
        logFirst := func() {
                firstOnce.Do(func() { s.logTunnelFirstByte(host, time.Since(tstart)) })
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

        // Copy phase. A processed host with a first-byte budget runs the
        // budgeted shape (ADR-0018/0019) — which owns the first record's
        // write, because a salvage re-dial has to replay it; every other
        // tunnel keeps the plain two-goroutine relay, byte-for-byte the
        // pre-M8-s behavior.
        if fragmentTLS && s.cfg.FirstByteTimeout > 0 {
                s.copyFirstByteBudgeted(client, upstream, host, port, flow, tstart, logFirst, tcloser, first, isTLS)
                return
        }
        var up io.Writer = upstream
        if isTLS && fragmentTLS {
                preset, fcfg := s.fragmentData()
                fconn := fragment.NewConn(upstream, preset, fcfg, s.cfg.RNG)
                up = fconn
        }
        if len(first) > 0 {
                if _, err := up.Write(first); err != nil {
                        s.countConnError()
                        return
                }
                s.stats.AddBytesUp(int64(len(first)))
                flow.up.Add(int64(len(first)))
        }
        s.copyPlain(client, upstream, host, flow, tstart, logFirst)
}

// copyPlain is the original relay shape: both directions copy concurrently
// from the start; the first downstream read fires onFirst (the first-byte
// line). Every tunnel without a first-byte budget runs this — unrouted
// relays and budget-disabled runs keep the pre-M8-s behavior exactly.
func (s *Server) copyPlain(client *bufferedConn, upstream net.Conn, host string, flow *flowMeter, tstart time.Time, onFirst func()) {
        // Idle-wrapped ends for the copy phase.
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
        s.logTunnelDone(host, time.Since(tstart), flow)
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

// copyFirstByteBudgeted is the processed-host relay shape (M8-s,
// ADR-0018; M8-t, ADR-0019; M8-v, ADR-0020): after the first record
// went upstream, the origin's first byte must arrive within
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
// patience). When the retries run out, the flow fails FAST exactly as
// at M8-s: both ends closed, the error counted, "tunnel first-byte
// failed reason=..." in the report, the answer set demoted. Absorbed
// attempts log "tunnel first-byte retry reason=... attempt=N"; a flow
// saved by them logs "tunnel first-byte salvaged dials=N". Unrouted
// relays never enter this shape (the D46 scoping).
//
// Shape: the pump (pumpClient) reads the client and writes live to the
// current origin through the swap-able link, so a chatty client is
// never blocked behind the budget; the main goroutine reads the
// origin's first byte under the deadline. Once the origin speaks, the
// pump is stopped (its reads are all delivered) and the plain
// two-goroutine relay takes over byte-for-byte — under the stall
// watch, which is the only M8-v addition to the relay itself.
func (s *Server) copyFirstByteBudgeted(client *bufferedConn, upstream net.Conn, host string, port int, flow *flowMeter, tstart time.Time, onFirst func(), tc *tunnelCloser, first []byte, isTLS bool) {
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
        failFlow := func(reason string) {
                link.swap(nil) // unblocks a pump stuck writing before anything else
                if pumpRunning {
                        stopClientPump(client, pumpStop, pumpDone)
                        pumpRunning = false
                }
                tc.closeBoth()
                s.countConnError()
                s.logTunnelFirstByteFailed(host, reason)
                s.logTunnelDone(host, time.Since(tstart), flow)
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
                        go s.watchStall(host, tc, lastUp, lastDown, stallStop)
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
                s.logTunnelDone(host, time.Since(tstart), flow)
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
                        // here is byte-for-byte the plain copy's).
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
                                s.logTunnelDone(host, time.Since(tstart), flow)
                                return
                        }
                        s.stats.AddBytesDown(int64(n))
                        flow.down.Add(int64(n))
                        putBuf(buf)
                        onFirst()
                        if redials > 0 {
                                s.logTunnelFirstByteSalvaged(host, redials)
                        }
                        if !stopClientPump(client, pumpStop, pumpDone) {
                                // Pathological pump stop: never risk a second
                                // reader — the flow dies here, honestly.
                                tc.closeBoth()
                                s.countConnError()
                                s.logTunnelDone(host, time.Since(tstart), flow)
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
                if ip := remoteIP(origin); ip != "" {
                        deadIPs[ip] = true
                }
                raced := maxRedial - redials
                redials = maxRedial
                s.invalidateDNS(host) // the set that went silent is suspect (the M8-s loop)
                for attempt := 1; attempt <= raced; attempt++ {
                        s.logTunnelFirstByteRetry(host, reason, attempt)
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
                ips = dedupeIPs(excludeDead(ips, deadIPs))
                snapshot := replay.bytes()

                // The candidates. Each dials its rotated start of the live
                // set (DialHost walks the whole list with its stagger, so a
                // blackholed address still fails over inside the candidate),
                // replays the buffered client bytes byte-exact through a
                // fresh fragment conn, and waits one budget for the first
                // byte. First speaker wins.
                type salvageResult struct {
                        conn  net.Conn
                        first []byte
                        death string
                }
                results := make(chan salvageResult, raced)
                for i := 0; i < raced; i++ {
                        list := rotateIPs(ips, i)
                        go func(list []net.IP) {
                                ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeoutOverall)
                                defer cancel()
                                conn, derr := s.cfg.Dialer.DialHost(ctx, "tcp", host, port, list)
                                if derr != nil {
                                        results <- salvageResult{death: "dial"}
                                        return
                                }
                                if werr := s.writeReplay(conn, snapshot, len(first), isTLS, preset, fcfg); werr != nil {
                                        conn.Close()
                                        results <- salvageResult{death: firstByteReason(werr)}
                                        return
                                }
                                cbuf := getBuf()
                                conn.SetReadDeadline(time.Now().Add(s.cfg.FirstByteTimeout))
                                cn, cerr := conn.Read(*cbuf)
                                conn.SetReadDeadline(time.Time{})
                                if cn > 0 {
                                        firstDown := append([]byte(nil), (*cbuf)[:cn]...)
                                        putBuf(cbuf)
                                        results <- salvageResult{conn: conn, first: firstDown}
                                        return
                                }
                                putBuf(cbuf)
                                conn.Close()
                                results <- salvageResult{death: firstByteReason(cerr)}
                        }(list)
                }

                // Collect: the first speaking origin wins; the still-pending
                // candidates are reaped in the background (their conns
                // closed, their late deaths still demoted — one per dead
                // attempt, the M8-o loop).
                var winner net.Conn
                var winnerFirst []byte
                deaths := 0
                lastDeath := reason
                for {
                        res := <-results
                        if res.conn != nil {
                                winner = res.conn
                                winnerFirst = res.first
                                break
                        }
                        deaths++
                        lastDeath = res.death
                        s.invalidateDNS(host)
                        if deaths == raced {
                                failFlow(lastDeath)
                                return
                        }
                }
                go func(pending int) {
                        for i := 0; i < pending; i++ {
                                res := <-results
                                if res.conn != nil {
                                        res.conn.Close()
                                } else {
                                        s.invalidateDNS(host)
                                }
                        }
                }(raced - deaths - 1)
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
                        s.logTunnelDone(host, time.Since(tstart), flow)
                        return
                }
                s.stats.AddBytesDown(int64(len(winnerFirst)))
                flow.down.Add(int64(len(winnerFirst)))
                onFirst()
                s.logTunnelFirstByteSalvaged(host, redials)
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
