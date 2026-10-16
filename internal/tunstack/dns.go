// dns.go — the virtual DNS (docs/android-port 04 §2.5/§7, ADR-0009/0010;
// roadmap M4).
//
// The routed apps send their DNS to the TUN (the service points the
// resolver at a virtual address, doc 04 §2.4). This file answers those
// queries WITHOUT asking the network anything:
//
//   - A    → a fake address from the reserved 198.18.0.0/15 pool
//     (RFC 2544 benchmark range — never routed on the public
//     internet). The mapper remembers fake→domain so the TCP
//     side can hand the DOMAIN to the SOCKS5 door (ATYP 0x03),
//     where the same MatchAnyDomain fragmentation decision the
//     browser path makes applies (socks5.go's design note).
//     The real resolution happens at the door, through the
//     engine's DoH resolver — the same resolver, cache and
//     privacy rules as the desktop.
//   - AAAA → NOERROR with zero answers, so apps stay on IPv4 (the
//     desktop default, QueryAAAA off; ADR-0009).
//   - other types → NOERROR with zero answers (recorded choice).
//   - malformed   → FORMERR when a header exists to echo, drop when it
//     does not; unsupported opcodes → NOTIMP.
//
// The responder is a pure function over the wire bytes: fully host
// testable, no sockets. The stack's UDP handler feeds it.
package tunstack

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
)

// DNS qtypes and classes this responder distinguishes (RFC 1035).
const (
	qtypeA    = 1
	qtypeAAAA = 28

	qclassIN = 1
)

// DNS rcodes used here.
const (
	rcodeNoError = 0
	rcodeFormErr = 1
	rcodeNotImp  = 4
)

// fakeIPPoolBase and fakeIPPoolSize define the reserved 198.18.0.0/15
// pool the mapper allocates from (131,070 usable addresses; the subnet
// and broadcast extremes are skipped).
var (
	fakeIPPoolBase = net.IP{198, 18, 0, 0}.To4()
	fakeIPPoolSize = uint32(1) << 17 // /15
)

// fakeIPReports reports whether ip is inside the fake pool.
func fakeIPReports(ip net.IP) bool {
	ip = ip.To4()
	return ip != nil && ip[0] == 198 && (ip[1] == 18 || ip[1] == 19)
}

// dnsMapper allocates stable fake addresses per domain name and keeps
// the reverse mapping. Bounded: when the table reaches max domains the
// oldest allocation is evicted (FIFO). An evicted domain's stale fake
// address gets a fail-fast RST on the TCP side (unmapped fake), and the
// client's next DNS query hands out a fresh one — the TTL is short, so
// this is the normal renewal path, not an error.
type dnsMapper struct {
	mu sync.Mutex

	fwd   map[string]net.IP // lowercase domain → fake IP
	rev   map[string]string // fake IP string → domain
	order []string          // allocation FIFO for eviction
	head  int               // ring head (order is reused as a ring)
	next  uint32            // next pool offset, starting at 1
	max   int
}

func newDNSMapper(max int) *dnsMapper {
	if max < 1 {
		max = 4096
	}
	return &dnsMapper{
		fwd:  make(map[string]net.IP, max),
		rev:  make(map[string]string, max),
		max:  max,
		next: 1,
	}
}

// fakeFor returns the fake address for domain, allocating (and evicting
// if needed) on first sight. The name is lowercased: DNS is
// case-insensitive and 0x20-randomized queries must map to one entry.
func (m *dnsMapper) fakeFor(domain string) net.IP {
	domain = strings.ToLower(domain)
	m.mu.Lock()
	defer m.mu.Unlock()
	if ip, ok := m.fwd[domain]; ok {
		return ip
	}
	if len(m.fwd) >= m.max {
		// Evict the oldest allocation (ring head).
		old := m.order[m.head]
		if ip, ok := m.fwd[old]; ok {
			delete(m.rev, ip.String())
		}
		delete(m.fwd, old)
		m.order[m.head] = domain
		m.head = (m.head + 1) % m.max
	} else {
		m.order = append(m.order, domain)
		if len(m.order) == m.max {
			m.head = 0 // ring is full from now on
		}
	}
	ip := m.allocateLocked()
	// On the (astronomically unlikely) wrap-around collision, retire the
	// previous holder: fail-fast beats a wrong mapping.
	if prev, ok := m.rev[ip.String()]; ok {
		delete(m.fwd, prev)
	}
	m.fwd[domain] = ip
	m.rev[ip.String()] = domain
	return ip
}

