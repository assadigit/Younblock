// socks5.go — the client half of the door (the server half is
// internal/proxy/socks5.go). One dial, one CONNECT, minimal surface:
// exactly what the stack needs and nothing more.
package tunstack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// SOCKS5 wire constants (RFC 1928) — client side.
const (
	socks5Hello  = byte(0x05)
	socks5NoAuth = byte(0x00)
	socks5Conn   = byte(0x01)
	socks5Atyp4  = byte(0x01)
	socks5AtypDN = byte(0x03)
	socks5Atyp6  = byte(0x04)
)

// errSocks5 marks protocol failures from the door.
var errSocks5 = errors.New("socks5")

// socks5Dial opens one CONNECT tunnel through the door and returns the
// established connection. target may be a domain, an IPv4 or an IPv6
// literal — the address type is chosen accordingly. The ctx bounds the
// whole handshake; the returned conn carries no deadlines (the splice
// and the door's own idle rules own them from here).
func socks5Dial(ctx context.Context, door, target string, port int) (net.Conn, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", door)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// Greeting: VER 05, one method, no-auth.
	if _, err := conn.Write([]byte{socks5Hello, 0x01, socks5NoAuth}); err != nil {
		conn.Close()
		return nil, err
	}
	var method [2]byte
	if _, err := io.ReadFull(conn, method[:]); err != nil {
		conn.Close()
		return nil, err
	}
	if method[0] != socks5Hello || method[1] != socks5NoAuth {
		conn.Close()
		return nil, fmt.Errorf("%w: door refused no-auth (%02x %02x)", errSocks5, method[0], method[1])
	}

	// Request: CONNECT with the address type implied by target.
	req := []byte{socks5Hello, socks5Conn, 0x00}
	if ip := net.ParseIP(target); ip != nil && ip.To4() != nil {
		req = append(req, socks5Atyp4)
		req = append(req, ip.To4()...)
	} else if ip := net.ParseIP(target); ip != nil {
		req = append(req, socks5Atyp6)
		req = append(req, ip.To16()...)
	} else {
		if len(target) == 0 || len(target) > 255 {
			conn.Close()
			return nil, fmt.Errorf("%w: bad target length %d", errSocks5, len(target))
		}
		req = append(req, socks5AtypDN, byte(len(target)))
		req = append(req, target...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	// Reply: VER REP RSV ATYP BND.ADDR BND.PORT.
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		conn.Close()
		return nil, err
	}
	if head[0] != socks5Hello {
		conn.Close()
		return nil, fmt.Errorf("%w: reply version %02x", errSocks5, head[0])
	}
	if head[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("%w: door refused, REP %02x", errSocks5, head[1])
	}
	var skip []byte
	switch head[3] {
	case socks5Atyp4:
		skip = make([]byte, 4+2)
	case socks5AtypDN:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			conn.Close()
			return nil, err
		}
		skip = make([]byte, int(n[0])+2)
	case socks5Atyp6:
		skip = make([]byte, 16+2)
	default:
		conn.Close()
		return nil, fmt.Errorf("%w: reply ATYP %02x", errSocks5, head[3])
	}
	if _, err := io.ReadFull(conn, skip); err != nil {
		conn.Close()
		return nil, err
	}

	// Established.
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
