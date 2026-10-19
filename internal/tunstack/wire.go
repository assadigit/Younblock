// wire.go — the TUN wire: an io.ReadWriteCloser carrying one raw IP
// packet per Read/Write, turned into a netstack link endpoint
// (docs/android-port 04 §2.5; roadmap M4).
//
// Android passes the VpnService's TUN fd as os.NewFile(fd, "tun") —
// VpnService TUNs deliver plain IP packets with no preamble
// (IFF_NO_PI semantics), so offset handling is deliberately absent.
// The pattern (a channel endpoint fed by a read loop, drained by a
// write loop) is the same one tun2socks' iobased device uses in
// production; gVisor's fd-based endpoint (fdbased, one readv/sendmsg
// path) is the documented upgrade if the M4 device measurements demand
// fewer syscalls — same seam, one struct.
//
// The reader tolerates short/oversized datagrams (skips them) and exits
// on the first hard error or Close; the writer drains the outbound
// queue until the context is canceled. Both loops are started exactly
// once, on Attach — netstack attaches the endpoint when the NIC is
// created, and the transport handlers are registered before that
// (tun2socks' ordering note: handlers first, then NIC, or packets race
// the registration).
package tunstack

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// outboundQueueLen bounds the stack→device packet queue; overflow drops
// packets (the same trade every NIC makes).
const outboundQueueLen = 1 << 10

// wireEndpoint is a stack.LinkEndpoint over an io.ReadWriteCloser.
type wireEndpoint struct {
	*channel.Endpoint

	rw io.ReadWriteCloser
	mtu uint32
	// v6Drops counts the IPv6 packets discarded at the pump (M8-z,
	// ADR-0027 §8.4/§7: ADR-0010's fail-safe, made countable — the
	// report's v6Dropped counter reads it through the Stack).
	v6Drops *int64

	once sync.Once
	wg   sync.WaitGroup
}

// newWireEndpoint wraps rw (one IP packet per Read/Write) as a link
// endpoint with the given MTU. v6Drops (may be nil) receives the v6
// discard count.
func newWireEndpoint(rw io.ReadWriteCloser, mtu uint32, v6Drops *int64) *wireEndpoint {
	if mtu == 0 {
		mtu = 1500
	}
	return &wireEndpoint{
		Endpoint: channel.New(outboundQueueLen, mtu, ""),
		rw:       rw,
		mtu:      mtu,
		v6Drops:  v6Drops,
	}
}

// Attach starts the pump loops once and forwards to the channel
// endpoint (netstack delivers inbound packets to this dispatcher).
func (e *wireEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.Endpoint.Attach(dispatcher)
	e.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		e.wg.Add(2)
		go func() {
			defer e.wg.Done()
			e.dispatchLoop(cancel)
		}()
		go func() {
			defer e.wg.Done()
			e.outboundLoop(ctx)
		}()
	})
}

// dispatchLoop reads packets from the wire and injects them inbound.
// It owns cancel: when the wire dies (EOF, error, Close), the outbound
// loop must stop too.
func (e *wireEndpoint) dispatchLoop(cancel context.CancelFunc) {
	defer cancel()
	buf := make([]byte, e.mtu)
	for {
		n, err := e.rw.Read(buf)
		if err != nil {
			return
		}
		if n == 0 || uint32(n) > e.mtu {
			continue
		}
		switch header.IPVersion(buf[:n]) {
		case header.IPv4Version:
		case header.IPv6Version:
			// Deliberately not injected: this stack is IPv4-only
			// (ADR-0010 — v6 flows blackhole in the TUN, no leak). M8-z
			// (ADR-0027 §8.4): the discard is COUNTED now — the report's
			// v6Dropped counter makes the policy visible instead of silent.
			if e.v6Drops != nil {
				atomic.AddInt64(e.v6Drops, 1)
			}
			continue
		default:
			continue
		}
		if !e.IsAttached() {
			continue
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(append([]byte(nil), buf[:n]...)),
		})
		e.InjectInbound(header.IPv4ProtocolNumber, pkt)
		pkt.DecRef()
	}
}

// outboundLoop drains stack-generated packets to the wire.
func (e *wireEndpoint) outboundLoop(ctx context.Context) {
	for {
		pkt := e.ReadContext(ctx)
		if pkt == nil {
			return
		}
		e.writePacket(pkt)
	}
}

// writePacket writes one outbound packet to the wire.
func (e *wireEndpoint) writePacket(pkt *stack.PacketBuffer) {
	defer pkt.DecRef()
	b := pkt.ToBuffer()
	defer b.Release()
	if _, err := e.rw.Write(b.Flatten()); err != nil {
		return
	}
}

// Close stops the pumps, closes the channel endpoint and the wire.
// Idempotent; safe from any goroutine. The caller that owns the
// underlying fd (the mobile facade on Android) hands it over for good —
// after Close the fd is released.
func (e *wireEndpoint) Close() {
	e.Endpoint.Close()
	_ = e.rw.Close()
}

// wait blocks until both loops returned (used by Stack.Stop).
func (e *wireEndpoint) wait() { e.wg.Wait() }

// tcpip compatibility guard: the endpoint must satisfy the interface
// netstack expects of link endpoints (compile-time check).
var _ stack.LinkEndpoint = (*wireEndpoint)(nil)