// allocateLocked produces the next pool address as 198.18/19.x.y.
func (m *dnsMapper) allocateLocked() net.IP {
	off := m.next
	m.next++
	if m.next >= fakeIPPoolSize {
		m.next = 1
	}
	ip := make(net.IP, 4)
	copy(ip, fakeIPPoolBase)
	binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(ip)+off)
	return ip
}

// domainFor reverses a fake address to its domain.
func (m *dnsMapper) domainFor(ip net.IP) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	domain, ok := m.rev[ip.String()]
	return domain, ok
}

// respondDNS builds the response for one raw query. ok=false means the
// query is unanswerable garbage (< header): drop it silently.
func respondDNS(m *dnsMapper, query []byte) (response []byte, ok bool) {
	if len(query) < 12 {
		return nil, false
	}
	flags := binary.BigEndian.Uint16(query[2:4])
	opcode := (flags >> 11) & 0xF
	qdcount := binary.BigEndian.Uint16(query[4:6])

	if opcode != 0 {
		return dnsError(query[:2], rcodeNotImp), true
	}
	if qdcount != 1 {
		return dnsError(query[:2], rcodeFormErr), true
	}

	name, qstart, ok := parseQName(query)
	if !ok || len(query) < qstart+4 {
		return dnsError(query[:2], rcodeFormErr), true
	}
	qtype := binary.BigEndian.Uint16(query[qstart : qstart+2])
	qclass := binary.BigEndian.Uint16(query[qstart+2 : qstart+4])
	question := query[12 : qstart+4] // the raw question bytes, echoed verbatim

	var answer []byte
	if qtype == qtypeA && qclass == qclassIN && name != "" {
		ip := m.fakeFor(name)
		// name pointer to offset 12 (the question), TYPE A, CLASS IN,
		// TTL 30, RDLENGTH 4, RDATA.
		answer = []byte{0xC0, 0x0C, 0x00, qtypeA, 0x00, qclassIN,
			0x00, 0x00, 0x00, 30, // TTL 30s
			0x00, 0x04, ip[0], ip[1], ip[2], ip[3]}
	}
	return buildResponse(query, question, answer, rcodeNoError), true
}

// dnsError builds a header-only error response (rcode set, QD=0).
func dnsError(id []byte, rcode uint16) []byte {
	resp := make([]byte, 12)
	copy(resp, id)
	binary.BigEndian.PutUint16(resp[2:4], 0x8000|rcode) // QR=1, opcode 0
	return resp
}

// buildResponse assembles ID + echo flags + the question + the answer.
func buildResponse(query, question, answer []byte, rcode uint16) []byte {
	flags := 0x8000 | // QR: response
		0x0080 | // RA: recursion available (we answer from the mapper)
		(int(query[2])&0x01)<<8 | // echo RD
		int(rcode)
	ancount := 0
	if len(answer) > 0 {
		ancount = 1
	}
	resp := make([]byte, 0, 12+len(question)+len(answer))
	var hdr [12]byte
	copy(hdr[:2], query[:2])
	hdr[2] = byte(flags >> 8)
	hdr[3] = byte(flags)
	binary.BigEndian.PutUint16(hdr[4:6], 1) // QDCOUNT: the echoed question
	binary.BigEndian.PutUint16(hdr[6:8], uint16(ancount))
	resp = append(resp, hdr[:]...)
	resp = append(resp, question...)
	resp = append(resp, answer...)
	return resp
}

// parseQName parses the question name starting at offset 12, returning
// the dotted name (no trailing root dot), the offset just past it, and
// whether it was well-formed. Compression pointers are not accepted in
// questions (they are only legal in later sections; a query that uses
// one here is malformed).
func parseQName(msg []byte) (name string, next int, ok bool) {
	var b strings.Builder
	off := 12
	total := 0
	for {
		if off >= len(msg) {
			return "", 0, false
		}
		n := int(msg[off])
		if n == 0 {
			off++
			break
		}
		if n > 63 || n&0xC0 != 0 { // label too long, or a pointer
			return "", 0, false
		}
		if off+1+n > len(msg) {
			return "", 0, false
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.Write(msg[off+1 : off+1+n])
		total += n + 1
		if total > 254 { // 255 incl. the root label
			return "", 0, false
		}
		off += 1 + n
	}
	return b.String(), off, true
}
