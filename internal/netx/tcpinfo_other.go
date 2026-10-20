//go:build !linux

// The non-Linux no-op (M8-af, ADR-0043, spec W1 "+ no-op for other
// platforms"): Windows desktops and every other build keep their
// behavior byte-for-byte — googlevideo flows there simply never produce
// tcp evidence, callers log "tcpinfo=unavailable" once, and the report
// counts the flows as unknown instead of guessing.
package netx

import "net"

func snapshotTCPInfoPlatform(c net.Conn) (TCPInfo, error) {
	return TCPInfo{}, ErrTCPInfoUnavailable
}
