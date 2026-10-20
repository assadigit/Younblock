//go:build linux

// The Linux TCP_INFO implementation (M8-af, ADR-0043, spec W1): the
// kernel's own per-socket counters, read through the raw fd the same
// way the keepalive round (M8-ad, ADR-0037) reads its options back —
// SyscallConn.Control hands the fd, GetsockoptTCPInfo fills the struct.
// No packet capture, no new permissions: the socket is already ours.
package netx

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func snapshotTCPInfoPlatform(c net.Conn) (TCPInfo, error) {
	if c == nil {
		return TCPInfo{}, ErrTCPInfoUnavailable
	}
	// *net.TCPConn (the dialer's product) carries SyscallConn; wrapper
	// chains do not — callers pass the raw origin conn.
	sc, ok := c.(syscall.Conn)
	if !ok {
		return TCPInfo{}, ErrTCPInfoUnavailable
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return TCPInfo{}, ErrTCPInfoUnavailable
	}
	var info TCPInfo
	var ierr error
	ctrl := func(fd uintptr) {
		ti, err := unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
		if err != nil {
			ierr = err
			return
		}
		info = TCPInfo{
			Acked:      ti.Bytes_acked,
			Sent:       ti.Bytes_sent,
			Retrans:    ti.Bytes_retrans,
			Unacked:    ti.Unacked,
			RttMS:      uint64(ti.Rtt) / 1000, // kernel: microseconds
			MSS:        ti.Snd_mss,
			Rcvd:       ti.Bytes_received,
			LastRecvMS: ti.Last_data_recv, // kernel: milliseconds-ago
			LastAckMS:  ti.Last_ack_recv,  // kernel: milliseconds-ago
		}
	}
	if err := raw.Control(ctrl); err != nil {
		return TCPInfo{}, ErrTCPInfoUnavailable
	}
	if ierr != nil {
		return TCPInfo{}, ierr
	}
	return info, nil
}
