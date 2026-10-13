// Package netx is the one place outbound sockets are created (file 01,
// section 3: "One outbound door"). Every upstream connection and every DoH
// connection goes through this dialer. The optional Control hook lets a
// future Android build exempt our own sockets from its tunnel; desktop
// leaves it nil.
package netx

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
)

// ControlFunc matches net.Dialer's Control hook so Android can call
// VpnService.protect(fd) before the socket connects. Nil on desktop.
type ControlFunc = func(network, address string, c syscall.RawConn) error

// Dialer dials website addresses with per-IP and overall timeouts.
type Dialer struct {
	// PerIPTimeout bounds one address attempt (default from config).
	PerIPTimeout time.Duration
	// OverallTimeout bounds the whole try-every-address run.
	OverallTimeout time.Duration
	// Control is the optional socket hook (nil on desktop).
	Control ControlFunc
}

// New returns a dialer with the given timeouts and optional control hook.
func New(perIP, overall time.Duration, control ControlFunc) *Dialer {
	return &Dialer{PerIPTimeout: perIP, OverallTimeout: overall, Control: control}
}

// DialIP dials one address (ip:port) honoring the per-IP timeout and ctx.
func (d *Dialer) DialIP(ctx context.Context, network string, ip net.IP, port int) (net.Conn, error) {
	nd := net.Dialer{Timeout: d.PerIPTimeout, Control: d.Control}
	addr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
	return nd.DialContext(ctx, network, addr)
}

// DialHost tries every address in order (file 03, R4) within the overall
// timeout, returning the first connection that succeeds. host is used only
// in the error message, never dialed directly.
func (d *Dialer) DialHost(ctx context.Context, network string, host string, port int, ips []net.IP) (net.Conn, error) {
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses to dial for %s", safeHost(host))
	}
	if d.OverallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.OverallTimeout)
		defer cancel()
	}
	var lastErr error
	for _, ip := range ips {
		if ctx.Err() != nil {
			break
		}
		conn, err := d.DialIP(ctx, network, ip, port)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = ctx.Err()
	}
	return nil, fmt.Errorf("all %d addresses failed for %s: %w", len(ips), safeHost(host), lastErr)
}

// safeHost keeps hostnames out of error strings unless the caller enables it;
// errors are logged, and file 03 R21 keeps names out of logs by default.
// The engine replaces this with a redacting wrapper when logDomains is off;
// here we keep the address family only.
func safeHost(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return "address"
	}
	return "host"
}
