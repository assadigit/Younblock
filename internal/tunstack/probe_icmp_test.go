package tunstack

// The QUIC fast-fail contract (M8-j): a routed app's UDP flow to a
// non-DNS port (443 — QUIC) must get an ICMP destination-unreachable /
// port-unreachable back, never a silent blackhole. A blackholed QUIC
// makes Chromium-family players retransmit into the void before falling
// back to TCP — the 30-60s video-start delay Ali reported from the
// debug.3 device circle was investigated against exactly this seam; the
// host-level proof lives here so the behavior can never regress quietly
// (the older TestUDPNon53Dropped asserts the drop COUNTER; this test
// asserts the refusal the phone actually receives).

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func ipv4Checksum(h []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(h); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(h[i : i+2]))
	}
	if len(h)%2 == 1 {
		sum += uint32(h[len(h)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}

func udp4Checksum(srcIP, dstIP, udp, payload []byte) uint16 {
	var sum uint32
	feed := func(b []byte) {
		for i := 0; i+1 < len(b); i += 2 {
			sum += uint32(binary.BigEndian.Uint16(b[i : i+2]))
		}
		if len(b)%2 == 1 {
			sum += uint32(b[len(b)-1]) << 8
		}
	}
	pseudo := make([]byte, 12)
	copy(pseudo[0:4], srcIP)
	copy(pseudo[4:8], dstIP)
	pseudo[9] = 17
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(udp)+len(payload)))
	feed(pseudo)
	feed(udp)
	feed(payload)
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}

// buildUDP4Packet assembles one valid IPv4/UDP datagram (checksums
// included — gVisor validates the UDP checksum before it decides to
// refuse the flow).
func buildUDP4Packet(srcIP, dstIP net.IP, sport, dport uint16, payload []byte) []byte {
	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	binary.BigEndian.PutUint16(udp[6:8], udp4Checksum(srcIP, dstIP, udp, payload))

	ip := make([]byte, 20)
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(20+len(udp)+len(payload)))
	binary.BigEndian.PutUint16(ip[4:6], 0x1234)
	ip[8] = 64
	ip[9] = 17
	copy(ip[12:16], srcIP)
	copy(ip[16:20], dstIP)
	binary.BigEndian.PutUint16(ip[10:12], ipv4Checksum(ip))

	return append(append(ip, udp...), payload...)
}

func TestUDP443FastFailICMP(t *testing.T) {
	door, _, _ := testDoor(t)
	_, phoneSide := newTun(t, door, nil)

	// A QUIC-shaped datagram: 10.111.0.2:50000 -> 8.8.8.8:443.
	packet := buildUDP4Packet(
		net.IP{10, 111, 0, 2}.To4(),
		net.IP{8, 8, 8, 8}.To4(),
		50000, 443,
		[]byte("QUIC-ish datagram"),
	)
	if _, err := phoneSide.Write(packet); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The stack must answer with ICMP dest-unreachable (3) / port-
	// unreachable (3), quoting the original datagram — the phone kernel
	// turns that into ECONNREFUSED on the app's connected UDP socket,
	// which is what breaks QUIC fast instead of letting it retransmit
	// into a blackhole.
	buf := make([]byte, 2000)
	done := make(chan struct{})
	var gotN int
	var gotErr error
	go func() {
		defer close(done)
		gotN, gotErr = phoneSide.Read(buf)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("no ICMP came back for UDP/443 within 2s — QUIC would blackhole")
	}
	if gotErr != nil {
		t.Fatalf("read: %v", gotErr)
	}
	p := buf[:gotN]
	if header.IPVersion(p) != header.IPv4Version {
		t.Fatalf("response is not IPv4 (version %d)", header.IPVersion(p))
	}
	ip := header.IPv4(p)
	if ip.Protocol() != uint8(header.ICMPv4ProtocolNumber) {
		t.Fatalf("response proto = %d, want ICMP", ip.Protocol())
	}
	icmp := header.ICMPv4(p[header.IPv4MinimumSize:])
	if icmp.Type() != header.ICMPv4DstUnreachable || icmp.Code() != header.ICMPv4PortUnreachable {
		t.Fatalf("ICMP type/code = %d/%d, want 3/3 (dest/port unreachable)",
			icmp.Type(), icmp.Code())
	}
	// The ICMP payload quotes the refused datagram's IP header + at
	// least the UDP header, so the kernel can match it to the socket.
	quote := p[header.IPv4MinimumSize+header.ICMPv4MinimumSize:]
	if len(quote) < header.IPv4MinimumSize+header.UDPMinimumSize {
		t.Fatalf("ICMP quote too short: %d bytes", len(quote))
	}
	if binary.BigEndian.Uint16(quote[header.IPv4MinimumSize+2:]) != 443 {
		t.Fatalf("ICMP quote does not carry the refused dport 443")
	}
}
