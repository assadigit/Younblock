package proxy

// The M8-ai shapes (ADR-0048), pinned by test:
//
//   1. The drain fix — a substitute candidate that dies its round on
//      a MEDIA-certified pool origin boxes the IP (the rotation guard)
//      but NEVER revokes the certification: the pool's gvideo count
//      survives the death, goodDrops stays 0, the lane is dry exactly
//      as long as the box lives. The debug.26 report measured the
//      pre-fix shape end-to-end: the session's payload-fed marks were
//      demoted mid-storm (goodDrops=16), the door went back to
//      fast-failing, and the app ground 150 resets against a
//      blackholed cluster.
//
//   2. The feed clears the box — MarkMediaVerifiedIP on a boxed origin
//      puts it straight back in the substitute lane: a fresh media
//      delivery is direct liveness evidence that outranks the stale
//      death that boxed it (the rehabilitation write's own contract).
//
//   3. The size class — a substituted flow that ends below
//      PayloadVerifiedMinimum counts small (the fast verdict answer
//      every debug.23/debug.26 substitute win measured); one that
//      carries real media counts big and payload-feeds the origin.
//
//   4. The door's verdict lane and its REASON — a dead-marked cluster
//      with an empty pool refuses with "cluster-dead" and counts the
//      fast-fail; the pool's presence turns the same host into a
//      counted, logged verdict pass; non-gvideo hosts never gate.
//
//   5. The no-downgrade rule — a first-byte win's handshake mark
//      (markGoodIP(ip, false)) refreshes a media-certified origin
//      WITHOUT clearing the flag. The old write replaced the entry,
//      so every winner un-flagged the pool origin that carried it:
//      the debug.26 session fed six payload certifications, ended
//      holding two, and its verdict lane starved mid-storm exactly
//      one win at a time.
//
// Both-ways proof: with dropGoodIP's gvideo guard stashed, test 1
// FAILS on the demoted mark (gvideo 0, goodDrops 1); with
// MarkMediaVerifiedIP's clearDeadIP stashed, tests 1 and 2 FAIL on
// the still-boxed origin; with the size-class hunk stashed, test 3
// FAILS at small=0/big=0; with markGoodIP's no-downgrade guard
// stashed, test 3 FAILS on the un-flagged pool (flow B loses its
// substitute and terminals); with the verdict counter stashed, test 4
// FAILS at verdicts=0, and with the reason strings stashed it fails
// on the wrong refusal word.

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/stats"
)

// drainFixture builds the shape-1 world: one googlevideo flow host
// whose whole answer set is corpses, plus a pool origin that is
// media-certified but ALSO a corpse for the round that draws it (the
// origin went black between certifying and serving — its substitute
// candidate dies naturally, the strongest demote evidence the old
// code acted on).
func drainFixture(t *testing.T) (resolver *hostResolver, port int) {
	t.Helper()
	_, port = silentListenerAt(t, "127.0.0.1", 0)
	_, _ = silentListenerAt(t, "127.0.0.2", port)
	_, _ = silentListenerAt(t, "127.0.0.3", port)
	_, _ = silentListenerAt(t, "127.0.0.4", port) // the pool origin, silent this round
	resolver = &hostResolver{hosts: map[string][]net.IP{
		"rr3---sn-ccc.googlevideo.com": {
			net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"),
		},
	}}
	return resolver, port
}

