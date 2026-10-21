package proxy

// deadseed_test.go — the persisted dead-cluster marks' pins (M8-an,
// ADR-0053, doc 11 §5.3's pre-registered fix): SeedGvideoDeadClusters
// plants exactly the mark the door reads (gvideoClusterDead — the
// instant-refusal check), the seed respects the evidence ranks (a
// live in-session mark is never shortened; the own-dial rehab clears
// even a seed), the probe handout serves seeded marks, and the list
// the engine persists round-trips. Both-ways proofs are recorded in
// ADR-0053's Evidence section.

import (
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
)

// TestSeedGvideoDeadClustersPlantsTheDoorMark: the both-ways core —
// without a seed the door check is false for the blackholed cluster's
// hosts (second zero of a session: the debug.30 state, the app's
// ladder pays the discovery); with the seed it is true immediately.
func TestSeedGvideoDeadClustersPlantsTheDoorMark(t *testing.T) {
	srv, st := newTestServer(t, nil)
	hosts := []string{
		"rr2---sn-qxau5-btqk.googlevideo.com",
		"rr5---sn-qxau5-btqs.googlevideo.com",
	}

	// Before the seed: the per-session amnesia the debug.30 report
	// convicted — no mark, the door would dial and burn the budget.
	for _, h := range hosts {
		if srv.gvideoClusterDead(h) {
			t.Fatalf("cluster dead before any evidence for %s", h)
		}
	}

	srv.SeedGvideoDeadClusters([]string{"sn-qxau5"})

	for _, h := range hosts {
		if !srv.gvideoClusterDead(h) {
			t.Errorf("seeded mark not live for %s: the door would pay the discovery instead of fast-failing", h)
		}
	}
	// Every seeded mark counted once — the report's seeded= half.
	if got := st.Diagnostic().GvideoClusterSeeds; got != 1 {
		t.Errorf("seeded counter = %d, want 1", got)
	}
	// The persisted shape: the list carries exactly the live marks.
	if list := srv.GvideoDeadClusterList(); len(list) != 1 || list[0] != "sn-qxau5" {
		t.Errorf("GvideoDeadClusterList = %v, want [sn-qxau5]", list)
	}
}

// TestSeedGvideoDeadClustersRespectsTheEvidenceRanks: a seed never
// SHORTENS a live in-session mark (the fresh measurement outranks the
// stale one), an invalid key never enters the table, and the own-dial
// rehabilitation clears even a seeded mark — the safety net the
// pre-registration named.
func TestSeedGvideoDeadClustersRespectsTheEvidenceRanks(t *testing.T) {
	srv, st := newTestServer(t, nil)

	// A measured death: three distinct hosts inside the window mark
	// the cluster dead for the full GvideoClusterDeadTTL.
	for _, h := range []string{
		"rr1---sn-qxau5-btqd.googlevideo.com",
		"rr2---sn-qxau5-btqk.googlevideo.com",
		"rr3---sn-qxau5-btqy.googlevideo.com",
	} {
		srv.noteGvideoDeadHost(h)
	}
	measured := srv.gvideoClusters.clusters["sn-qxau5"]
	if measured == nil || time.Until(measured.until) <= 0 {
		t.Fatalf("the measured mark did not land")
	}
	fullUntil := measured.until

	// A seed (with the config's shorter TTL) must not shorten it.
	srv.SeedGvideoDeadClusters([]string{"sn-qxau5", "not-a-cluster", ""})
	if got := srv.gvideoClusters.clusters["sn-qxau5"].until; !got.Equal(fullUntil) {
		t.Errorf("the seed moved a live measured mark: %v -> %v", fullUntil, got)
	}
	if _, ok := srv.gvideoClusters.clusters["not-a-cluster"]; ok {
		t.Error("an invalid key entered the cluster table")
	}
	// The seed planted NOTHING (the measured mark outranked it), so
	// the counter did not move — seeded= counts marks the file actually
	// put live, never a no-op over fresher evidence.
	if got := st.Diagnostic().GvideoClusterSeeds; got != 0 {
		t.Errorf("seeded counter = %d, want 0 (the measured mark outranked the seed)", got)
	}

	// The rehab: an answered own-dial clears the seeded mark — a seed
	// that went stale overnight costs at most one window.
	srv.noteGvideoClusterAlive("rr2---sn-qxau5-btqk.googlevideo.com")
	if srv.gvideoClusterDead("rr2---sn-qxau5-btqk.googlevideo.com") {
		t.Error("the own-dial rehab did not clear the seeded mark")
	}
}

