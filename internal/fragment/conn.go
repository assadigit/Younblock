package fragment

import (
	"net"
	"sync"
	"time"
)

// Conn is a net.Conn wrapper whose first Write is sent as several small
// writes with a pause between them (file 01, section 5). Everything after
// the first Write passes through untouched.
//
// Go enables TCP_NODELAY by default and this package never touches it, so
// the OS cannot silently merge the pieces back together (file 03, R10).
type Conn struct {
	net.Conn
	preset Preset
	cfg    Config
	rng    RNG

	mu   sync.Mutex
	done bool
}

// NewConn wraps inner so its first Write is fragmented per preset.
func NewConn(inner net.Conn, preset Preset, cfg Config, rng RNG) *Conn {
	if !preset.Valid() {
		preset = Balanced
	}
	if rng == nil {
		rng = NewDefaultRNG()
	}
	return &Conn{Conn: inner, preset: preset, cfg: cfg, rng: rng}
}

// Write fragments the first call, then forwards everything.
func (c *Conn) Write(b []byte) (int, error) {
	c.mu.Lock()
	first := !c.done
	c.done = true
	c.mu.Unlock()
	if !first || len(b) == 0 {
		return c.Conn.Write(b)
	}
	pieces := Plan(len(b), c.preset, c.rng, c.cfg)
	if len(pieces) <= 1 {
		piecesTotal.Add(1) // one segment, still the fragmenter's write (M8-af, W9)
		return writeAll(c.Conn, b)
	}
	off := 0
	for i, size := range pieces {
		if i > 0 && c.cfg.Pause > 0 {
			time.Sleep(c.cfg.Pause)
		}
		n, err := writeAll(c.Conn, b[off:off+size])
		off += n
		if err != nil {
			return off, err
		}
		piecesTotal.Add(1) // one TCP segment on the wire per piece (M8-af, W9)
	}
	return off, nil
}

func writeAll(conn net.Conn, b []byte) (int, error) {
	total := 0
	for total < len(b) {
		n, err := conn.Write(b[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