// TestSubstituteLossKeepsTheMediaMark (shape 1, the drain fix's
// core): the substitute's natural death on a media-certified origin
// boxes the IP but the certification survives — and the next media
// feed puts the origin straight back in the lane.
func TestSubstituteLossKeepsTheMediaMark(t *testing.T) {
	resolver, port := drainFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
	})
	// The pool origin is media-certified (the W7 evidence class) but
	// silent on the wire this round — the substitute candidate that
	// draws it dies naturally after the full budget.
	srv.MarkMediaVerifiedIP("127.0.0.4")
	if _, gvideo := srv.GoodIPStats(); gvideo != 1 {
		t.Fatalf("pool gvideo = %d before the flow, want 1", gvideo)
	}

	// The flow: the corpses lose the round, the substitute loses too
	// (its origin is silent) — the terminal is the flow's honest end.
	c := dialConnect(t, srv.Addr(), "rr3---sn-ccc.googlevideo.com", port)
	if _, err := c.Write(tlsLikeRecord(64)); err != nil {
		t.Fatalf("write record: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _ = c.Read(make([]byte, 128)) // the terminal's close
	_ = c.Close()
	if failed := pollLines(log, "WARN tunnel first-byte failed ", 1, 3*time.Second); len(failed) == 0 {
		t.Fatal("the all-dead round must terminal")
	}

	// The certification SURVIVED the death (M8-ai, ADR-0048): the
	// pre-fix demote dropped the mark right here (gvideo 1→0,
	// goodDrops+1) and the debug.26 storm followed.
	if _, gvideo := srv.GoodIPStats(); gvideo != 1 {
		t.Fatalf("pool gvideo = %d after the substitute's death, want 1 (media certification survives demotes)", gvideo)
	}
	if d := st.Diagnostic(); d.GoodDrops != 0 {
		t.Fatalf("goodDrops = %d, want 0 (the media mark is not demotable)", d.GoodDrops)
	}
	// The box still owns rotation: the boxed origin is invisible to
	// the substitute lane for exactly as long as its penalty lives.
	if got := srv.goodSubstitutes(nil, 1); got != nil {
		t.Fatalf("goodSubstitutes = %v right after the death, want nil (the box guards rotation)", got)
	}
	if boxed := srv.PenalizedIPs(4); len(boxed) == 0 {
		t.Fatal("the dead pool origin must be in the penalty box")
	}

	// The next media feed (the origin's own cluster delivered again)
	// clears the box and the lane re-engages at once.
	srv.MarkMediaVerifiedIP("127.0.0.4")
	if got := srv.goodSubstitutes(nil, 1); got == nil {
		t.Fatal("goodSubstitutes = nil after a fresh media feed on the boxed origin, want the lane re-engaged")
	}
}

// TestMediaFeedClearsThePenaltyBox (shape 2, the W2 core in
// isolation): a media-certified origin takes a death mark, hides from
// the lane for the box's life, and a fresh certification puts it
// straight back — the delivery outranks the stale death.
func TestMediaFeedClearsThePenaltyBox(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.Logger = &recLogger{}
	})
	srv.MarkMediaVerifiedIP("127.0.0.9")
	srv.markDeadIP("127.0.0.9") // the death: box + (pre-fix) demote
	if got := srv.goodSubstitutes(nil, 1); got != nil {
		t.Fatalf("goodSubstitutes = %v while the media origin is boxed, want nil", got)
	}
	srv.MarkMediaVerifiedIP("127.0.0.9") // fresh delivery evidence
	if got := srv.goodSubstitutes(nil, 1); got == nil {
		t.Fatal("goodSubstitutes = nil after the fresh media feed, want the box cleared (delivery outranks the stale death)")
	}
}

// TestSubstitutedFlowsClassifyBySize (shape 3): the small echo answer
// counts small — the verdict shape every cross-cluster substitute win
// has measured in the field — and the media-sized delivery counts big
// and payload-feeds its origin.
func TestSubstitutedFlowsClassifyBySize(t *testing.T) {
	resolver, _, port := substFixture(t)
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.Resolver = resolver
		c.FirstByteTimeout = 250 * time.Millisecond
		c.PayloadVerifiedMinimum = 4096
	})
	primePool(t, srv, port)

	// Flow A — the small verdict: a 64-byte echo answer through the
	// substitute, the flow ends small.
	c := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	hello := tlsLikeRecord(64)
	if _, err := c.Write(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	got := make([]byte, len(hello))
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read echo (the substitute must save the flow): %v", err)
	}
	_ = c.Close()
	pollDiag(t, st, func(d stats.All) int64 { return d.SubstSmall }, 1, "the small verdict must classify")

	// The win must NOT eat the origin's own certification (M8-ai,
	// ADR-0048): the old markGoodIP(ip, false) write REPLACED the
	// media mark with a handshake mark — every substituted win
	// un-flagged the pool origin that carried it, and the lane
	// starved one win at a time (the debug.26 session fed six
	// certifications and ended holding two).
	if _, gvideo := srv.GoodIPStats(); gvideo != 1 {
		t.Fatalf("pool gvideo = %d after the substituted win, want 1 (a handshake mark never downgrades media evidence)", gvideo)
	}

	// Flow B — the big carry: the same shape, then a media-sized
	// payload rides the substituted flow's relay.
	b := dialConnect(t, srv.Addr(), "rr3---sn-aaa.googlevideo.com", port)
	if _, err := b.Write(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	gotHello := make([]byte, len(hello))
	_ = b.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(b, gotHello); err != nil {
		t.Fatalf("read echo (the substitute must save the flow): %v", err)
	}
	big := make([]byte, 8192)
	if _, err := b.Write(big); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	_ = b.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := io.ReadFull(b, make([]byte, len(big))); err != nil {
		t.Fatalf("read payload back: %v", err)
	}
	_ = b.Close()
	pollDiag(t, st, func(d stats.All) int64 { return d.SubstBig }, 1, "the media carry must classify big")
	pollDiag(t, st, func(d stats.All) int64 { return d.SubstWins }, 2, "both flows must be substitution wins")

	d := st.Diagnostic()
	if d.SubstSmall != 1 {
		t.Fatalf("substSmall = %d, want exactly 1 (the small flow)", d.SubstSmall)
	}
	if d.GvideoPayloadFeeds < 1 {
		t.Fatalf("gvideoPayloadFeeds = %d, want >=1 (the big flow feeds its origin)", d.GvideoPayloadFeeds)
	}
}

