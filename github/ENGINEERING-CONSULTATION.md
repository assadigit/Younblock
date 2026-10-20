# Younblock — Engineering Consultation Document

**Subject:** YouTube video media transfer failure on Iranian networks — `*.googlevideo.com` body starvation while every other YouTube host class stays fully functional.

**Field report analyzed:** `1.6.6-debug.23 diagnostic report`, Android, generated `2026-10-20T18:47:34Z` (≈ 22:17 Tehran, evening).

**Build under test:** v1.6.6-debug.23, versionCode 22 (commit `a57a215`, APK `v1.6.6-debug23.apk`, 55,283,553 B, SHA-256 `c5d48c50a8c56e061b3c002db04d9711aa139668e7eaae848ed9db3a7c718c7c`).

**Purpose of this document:** the project owner wants to consult an external backend/network engineer. This file is written to be fully self-contained: no prior context with the project is assumed. It contains (1) the problem definition, (2) what has and has not been tested, (3) a summary of the logs, and (4) our ranked assumptions — plus the architecture background, the reference systems, and the concrete questions we would like the consultant to attack.

**What Younblock is:** an Android app (Go core, gVisor TUN `VpnService`) that captures all device traffic, resolves through its own DoH stack, and relays TCP/443 flows through a fragmented-TLS lane designed to survive SNI-based DPI. It is a *direct* circumvention tool — no VPS, no proxy server, no encryption tunnel to a third party. The censor sees the client's real IP talking to Google's real IPs; our only weapons are packet shaping and origin selection.

---

## 1. Defining the problem

### 1.1 One-paragraph definition

On the tester's Iranian network, the app keeps **YouTube's feed path fully functional** — home refreshes, the internal API (`youtubei.googleapis.com`), thumbnails (`i.ytimg.com`, `yt3.ggpht.com`), stats/heartbeat (`s.youtube.com`) all resolve, dial, handshake, and deliver complete responses — while **the video media path (`*.googlevideo.com`) never transfers meaningful data**. Media flows complete their TCP dial (58–230 ms) and TLS handshake (first byte 500–750 ms, 100% success), the client sends its HTTP requests, and then the response body arrives at **2.8–12.1 KB per connection before total silence**. The engine's stall watchdog reaps each flow after 8–10 s of an unanswered request; the YouTube player retries with new connections and new `rr*` hostnames; every attempt repeats the same shape. The spinner never resolves. Total session download: **1.4 MB** — a few percent of one 10-minute video.

### 1.2 Symptom history — how it escalated to this point

