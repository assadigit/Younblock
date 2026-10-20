// Package netx — TCPInfo is the kernel-level evidence snapshot one
// googlevideo flow's origin socket carries (M8-af, ADR-0043, spec W1):
// the debug.24 round's decisive measurement. Six rounds of field reports
// proved the TLS handshake completes and the request is sent while the
// media body never arrives — but no measurement ever said WHETHER the
// request was even ACKed by the far side. Two snapshots of the kernel's
// own TCP counters answer it without any packet capture:
//
//   - Snapshot A, right after the client's first post-handshake bytes
//     (the request) were written to the origin: the socket's
//     pre-request/early-request state.
//   - Snapshot B, when the flow ends (done, stall kill, error).
//
// If BytesAcked at B did not move past A while retransmits and unacked
// grew, the request never reached the server (upstream loss — the
// censor drops our data after the handshake). If acked moved but
// BytesReceived did not, the request arrived and the answer is being
// dropped or throttled (downstream loss). Both moved and the flow still
// starved: the server or HTTP layer. The classification lives in
// ClassifyTCPVerdict; the flow wiring lives in internal/proxy.
//
// Platform shape: implemented on Linux (the Android lane and every
// Linux desktop) via GetsockoptTCPInfo(TCP_INFO) on the socket's raw fd;
// every other platform gets the honest "unavailable" error — callers
// log it once and continue (the report's tcp verdicts then carry
// unknown counts, never a guess).
package netx

import (
	"fmt"
	"net"
)

// TCPInfo is the subset of the kernel's tcp_info one evidence snapshot
// needs. Durations keep their kernel units documented per field.
type TCPInfo struct {
	// Acked is tcpi_bytes_acked: how many payload bytes the peer has
	// ACKed on this socket.
	Acked uint64
	// Sent is tcpi_bytes_sent: how many payload bytes left this socket.
	Sent uint64
	// Retrans is tcpi_bytes_retrans: payload bytes retransmitted (the
	// "the far side did not take them the first time" meter).
	Retrans uint64
	// Unacked is tcpi_unacked: segments currently in flight without an
	// ACK (a number that stays > 0 while the peer stays silent).
	Unacked uint32
	// RttMS is tcpi_rtt (microseconds) rendered in milliseconds.
	RttMS uint64
	// MSS is tcpi_snd_mss: the send max segment size.
	MSS uint32
	// Rcvd is tcpi_bytes_received: payload bytes this socket received
	// from the peer (the answer meter — the number that never moves
	// when a body-starving origin goes silent).
	Rcvd uint64
	// LastRecvMS is tcpi_last_data_recv: milliseconds since the peer's
	// last data segment.
	LastRecvMS uint32
	// LastAckMS is tcpi_last_ack_recv: milliseconds since the peer's
	// last ACK.
	LastAckMS uint32
}

// ErrTCPInfoUnavailable is the no-op platforms' honest answer (and the
// shape a non-TCP or closed conn produces): callers latch it once.
var ErrTCPInfoUnavailable = fmt.Errorf("tcpinfo: unavailable on this platform or connection")

// SnapshotTCPInfo reads the kernel's TCP counters for c's socket. Only
// raw TCP conns (*net.TCPConn or an unbroken chain to one) can serve —
// wrappers that hide the fd answer the unavailable error, which is why
// the proxy's evidence holder is always handed the raw dialed origin.
func SnapshotTCPInfo(c net.Conn) (TCPInfo, error) {
	return snapshotTCPInfoPlatform(c)
}

// ClassifyTCPVerdict applies the debug.24 decision rule (spec W1,
// ADR-0043) to an A→B snapshot pair of one googlevideo flow's origin
// socket:
//
//   - "upstream-lost": acked at B did not move past A while retrans
//     and/or unacked grew — the request packets were dropped on the
//     way out (the censor drops upstream data after the handshake).
//     Fragmentation tuning will not fix this.
//   - "downstream-lost": acked moved past A (the request ARRIVED) but
//     rcvd did not — the answer is being dropped or throttled on the
//     way back.
//   - "both-ok": both directions moved — the transport carried both
//     ways; a flow that still starved points at the server or HTTP
//     layer.
//   - "unknown": anything else (no A, no B, unavailable, or a pair the
//     rule cannot read — never a guess).
func ClassifyTCPVerdict(a, b TCPInfo) string {
	if b.Acked <= a.Acked {
		if b.Retrans > a.Retrans || b.Unacked > a.Unacked {
			return "upstream-lost"
		}
		return "unknown"
	}
	if b.Rcvd <= a.Rcvd {
		return "downstream-lost"
	}
	return "both-ok"
}
