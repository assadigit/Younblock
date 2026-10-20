// Package fragment — the emission's packet accounting (M8-af,
// ADR-0043, spec W9): hello COUNT was measured (gvideoHellos since
// debug.23) but hello PACKET count never was — and the censor may key
// on packets per source, not connections: one `legacy` hello is up to
// 300 TCP segments at 1 ms spacing, so 81 hellos is ~24,000 tiny
// packets in an 85 s window. piecesTotal counts every TCP segment the
// fragmenter writes (each piece is one write; TCP_NODELAY keeps the
// kernel from merging them back), process-wide, read as a snapshot so
// the network test can compute per-method deltas.
package fragment

import "sync/atomic"

// piecesTotal is the process-wide count of TCP segments the
// fragmenter has written (M8-af, ADR-0043, spec W9).
var piecesTotal atomic.Int64

// PiecesTotal reports how many TCP segments the fragmenter has written
// since process start (the report's fragPackets number; the network
// test reads deltas per method).
func PiecesTotal() int64 { return piecesTotal.Load() }
