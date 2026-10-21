package engine

// deadclusters.go — the persisted googlevideo dead-cluster marks (M8-an,
// ADR-0053): doc 11 §5.3's pre-registered candidate fix, convicted into
// shipping by the debug.30 field report. The cluster memory (M8-ah,
// ADR-0047) was per-session by construction — three distinct hosts
// silent-terminal inside the window before a cluster is marked — so every
// fresh connect started with an EMPTY table, and the connect-time
// hand-off (M8-am) opened a cold-started YouTube app into exactly that
// second-zero state: the app's ladder burned ~2.5 s terminal budgets on
// the blackholed cluster before the marks existed, the terminal burn is
// what the breaker's arm gate sums across clusters, and the felt result
// was the old 1–2 minute first-video open. This file is the memory's
// persistence lane, the pool file's mirror image (ADR-0033/0047):
// GoogleVideoDeadClustersFile beside GoogleVideoGoodIPsFile, seeded at
// start with the shorter GvideoClusterSeedTTL, saved on the same cadence
// and Stop-time flush, self-correcting through the re-probe and the
// own-dial rehabilitation. A missing or unreadable file is an empty
// list — the lane is an optimization, never a dependency.

import (
	"os"
	"strings"
)

// deadClusterMaxPersist bounds the persisted dead-cluster list (M8-an,
// ADR-0053): the cluster population a hostile network can blackhole is
// single digits (the reports' whole history names one at a time —
// sn-qxau5 since debug.25); 16 is plenty of memory and a trivially
// small file.
const deadClusterMaxPersist = 16

// loadDeadClusters reads the persisted dead-cluster marks (M8-an,
// ADR-0053): one sn-cluster key per line, the clusters whose dead marks
// were live when the last session wrote the file. The seeder validates
// the keys (validGvideoClusterKey at the proxy door) — a stale or
// corrupted line never poisons the memory, exactly the pool loader's
// rule.
func (e *Engine) loadDeadClusters() []string {
	e.goodMu.Lock()
	path := e.deadClusterPath
	e.goodMu.Unlock()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var keys []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		keys = append(keys, line)
		if len(keys) >= deadClusterMaxPersist {
			break
		}
	}
	return keys
}

// saveDeadClustersIfChanged writes the live dead-cluster marks to
// GoogleVideoDeadClustersFile when they changed since the last write
// (M8-an, ADR-0053) — called from the gvideo loop's periodic tick and
// its Stop-time flush beside saveGoodPoolIfChanged; goodMu serializes
// the change-check-and-write so the two lanes never interleave. Write
// failures log and leave the file for the next change — the marks are
// an optimization, never a dependency.
func (e *Engine) saveDeadClustersIfChanged() {
	srv := e.RunningServer()
	if srv == nil {
		return
	}
	e.goodMu.Lock()
	path := e.deadClusterPath
	if path == "" {
		e.goodMu.Unlock()
		return
	}
	list := srv.GvideoDeadClusterList()
	var snap strings.Builder
	for _, key := range list {
		snap.WriteString(key)
		snap.WriteByte('\n')
	}
	snapshot := snap.String()
	if snapshot == e.deadClusterLast {
		e.goodMu.Unlock()
		return
	}
	e.deadClusterLast = snapshot
	e.goodMu.Unlock()
	if err := os.WriteFile(path, []byte(snapshot), 0o600); err != nil {
		e.logger.Warn("dead-cluster save failed reason=" + errKind(err))
	}
}
