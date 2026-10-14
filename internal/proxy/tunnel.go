package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
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
func (s *Server) runBidirectionalCopy(client *bufferedConn, upstream net.Conn, host string, fragmentTLS bool) {
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
	}

	// Idle-wrapped ends for the copy phase.
	cIdle := &idleConn{Conn: client, timeout: s.cfg.IdleTimeout}
	uIdle := &idleConn{Conn: upstream, timeout: s.cfg.IdleTimeout}

	done := make(chan struct{}, 2)
	go func() {
		s.copyDir(cIdle, uIdle, true) // client -> upstream counts as up
		closeWrite(upstream)
		done <- struct{}{}
	}()
	go func() {
		s.copyDir(uIdle, cIdle, false) // upstream -> client counts as down
		closeWrite(client.Conn)
		done <- struct{}{}
	}()
	<-done
	<-done
}

// copyDir moves bytes one way, counting them.
func (s *Server) copyDir(src io.Reader, dst io.Writer, isUp bool) {
	buf := getBuf()
	defer putBuf(buf)
	for {
		n, rerr := src.Read(*buf)
		if n > 0 {
			if _, werr := dst.Write((*buf)[:n]); werr != nil {
				if !isEOF(rerr) {
					s.countConnError()
				}
				return
			}
			if isUp {
				s.stats.AddBytesUp(int64(n))
			} else {
				s.stats.AddBytesDown(int64(n))
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
