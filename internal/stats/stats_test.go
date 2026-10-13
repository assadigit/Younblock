package stats

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCountersConcurrent(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				c.AddBytesDown(3)
				c.AddBytesUp(2)
				c.ConnOpened()
				c.ConnClosed()
			}
		}()
	}
	wg.Wait()
	d := c.Diagnostic()
	if d.BytesDown != 8*1000*3 || d.BytesUp != 8*1000*2 {
		t.Errorf("byte counts = %d/%d, want %d/%d", d.BytesDown, d.BytesUp, 8*1000*3, 8*1000*2)
	}
	if d.ConnsTotal != 8000 || d.ConnsLive != 0 {
		t.Errorf("conn counts = %d total / %d live, want 8000/0", d.ConnsTotal, d.ConnsLive)
	}
}

func TestSnapshotShapeAndRates(t *testing.T) {
	c := New()
	c.AddBytesDown(1000)
	c.AddBytesUp(400)
	c.ConnOpened()
	time.Sleep(1100 * time.Millisecond)
	s := c.Sample()
	if s.ActiveConns != 1 {
		t.Errorf("activeConns = %d, want 1", s.ActiveConns)
	}
	if s.UptimeSec < 1 {
		t.Errorf("uptimeSec = %d, want >= 1", s.UptimeSec)
	}
	if s.RateDown <= 0 || s.RateUp <= 0 {
		t.Errorf("rates = %f/%f, want positive after 1s sample", s.RateDown, s.RateUp)
	}
	// After a sample with no traffic the rates drop to zero.
	time.Sleep(600 * time.Millisecond)
	s2 := c.Sample()
	if s2.RateDown != 0 || s2.RateUp != 0 {
		t.Errorf("rates = %f/%f, want 0/0 with no traffic", s2.RateDown, s2.RateUp)
	}
	j, err := json.Marshal(s2)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"bytesDown", "bytesUp", "rateDown", "rateUp", "activeConns", "uptimeSec"} {
		if !strings.Contains(string(j), `"`+key+`"`) {
			t.Errorf("snapshot JSON %s missing key %q", j, key)
		}
	}
}

func TestDnsAndPanicCounters(t *testing.T) {
	c := New()
	c.DnsOK()
	c.DnsOK()
	c.DnsFail()
	c.Panic()
	d := c.Diagnostic()
	if d.DnsOK != 2 || d.DnsFail != 1 || d.Panics != 1 {
		t.Errorf("dns/panic counters = %d/%d/%d, want 2/1/1", d.DnsOK, d.DnsFail, d.Panics)
	}
}
