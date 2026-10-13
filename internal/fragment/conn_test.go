package fragment

import (
	"net"
	"testing"
	"time"
)

// countingConn records how many separate Write calls reach the socket and
// how many bytes each carried.
type countingConn struct {
	net.Conn
	writes []int
}

func (c *countingConn) Write(b []byte) (int, error) {
	c.writes = append(c.writes, len(b))
	return len(b), nil
}

func TestConnFirstWriteFragmented(t *testing.T) {
	inner := &countingConn{Conn: nilWriter{}}
	cfg := Config{Min: 10, Max: 20, Pause: 0}
	fc := NewConn(inner, Balanced, cfg, seededRand(3))
	payload := make([]byte, 500)
	n, err := fc.Write(payload)
	if err != nil || n != 500 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if len(inner.writes) < 2 {
		t.Fatalf("first write reached the socket as %d writes, want >= 2", len(inner.writes))
	}
	// Second write passes through untouched.
	n, err = fc.Write(payload[:7])
	if err != nil || n != 7 {
		t.Fatalf("second Write = %d, %v", n, err)
	}
	if inner.writes[len(inner.writes)-1] != 7 {
		t.Fatalf("last socket write = %d bytes, want 7", inner.writes[len(inner.writes)-1])
	}
	total := 0
	for _, w := range inner.writes {
		total += w
	}
	if total != 507 {
		t.Fatalf("total bytes written = %d, want 507", total)
	}
}

func TestConnShortFirstWriteStillSplitsPerSpec(t *testing.T) {
	// File 01 section 5: for length >= 2 there are at least 2 pieces,
	// so a 3-byte first write becomes two socket writes.
	inner := &countingConn{Conn: nilWriter{}}
	fc := NewConn(inner, Strong, PresetConfig(Strong), seededRand(1))
	if _, err := fc.Write([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if len(inner.writes) != 2 {
		t.Fatalf("writes = %v, want two writes for a 3-byte message", inner.writes)
	}
	if inner.writes[0]+inner.writes[1] != 3 {
		t.Fatalf("bytes = %v, want 3 total", inner.writes)
	}
}

func TestConnLegacyManyPieces(t *testing.T) {
	inner := &countingConn{Conn: nilWriter{}}
	fc := NewConn(inner, Legacy, PresetConfig(Legacy), seededRand(11))
	payload := make([]byte, 2000)
	if _, err := fc.Write(payload); err != nil {
		t.Fatal(err)
	}
	if len(inner.writes) > 300 {
		t.Fatalf("legacy used %d writes, want <= 300", len(inner.writes))
	}
	total := 0
	for _, w := range inner.writes {
		total += w
	}
	if total != 2000 {
		t.Fatalf("legacy wrote %d bytes, want 2000", total)
	}
}

func TestMeasureSleepReturnsSomethingReal(t *testing.T) {
	d := MeasureSleep()
	if d < time.Millisecond {
		t.Errorf("measured sleep = %v, want >= 1ms", d)
	}
}

type nilWriter struct{ net.Conn }

func (nilWriter) Write(b []byte) (int, error) { return len(b), nil }
func (nilWriter) Read(b []byte) (int, error)  { return 0, net.ErrClosed }
func (nilWriter) Close() error                { return nil }
