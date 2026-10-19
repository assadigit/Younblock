package tunstack

// The M8-z work item D shapes (ADR-0026), pinned by test:
//
//   1. udp443 dropped — the QUIC fast-fail's timeline line lands when a
//      UDP flow to port 443 is dropped (no names — R21), and the NEXT
//      TCP flow to the SAME host logs the elapsed time between the drop
//      and the fallback ("udp443 fallback elapsed=…ms"), the QUIC
//      fallback's cost measured on the report.
//
//   2. v6Dropped — the IPv6 packets the wire pump discards (ADR-0010's
//      fail-safe) are COUNTED now, visible in Stats instead of silent.
//
// The ICMP port-unreachable itself is NOT re-implemented here: the
// existing TestUDP443FastFailICMP (M8-j) already pins that gVisor sends
// type 3/code 3 for these spoofed/promiscuous flows when handleUDP
// returns false — the spec's §7.1 "first write a host test" step is
// that test, kept as the guard; this round only ADDS the visibility.
//
// Both-ways proof: with the timeline lines stashed, test 1 FAILS (no
// udp443 dropped line, no fallback line); with the v6 counter stashed,
// test 2 FAILS (v6Dropped stays 0).

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// recLog records the stack's log lines (message text only — the shape
// the report's ring keeps).
type recLog struct {
	mu   sync.Mutex
	msgs []string
}

func (l *recLog) Info(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, msg)
}
func (l *recLog) Warn(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, msg)
}
func (l *recLog) Error(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, msg)
}

func (l *recLog) lines(prefix string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, m := range l.msgs {
		if strings.HasPrefix(m, prefix) {
			out = append(out, m)
		}
	}
	return out
}

// fakeIPFor asks the stack's virtual DNS for name's A record through
// the phone side and returns the fake IP the answer carries.
func fakeIPFor(t *testing.T, phone *phoneStack, name string) net.IP {
	t.Helper()
	dns := phone.dialUDP(t, net.ParseIP("10.111.0.53"), 53)
	defer dns.Close()
	q := dnsQuery(0x4242, name, qtypeA)
	if _, err := dns.Write(q); err != nil {
		t.Fatalf("dns write: %v", err)
	}
	_ = dns.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1500)
	n, err := dns.Read(buf)
	if err != nil {
		t.Fatalf("dns read: %v", err)
	}
	resp := buf[:n]
	// One A answer: the LAST 4 bytes are the RDATA.
	if len(resp) < 16 {
		t.Fatalf("short dns response (%d bytes)", len(resp))
	}
	return net.IP(append([]byte(nil), resp[len(resp)-4:]...))
}