// TestDoorVerdictLaneCountsAndReasons (shape 4): the dead cluster's
// door refuses with the cluster-dead reason while the pool is empty,
// and the pool's presence turns the same host into a counted, logged
// verdict pass — the lane's engagement number the next report reads
// against fastFails.
func TestDoorVerdictLaneCountsAndReasons(t *testing.T) {
	log := &recLogger{}
	srv, st := newTestServer(t, func(c *Config) {
		c.Logger = log
		c.LogHostnames = true
	})
	// Three distinct hosts of one sn-cluster silent-terminalled —
	// the cluster carries the dead mark.
	for _, h := range []string{
		"rr1---sn-zzz-a.googlevideo.com",
		"rr2---sn-zzz-b.googlevideo.com",
		"rr3---sn-zzz-c.googlevideo.com",
	} {
		srv.noteGvideoDeadHost(h)
	}

	// The empty-pool refusal: instant, honest about its reason.
	fourth := "rr9---sn-zzz-q.googlevideo.com"
	if reason := srv.gvideoDoorBlockReason(fourth); reason != "cluster-dead" {
		t.Fatalf("door reason = %q, want cluster-dead (the empty pool has nothing to serve)", reason)
	}
	if d := st.Diagnostic(); d.GvideoClusterFastFail != 1 || d.GvideoClusterVerdicts != 0 {
		t.Fatalf("fastFail=%d verdicts=%d, want 1/0 (the refusal counted, the lane did not engage)", d.GvideoClusterFastFail, d.GvideoClusterVerdicts)
	}
	if fails := pollLines(log, "WARN gvideo cluster fast-fail", 1, 2*time.Second); len(fails) == 0 {
		t.Fatal("the fast-fail line must log")
	}

	// The pool's presence is the verdict: the same dead cluster's
	// next flow passes, counted and logged.
	srv.MarkMediaVerifiedIP("127.0.0.9")
	if reason := srv.gvideoDoorBlockReason(fourth); reason != "" {
		t.Fatalf("door reason = %q with a fed pool, want the pass (\"\")", reason)
	}
	if d := st.Diagnostic(); d.GvideoClusterVerdicts != 1 {
		t.Fatalf("gvideoClusterVerdicts = %d, want 1 (the lane's engagement counted)", d.GvideoClusterVerdicts)
	}
	if verdicts := pollLines(log, "INFO gvideo cluster verdict", 1, 2*time.Second); len(verdicts) == 0 {
		t.Fatal("the verdict line must log")
	}

	// Non-gvideo hosts never gate — the feed keeps full service.
	if reason := srv.gvideoDoorBlockReason("www.example.com"); reason != "" {
		t.Fatalf("door reason = %q for a non-gvideo host, want the pass (\"\")", reason)
	}
}

// pollDiag waits for one diagnostic field to reach want (the done-line
// classification lands when the relay unwinds, not inline).
func pollDiag(t *testing.T, st *stats.Counters, get func(stats.All) int64, want int64, what string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if get(st.Diagnostic()) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: diagnostic never reached %d", what, want)
}
