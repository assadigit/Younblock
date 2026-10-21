# 11 — The Video-Opening War: Method and Record (M8-aa through M8-ak)

**Status:** Complete record, written 2026-10-21 at the debug.28 field
verdict ("by far our best version yet — 90% of the initial delay of
each video is gone"). This document exists because the tester asked
for it directly: *"document whatever you did to not miss this
accomplishment/method."* It is the synthesis; the round-by-round
primary sources are `08-architecture-decisions.md` (ADR-0027 through
ADR-0050), `PROGRESS.md` (the M8 sections), and the CHANGELOG.
**§5.1, same day: the verdict** — the debug.29 field report answered
the question §5 decomposed.

---

## 1. The scoreboard

| Build | Round | The tester's experience | The number that told the story |
|---|---|---|---|
| debug.18–.22 | M8-y–ae | feed works, videos take 1–2 minutes to open | terminals burning full 2.5–5 s budgets; the breaker's 45 s cooldowns covering the whole open |
| debug.24 | M8-af | same | the report itself proved none of the theories — the prove-before-change round (W-series sensors landed) |
| debug.25 | M8-ag | "same problem… every video takes initially 1-2 minutes" | cooldowns=7 × ~15 s ≈ the whole latency; a healthy pool starved behind the door |
| debug.26 | M8-ah | "first video still buffers (less though), later videos mostly fast" | dead=1 (the cluster memory), payloadVerified=6 (the pool fed itself), fastFails=150 (the app's reset ladder) |
| debug.27 | M8-ai | "did not even open a single video, only feed refreshed" | attempts=40 wins=37 **big=0** — the verdict lane pinned the player to the dead URL set; dead=0 (substituted flows never terminal) |
| **debug.28** | M8-aj | **"by far our best version yet — 90% of the initial delay gone"** | dead=1, fastFails=90, cooldowns=1 lifted in 730 ms, auto-timeline **1000 ms** average over 5 pairs, 2.1 MB delivered on a live cluster |
| debug.29 | M8-ak | "I think it became better — every video opened in 5–6 seconds" | auto-timeline **500 ms × 6 pairs** (gvideo-gated) — the engine's share measured at ~10% of the felt load; the rest is the app's own ladder (fastFails=84) and buffering (§5.1) |

The arc in one sentence: a 1–2 minute video open became ~1 second by
learning, from the tester's own diagnostic reports, **which wire
answers mean what** — and refusing every theory the reports did not
prove.

---

## 2. The method (the part worth preserving)

### 2.1 The report is the instrument

Every round begins with the tester's pasted diagnostic report and
nothing else. The report was designed, round by round, so that each
new hypothesis could be **falsified by a counter it already carries**:
`gvideo clusters: dead=… fastFails=… verdicts=… probes=…
rehabilitated=… payloadVerified=…`, `substitution: attempts=… wins=…
big=… small=…`, `gvideo discipline: hellos=… cooldowns=… lifts=…
scouts=…`, the per-host top-10 table, the last-200 log lines, and the
auto-timeline. When a question could not be answered by an existing
counter, the round that asked it added the counter — debug.27's
`big=/small=` existed precisely so its own failure would be
measurable (it was: `big=0 small=37`).

### 2.2 Diagnosis from the report's own evidence only

No fix ships on theory. Each round's diagnosis section quotes the
report lines that prove the mechanism:

- debug.25's report carried **two proofs the wire was healthy**
  (a network test 5/5 answered; a flow that delivered 406 KB right
  before the door went quiet) — which is what convicted the breaker
  of starving a healthy line (ADR-0046).
- debug.26's report showed every `sn-qxau5-*` host with
  `worstFirstByte=0ms` and `down=0 B` while `sn-aj4g55`/`u14g55`/
  `4g5ednkd` delivered megabytes — an **IP-range blackhole**, not an
  SNI flag (the one-shot dead-origin probe confirmed it: silent under
  every SNI, ADR-0047).
- debug.27's report showed 37 substituted wins at ~5 KB each with
  **zero** payload verifications on substituted flows — the
  cross-cluster substitute had never once carried media in four
  rounds; the field falsified the bet in one session (ADR-0049).

### 2.3 Both-ways proofs, executed

Every behavior change is proven in both directions before it ships:
stash the hunk → the pinning test **FAILS** → restore → it passes.
The proof runs are recorded in each ADR's Evidence section (count the
"FAIL-as-expected / PASS-restored" lines: six in M8-ai, three in
M8-aj, four in M8-ak). Twice the discipline caught bugs the field
never saw: M8-ai's own test caught `markGoodIP` un-flagging media
marks mid-test, and M8-aj's re-pinned report line caught the
substitution off-switch being a no-op since debug.24.

### 2.4 Experiments designed to fail informatively

The debug.27 verdict-lane bet is the model: ADR-0048's "What this ADR
does NOT decide" section pre-registered the exact question (*does an
HTTP verdict shorten the player's URL-set ladder?*), the counters to
answer it (`verdicts=`, `big=/small=`), and the retreat plan (*if
not, the door goes back to fast-fail*). The bet lost; the loss cost
one session and produced the cleanest experiment result in the
project's history. **A/B switches mean never having to be right on
the first try — only accountable for the second.**

### 2.5 The paper trail

Every round: an ADR (six sections, house format), a PROGRESS section,
a CHANGELOG entry, a README paragraph, a versionCode bump with the
round's comment (D84), a foreground build, an APK named
`v1.6.6-debugNN.apk` (D103), a REST release set to Latest, the asset
re-downloaded and byte-compared (SHA-256 recorded), and a worklog
append. The chain is reconstructible from the repo alone — proven the
hard way when the build sandbox was wiped between rounds and the
debug.28 build reproduced byte-honestly from the repository.

---

## 3. The arc, round by round

**M8-ae/af (debug.23/24) — the wire-discipline and prove-before-change
rounds.** The breaker (ADR-0041) answered "the wire goes quiet after
too many googlevideo terminals" with cooldowns; the W-series sensors
(ADR-0043–0045: kernel-TCP A/B snapshots, the TLS record timeline,
the win= class, the payload-verified pool bar) turned the next reports
from stories into measurements.

**M8-ag (debug.25, ADR-0046) — the breaker stops starving the
player.** The report proved the wire healthy while 58 door refusals
starved it: the pool escape (a media-verified address serves through
a cooldown), the scout (one real flow probes an armed cooldown), the
lift (any googlevideo first byte ends the cooldown), 45 s → 15 s.

**M8-ah (debug.26, ADR-0047) — the dead-cluster memory.** The
blackhole signature (dial accepts, TLS never answers) keyed by the
`sn-XXXX` cluster token: three distinct silent hosts mark the cluster
dead, its hosts fail instantly at the door (zero wire), a per-cluster
probe watches for recovery, and the payload-fed pool certifies
origins from delivered bytes. The multi-cluster arm gate keeps the
breaker out of single-cluster blackholes entirely.

**M8-ai (debug.27, ADR-0048) — the pool keeps its food.** The pool
was an outflow machine: every first-byte win un-flagged its own
origin (the `markGoodIP` replace), substitute deaths and pending
kills demoted pool origins. Fixed (the no-downgrade rule, the demote
guard, the feed clears the box) — and the fed pool exposed the
verdict lane for the experiment it was.

**M8-aj (debug.28, ADR-0049) — the falsification.** The verdict lane
kept the player on the dead URL set all session (resets escalate the
player's ladder; HTTP answers sedate it) and starved the cluster
memory (substituted flows never terminal). The four-round experiment
retired OFF at the settings layer (schema v7), the latent off-switch
no-op fixed (`goodSubstKnob` 0 → −1), the door returned to
probe → fast-fail. **This is the build the tester called the best
yet.**

**M8-ak (debug.29, ADR-0050) — the instrument sharpens.** The
auto-timeline re-scopes to googlevideo (a play is a googlevideo flow
after googlevideo quiet; first-data is a googlevideo first byte), so
the remaining 5–10% residual can be measured as what it is — play →
video data — instead of being closed by feed bursts (the debug.28
report's 1.4 MB `i.ytimg` bursts were closing pairs that carried no
video).

**M8-al (no build) — the verdict.** The debug.29 field report
answered §5's question: the engine's share of the open measured at
**500 ms** (6 gvideo-gated pairs), the tester's felt load at 5–6 s —
the difference is the app's own dead-host ladder plus buffering, not
the chain (§5.1). The engine's levers on the ladder are
field-tested exhausted; the war ends at the release state.

---

## 4. The semantic discoveries (the reusable knowledge)

1. **The censor's observed classes** (this network, these rounds):
   an **IP-range blackhole** on specific cache clusters (the dial
   accepts, no TLS byte ever returns — confirmed by the one-shot
   probe silent under every SNI), **per-origin dirt** (mixed answer
   sets where racing and salvage recover), and transient resets
   (temporal, never an IP verdict). The per-source SNI-flag
   hypothesis that motivated the breaker was never the mobile
   blocker's shape — the breaker survives for multi-cluster bleed,
   which the arm gate scopes it to.
2. **The player's ladder semantics**: the YouTube app retries a
   dead URL set on connection resets until its ladder escalates to a
   fresh player-API fetch (which returns live clusters); an
   HTTP-level answer on the same URLs **pins** it there. The engine's
   job is to fail fast at the connection level and never answer for
   the dead set — the fast-fail reset is not a cost, it is the
   protocol.
3. **The cluster is the unit** (`rr1---sn-qxau5-btqd` keys to
   `sn-qxau5`): per-host state misses the blackhole; per-cluster
   memory with probe-and-rehabilitate catches it.
4. **Evidence classes have ranks**: a delivered payload (≥64 KB)
   outranks a probe answer, which outranks a handshake, which never
   outranks a fresh death. The pool's integrity rules (M8-ai) are
   exactly these ranks made non-downgradable.
5. **QUIC on this stack is a non-cost**: gVisor's ICMP
   port-unreachable for udp443 has been on the wire since the M8-j
   work; the "fallback elapsed" log lines are heuristic pairings (a
   drop beside the *next* TCP flow to the same host, whenever the
   app's ladder gets there), not QUIC timeouts.
6. **The app is part of the system**: three of the five victories
   (scout, lift, fast-fail) are the engine *cooperating with the
   app's own retry ladder* rather than replacing it.

---

## 5. The remaining 5–10% (the residual, decomposed)

The tester's report of debug.28: 90% of the initial delay gone; a
small residual remains on mobile that the desktop lanes almost never
show. The debug.28 report's evidence decomposes it into candidates,
each with its measurement path:

1. **The app's dead-host ladder residue** (unlucky opens): the app
   still cycles qxau5 hosts against instant resets (fastFails=90)
   before its ladder escalates — the app's own schedule owns this
   time; the engine's share is already zero per attempt. *Measure:*
   the gvideo-gated auto-timeline pairs (debug.29) against the
   fast-fail log lines' timestamps.
2. **Per-flow setup cost**: every new connection pays ~150 ms DoH +
   ~100 ms dial + ~350–400 ms to first byte through the chain
   (~500–600 ms total, visible on every `tunnel up`/`first-byte`
   pair). The desktop lanes open fewer flows per video (browser
   pooling) and never pay the tun-stack's connection churn.
   *Measure:* the first-byte percentiles per host class, already in
   the top-10 table.
3. **The first-video discovery cost**: the dead-cluster memory is
   per-session — the first unlucky open after start (or after the
   10-minute mark TTL) pays ~3 flows × ~2.5 s before the cluster is
   marked. *Candidate fix for a future round:* persist cluster marks
   beside the pool file (the ADR-0047 pool-file precedent), with a
   short seeded TTL and the existing probe/rehab as the safety net.
4. **Stall-kill churn on API hosts**: `www.youtube.com`/`i.ytimg`
   flows killed at ~8–10 s idle with requests pending force the app
   to re-open connections (each re-open pays the setup cost again).
   Low counts this session (lean=3, rich=3); the desktop lanes keep
   their keep-alives longer by construction.
5. **First-data → first-frame buffering**: the auto-timeline marks
   the first video *bytes*, the tester feels the first *frame*; the
   gap is the app's buffer policy against the chain's throughput
   (2.1 MB in ~15 s observed). *Measure:* pair the auto-timeline with
   the tester's manual marks (the buttons stay authoritative).

The next report decides which of these owns the residual; none of
them is a regression, and none justifies touching the machinery that
produced the 90%.

### 5.1 The verdict (the debug.29 report, 2026-10-21)

The tester: *"I think it became better — every video opened in 5–6
seconds initial load."* The instrument read: **auto-timeline
play → first-data elapsed=500 ms over 6 pairs** (gvideo-gated),
`dead=1 fastFails=84 verdicts=0`, `payloadVerified=5`,
`cooldowns=1 lifts=1 scouts=1`, 12.5 MB delivered on the session.
The decomposition resolves:

- **The engine's share of the felt load is ~10%, and it is
candidate 2 — the chain's structural price, measured.** Play →
video data through DoH + dial + TLS + first byte = 500 ms, and
uniformly: every live-host `tunnel first-byte` this session sat in
the 443–650 ms band. Candidate 3 (the discovery cost, the
pre-registered persistence fix) is **not indicted**: six pairs
averaging 500 ms leaves no room for a multi-second outlier (the
worst possible single pair is ≲1.5 s even if the other five ran at
300 ms), the dead mark held all session (`dead=1`, marked early),
and the pool fed itself five times — the memory did its job from
the first open.
- **Candidate 1 owns the residual.** The log shows the app cycling
the dead `sn-qxau5` set before every open — the same host
(`rr2---sn-qxau5-btqk`) retried across 11+ seconds at the app's
own ~1.3 s rhythm, every attempt answered instantly at the door
(zero wire, `fastFails=84`), until its ladder escalates to a fresh
player fetch; then the engine's 500 ms; then first-data →
first-frame buffering (~0.5–1 s at the observed throughput) —
**press-play → frame ≈ 5–6 s, matching the felt number**. The
engine cannot run the app's ladder faster than the app's own
timers: the instant reset is the field-tested optimum (the
HTTP-verdict alternative was the debug.27 sedative — §3, M8-aj).
- **Candidate 4 stays post-open lifecycle** (`stallKillsRich=7`,
all killed at 10 s+ idle — between-videos connection churn, not
initial load).
- **Why the desktop lanes barely feel it:** the same dead-URL
ladder exists in the browser player, but pooled connections and a
different retry schedule amortize the cycling the app pays per
video.

**Conclusion: this is the release state.** The one untried lever —
failing dead-cluster hostnames at the DNS layer, so the app may
skip unresolvable URLs instead of cycling them — stays a
**post-release experiment candidate** (the pre-registered shape:
an A/B knob, `dnsSkips=` counters, the retreat plan), not a blocker:
the app's reaction shape is unknown, and negative DNS caching could
outlive the engine's dead mark (10-minute TTL), undermining the
probe/rehabilitate path. The arc ends where §1 said it did: a 1–2
minute open became 5–6 seconds, of which the engine's share is half
a second.

### 5.2 The owner's hand-off (M8-am, D105: the connect-time sidestep)

The verdict above attributes the remaining ~5 s to the YouTube
app's own machinery — and the owner answered with a sidestep rather
than an engine change: when Younblock connects, the shell now
refreshes (drops the app's cached background process) and opens
the YouTube app itself (ADR-0051, D105: a Settings row, on by
default). A cold start through the already-routing tunnel never
cycles the pre-connect ladder at all — the app wakes to a tunnel
where the blackholed cluster is already marked dead and the pool
is already fed, which is exactly the state the debug.28/29 engine
had built by mid-session, now present from the first second. The
engine's own code is untouched by this (no proxy, dialer, pool or
tier change): the hand-off moves the APP's starting line, not the
chain's cost. What the next report can measure: whether the first
video after connect opens measurably faster than the 5–6 s
mid-session baseline — the one number this war has not yet taken.

---

## 6. Lessons for the next engineer

- **Never ship a theory the report didn't prove.** The SNI-flag
  breaker shipped on a plausible theory and cost two rounds; every
  round after the prove-before-change gate moved the number.
- **Make the instrument first.** Half the victories are counters
  (payloadVerified, fastFails, big/small, verdicts) that turned an
  argument into an arithmetic fact.
- **Pre-register the retreat.** ADR "does NOT decide" sections turned
  the debug.27 regression from a disaster into a decisive experiment.
- **The bug you can't see is in the wiring.** The two worst latent
  bugs (the `markGoodIP` replace, the `goodSubstKnob` 0-vs-−1 no-op)
  were both found by the round's own tests, not the field — keep the
  pins honest and let them fail loudly.
- **Respect the app.** The wins came from failing fast in the
  app's own language (resets) and feeding its ladder (fresh URLs on
  live clusters), not from pretending to be smarter than its player.

---

*Primary sources: ADR-0027…ADR-0050 in
[`08-architecture-decisions.md`](08-architecture-decisions.md); the
M8 sections of the repository `PROGRESS.md`; the CHANGELOG's debug
build entries; the release notes of
`v1.6.6-android-debug.18` … `v1.6.6-android-debug.29`.*