// TestUDP443TimelineLogsFallback (shape 1, the fix's core): the QUIC
// drop line lands without any name, and the next TCP flow to the SAME
// host carries the elapsed fallback time — with the host name only
// when LogHostnames is on.
func TestUDP443TimelineLogsFallback(t *testing.T) {
	door, _, origin := testDoor(t)
	log := &recLog{}
	_, phoneSide := newTun(t, door, func(c *Config) {
		c.Logger = log
		c.LogHostnames = true
	})
	phone := newPhone(t, phoneSide)

	fake := fakeIPFor(t, phone, "video.example.com")
	if !fakeIPReports(fake) {
		t.Fatalf("the virtual DNS must answer a fake-pool address, got %v", fake)
	}

	// The QUIC attempt: UDP to the fake address's :443 — dropped.
	quic := phone.dialUDP(t, fake, 443)
	if _, err := quic.Write([]byte("QUIC-ish datagram")); err != nil {
		t.Fatalf("udp write: %v", err)
	}
	quic.Close()

	// The TCP fallback to the same host (the door reverses the fake
	// address to the domain; the resolver points at the echo origin).
	conn := phone.dialTCP(t, fake, uint16(origin.Port))
	if _, err := conn.Write([]byte("hello fallback")); err != nil {
		t.Fatalf("tcp write: %v", err)
	}
	got := make([]byte, len("hello fallback"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(got); err != nil {
		t.Fatalf("tcp read through the door: %v", err)
	}

	drops := log.lines("udp443 dropped")
	if len(drops) == 0 {
		t.Fatal("the udp443 drop line never landed — the QUIC fast-fail's timeline is invisible")
	}
	if strings.Contains(drops[0], "host=") {
		t.Fatalf("the drop line must carry NO names (R21): %q", drops[0])
	}
	fallbacks := log.lines("udp443 fallback elapsed=")
	if len(fallbacks) == 0 {
		t.Fatal("the next TCP flow to the same host must log the fallback elapsed time")
	}
	if !strings.Contains(fallbacks[0], "host=video.example.com") {
		t.Fatalf("with LogHostnames on, the fallback line must name the host: %q", fallbacks[0])
	}
}

// TestUDP443FallbackHostlessWithoutLogHostnames (guard): the fallback
// line carries the elapsed time but NO host name when LogHostnames is
// off. Passes on both sides.
func TestUDP443FallbackHostlessWithoutLogHostnames(t *testing.T) {
	door, _, origin := testDoor(t)
	log := &recLog{}
	_, phoneSide := newTun(t, door, func(c *Config) {
		c.Logger = log
		c.LogHostnames = false
	})
	phone := newPhone(t, phoneSide)

	fake := fakeIPFor(t, phone, "video.example.com")
	quic := phone.dialUDP(t, fake, 443)
	quic.Write([]byte("QUIC-ish datagram"))
	quic.Close()

	conn := phone.dialTCP(t, fake, uint16(origin.Port))
	conn.Write([]byte("hello"))
	got := make([]byte, 5)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.Read(got)

	for _, line := range log.lines("udp443") {
		if strings.Contains(line, "host=") {
			t.Fatalf("a line leaked a host name without LogHostnames: %q", line)
		}
	}
	if len(log.lines("udp443 fallback elapsed=")) == 0 {
		t.Fatal("the fallback line must still land (elapsed time is not a name)")
	}
}

// TestV6DroppedCountsDiscardedPackets (shape 2, the fix's core): an
// IPv6 packet written to the wire dies at the pump (ADR-0010) and is
// now COUNTED — Stats().V6Dropped instead of silence.
func TestV6DroppedCountsDiscardedPackets(t *testing.T) {
	door, _, _ := testDoor(t)
	tun, phoneSide := newTun(t, door, nil)

	before := tun.Stats().V6Dropped
	// A minimal IPv6 header (version 6, nothing else needed — the pump
	// only looks at the version nibble before discarding).
	v6 := make([]byte, 40)
	v6[0] = 0x60
	if _, err := phoneSide.Write(v6); err != nil {
		t.Fatalf("v6 write: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tun.Stats().V6Dropped > before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("V6Dropped = %d, want > %d — the v6 discard must be counted", tun.Stats().V6Dropped, before)
}

// TestV6DropsDoNotDisturbTheIPv4Path (guard): the v6 discard path
// leaves the v4 pump untouched — an ICMP/IPv4 packet after the v6 one
// still flows. Passes on both sides.
func TestV6DropsDoNotDisturbTheIPv4Path(t *testing.T) {
	door, _, _ := testDoor(t)
	_, phoneSide := newTun(t, door, nil)

	v6 := make([]byte, 40)
	v6[0] = 0x60
	phoneSide.Write(v6)
	// The pinned M8-j shape: UDP/443 still gets its ICMP refusal.
	packet := buildUDP4Packet(
		net.IP{10, 111, 0, 2}.To4(),
		net.IP{8, 8, 8, 8}.To4(),
		50001, 443,
		[]byte("still working"),
	)
	if _, err := phoneSide.Write(packet); err != nil {
		t.Fatalf("v4 write: %v", err)
	}
	buf := make([]byte, 2000)
	type readRes struct {
		n   int
		err error
	}
	ch := make(chan readRes, 1)
	go func() {
		n, err := phoneSide.Read(buf)
		ch <- readRes{n, err}
	}()
	var n int
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("no ICMP came back after the v6 discard: %v", r.err)
		}
		n = r.n
	case <-time.After(2 * time.Second):
		t.Fatal("no ICMP came back after the v6 discard within 2s")
	}
	p := buf[:n]
	if header.IPVersion(p) != header.IPv4Version {
		t.Fatalf("response is not IPv4 (version %d)", header.IPVersion(p))
	}
	if header.IPv4(p).Protocol() != uint8(header.ICMPv4ProtocolNumber) {
		t.Fatalf("response proto = %d, want ICMP", header.IPv4(p).Protocol())
	}
}
