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
	// FallbackDelay is how long one address attempt may hold the
	// stage before the NEXT address is started alongside it
	// (Happy-Eyeballs style, RFC 8305's reasoning: a filtered
	// network that blackholes the first address silently should
	// not cost a full PerIPTimeout per address). Zero → the
	// package default (DefaultFallbackDelay). The first attempt
	// always starts immediately; attempt k starts at k×
	// FallbackDelay; the first connection that completes wins and
	// the losers are canceled (M8-n, ADR-0013).
	FallbackDelay time.Duration
}

// DefaultFallbackDelay staggers parallel address attempts. 750 ms is
// far above any healthy connect RTT (so a working first address is
// always alone in flight) and far below DialTimeoutPerIP (5 s), so a
// blackholed front address costs ~0.75 s + RTT instead of a full
// silent 5 s wait per address.
const DefaultFallbackDelay = 750 * time.Millisecond

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

// DialHost tries the addresses with staggered parallel attempts
// (file 03 R4's "try every address" with fast failover, M8-n/ADR-0013):
// attempt 0 starts immediately, attempt k starts k×FallbackDelay later,
// every attempt keeps its full PerIPTimeout, and the first connection
// that completes wins — the losers are canceled. host is used only in
// the error message, never dialed directly.
func (d *Dialer) DialHost(ctx context.Context, network string, host string, port int, ips []net.IP) (net.Conn, error) {
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses to dial for %s", safeHost(host))
	}
	if d.OverallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.OverallTimeout)
		defer cancel()
	}
	fallback := d.FallbackDelay
	if fallback <= 0 {
		fallback = DefaultFallbackDelay
	}

	// stop cancels the moment a winner exists; every goroutine then
	// unwinds (in-flight DialIPs abort on ctx, pending stagger
	// timers exit) and reports into the buffered channel, so the
	// collector below always reads exactly len(ips) results.
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	type attempt struct {
		conn net.Conn
		err  error
	}
	results := make(chan attempt, len(ips))
	for i, ip := range ips {
		go func(i int, ip net.IP) {
			if i > 0 {
				timer := time.NewTimer(time.Duration(i) * fallback)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					results <- attempt{err: ctx.Err()}
					return
				}
			}
			conn, err := d.DialIP(ctx, network, ip, port)
			results <- attempt{conn: conn, err: err}
		}(i, ip)
	}

	var lastErr error
	var winner net.Conn
	for range ips {
		a := <-results
		if a.err == nil {
			if winner == nil {
				winner = a.conn
				stop() // losers: cancel what is pending, close what landed
			} else {
				_ = a.conn.Close() // a parallel success that lost the race
			}
			continue
		}
		lastErr = a.err
	}
	if winner != nil {
		return winner, nil
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
