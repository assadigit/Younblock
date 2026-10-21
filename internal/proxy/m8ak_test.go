package proxy

// The M8-ak shapes (ADR-0050), pinned by test:
//
//   1. The auto-timeline's signals are video-scoped at the SOURCE: a
//      routed flow to a *.googlevideo.com host bumps GvideoFlows and
//      its first byte bumps GvideoFirstBytes; a routed flow to any
//      other YouTube-list host (www.youtube.com — the feed's own
//      class) bumps neither, while RoutedFlows moves for both. The
//      debug.28 report's auto-timeline closed pairs on 1.4 MB
//      i.ytimg feed bursts that carried no video at all — the
//      residual-investigation round needs the heuristic to measure
//      play -> video data, and the counters it reads must be scoped
//      where they are born.
//
// Both-ways proof: with logTunnelUp's gvideo gate stashed, the
// googlevideo leg of test 1 FAILS on GvideoFlows=0; with
// logTunnelFirstByte's counter stashed, it FAILS on
// GvideoFirstBytes=0.

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/stats"
)

// TestGvideoTimelineSignalsAreVideoScoped (shape 1): the play and
// data signals the auto-timeline reads are googlevideo-only, counted
// at the flow's own milestones.
func TestGvideoTimelineSignalsAreVideoScoped(t *testing.T) {
	port := echoOnlyFixture(t)
	resolver := &hostResolver{hosts: map[string][]net.IP{
		"rr3---sn-ccc.googlevideo.com": {net.ParseIP("127.0.0.4")},
		"www.youtube.com":              {net.ParseIP("127.0.0.4")},
	}}
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})

	// The googlevideo flow: its start and its first byte both count.
	c := dialConnect(t, srv.Addr(), "rr3---sn-ccc.googlevideo.com", port)
	hello := tlsLikeRecord(64)
	if _, err := c.Write(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	got := make([]byte, len(hello))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	_ = c.Close()
	pollDiag(t, st, func(d stats.All) int64 { return d.GvideoFirstBytes }, 1, "the googlevideo first byte must count")
	if d := st.Diagnostic(); d.GvideoFlows != 1 {
		t.Fatalf("gvideoFlows = %d, want 1 (the googlevideo flow start)", d.GvideoFlows)
	}

	// The feed-class flow: routed, counted for the old burst signal —
	// but the video-scoped twins stay still.
	f := dialConnect(t, srv.Addr(), "www.youtube.com", port)
	if _, err := f.Write(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	fgot := make([]byte, len(hello))
	_ = f.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(f, fgot); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	_ = f.Close()
	pollDiag(t, st, func(d stats.All) int64 { return d.RoutedFlows }, 2, "both flows are routed")
	d := st.Diagnostic()
	if d.GvideoFlows != 1 || d.GvideoFirstBytes != 1 {
		t.Fatalf("video-scoped signals moved on a feed-class flow: gvideoFlows=%d gvideoFirstBytes=%d, want 1/1", d.GvideoFlows, d.GvideoFirstBytes)
	}
}

// echoOnlyFixture binds one full-echo origin at 127.0.0.4 on a free
// port (the flows of this round are all answered — the milestone
// counters are the subject, not the racing).
func echoOnlyFixture(t *testing.T) int {
	t.Helper()
	_, port := silentListenerAt(t, "127.0.0.1", 0) // the free-port probe
	echoListenerAt(t, net.JoinHostPort("127.0.0.4", strconv.Itoa(port)))
	return port
}