// TestSeedGvideoDeadClustersProbeHandout: the seeded mark's re-probe
// is served on the door's cadence — the first probe waits out its
// window like a fresh mark (probedAt stamps at seed time), so a
// lifted blackhole is learned inside one GvideoClusterReprobeEvery.
func TestSeedGvideoDeadClustersProbeHandout(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	const host = "rr2---sn-qxau5-btqk.googlevideo.com"

	srv.SeedGvideoDeadClusters([]string{"sn-qxau5"})
	if srv.claimGvideoClusterProbe(host) {
		t.Fatal("the first probe handout must wait out its window (probedAt stamps at seed time)")
	}
}

// TestGvideoDeadClusterListOnlyLiveMarks: the persisted list carries
// only LIVE marks — an expired mark (a cluster that stayed dead past
// its TTL without re-marking) never rides into the next session, and
// the list is sorted so the file snapshot is deterministic.
func TestGvideoDeadClusterListOnlyLiveMarks(t *testing.T) {
	srv, _ := newTestServer(t, nil)

	// Expired marks are not listed: age the cluster state past the
	// seed TTL by hand (the table's own clock is time.Now).
	for _, h := range []string{
		"rr1---sn-4g5ednkd-1ul.googlevideo.com",
		"rr2---sn-4g5ednkd-1ul.googlevideo.com",
		"rr3---sn-4g5ednkd-1ul.googlevideo.com",
	} {
		srv.noteGvideoDeadHost(h)
	}
	for _, h := range []string{
		"rr1---sn-aj4g55-npo4.googlevideo.com",
		"rr2---sn-aj4g55-npo4.googlevideo.com",
		"rr3---sn-aj4g55-npo4.googlevideo.com",
	} {
		srv.noteGvideoDeadHost(h)
	}
	list := srv.GvideoDeadClusterList()
	if len(list) != 2 {
		t.Fatalf("list = %v, want the two live marks", list)
	}
	if list[0] != "sn-4g5ednkd" || list[1] != "sn-aj4g55" {
		t.Errorf("list = %v, want sorted [sn-4g5ednkd sn-aj4g55]", list)
	}
	// Age both past the dead TTL: nothing rides on.
	srv.gvideoClusters.mu.Lock()
	for _, st := range srv.gvideoClusters.clusters {
		st.until = time.Now().Add(-time.Second)
	}
	srv.gvideoClusters.mu.Unlock()
	if list := srv.GvideoDeadClusterList(); len(list) != 0 {
		t.Errorf("expired marks persisted: %v", list)
	}
}

// TestSeedTTLConfigDefault: the seeded mark's TTL defaults to the
// config's GvideoClusterSeedTTL when the Config leaves it zero —
// half the in-session GvideoClusterDeadTTL, the GoodPoolSeedTTL
// precedent's shape.
func TestSeedTTLConfigDefault(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	if got := srv.cfg.GvideoClusterSeedTTL; got != config.GvideoClusterSeedTTL {
		t.Errorf("seed TTL = %v, want the config default %v", got, config.GvideoClusterSeedTTL)
	}
	if config.GvideoClusterDeadTTL <= config.GvideoClusterSeedTTL {
		t.Errorf("seed TTL %v must be shorter than the dead TTL %v", config.GvideoClusterSeedTTL, config.GvideoClusterDeadTTL)
	}
}
