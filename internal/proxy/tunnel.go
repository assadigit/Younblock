package proxy

import (
        "errors"
        "fmt"
        "io"
        "net"
        "sync"
        "sync/atomic"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/fragment"
)

// tunnelCloser closes both sockets of a tunnel exactly once (file 03, R2).
type tunnelCloser struct {
        once sync.Once
        a    net.Conn
        b    net.Conn
}

func (t *tunnelCloser) closeBoth() {
        t.once.Do(func() {
                if t.a != nil {
                        t.a.Close()
                }
                if t.b != nil {
                        t.b.Close()
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

func (s *Server) runBidirectionalCopy(client *bufferedConn, upstream net.Conn, host string, fragmentTLS bool) {
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
                s.copyDir(uIdle, cIdle, false, flow, logFirst) // upstream -> client counts as down
                closeWrite(client.Conn)
                done <- struct{}{}
        }()
        <-done
        <-done
        s.logTunnelDone(host, time.Since(tstart), flow)
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