| Build | Field symptom (tester's own words/behavior) |
|---|---|
| debug.18–19 | Videos took a long time to *start* (~45 s cold start) but eventually played. |
| debug.20 | "Way way worse." (Evening test — first report of a time-of-day regime change.) |
| debug.21 | "Video lags to begin." New: mid-video stalls around minute 5 of a 10-minute video; **a page refresh recovered playback instantly**. More frequent on Linux, less on Windows. |
| debug.22 | "It doesn't even open video after a while. But the feed refreshes. Probably one of the worst versions." |
| **debug.23 (this report)** | "Feed refreshed, but video failed to load, kept loading forever." — the same class split, now measured in detail (below). |

The trajectory — *start lag → mid-stream stall recoverable by refresh → complete open failure with a working feed* — is the single most important behavioral fact. Each stage is a **deeper starvation of the same media path**, and the last stage is stable across refreshes.

### 1.3 The class split, precisely (this report's own numbers)

| Host class | Flows | Failures | Delivered payloads (evidence) | Verdict |
|---|---|---|---|---|
| `rr*---sn-qxau5-btq*.googlevideo.com` (media caches) | 13 | **13 (100%)** | 2.8–12.1 KB per flow, then silence | **Broken** |
| `redirector.googlevideo.com` (media redirect) | 9 | 3 | some 3.9 KB responses complete; others stall at 5.2–20.8 KB | Mostly works |
| `youtubei.googleapis.com` (API) | 4 | 2 | 365.4 KB and 149.6 KB full responses | **Works** |
| `i.ytimg.com` (thumbnails) | 7 | 4 | 233.0 / 181.0 / 116.4 KB full images | **Works** |
| `yt3.ggpht.com` (avatars) | 10 | 5 | 9.5 KB images, repeated | **Works** |
| `s.youtube.com` (stats) | 4 | 2 | completes | Works |
| `www.youtube.com` | 1 | 1 | completes (slow first byte) | Works |

All of this is **concurrent, not temporal**: in the same 85-second log window, API responses and thumbnail images complete in full while media flows starve. This is not a general network brownout; it is discrimination by destination class.

### 1.4 Environment facts and unknowns

- Android device (model not recorded — **data gap**), app in TUN VPN mode (`tun: active flows=12 rstUnmapped=0 overCap=0 doorFailures=0`).
- Tehran timezone. This report was generated at ~22:12 local (evening). A prior round documented a **regime flip between 04:17 and 18:04 Tehran** on the same code lineage (dead-origin probes went silent → answered; DoH dials 6 → 59), so time-of-day is a known variable.
- **ISP not recorded** — the public reference projects for this ecosystem are explicitly ISP-specific (see §5). **Data gap.**
- Config: `mode=youtube method=auto port=4525 fragmentAllRouted=true`. Since debug.22, `auto` resolves to the **legacy** fragmentation preset: the ClientHello is cut into 300 random fragments sent at 1 ms intervals (byte-compatible with the tester's proven Python prototype, `backend.py`).
- UDP/443 (QUIC) is dropped by design (ICMP fast-fail), forcing the app onto TCP.

### 1.5 What "failure" means mechanically (the loop the player is stuck in)

1. Player opens 3–5 parallel media connections to fresh `rr*` hostnames (DNS returns the **same `sn-qxau5` cluster every time** — see A5).
2. Each flow: dial OK → fragmented TLS hello OK → first byte in 500–750 ms → handshake completes → client sends range request(s) → **2.8 KB arrives, then nothing**.
3. Stall watchdog kills the flow at 8 s (lean) / 10 s (rich-pending) with `cause=pending` — the request was outstanding and the origin never answered.
4. Player retries: new connections, new hostnames, sometimes 1000+ re-requests on a single keep-alive connection (one flow uploaded **172.7 KB** — 14× what it received — over 77 s while receiving 12.1 KB, ≈ **160 B/s**).
5. Goto 1. Forever.

---

## 2. What has been tested — and what has not

Six field-test rounds shipped before this report (each with unit tests "both-ways" — the fix stashed to prove the test fails — but only the field reports are truth):

### 2.1 Chronology of what was changed and what the field said

| Round | Hypothesis tested | Change shipped | Field outcome |
|---|---|---|---|
| debug.18 (M8-z) | Cold-start latency from DoH serialization | DoH hedged walk, serve-stale, negative re-ask groundwork | Start lag persisted (~45 s) |
| debug.19 (M8-aa) | Dead cache-node IPs pinned by DNS | Origin probing, penalty box, good-IP preference, raced first-byte dialing, salvage re-dials | Start lag persisted |
| debug.20 (M8-ab) | Dead-origin pool can't be escaped by hostname rotation | **Cross-origin substitution** (dial a verified-good pool IP with the dead host's SNI), fast-start probe lane, churn-kick probe lane, persisted good pool | "Way way worse" (evening regime; mid-stream decay) |
| debug.21 (M8-ac) | Our own death-classification was self-harming under a hostile regime | Evidence-weighted marking (reset = 20 s box no demote; pending-stall = demote; idle = never), box rehabilitation via probe, front rotation, DoH brownout gating, removed the 45 s probe lane | Start lag + **minute-5 mid-video stall**, refresh recovers, Linux > Windows |
| debug.22 (M8-ad) | Long-lived connections wedge (90 s budgets, dead keepalive, trickle policing) | Rich-pending 10 s fast kill, tuned TCP keepalive (15/5/3), trickle kill, stall kills never invalidate DNS, `auto` → legacy 300-fragment preset | "Doesn't even open video after a while. Feed refreshes." Worst so far |
| **debug.23 (M8-ae)** | Our own hello **volume** flags the source per SNI class (engine emitted ~10–40× the reference implementation's googlevideo hello volume) | **Wire discipline**: one hello on verified-good fronts, race skipped, once-per-round DNS invalidation, probe trickle (10 IPs / 15 min, no start pass, no churn kick), **googlevideo emission breaker** (3 first-byte terminals in 30 s → 45 s door-block with zero wire) | **This report**: hello volume collapsed (81 hellos / 53 routed flows), breaker never armed (`cooldowns=0`), and the media starvation is **unchanged** |

### 2.2 Hypotheses retired with evidence (tested and disproven)

- **DNS resolution path** — `dns ok 111 fail 0`; `dns=0ms` (cache) on most dials, 192–251 ms on fresh resolutions; DoH `dials 13 reuse 22 queries 35 hedgeWins 0 brownouts=1`; `dns detail: no server failures recorded`. The DoH lane has serve-stale, negative re-ask, singleflight, and hedging. Not the problem.
- **TCP dial path** — `dialNear 50 dialFar 4`, zero dial timeouts; media dials 58–230 ms. Not the problem.
- **TLS hello path** — first byte arrived on **100% of flows including all 13 media flows** (500–750 ms); `gvideo discipline: hellos=81 cooldowns=0`; zero resets this session (vs. 35 probe-reset verdicts in the earlier hostile-regime report). The hello layer is *clean* right now. Not the problem.
- **TUN routing** — `rstUnmapped=0 overCap=0 doorFailures=0`, UDP fast-fail working (`udp443 fallback elapsed=1-3ms` on most hosts). Not the problem.
- **App/API layer** — feed, API, thumbnails all deliver (see §1.3). Not the problem.
- **"We flag ourselves with hello volume"** — the debug.23 discipline landed as designed (volume ≈ reference levels) and the failure is byte-for-byte the same shape. Either volume was never the operative variable, or the flag it feeds **persists on a timescale longer than our quiet windows**, or the harm has moved down to the body layer. Volume discipline is necessary hygiene regardless; it is not sufficient.
- **Stall/keepalive wedge** (debug.22's target) — the 60–90 s wedges are gone; kills land at 8–10 s. The *symptom* moved, the *disease* didn't.

### 2.3 What has NOT been tested — the open matrix (the consultant's terrain)

1. **Media-body verification.** Every "verified good" verdict in the system — origin probes, raced first-byte wins, substituted wins — certifies a **TLS handshake only**. Nothing in the engine has ever downloaded an actual ranged `/videoplayback` chunk to certify an IP as *media-capable*. Untested: whether **any** googlevideo IP on this network can deliver media bytes at all right now.
2. **Substitution on/off A/B.** The substitute candidate engages automatically when the pool holds gvideo-flagged marks. It has never been field-tested as an isolated variable. In this report 18/18 substitution "wins" produced flows that delivered ≤ 12.1 KB and died — circumstantial evidence of zero benefit and possible harm (it displaces a raced slot and binds the flow to a pool IP).
3. **Time-of-day controlled comparison on one build.** The documented 4 AM vs 6 PM regime flip was observed *across different builds*. A morning re-test of debug.23 has been requested from the tester but has not happened yet.
4. **ISP identity and alternate networks.** Never recorded. The public reference tuning tables are per-ISP (MCI: 80–250 fragments; Irancell: 10–40). A second SIM / different WiFi on the same device at the same hour is untested.
5. **sn-cluster escape.** Every `rr*` hostname this session resolved into the **same `sn-qxau5-btq{r,d,s,y}` cluster**. Untested: rewriting videoplayback hostnames onto a *different* sn cluster (cache pool) — the app's own hostname rotation never leaves Google's geo-assigned cluster for this resolver.
6. **UDP/443 (QUIC) passthrough.** Dropped by design; 91 packets dropped this session, one 58 s fallback window on `s.youtube.com`. QUIC passthrough is untested (and would forfeit hello fragmentation).
7. **Raw-path controls (no app in the loop).** Never measured: `curl`/openssl from the device or a tethered PC through the fragment lane to a googlevideo IP, with and without SNI, same hour. There is **no on-path packet capture at all** — everything we know is engine-internal counters.
8. **Playback quality floor.** The release notes asked the tester to try 240p/360p when the data path is squeezed (throttle-shaped failures sometimes pass low-bitrate streams). Not reported back.
9. **backend.py side-by-side under identical conditions.** The tester asserts the Python reference works on their network. It has never been run as a control at the same hour/device as a failing Go session.
10. **TLS record-level / application-layer byte accounting.** The engine counts bytes but not records. We currently **cannot distinguish** "handshake completed, zero application data" from "small HTTP error response" in those 2.8 KB flows. This is a pure instrumentation gap (fixable engine-side; see §7).

### 2.4 Instrumentation gaps (what we cannot currently see)

- No per-flow TLS record timeline (count, sizes, inter-arrival).
- No first-**application**-byte metric (`first-byte` today = first TLS record from the server, i.e. the ServerHello).
- No per-flow flag on the `done` line saying whether the winner was the original DNS answer or a substituted pool IP.
- The emission breaker's sensor counts only first-byte-class terminals (`gvideoTerminals`) — stall kills are invisible to it (see A4).
- The 2.8 KB bodies are encrypted relays; we log sizes but not record boundaries, so their content class (handshake-only vs HTTP error vs first media slice) is inferred, not measured.

---

## 3. Summary of the logs (the debug.23 report)

### 3.1 Counters, annotated

```
platform: android — 1.6.6-debug.23 (versionCode 22), generated 2026-10-20T18:47:34Z
state: on  mode: youtube  method: auto  port: 4525  fragmentAllRouted=true
response time: 1048 ms            ← status call latency (includes a DoH freshness step)
conns total 55, live 5, errors 17  ← 17/55 = 31% of connections errored
dns ok 111, fail 0                 ← resolver healthy
up 637.2 KB, down 1.4 MB           ← whole session ≈ 3% of one 10-min video
plainFirstByteTimeouts 0, dialNear 50, dialFar 4, slowFirstBytes 4
stallKillsLean 17, stallKillsRich 14, racedSaves 2, salvagedSaves 0
routedFlows 53, gvideoTerminals 0, goodDrops 17
doh: dials 13, reuse 22, queries 35, hedgeWins 0, brownouts=1
stall kills: lean=17 (pending=10 idle=7) rich=14 (pending=14 trickle=0) leanBudget=8s
origin probe: passes=0 ips=0 answered=0 silent=0 reset=0 good=4 penalized=6 strikes=6 rehabilitated=0
substitution: attempts=18 wins=18 pool=4 (gvideo 0)
gvideo discipline: hellos=81 cooldowns=0
tun: active flows=12 rstUnmapped=0 overCap=0 doorFailures=0 dnsQueries=22 udpDropped=91 v6Dropped=10
auto-timeline: play-suspected -> first-data elapsed=54499ms pairs=2
```

Reading of the decisive lines:

- **`gvideo discipline: hellos=81 cooldowns=0`** — the round's volume goal landed (81 hellos against 53 routed flows ≈ 1.5/flow overall; ≈ 3.5 per *media-class* flow — primary + 2 raced rotations + substitute, with no probe hellos and no salvage re-dials). Compare debug.22's steady-state ≈ 10–40× the reference volume. **And it changed nothing** — which retires "hello volume alone is the operative variable" (§2.2).
- **`gvideoTerminals 0` vs. 13 dead media flows** — no media flow died at first byte. They all died **mid-stream** (stall kills, `cause=pending`). The emission breaker's sensor (`failFlow` terminals) never saw them, hence `cooldowns=0`. The breaker watched the wrong death class all session.
- **`substitution: attempts=18 wins=18 pool=4 (gvideo 0)` + `goodDrops 17`** — every substitution "win" was a first-byte win whose flow subsequently stalled and demoted its winner: 18 wins, 17 drops, pool drained to 4 (all non-gvideo API fronts — the only IPs that ever delivered real payloads). The gvideo pool at report time is **empty**; the mark→win→stall→drop cycle consumed it. (Substitute candidates are drawn *only* from gvideo-flagged marks — so the 18 attempts ran on the session-start persisted pool and early marks; by report time nothing gvideo-flagged survives.)
- **`stall kills: lean=17 (pending=10 idle=7) rich=14 (pending=14 trickle=0)`** — pending dominates 24/31: the client's requests were outstanding and the origins went silent. `trickle=0`: the trickle killer never fired (its 1 MB arming floor was never reached — no flow ever got close to 1 MB).
- **`origin probe: passes=0`** — the probe trickle runs every 15 min and the log window (and likely the session) is shorter, plus one brownout. No rehabilitation happened; 6 penalized IPs sat boxed the whole session.
- **`auto-timeline: play-suspected -> first-data elapsed=54499ms pairs=2`** — ~55 s from play to even a *down-burst* (which is API/thumbnail bytes, not media). Matches "kept loading forever."

### 3.2 The one-shot dead-origin probe (at report time)

```
probe dead-origin sni=www.google.com result=silent
probe dead-origin sni=none        result=silent    (× 2 penalized IPs, both variants silent)
```

Two of the boxed IPs are **fully TLS-silent to any SNI** — genuinely blackholed at the hello layer. The IP population is therefore mixed: some IPs hello-dead (correctly boxed), the rest hello-alive but **body-dead** (the class no current mechanism can act on).

### 3.3 The four key excerpts (from the 85-second log window, 18:46:13 → 18:47:38)

**(a) The substitution false-win cycle** — three hosts, three identical corpses:

```
18:46:32.631  tunnel up class=routed rtt-class=near dns=193-251ms dial=58-84ms host=rr{3,4,5}---sn-qxau5-btq{d,y,s}.googlevideo.com
18:46:33.329  tunnel first-byte class=routed 697ms host=rr4---sn-qxau5-btqy.googlevideo.com
18:46:33.329  tunnel first-byte substituted class=routed dials=2 host=rr4---sn-qxau5-btqy.googlevideo.com
              (same pair for rr5 and rr3, 724 ms / 742 ms)
18:46:42.375  tunnel done class=routed dur=9744ms up=3.1 KB down=2.8 KB host=rr3---sn-qxau5-btqd.googlevideo.com
18:46:42.375  tunnel stall killed class=routed idle=8038ms down=2.8 KB tier=lean cause=pending host=rr3-...
              (same for rr4 and rr5 — dur≈10.2 s, down=2.8 KB each, cause=pending)
```

Three different hostnames, dials milliseconds apart, all "won" by substitution, all delivered **exactly 2.8 KB**, all killed with the client's request outstanding. The identical byte count across three different origins is the signature of a fixed-size response shape — either handshake-completion with zero application data, or one identical small HTTP response (see §4, A2; we cannot yet distinguish — §2.4).

**(b) The 77-second starvation flow** — the player begging a silent origin:

```
18:47:20.037  tunnel done class=routed dur=77123ms up=172.7 KB down=12.1 KB host=rr1---sn-qxau5-btqr.googlevideo.com
18:47:20.037  tunnel stall killed class=routed idle=10156ms down=12.1 KB tier=rich cause=pending host=rr1-...
```

A "rich" (payload-proven) connection lived 77 s, received 12.1 KB (**≈160 B/s**), while the client uploaded **172.7 KB** — a 14:1 upload:download inversion on a download flow. This is the throttle-to-zero shape on the original-IP path (this flow predates the substitution burst; its first byte was 1508 ms, the session's slowest, not substitution-tagged).

**(c) The healthy control class, in the same seconds** (the split is concurrent):

```
18:46:14.119  tunnel done class=routed dur=17323ms up=3.8 KB down=181.0 KB host=i.ytimg.com        (full image)
18:46:17.307  tunnel done class=routed dur=20667ms up=66.1 KB down=365.4 KB host=youtubei.googleapis.com  (full API response)
18:47:15.880  tunnel done class=routed dur=21572ms up=3.5 KB down=116.4 KB host=i.ytimg.com        (full image)
18:47:21.831  tunnel done class=routed dur=15605ms up=24.0 KB down=149.6 KB host=youtubei.googleapis.com
```

Note: these support-class flows deliver their payloads and are then killed in the *keep-alive* phase (an outstanding follow-up request went unanswered ≥10 s) — support-class failures are "died after delivering"; media-class failures are "never delivered." The distinction is bulk throughput, not liveness.

**(d) QUIC retry waves** (secondary):

```
udp443 dropped (×91 across the window, in pairs)
18:47:05.303  udp443 fallback elapsed=58373ms host=s.youtube.com
18:47:20.305  udp443 fallback elapsed=21373ms host=yt3.ggpht.com
18:46:32.293  udp443 fallback elapsed=1ms host=rr3---sn-qxau5-btqd.googlevideo.com   (fast-fail working)
```

The ICMP fast-fail works per-flow (1–3 ms fallbacks), but the app re-attempts QUIC in waves — one 58 s window on `s.youtube.com` — stretching time-to-first-frame and multiplying connection attempts before the TCP path even starts.

### 3.4 Derived metrics (this round's decision numbers)

| Metric | Value | Interpretation |
|---|---|---|
| hellos / routed flows | 81 / 53 ≈ **1.5** | Volume discipline achieved (debug.22 was ~10–40× reference) |
| hellos / media-class flow | ≈ 3.5 | One raced round per flow (primary + 2 rotations + substitute); no probe/salvage hellos |
| Media-flow failure rate | **13/13 = 100%** | With **100% first-byte success** — the failure is strictly post-handshake |
| First-byte success (all flows) | 100% | Hello layer clean; zero resets this session |
| Effective media throughput | 2.8–12.1 KB / flow; ≈160 B/s on the long flow | 3–4 orders of magnitude below requirement |
| Breaker arms | **0** (sensor blind: 0 terminals vs 13 stall deaths) | Design defect, not root cause |
| Substitution win→drop | 18 wins → 17 goodDrops, pool gvideo 4→**0** | The pool's certification criterion is invalid in this regime |
| Pending share of kills | 24/31 = 77% | Requests outstanding, origins silent — the dominant death shape |
| Play → first data | 54.5 s ×2 pairs | Even non-media bursts took ~a minute |

---

## 4. Assumptions (ranked, with evidence and falsification tests)

> These are hypotheses, not conclusions. Each carries the test that would kill it. We are actively seeking an outside view on the priors.

### A1 — SNI-class body discrimination by the middlebox (primary hypothesis)

**Statement:** the censorship path permits the TLS handshake for `*.googlevideo.com` but starves the response *body* — per SNI class, likely after a per-(source, SNI-class) flag accrues over minutes of activity. The "after a while" onset, the feed staying healthy (different SNI class), and the concurrent split in §3.3(c) are all this hypothesis's fingerprints.

**Supporting:** 100% first-byte success with 100% media failure; identical 2.8 KB corpses across three origins; 160 B/s on a proven connection; the public reference ecosystem documents per-ISP DPI tuning and mid-flow interference (§5); the documented time-of-day regime flips.

**Counter / open:** we have never measured record-level timing (§2.4), so "handshake passes, body starved" is inferred from byte counts; a Google-side or cluster-side degradation could mimic parts of it (see A7).

**Falsify in one evening:** instrument the TLS record timeline per flow (engine-side, no new permissions); run a raw control (curl through the fragment lane to a googlevideo IP, with/without SNI, same hour, no app); repeat the same video at 07:00 vs 22:00 on the same build; run once on a different ISP.

### A2 — Substitution false-wins actively amplify the failure

**Statement:** the good pool's IPs are certified by handshake only; in this regime every gvideo-flagged mark is a false positive. Each substituted flow displaces a raced slot, "wins" the first-byte race, and binds the flow to an origin that can handshake but cannot serve media — while the original DNS answer's candidate (which might have worked) is closed as a race loser. The system cannot learn from this: win → mark → stall → drop → pool drains to zero gvideo marks.

**Supporting:** `attempts=18 wins=18` with zero substituted flows delivering >12.1 KB; `goodDrops=17` ≈ the win count; `pool=4 (gvideo 0)` at report time; the mark/drop churn matches the counters exactly.

**Counter:** the original-IP path starved too (excerpt b), so substitution is not *necessary* for failure — it may only be *useless* rather than harmful; we have no A/B.

**Falsify:** one build with `GoodIPSubstitution=0` (the knob exists), same hour, same device; per-flow winner-class on the done line (instrumentation).

### A3 — The pool can never re-learn under the current criteria (structural consequence of A2)

**Statement:** with first-byte as the only certification, a body-starved network makes the gvideo pool a self-consuming buffer: wins refresh false marks, pending kills drop them, the 15-min probe trickle (10 IPs/pass, brownout-gated, `passes=0` here) cannot rehabilitate or discover, and cold starts depend on a persisted pool of yesterday's false positives (5-min seed TTL).

**Supporting:** pool 4 (gvideo 0) after 18 wins; zero rehabilitations; goodDrops ≈ wins.

**Falsify:** watch pool size across a 30-minute session; add a media-byte verification class (§7) and observe whether any IP can be certified at all — that measurement *is also* the A1 test.

### A4 — The emission breaker's sensor watches the wrong death class (design defect, not root cause)

**Statement:** the breaker arms on `gvideoTerminals` (first-byte-class terminal deaths, `failFlow`). In the current regime media flows die by **stall kill** instead — 13 deaths, zero terminals, `cooldowns=0`. Had the sensor seen them, the engine would have gone quiet on googlevideo for ~45 s windows and forced the app into its own hostname rotation — which is exactly the recovery the reference ecosystem prescribes ("refresh 2–3 times and wait", §5).

**Supporting:** the report's own numbers (§3.1); the code path is unambiguous (stall kills demote via `dropGoodIP` only; they never call `failFlow`).

**Counter:** none — this one is provable from the log alone. It doesn't explain the starvation; it explains why the engine had no answer to it.

**Falsify/fix:** extend the sensor to pending-class googlevideo stall kills; field-verify `cooldowns>0` and (if A1 holds) observe whether quiet windows + rotation recover playback.

### A5 — sn-cluster convergence defeats hostname rotation

**Statement:** every `rr*` hostname this session resolved into the **same `sn-qxau5-btq{r,d,s,y}` cache cluster** (Google's geo-DNS for this resolver/region). The player's own recovery — new video, new `rr` hostname — never leaves the harmed cluster, so rotation cannot escape; and our substitution pool (when non-empty) draws from the same cluster's IPs.

**Supporting:** all four observed sub-codes share `qxau5`; Google returns this cluster consistently to Iranian resolvers; the reference projects' "rotate hostnames" recovery works *for them* (their clients resolve from different vantage points / the harm there was per-IP, not per-cluster).

**Counter:** we have never observed a *different* cluster from this network, so "other clusters are healthier" is untested conjecture.

**Falsify:** DNS-answer rewrite experiment — resolve a googlevideo name through a non-Iranian egress (or use a known foreign sn code), pin the media flow to that cluster's IP, and measure record-level throughput; compare with the native cluster.

### A6 — QUIC drop adds startup latency (secondary, not the media failure)

**Statement:** the UDP/443 drop forces the app through retry waves before TCP fallback (91 drops, one 58 s window). It stretches time-to-first-frame and multiplies connection attempts, but cannot explain the class split (feed works over the same TCP lane).

**Falsify:** count QUIC waves preceding the first media byte; optionally gate the drop off for `s.youtube.com` only.

### A7 — Google-side / CDN-path degradation (low prior, kept honest)

**Statement:** the `qxau5` cluster's path to Iranian users is degraded at Google's or transit side (peering), independent of censorship.

**Supporting:** none direct — only that we cannot yet exclude it.

**Counter:** the time-of-day regime flips and the "after a while" accrual pattern fit a censor-side flag far better; Google-side degradations do not usually respect SNI class boundaries this cleanly.

**Falsify:** fetch the same videoplayback URL from a VPS outside Iran (cluster health check); compare throughput to the same IP from inside.

### What we believe the root cause is **not**

Our own hello volume (measured at reference levels now), DNS, dialing, the TUN, the app, the API layer, and the stall/keepalive machinery (all measured healthy or fixed in prior rounds). The failure lives **between the TLS handshake and the first megabyte of media body, on one SNI class** — and our current instrumentation ends exactly at its boundary.

---

## 5. Reference systems (the working baselines we are compared against)

### 5.1 `backend.py` (in-repo, the tester's proven prototype; 244 lines)

- Single dial per CONNECT, **no racing, no probing, no substitution, no penalty box, no background traffic** — one connection, one fragmented hello, one relay.
- `num_fragment=300`, `fragment_sleep=0.001` — the hello is cut into 300 random fragments at 1 ms (this is the "legacy" preset our `auto` method now matches since debug.22).
- `my_socket_timeout=21` s; `TCP_NODELAY` on.
- DoH through its own fragmented lane, with a **forever DNS cache** (a plain dict — an IP that once worked is never re-resolved; an accidental sticky good-pool).
- The tester reports it works on their network. **The critical contrasts with our Go engine:** zero background googlevideo emission (we now match this), zero re-dials on failure (the Python lets the *browser* retry; we absorb retries in-engine), and — importantly — it serves a **desktop browser**, not the Android YouTube app inside a TUN (different connection counts, no QUIC-in-VPN retry storms, different player behavior).

### 5.2 `GFW-knocker/gfw_resist_HTTPS_proxy` (public, same ecosystem)

- The current recipe is xray-core ≥1.8.3 native TLS fragmentation (serverless config, no intermediate server); the older `pyprox` uses 87 fragments @ 5 ms.
- The README's tuning table is **per-ISP**: MCI (همراه اول) 80–250 fragments, Irancell (ایرانسل) 10–40; `fragment_sleep` 1–10 ms — i.e., the DPI's tolerance differs by operator, which is why our unrecorded ISP is a data gap.
- Its own recovery model, verbatim: *"اگر روی نت شما ویدئو ها load نشد دو سه بار refresh صفحه بزنید و چند لحظه صبر کنید"* — "if videos don't load on your network, refresh the page 2–3 times and wait a bit." **App-level hostname rotation after a failure streak** — the same recovery our (sensor-blind) breaker was designed to force.
- It also concedes: *"ممکنه ایپی ها روی نت شما فیلتر باشد"* — "the IPs may be filtered on your network" — IP-quality is a first-class variable in this ecosystem.

### 5.3 `mahdisml/WebImmortalGuards` (public)

The same Python lineage packaged as an app; focuses on DNS-manipulation protection; no origin intelligence. Reviewed during the debug.23 design round (its API was unreachable at this document's writing time; the characterization is from that earlier review).

---

## 6. Questions we want to put to the consultant

1. **Does per-SNI-class body throttling match your field model of the Iranian DPI stack?** Handshake-pass / body-starve on `*.googlevideo.com` specifically, with `youtube.com`-class untouched concurrently. Which mechanism would you expect — per-class token buckets, mid-flow blackholing keyed on the (never-encrypted) SNI, or destination-IP-group policing?
2. **The identical 2.8 KB corpses** (three origins, same byte count, requests outstanding): handshake-completion-only (zero application data) or one identical small HTTP response? Which is more consistent with your experience, and what cheap measurement would discriminate fastest?
3. **Flag keying:** would you expect the "after a while" accrual to key on (client IP, SNI class), (client IP, connection count), or per-destination-IP? The feed staying healthy points away from pure client-IP keying — do you agree?
4. **Is sn-cluster forcing worth engineering**, or does Google's geo-DNS defeat it (all clusters equally harmed from Iranian vantage points)?
5. **Priority ordering** for the next round under a one-tester field budget: (a) TLS-record-level diagnostics, (b) substitution media-verification or removal, (c) breaker sensor extension, (d) sn-cluster escape experiment, (e) QUIC passthrough?
6. Anything in the counters (§3.1) or excerpts (§3.3) that contradicts our reading, or any cheap on-device control we have not thought of?

---

## 7. Proposed next-round sketch (draft — deliberately NOT shipped with this document)

This round is documentation-only, per the project owner's request. For completeness, the current thinking, in dependency order:

1. **Instrumentation first** (everything else depends on it): per-flow TLS record timeline (count/sizes/inter-arrival), first-application-byte metric, winner-class (original vs substituted) on the done line. All offline-testable, zero field risk.
2. **Breaker sensor extension**: pending-class googlevideo stall kills feed the emission breaker. Provable in unit tests today; field-verifiable via `cooldowns>0`.
3. **Substitution re-scoped**: no gvideo substitution until an IP holds a *media-byte* evidence class; or disabled outright pending the A/B.
4. **The A1/A5 discriminating experiment** (record-timeline + cluster rewrite + raw curl control) — the one evening of measurement that would collapse the hypothesis space.

---

## Appendix A — Raw diagnostic report (verbatim, as received)

```
Younblock 1.6.6-debug.23 diagnostic report
platform: android
generated: 2026-10-20T18:47:34Z
state: on (detail "")
mode: youtube method: auto port: 4525 fragmentAllRouted=true
response time: 1048 ms
measured sleep for 1ms request: 1 ms
counters: conns total 55, live 5, errors 17, panics 0
counters: dns ok 111, fail 0, up 637.2 KB, down 1.4 MB
counters: plainFirstByteTimeouts 0, dialNear 50, dialFar 4, slowFirstBytes 4
counters: stallKillsLean 17, stallKillsRich 14, racedSaves 2, salvagedSaves 0, routedFlows 53, gvideoTerminals 0, goodDrops 17
doh: dials 13, reuse 22, queries 35, hedgeWins 0 (#1:0 #2:0 #3:0 #4:0 #5:0) brownouts=1
self-test: not run in this session
top 10 slowest or failed hosts:
  host=yt3.ggpht.com class=routed flows=10 failures=5 plainKills=0 worstFirstByte=743ms
  host=rr1---sn-qxau5-btqr.googlevideo.com class=routed flows=4 failures=4 plainKills=0 worstFirstByte=1508ms
  host=i.ytimg.com class=routed flows=7 failures=4 plainKills=0 worstFirstByte=822ms
  host=redirector.googlevideo.com class=routed flows=9 failures=3 plainKills=0 worstFirstByte=1086ms
  host=rr3---sn-qxau5-btqd.googlevideo.com class=routed flows=3 failures=3 plainKills=0 worstFirstByte=742ms
  host=rr4---sn-qxau5-btqy.googlevideo.com class=routed flows=3 failures=3 plainKills=0 worstFirstByte=727ms
  host=rr5---sn-qxau5-btqs.googlevideo.com class=routed flows=3 failures=3 plainKills=0 worstFirstByte=724ms
  host=youtubei.googleapis.com class=routed flows=4 failures=2 plainKills=0 worstFirstByte=705ms
  host=s.youtube.com class=routed flows=4 failures=2 plainKills=0 worstFirstByte=664ms
  host=www.youtube.com class=routed flows=1 failures=1 plainKills=0 worstFirstByte=626ms
plain-relay: flows=1 firstByteTimeouts=0 worstFirstByte=1372ms
stall kills: lean=17 (pending=10 idle=7) rich=14 (pending=14 trickle=0) leanBudget=8s
origin probe: passes=0 ips=0 answered=0 silent=0 reset=0 good=4 penalized=6 strikes=6 rehabilitated=0
substitution: attempts=18 wins=18 pool=4 (gvideo 0)
gvideo discipline: hellos=81 cooldowns=0
dns detail: no server failures recorded
dead-origin probe (one-shot, 2 penalized IPs):
probe dead-origin sni=www.google.com result=silent
probe dead-origin sni=none result=silent
probe dead-origin sni=www.google.com result=silent
probe dead-origin sni=none result=silent
timeline: no marks (the "I pressed play" / "First frame" buttons stamp it)
auto-timeline: play-suspected -> first-data elapsed=54499ms pairs=2 (heuristic; the buttons stay authoritative)
android shell: 1.6.6 (versionCode 22)
tun: active flows=12 rstUnmapped=0 overCap=0 doorFailures=0 dnsQueries=22 udpDropped=91 v6Dropped=10
```

The full 200-line log window (18:46:13.773 → 18:47:38.548) is preserved in the project owner's message thread; the annotated excerpts in §3.3 cover every distinct event class in it: media starvation kills, substitution wins, the 77 s / 172.7 KB-up inversion flow, healthy API/thumbnail completions, QUIC drop/fallback lines, and the four one-shot dead-origin probes.

## Appendix B — Current configuration values (debug.23)

| Knob | Value | Role |
|---|---|---|
| FirstByteTimeout | 2.5 s | budget for the origin's first TLS record |
| FirstByteRaceDials | 2 | parallel candidates per raced round (+1 substitute when pool has gvideo marks) |
| FirstByteSalvageDials | 2 | in-flow re-dials after a dead first byte |
| StallTimeout (lean) | 15 s default / **8 s on Android** | both-silent reap for unproven flows |
| StallRichTimeout / Pending / Bytes | 90 s / 10 s / 32 KB | proven-flow hygiene / fast kill with request outstanding / escalation floor |
| TrickleWindow / MinBytes / FloorBytes | 12 s / 128 KB / 1 MB | per-connection throttle policing (never armed this session) |
| OriginKeepAlive idle/interval/count | 15 s / 5 s / 3 | surfaces blackholed peers in ~30 s |
| DeadIPPenalty + strikes | 60 s, escalating [60 s, 2 m, 5 m] | timeout-class death demotion + box |
| ResetIPPenalty | 20 s | reset-class death: box only, no demote |
| OriginProbeGoodTTL | 15 m | good-mark lifetime |
| GoodIPSubstitution / PoolDials | 1 / 3 | substitute candidate engaged, up to 3 per round |
| GoodPoolSeedTTL | 5 m | persisted pool (`googlevideo-good-ips.txt`, cap 32) seed lifetime |
| OriginProbePerPass / GoogleVideoProbeEvery / ProbeRehabPerPass | 10 / 15 m / 4 | the probe trickle |
| GvideoBreakerTerminals / Window / Cooldown | 3 / 30 s / 45 s (+≤10 s jitter) | the emission breaker (**sensor: first-byte terminals only — the A4 defect**) |
| DNSCacheNegativeReask | 1 s | resolver negative re-ask rate limit |
| DoH | 5 servers, hedged walk, serve-stale 6 h, brownout gate 3 dials/60 s | resolver stack |
| Fragmentation (`auto`) | **legacy**: 300 fragments @ 1 ms, sweep legacy→strong→balanced→light | matches `backend.py` |
| UDP/443 | dropped with ICMP 3/3 fast-fail | forces TCP |

## Appendix C — Build provenance and repo map (for the consultant)

- Build: commit `a57a215` ("M8-ae: the wire-discipline round (ADR-0040/0041/0042) — debug.23, versionCode 22"); APK `v1.6.6-debug23.apk` (55,283,553 B, SHA-256 `c5d48c50a8c56e061b3c002db04d9711aa139668e7eaae848ed9db3a7c718c7c`), signed with the project's pinned debug key; release `v1.6.6-android-debug.23` (id 418735179).
- Code map: `internal/proxy/tunnel.go` (flow lifecycle: race, stall tiers, substitution, breaker feed), `internal/proxy/server.go` (good pool, penalty box, probe jobs, door blocking), `internal/engine/engine.go` (background lanes: prewarm, probe trickle, brownout gate), `internal/doh/` (resolver), `internal/fragment/` (hello presets), `internal/config/defaults.go` (every knob, one-line comments), `mobile/` (Android TUN + shell), `backend.py` (the Python reference).
- Decision log: `docs/android-port/08-architecture-decisions.md` (ADR-0001…0042, the full design history); `DECISIONS.md`, `PROGRESS.md`, `CHANGELOG.md` at the repo root.

---

*Prepared 2026-10-21 by the Younblock development agent at the project owner's request. Field data is pasted from the app's built-in diagnostics screen by the project owner (the tester); no on-path capture exists, so every claim above is grounded in engine-internal counters and logs, and every inference is labeled as such.*
