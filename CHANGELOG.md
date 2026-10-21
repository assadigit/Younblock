# Changelog

All notable changes to Younblock are documented here. Versions are the
GitHub release tags (what you download); the in-app version is noted beside
each one. The full engineering narrative behind every entry lives in
PROGRESS.md, and the reasoning in DECISIONS.md (D-numbers referenced below).

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
dates are Asia/Tehran. Binaries, SHA-256 checksums and upgrade notes for
every release are on the
[Releases page](https://github.com/assadigit/Younblock/releases).

## [Unreleased]

### Added (2026-10-21 — M8-am, debug.30, versionCode 29; the release-candidate polish — the owner's two pre-release asks, D104/D105)

- **The real launcher icon (D104).** The owner's own `icon.ico` —
  the padlock with the white play triangle in its red body, the gold
  lightning bolt with its light outline, the impact dashes — is now
  the app icon: traced from the 32 px source to crisp vector
  geometry (per-layer IoU-verified against the source, VLM-checked
  faithful), shipped as the adaptive icon (the brand dark behind
  the mark, every launcher mask respected) with legacy raster tiles
  for older contexts. The notification keeps the simple pill mark —
  the status bar tints small icons monochrome, where the pill reads
  and the detailed mark would not.
- **The connect-time hand-off (D105).** When Younblock connects —
  the moment the tunnel is genuinely routing — the app now
  refreshes (drops the cached background process) and opens the
  YouTube app, so the first video cold-starts through the live
  tunnel instead of resuming the app's pre-connect dead-host retry
  ladder (doc 11 §5.1 measured that ladder owning most of the felt
  5–6 s first-open load; the hand-off is the owner's sidestep,
  §5.2). A new Settings row in the menu sheet carries the switch —
  **on by default** per the ruling, saved like every setting, read
  fresh at the moment of connect; a change applies on the next
  connect. Once per connect: a degraded-recovery flap never
  re-launches over the user, and nothing opens unless the TUN is
  actually bound. Playback behavior is untouched — the engine's
  proxy/dialer/pool/tier code is byte-identical to the blessed
  debug.28/29 state; this build is the release candidate wearing
  its final shell.

### Fixed / Added (2026-10-21 — M8-al, no build; the debug.29 field verdict, recorded)

- **"I think it became better — every video opened in 5–6 seconds
  initial load."** The gvideo-gated auto-timeline's first field
  verdict: **play → first video data = 500 ms over 6 pairs** — the
  engine's share of the tester's felt load is ~10%, and it is the
  chain's structural price (every live-host first byte this session
  sat in the 443–650 ms band; DoH + dial + TLS + first byte). The
  remaining ~5 s is the YouTube app's own dead-host ladder — the
  report's log shows it cycling the blackholed `sn-qxau5` set
  before every open (the same host retried across 11+ seconds at
  the app's own ~1.3 s rhythm, each attempt answered instantly at
  the door, `fastFails=84`) before its ladder escalates to a fresh
  player fetch — plus first-frame buffering. The pre-registered
  persistence candidate is NOT indicted (no pair escaped the
  average; the dead mark held all session; the pool fed itself five
  times). Recorded in
  `docs/android-port/11-the-video-opening-war.md` §5.1 (plus the
  scoreboard row and arc paragraph): **this is the release
  state** — a 1–2 minute open became 5–6 seconds, of which the
  engine's share is half a second. The one untried lever (DNS-layer
  failure for dead-cluster hostnames) is noted there as a
  post-release experiment candidate, not a blocker. No code, no
  versionCode consumed; `v1.6.6-android-debug.29` (versionCode 28)
  remains the latest artifact and the release candidate.

### Fixed / Added (2026-10-21 — M8-ak; shipping in `v1.6.6-android-debug.29`, versionCode 28)

- **Build artifact:** commit `f84d562` ("M8-ak: the
  method-documentation round (ADR-0050, doc 11) -- debug.29,
  versionCode 28"); release `v1.6.6-android-debug.29` (id 419414818,
  the repo's Latest), APK asset `v1.6.6-debug29.apk` (57,114,657 B,
  SHA-256
  `e1a306b4b230f242846ea2d05c12439c4752cf39a902a07a656405132cfd9904`,
  the pinned debug certificate c76bb7c6…8551d, versionCode 28 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 by aapt; the round's
  timeline/version strings verified in libgojni.so; the asset
  re-downloaded and cmp'd byte-identical).

- **"By far our best version yet — 90% of the initial delay of each
  video is gone… still worth investigating the remaining 10-5% delay
  in mobile… document whatever you did to not miss this
  accomplishment/method."** (1) **The method is documented**:
  `docs/android-port/11-the-video-opening-war.md` — the scoreboard
  (the 1–2 minute opens of debug.18 → the ~1 second opens of
  debug.28), the loop method (the report as the instrument, diagnosis
  from the report's own evidence, executed both-ways proofs,
  pre-registered retreats, the paper trail), the round-by-round arc,
  the six semantic discoveries, the residual decomposition, and the
  lessons — indexed in the pack's reading order. (2) **The residual
  gets its instrument (ADR-0050)**: the auto-timeline re-scopes to
  googlevideo — a play is a googlevideo flow after googlevideo quiet
  (feed refreshes never touch googlevideo), first-data is a
  googlevideo first byte (with the substitution lane retired, that is
  a live video flow answering). The old heuristic closed pairs on
  1.4 MB i.ytimg feed bursts that carried no video at all; the next
  report's pairs measure play → video data — the tester's actual
  "initial delay" — and decompose the remaining 5–10% (the app's
  dead-host ladder, the ~500–600 ms per-flow setup cost, the
  first-video discovery cost, stall-kill churn, buffering) by
  comparison with the fast-fail timestamps.

### Fixed / Added (2026-10-21 — M8-aj; shipping in `v1.6.6-android-debug.28`, versionCode 27)

- **Build artifact:** commit `874b768` ("M8-aj: the lane retires
  round (ADR-0049) -- debug.28, versionCode 27"); release
  `v1.6.6-android-debug.28` (id 419378998, the repo's Latest), APK
  asset `v1.6.6-debug28.apk` (57,097,057 B, SHA-256
  `4fda35bc9559b5144c4b7326cf857863ca9410913eb74c7a539d1c15135e2116`,
  the pinned debug certificate c76bb7c6…8551d, versionCode 27 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 by aapt; the
  version/settings/verdict-lane strings verified in libgojni.so; the
  asset re-downloaded and cmp'd byte-identical).

- **"This one did not even open a single video, only feed refreshed" —
  the debug.27 report was the cleanest experiment result the project
  has had, and it killed the bet (ADR-0049).** `attempts=40 wins=37
  big=0 small=37`: the fed pool kept the verdict lane engaged on every
  dead-cluster attempt, every answer was the ~5 KB verdict shape — and
  the player NEVER left the dead URL set (one host took 20 flows, one
  every ~4 s; no live cluster was ever dialed; the session moved
  1.2 MB vs debug.26's 23.1 MB). Worse: `dead=0` — the cluster was
  never marked dead, because a substituted flow never terminals, and
  the cluster memory eats only silent terminals. In debug.26 the same
  regime recovered in ~40 s exactly because the lane (accidentally
  drained) let the flows terminal and the door's fast-fail RESETS let
  the player's ladder escalate to a fresh player-API fetch. **The RST
  is the ladder's escalation signal; the HTTP verdict is its
  sedative.**
- **The fix (ADR-0049): the cross-origin substitution experiment
  concludes OFF — four field rounds, zero media ever carried
  cross-cluster.** (1) Schema v7 retires it: `experimentSubstitution`
  defaults off and the migration flips it off on every existing file
  (a v6 file's on was the shipped default, never a choice; a v7
  file's explicit on is the owner's word and rides). (2) The off
  position becomes REAL: the debug.24 engine knob resolved the
  switch's off to `0`, which the proxy's Config defaulting reads as
  "use the default" — **the off switch has been a no-op at engine
  start since debug.24** (found by this round's own re-pinned report
  test before the field saw anything); it now returns `-1`, the
  knob's actual disabled shape. (3) Everything else from debug.27
  stays (the pool fixes, the counters, the honest door reasons); the
  door returns to debug.26's probe → fast-fail for dead clusters, the
  reset the player escalates on.

### Fixed / Added (2026-10-21 — M8-ai; shipping in `v1.6.6-android-debug.27`, versionCode 26)

- **Build artifact:** commit `bfc18ed` ("M8-ai: the pool keeps its
  food round (ADR-0048) -- debug.27, versionCode 26"); release
  `v1.6.6-android-debug.27` (id 419074187, the repo's Latest), APK
  asset `v1.6.6-debug27.apk` (56,144,837 B, SHA-256
  `3c402d83ddbcae8ab352f06a5d8b5f6081012e1b5addd6e0804627c97a0aaa9a`,
  the pinned debug certificate c76bb7c6…8551d, versionCode 26 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 by aapt; the M8-ai
  verdict-lane/door-reason/version strings verified in libgojni.so;
  the asset re-downloaded and cmp'd byte-identical).

- **"First video still buffers (less than before), later videos mostly
  open nearly quickly" — the debug.26 report showed the M8-ah machinery
  working and the remaining cost was the app's dead-cluster ladder
  (ADR-0048).** dead=1 (the cluster memory engaged), payloadVerified=6
  (the pool fed itself), cooldowns=1 lifts=1 scouts=1 (the breaker
  nearly silent), attempts=14 wins=12 (the substitute lane carried
  flows) — but fastFails=150: the player ground its hostname rotation
  against instant resets for ~40s per unlucky open because the verdict
  lane STARVED mid-storm. The pool was an OUTFLOW machine, drained
  through three mouths: substitute race losses demoted their pool
  origins (the M8-ab rule), pending stall kills demoted the origins
  substituted flows rode, and — the biggest, found by this round's own
  test before the field saw it — every first-byte WIN replaced the
  winner's good-table entry with a handshake mark, un-flagging
  media-certified origins one flow at a time (six certifications in,
  the session ended holding two).
- **The four fixes (ADR-0048), one property — a media certification
  (≥64KB delivered payload or an answered real-SNI probe) is revocable
  ONLY by its 15-minute TTL:** (1) the no-downgrade rule (a handshake
  mark refreshes a certified origin without clearing the flag);
  (2) the demote guard (death demotes skip media-certified marks — the
  strike-escalated penalty box stays the rotation guard); (3) the feed
  clears the box (a fresh media delivery outranks the stale death that
  boxed it); (4) the verdict lane made visible — the door's
  dead-cluster pool pass counts and logs (`verdicts=` on the clusters
  line), every substituted flow classifies by size (`big=`/`small=` on
  the substitution line), and the doors log their refusal REASON
  (`cluster-dead` vs `breaker-cooldown`) instead of blaming every
  refusal on the breaker. With the pool fed, every dead-cluster flow
  the app sends rides a substitute and answers on live wire instead of
  taking a reset — IF the player retires a URL set faster on an HTTP
  verdict than on resets, the unlucky opens lose their whole ladder
  (the next report's verdicts= vs fastFails= settles it).

### Fixed / Added (2026-10-21 — M8-ah; shipping in `v1.6.6-android-debug.26`, versionCode 25)

- **Build artifact:** commit `6eddbab` ("M8-ah: the dead-cluster
  memory round (ADR-0047) -- debug.26, versionCode 25"); release
  `v1.6.6-android-debug.26` (id 419033462, the repo's Latest), APK
  asset `v1.6.6-debug26.apk` (56,141,329 B, SHA-256
  `e4148c8bedbe882ecaacdc97b054b14c3399df4180d509456fc42407330ad3ff`,
  the pinned debug certificate c76bb7c6…8551d, versionCode 25 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 by aapt; the M8-ah
  cluster strings verified in libgojni.so; the asset re-downloaded
  and cmp'd byte-identical).

- **"Every video takes initially 1-2 minutes to open, after opening
  it's smooth" — the debug.25 report decomposed the delay exactly, and
  none of it was the breaker's fault this time (ADR-0047).** The whole
  `sn-qxau5` cluster is BLACKHOLED: 21 flows across rr3..rr7, every
  one `worstFirstByte=0ms` (the TCP dial accepts, no TLS byte ever
  returns), each burning its full 3-second first-byte budget — while
  `sn-aj4g55`, `sn-u14g55` and `sn-4g5ednkd` answered instantly and
  carried 1.8 MB / 559 KB. The 1-2 minutes were: ~21 three-second
  burns interleaved with 7 breaker cooldowns (7 × ~15s ≈ the whole
  delay — one arm even blocked a live-cluster dial), and the M8-ag
  escapes couldn't help: `pool=13 (gvideo 0)` — the network-test
  button was the pool's only food and it was never pressed
  (`attempts=0`), even as the scout flows themselves carried
  megabytes past the empty table.
- **The four fixes (ADR-0047):** (1) **dead-cluster memory** — three
  distinct hosts of one sn-cluster silent-terminal inside 10 minutes
  mark the CLUSTER dead: its hosts then fail INSTANTLY at the door
  (zero wire, no 3-second burn) so the app's hostname rotation
  reaches live clusters in seconds; one probe flow per dead cluster
  per 2 minutes watches for recovery, and an answered own-dial
  rehabilitates at once. (2) **the payload-fed pool** — a
  googlevideo flow that delivered ≥64 KB certifies its origin
  media-verified automatically (5× above the old false-positive
  class, 6× below the smallest real flow): the pool feeds itself
  from the app's own traffic, no button. (3) **the multi-cluster arm
  gate** — the breaker now needs terminals from TWO distinct
  clusters before arming: a single-cluster death burst is the
  blackhole signature (cluster memory's job), the per-source flag
  bleeds across clusters (the breaker's job). The debug.25 session
  would have armed ZERO times. (4) **the pool file's versioned
  format** (ADR-0045's flip condition) — persisted entries carry a
  per-line `media` marker; the next session seeds them back into the
  POOL, so after one good session every future session's FIRST video
  rides the verified origin.
- **Reading the next report:** a new `gvideo clusters:` line —
  `dead=… fastFails=… probes=… rehabilitated=… payloadVerified=…`.
  dead≥1 with fastFails>0 = the memory engaged; payloadVerified>0
  beside `pool=… (gvideo N>0)` = the pool fed itself; attempts/wins>0
  = the substitute lane finally carried flows; cooldowns≈0 in a
  single-cluster regime = the arm gate holds.
- Tests: the round's contract pin
  (TestClusterMemoryFastFailsTheDeadCluster: mark → instant 502 with
  zero wire (<500 ms measured) → pool override → own-dial rehab →
  payload feed, pool gvideo ≥ 2); the two arm tests rewired across
  two clusters (their old single-cluster shapes failed the moment the
  gate landed — its natural both-ways); the M8-ab persistence
  round-trip extended with a session-2 leg (the `media` line seeds
  `pool=1 (gvideo 1)`). Both-ways proofs EXECUTED for the door's
  cluster branch, the payload feed, the dead-host feed, and the media
  seeding (each stashed → its pin FAILS). Full suite green; -race
  green on the house gate; gofmt list identical to HEAD's. The
  engine's TestNetworkTestGatingAndStatus real-network flake appeared
  again at its documented ~10% HEAD rate — untouched, recorded in
  ADR-0046.

### Fixed / Added (2026-10-21 — M8-ag; shipping in `v1.6.6-android-debug.25`, versionCode 24)

- **Build artifact:** commit `0405f1f` ("M8-ag: the breaker stops
  starving the player (ADR-0046) -- debug.25, versionCode 24");
  release `v1.6.6-android-debug.25` (id 418991235, the repo's
  Latest), APK asset `v1.6.6-debug25.apk` (56,068,289 B, SHA-256
  `b29a3aab77b656a5651bc0cf82222025db65fa5a3c62d1bf65c4f227bfbdb251`,
  the pinned debug certificate c76bb7c6…8551d, versionCode 24 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 by aapt; the M8-ag
  string signatures verified in libgojni.so; the asset re-downloaded
  and cmp'd byte-identical).
- **"Didn't work, it even failed to play a video. but feed
  refreshes." — the debug.24 field report, read end to end, was the
  breaker finally working exactly as designed and killing playback
  with it (ADR-0046).** The W5 sensor fix armed the emission breaker
  on the play-burst's dead cache cluster (`sn-qxau5`: flows=3
  failures=3, worstFirstByte=0 ms each) — and then ONE 45 s+jitter
  cooldown blanketed the whole session: 58 door-dial refusals with
  zero wire, the player's retry budget dead by +46 s, video never
  opened, feed perfect. The same report proved the wire healthy
  TWICE: the network test answered 5/5 addresses under the real SNI
  (media-verifying the pool to 5 gvideo entries) and the one media
  flow that slipped through before the arm delivered 406 KB
  (win=orig). The breaker starved a healthy wire behind its own door
  — and starved the substitution pool with it (attempts=0: the
  door-block short-circuits before the raced round where substitutes
  engage).
- **The four fixes (ADR-0046):** (1) the **pool escape** — a
  media-verified address is answered-under-the-real-SNI evidence, the
  strongest wire-alive proof the engine owns, so a flow the pool can
  serve PASSES the door and rides the substitute lane instead of
  being silenced; (2) the **scout** — each cooldown's FIRST
  googlevideo flow goes to the wire (the app's own request, zero
  synthetic emission) so an armed breaker is never blind: against a
  dead cluster it terminal like any flow, against a live one it
  delivers video; (3) the **lift** — any googlevideo first byte
  (serial, raced, salvaged, substituted, or plain path) ends an
  active cooldown on the spot: a speaking wire falsifies the arm's
  premise, and the breaker becomes self-correcting in both
  directions; (4) the **budget** — the cooldown drops 45 s → 15 s:
  the block can never again outlive the player's measured retry
  patience (last googlevideo attempt +46 s in the report), while 15 s
  still spans the app's 2-7 s retry cadence for hostname rotation.
  The experiment switches keep their meaning: breaker-off is still
  the exact debug.22 shape; breaker-on with substitution off is still
  the pure M8-ae door (plus the one-flow scout).
- **Reading the next report:** the discipline line gains two numbers
  — `gvideo discipline: hellos=… cooldowns=… lifts=… scouts=…
  fragPackets=…`. `lifts>0` after a play attempt = the wire spoke
  through an armed breaker; `scouts>0` with `cooldowns>0` and
  playback recovered = the scout carried it; `attempts>0`/`wins>0`
  beside a fed pool = the substitution lane finally engaged through
  the pass.
- Tests: the two breaker door tests rewired to the new contract (the
  corpse fixture: scout dials once → second flow gets the 502 → a
  media-verified mark mid-cooldown lets the third flow through →
  lapse serves; the echo fixture: the scout's first byte lifts, the
  next flow passes on the lift) plus a pool-escape tail on the corpse
  test; both-ways proofs EXECUTED for all three escapes (scout claim
  stashed → FAIL on the scout's 502; pool check stashed → FAIL on
  the pool-primed flow's 502; lift hook stashed → FAIL on the missing
  lift) and for the default (the knob pin failed at 45 s before the
  change). Full suite green; -race green on the house gate (proxy +
  probe + netx); the engine -race flake in TestNetworkTestGatingAndStatus
  investigated and dismissed as pre-existing (a real-network DoH
  failure riding a 5 s poll — HEAD flakes at the same ~10% rate; 27/3
  over 30 HEAD runs vs 10/2 over 12).

### Fixed / Added (2026-10-21 — M8-af; shipping in `v1.6.6-android-debug.24`, versionCode 23)

- **The prove-before-change round (ADR-0043/0044/0045, the external
  engineer's debug.24 spec executed end to end): the cause of the
  media-body starvation is still unproven — the 2.8 KB corpses are
  probably handshake-only replies, but sizes suggest, they don't
  prove — so this build measures, and stops trusting "handshake
  succeeded" as proof an address can carry video.**
- **ADR-0043 (the evidence):** every googlevideo flow's done and
  stall lines now carry two snapshots of the kernel's own TCP
  counters — one right after the request went out, one at flow end
  — so the report's new `tcp verdicts:` line answers the question
  six rounds had to guess at: upstream-lost (our request packets
  never reached the server) vs downstream-lost (the request
  arrived, the answer is being dropped or throttled) vs both-ok
  (the transport worked — the server/HTTP layer is the suspect).
  A light TLS record timeline (`rec s2c=[…] c2s=[…]`, 12+8
  records, no decryption) plus `first-app-byte` turns the "2.8 KB
  corpse" inference into a measured fact; `win=orig|subst|race`
  names which candidate carried every flow; the fragmenter's TCP
  segment counter (`fragPackets=`) measures hello PACKETS for the
  first time (hello count was measured since debug.23; one legacy
  hello is up to 300 segments); and the report gains `device:` and
  `net:` lines (phone model and operator, permission-free reads —
  the reference tuning tables are per-ISP and no report had ever
  said which one the tester rode).
- **ADR-0044 (the switches and the probe):** the diagnostics
  screen gains an Experiments section — the method selector
  (legacy/strong/balanced/light), the substitution and
  emission-breaker toggles (settings v6, both shipping on, the
  pre-v6 migration flips them on per the v5 pattern), all three
  printed on every report's `experiment:` line and applied live to
  the next flow — and the "Run network test" button: up to six
  current googlevideo addresses probed under all four methods in
  the same minute (the preset-vs-hour confound finally broken),
  with the real SNI, with www.google.com, and with no SNI; an
  i.ytimg.com control per method; per-method packet counts; and
  the optional foreign-cluster lane (the embedded
  experiment_nodes.txt, empty until Ali supplies yt-dlp -g
  hostnames). Only the probe's `answered` class — an HTTP status
  line from a deliberately bad GET /videoplayback — certifies an
  address as media-capable.
- **ADR-0045 (the two behavior changes, decided on the debug.23
  consultation's own evidence):** the emission breaker's sensor
  sees PENDING stall kills now (the one PROVEN defect — 13 media
  stall deaths, gvideoTerminals=0, cooldowns=0: the breaker
  watched the wrong death class the whole session), and the
  substitution pool eats media-verified verdicts only — first-byte
  wins and handshake-only probe verdicts still PREFER (front of
  the dial lists) but never feed the pool (the debug.23 report
  measured the old pool consuming itself on false positives: 18
  substitution wins, every flow ≤12.1 KB, 17 goodDrops, gvideo
  4→0). The persisted pool file becomes the media-verified list
  going forward; seeds prefer, never substitute.
- Tests: both-ways for every behavior change (executed: the W5
  breaker feed stashed → the arming test fails at cooldowns=0; the
  W6 pool scope stashed → the rewired substitution test fails on
  the flow-fed pool; the W1 done-line wiring stashed → the
  evidence test fails on the missing fields; the W9 counter
  stashed → the packet test fails at zero; the W2 desync guard
  stashed → the parser test fails on garbage). Full suite green,
  race detector green on internal/proxy + internal/probe +
  internal/netx — after fixing a pre-existing stalled-fixture echo
  bug the round exposed (a single-read echo racing the fragmented
  hello's pieces: a ~10% flake all along, near-deterministic under
  -race; the fixtures now echo the whole first flight,
  byte-budgeted).

### Fixed / Added (2026-10-21 — M8-ae; shipping in `v1.6.6-android-debug.23`, versionCode 22)

- **"This time, it doesn't even open video after a while. But the
  feed refreshes. this is probably one of the worst versions" — plus
  two reference repositories offered as "knowing how this system
  works" — became the wire-discipline round (ADR-0040/0041/0042).**
  Reading gfw_resist_HTTPS_proxy (backend.py's family: one fragmented
  hello per connection, zero background traffic) and
  WebImmortalGuards against our engine showed the regression that
  compounded across every recovery mechanism since debug.13: ~4
  parallel legacy hellos per video connection plus 48-handshake
  background bursts — the exact scan-like volume a per-SNI-class
  source flag feeds on. Early session clean, then googlevideo dead
  for the rest of the session while youtube.com keeps serving (the
  feed refreshes). (1) **ADR-0040**: a primary whose origin holds a
  live verified mark skips the raced round — ONE hello, the
  reference steady-state shape; a raced round demotes the answer set
  once, not per dead candidate (the DoH churn halves). (2)
  **ADR-0041**: the probe pass is a trickle (OriginProbePerPass
  48→10, clamped inside ProbeOrigins; its own 15-minute
  GoogleVideoProbeEvery cadence; the start-of-session pass and the
  churn-kick lane deleted — a fresh engine is silent on googlevideo
  until the first flow speaks) and the EMISSION BREAKER replaces the
  churn kick: 3 googlevideo terminals inside 30 s arm a 45 s (+
  jitter) cooldown of zero googlevideo wire at both doors while
  non-gvideo hosts keep full service — the app's player rotates
  hostnames in the quiet (the reference recovery) and any per-source
  flag decays. (3) **ADR-0042**: the gvideoHellos meter counts every
  googlevideo-SNI hello on the wire (flows + substitutes + probes)
  and the report gains "gvideo discipline: hellos=N cooldowns=M".

### Fixed / Added (2026-10-20 — M8-ad; shipping in `v1.6.6-android-debug.22`, versionCode 21)

- **"After a while playing … minute 5 of a 10min video, it goes
  loading, and as soon as you refresh it fixes" + "the problem is
  not solved yet (video lags to begin)" + "read the backend script
  again, maybe it gives you ideas" — the tester's post-debug.21
  report named the mid-video wedge and pointed back at the legacy
  Python's proven techniques, and this round delivers both: the
  long-lived-connection fixes (ADR-0037/0038/0039).** (1) **Rich-flow
  liveness (ADR-0037)**: StallRichPendingTimeout (10 s) — a PROVEN
  flow whose client has a request outstanding (the player's next
  range request riding a dead pooled conn) closes at the fast budget
  with the demote landing and the noise quiet (the 90 s hygiene
  budget held the wedge far past the app's patience); tuned TCP
  keepalive on every origin dial (netx KeepAliveConfig Idle 15 s /
  Interval 5 s / Count 3, the DoH lane's dialer included — a
  blackholed peer surfaces as a read error in ~30 s instead of Go's
  default ~2 h, and the relay closes the app's side so its retry
  lands on a fresh origin); the trickle kill (TrickleWindow 12 s /
  TrickleMinBytes 128 KB / TrickleFloorBytes 1 MB) — a rich flow
  whose rolling window keeps delivering above zero but under the
  floor is being per-connection throttled (TCP delivery of a range
  response is line-rate bursty; the player paces REQUESTS, not
  bytes), and the kill closes both ends QUIETLY (no demote — a fresh
  conn to the same origin runs at full speed; no error count) so the
  player's retry lands on a fresh conn: engine-side what the tester's
  REFRESH did by hand. (2) **Stall kills never invalidate the answer
  set (ADR-0038)**: the lean stall kill's invalidateDNS call REMOVED
  (the kill is an origin-conn death; the per-IP demote carries the
  evidence; forcing a re-resolve fed the debug.20 report's churn
  spiral — dns=579-846 ms late-session, 59 DoH dials; the old Python
  cached DNS forever). The answer set's refresh stays with the
  dial-failure path (M8-o) where a whole set failed to connect. (3)
  **Auto starts at legacy (ADR-0039)**: presetFor(auto) returns
  fragment.Legacy — the old Python's 299-cut 1 ms hello, a preset
  that existed since the rewrite but was unreachable (auto started
  balanced, the sweep tried legacy LAST) — and HealSweepOrder walks
  DOWN the strength ladder (legacy → strong → balanced → light);
  under a censor that now RSTs recognizably-shaped hellos (35 of 77
  probe verdicts), weak splitting was never in the fight. (4)
  **Diagnostics**: the stall line gains the rich cause split
  (`rich=N (pending=Q trickle=T)`) — pending = the mid-video
  wedge's killer now bounded at 10 s, trickle = the per-connection
  throttle; stallRichPending/stallTrickle counters on the Diagnostic
  struct.

### Fixed / Added (2026-10-20 — M8-ac; shipping in `v1.6.6-android-debug.21`, versionCode 20)

- **"It became way way worse, it doesn't even play a video after a
  while" — the tester's FIRST debug.20 report (tested at 6:04 PM
  Tehran vs the 4:17 AM debug.19 test) measured a CENSOR REGIME
  CHANGE (dead-origin probe silent→answered, probe resets 0→35, DoH
  dials 6→59: the block moved from IP-blackholing to killing
  connections in flight) plus five self-harming interactions of the
  debug.20 machinery, and this round fixes the self-harm: the
  hostile-network fixes (ADR-0034/0035/0036).** (1) **The
  evidence-weighted marking (ADR-0034)**: markDeadIPClass — a RESET
  death boxes 20 s (ResetIPPenalty) WITHOUT demoting (temporal
  evidence), other first-byte deaths keep 60 s + demote; probe reset
  verdicts box 60 s (was 10 m), silent keeps 10 m; strike escalation
  [60s,5m,30m]→[60s,2m,5m]; PENDING stall kills (lastUp>lastDown: a
  client request the origin never answered) demote at BOTH tiers,
  IDLE kills never demote; the probe pass REHABILITATES up to 4 boxed
  IPs/pass (answered → cleared + good-marked + counted; neutral-SNI
  hellos never gvideo-flag the mark); liveIPs and goodSubstitutes
  ROTATE their verified fronts per call (one server-wide counter —
  consecutive flows spread across verified origins instead of the
  debug.20 report's www.youtube.com 17/17 concentration on one pinned
  origin). (2) **The cadence rebalance + DoH brownout (ADR-0035)**:
  the 45 s fast-start probe lane REMOVED with its knobs (the
  persisted seed + flow-fed marks own pool-stocking; the lane's
  verdicts were mostly resets and its bursts scan-like during
  hostility); the churn kick stays debounced 10 s; the dohBrownout
  tracker (≥3 DoH dials in 60 s → the prewarm/probe/churn lanes pause
  until 60 s of dial quiet; counted; injectable clock for tests).
  (3) **The cause-split diagnostics (ADR-0036)**: stall lines carry
  cause=pending|idle; stallLeanPending/Idle counters; the report's
  stall kills line reads "lean=N (pending=P idle=I)", the origin
  probe line gains rehabilitated=, the counters line gains goodDrops,
  the doh line gains brownouts= — the numbers that decide the next
  round (idle-dominant kills with recovery = lengthen the idle
  budget; pending-dominant = the fast-fail shape is right). Tests:
  proxy/flowkill_test.go (5 shapes) + engine/m8ac_test.go (2), all
  both-ways proven (rotation/reset-class/pending-demote/rehab/brownout
  neuters each FAIL their tests); full suite green (18 packages +
  mobile).

### Fixed / Added (2026-10-20 — M8-ab; shipping in `v1.6.6-android-debug.20`, versionCode 19)

- **"The problem persists" — the tester's FIRST debug.19 report
  measured 45 s of play→first-data (auto-timeline elapsed=44999ms,
  pairs=5) and this round removes its root cause: the dead-origin
  pool worked but its SCOPE was the failing hostname's own rotating
  answer set. The escape-the-pool fixes (ADR-0031/0032/0033).**
  (1) **The cross-origin substitution (ADR-0031)**: a
  *.googlevideo.com flow's raced round replaces ONE own-set rotation
  with a substitute candidate dialing the good pool's VERIFIED-LIVE
  gvideo origins (not in the current answer set, not boxed, freshest
  verdict first, up to 3 walked) — the identical fragmented
  ClientHello replays with the SNI unchanged (Google's video edge
  serves any googlevideo.com name; the Iranian hosts-file IP-pin
  practice rides exactly this), the dial count stays flat, a win logs
  "tunnel first-byte substituted" + substWins, a death demotes the
  POOL origin (never the host's answer set); flow first-byte
  SUCCESSES now feed the pool too (the session's own foreign-cache
  win arms every later flow — the loop the 45 s escape never closed);
  and a first-byte death on a pool origin DROPS its good mark and
  boxes it (the ADR-0028 demote actually implemented — the debug.19
  code's latent leak resurfaced dead IPs at the FRONT of dial lists
  once the box lapsed). Knob GoodIPSubstitution (negative = the exact
  debug.19 shape). (2) **The probe coverage + cadence (ADR-0032,
  Android-only lanes)**: the pass's name list walks the learned code
  store ROUND-ROBIN (one rr name per code, 12 codes a pass, the rr
  index rotating, PerPass 32→48 — every remembered code, foreign
  clusters included, is verified within a few passes instead of the
  freshest-2 local cluster forever), a fast-start cadence (45 s passes
  while the engine is <5 min old and the gvideo pool <8 — the
  debug.19 session ran ONE pass in its whole cold start), and the
  churn kick (failFlow on a googlevideo host bumps gvideoTerminals;
  the gvideo loop's 1 s watch kicks an immediate prewarm+pass,
  10 s debounce — verified origins land inside the app's next retry
  wave). (3) **The persisted good pool (ADR-0033)**:
  googlevideo-good-ips.txt (cap 32, app-private dir, beside the codes
  store) written on change (15 s check) and at Stop, seeded at
  listener start with a 5-minute TTL — the first video of session N+1
  rides the origins session N verified; stale seeds self-correct
  through the demote. (4) **Diagnostics**: the report's
  `substitution: attempts=N wins=M pool=G (gvideo V)` line (the
  round's headline number — wins>0 means the cross-cache bet paid),
  gvideoTerminals on the counters line, the substituted first-byte
  log line. Both-ways proven (substitution_test.go, m8ab_test.go);
  full suite green (18 packages + mobile); race-clean on the touched
  lanes.

### Fixed / Added (2026-10-20 — M8-aa; shipped in `v1.6.6-android-debug.19`, versionCode 18)

- **The first video is STILL slow after debug.18 — the tester's
  report measured the remaining cost and this round removes it: the
  dead-origin-pool fixes (ADR-0028/0029/0030).** The debug.18
  report's own numbers: all 38 remaining errors class=routed, ~20
  first-byte terminals at exactly one 2.5 s budget, three lean stall
  kills at exactly 15 s carrying half-finished responses, the top
  host 13 flows for 8 failures, and the dead-origin probe silent on
  BOTH sni=alt and sni=none (IP-level blackholing — the plain lane
  and DoH the debug.18 round fixed are now healthy). (1) **The
  verified prewarm (ADR-0028, Android-only)**: the 5-minute
  googlevideo prewarm loop (plus one pass at Start) now PROBES the
  answer sets with the exact wire shape a real flow sends — a
  fragmented ClientHello carrying the prewarmed name as SNI through
  the one netx dialer — and marks each answer IP: "answered" →
  verified-live for 15 min (every dial list starts there), "silent"
  or "reset" → dead for 10 min with strike escalation (60 s → 5 min
  → 30 min) ending the re-gamble. (2) **The raced-path salvage
  round (ADR-0028)**: an all-dead race now runs ONE more raced round
  over the answer set's untried unboxed origins before the terminal
  — the app's retry wave collapses into an in-engine budget;
  hopeless sets keep the fast ~2.6 s terminal. (3) **The Android
  lean stall budget 15 s → 8 s (ADR-0029)**: the three
  mid-response corpses each held the app's retry for the whole 15 s
  desktop budget; the platform gate lives in the engine's listener
  build, desktops keep 15 s byte-for-byte, and the report prints the
  budget that fired. (4) **Diagnostics (ADR-0030)**: an
  auto-timeline heuristic (play-suspected after routed quiet →
  first-data on the down-burst — the buttons stay authoritative)
  measures play→video even when the tester doesn't tap; new report
  lines carry the origin-probe pass's verdicts, the box's
  live/strikes counts, the good count, the stall tier split, and the
  racedSaves/salvagedSaves/routedFlows counters.

### Fixed / Added (2026-10-19 — M8-z; shipped in `v1.6.6-android-debug.18`, versionCode 17)

- **The FIRST video after opening the app takes 1–2 minutes (the
  cold-start round, per `docs/specs/debug18-cold-start.md`): the
  plain cohort gets fragmentation and a safety net, DoH gets one warm
  shared connection and a hedged server walk, the QUIC fallback gets
  measured, and the diagnostics report can finally PROVE all of it.**
  (1) **FragmentAllRouted (ADR-0024)**: on Android (only — desktops
  unchanged, hard rule) the first TLS record of EVERY routed app's
  non-`.ir` host is fragmented now, not just the 7 YouTube domains —
  the ad/attestation hosts the YouTube app needs at first play were
  the un-fragmented, silently-hanging suspects; the budgeted
  machinery stays scoped to the list hosts exactly as before, and a
  **new 8-second safety net** closes a silent non-listed TLS origin
  and counts it (`plainFirstByteTimeouts`) instead of hanging to the
  app's ~60 s patience. (2) **DoH (ADR-0025)**: one warm, pinged
  HTTP/2 connection shared by every query (a burst of fresh
  per-video names used to open one fragmented TLS handshake EACH),
  opened at Start and re-warmed within 1 s of a connection-shaped
  failure; the server walk is hedged (the next server at +300 ms, a
  fast failure triggers it immediately, first valid answer wins);
  two no-SNI IP-literal endpoints (1.1.1.1, 8.8.8.8 by IP) appended
  as last resorts; learned googlevideo cache-node codes are
  pre-resolved (rr1..rr5) every 5 minutes. (3) **UDP/443
  (ADR-0026)**: the ICMP port-unreachable stays gVisor's (pinned by
  the M8-j test) while the drop is now logged and the NEXT TCP flow
  to the same host logs the fallback's elapsed time; IPv6 discards
  are counted (`v6Dropped`). (4) **Diagnostics (ADR-0027)**:
  `class=routed|plain` and `rtt-class=near|far` on every tunnel
  line; a "Top 10 slowest or failed hosts" section; a plain-relay
  summary; per-DNS-failure server + phase (TCP/TLS/HTTP) + latency;
  a one-shot dead-origin SNI-vs-IP probe; and **"I pressed play" /
  "First frame" buttons** that stamp the timeline — the report
  prints that window's elapsed time and its failed/slow flows. This
  DEBUG build includes hostnames in the report (the first line says
  so); a release build never does.

### Fixed (2026-10-19 — M8-y; shipped in `v1.6.6-android-debug.17`, versionCode 16)

- **Videos take time to START playing: the eighth report (the first
  from the right build, versionCode 15) measured the two remaining
  sources, and both are memory failures — the fix gives the engine
  two memories.** The report first CLOSED the previous round in
  Ali's own words ("the problem of easily going next and back in
  same video instantly is fixed" — the tiered stall watch verified
  on-device) and then named what remains. (1) The DNS negative-cache
  lockout: `dns ok 142 / fail 50` with `doorFailures=50` — every
  resolver failure killed a connection at the door, and the failure
  lines PAIRED at the same millisecond (four pairs inside 22 ms),
  the signature of the negative cache answering instantly rather
  than the servers being asked. One momentary DoH blip
  negative-cached a batch of the video's fresh per-video hostnames,
  and for the negative's whole 15 s TTL every retry wave died at the
  door in milliseconds. The NEGATIVE RE-ASK (ADR-0023): a
  `DNS_UNREACHABLE` negative is evidence about the resolver's own
  path, not a verdict about the name — such a hit now re-asks the
  servers (concurrent retries collapse into one walk through the
  singleflight), rate-limited to one walk per second per host with
  the failure's own walk counting as the window's first probe, so
  every negative hit is either instant or a genuinely fresh walk and
  a recovered blip costs one round-trip instead of 15 seconds of
  dead connections. Server-verdict negatives (hijack/NXDOMAIN
  shapes) keep the instant fail. New report line: `dns negative
  re-ask (path failure, not a name verdict)`. (2) The dead-origin
  re-gamble: ~17 `reason=timeout` terminals at 2892–4304 ms plus ~8
  resets, all through the raced round, and the SECOND video's start
  paid the full discovery cost again — the dead-IP knowledge was
  per-flow (it died with each connection) while the authoritative
  DNS answer kept returning the same blackholed cache-node IPs, and
  each video's cache hostname is a NEW name. The ORIGIN-IP PENALTY
  BOX (ADR-0023): every classified first-byte death (the origin
  connected, or reset at the hello write, and never spoke) marks its
  IP for 60 seconds; every dial list — both doors, the raced
  candidates, the salvage re-dials — skips the marked IPs, and the
  knowledge crosses flows AND hostnames (the blackhole is per cache
  node, shared between videos). A preference, never a verdict: a
  list the filter would empty dials in full. Not marked: stall
  kills, client-side deaths, teardown-canceled deaths, dial
  failures. Also: the SOCKS5 door's DNS failure line now carries
  `reason=` in the message itself (the 50 bare lines of the report
  could not say why). Knobs: `DNSCacheNegativeReask` (1 s default;
  negative → the old behavior) and `DeadIPPenalty` (60 s default;
  negative → the old behavior).

### Fixed (2026-10-19 — M8-x; shipped in `v1.6.6-android-debug.16`, versionCode 15)

- **"Going next and back in the same video" lags: the stall watch was
  murdering the player's pooled video connections — it is now
  tiered, and a proven connection is only closed at 90 s, quietly.**
  Ali's seventh report (still versionCode 13 — the phone never ran
  debug.15) named the regression in his own words ("videos used to
  have lag for starting to play, but after that you could easily go
  next instantly; now even going next and back in the same video
  lags a lot"), and its log carried the proof: thirteen
  `tunnel stall killed idle=15xxx ms` lines in ~70 s, every one a
  connection that had already carried real data (2.1 MB, 1.6 MB,
  723.6 KB, 633.7 KB, 162.6 KB down). The player reuses pooled
  connections — buffer, quiet past 15 s, next range on the SAME
  connection — and the M8-v 15 s both-directions-idle watch kept
  closing them mid-quiet, so every next/seek re-dialed through a
  ~50 % dirty path (errors 70/122 ≈ 57 %), and each kill also
  demoted a proven-good origin and counted an error. The tiered
  watch (ADR-0022): a flow that proved itself — ≥ 32 KB delivered
  down (real payload), or one ≥ 3 s quiet gap that closed with new
  data (the pacing rhythm; the dead-origin cohort never woke, which
  is what separates the two shapes that overlap in bytes) — is
  closed only at `StallRichTimeout` = 90 s and QUIETLY (no demotion,
  no error count); the lean tier keeps the exact fast 15 s death for
  the never-proved shape. The stall report line now reads
  `tunnel stall killed idle=Nms down=N tier=rich|lean`. The build
  also carries the never-run debug.15 cargo unchanged: the
  first-dial race (`tunnel first-byte raced dials=N`) and the honest
  DNS probe (ClearNegative — this report's method line had flipped
  to balanced after one DNS false-failure fired the heal sweep).

### Fixed (2026-10-19 — M8-w; shipped in `v1.6.6-android-debug.15`, versionCode 14)

- **The per-video start wait, round four: the whole dial budget now
  races in parallel from the flow's start, and the health probe asks
  the DNS servers NOW instead of reading a stale failure memory.**
  Ali's sixth pasted diagnostics report (the first from the
  salvage-race + stall-watch build — versionCode 13) proved both of
  those working (the retry lines batching at the same millisecond,
  the stall watch killing at exactly the 15 s budget, saves at
  738–848 ms, the 60-second mid-stream hangs gone) and named the
  two remaining costs. Two changes (ADR-0021), both the
  failure-detection/hygiene species, never steering: **(1) the
  first-dial race** — the report's six terminal `reason=timeout`
  dones at 5320–5576 ms were the SERIAL FRONT's cost (attempt 0
  still walked its 2.5 s budget ALONE before the salvage race even
  launched) while CONCURRENT flows to the SAME hosts answered at
  72–162 ms in the same seconds — the dirt is per-origin, not
  per-moment — so a processed host's flow now races its primary
  origin against `FirstByteRaceDials` (default 2) extra candidates
  dialed AT THE SAME MOMENT (each on its own rotated start of the
  answer set, each replaying the client's first record byte-exact
  through a fresh fragment conn, the same preset per flow — D102),
  the FIRST origin to speak wins, an all-dead set fails at ~1
  budget instead of 2, and a raced save logs the new report line
  `tunnel first-byte raced dials=N` (a clean primary stays quiet —
  the race costs a healthy flow nothing observable); **(2) the
  honest dns probe** — the report's dns fail 16 with
  `DNS_UNREACHABLE` while the video downloaded 6.7 MB was the
  periodic check reading the 15 s negative cache a momentary DoH
  blip had written for the very name the app's own flows resolve,
  so the dns step now clears exactly that entry first
  (`Resolver.ClearNegative`; a good cached answer stays) and asks
  what the servers say NOW. Negative `FirstByteRaceDials` restores
  the previous behavior exactly (serial attempt 0 + the raced
  salvage re-dials).

### Fixed (2026-10-19 — M8-v; shipped in `v1.6.6-android-debug.14`, versionCode 13)

- **The per-video start wait, round three: the salvage's re-dials now
  RACE in parallel, and a relay that stalls mid-stream is closed in
  seconds instead of the app's minute.** Ali's fifth pasted
  diagnostics report (the first from the salvage build itself —
  versionCode 12) proved the salvage on-device (retry lines
  absorbing dead attempts, `tunnel first-byte salvaged dials=N`
  saving flows at 0.7–1.4 s, errors down to 37/81 ≈ 46 % from 59 %)
  and named the two remaining start-lag sources. Two changes
  (ADR-0020), both the failure-detection/hygiene species, never
  steering: **(1) the salvage race** — the report's 21 terminal
  `reason=timeout` dones at 8014–8227 ms were the SERIAL chain's
  cost (three 2.5 s budgets walked one after another, + 11 terminal
  resets at 0.6–1.9 s), so when the first origin dies at first byte
  the engine now dials ALL its remaining re-dial candidates AT ONCE
  (the live answer set re-resolved, the flow's dead IPs excluded,
  the lists rotated so candidates start on distinct origins), and
  the FIRST origin to speak wins — a silent-host terminal costs ~2x
  the budget (attempt 0 + the race), never 3x, and a save lands at
  the fastest live origin's answer; **(2) the stall watch** — the
  report's five video-start flows lived 57–64 s (first byte plus
  ~7 KB down, then total silence) until the APP's own ~60 s
  patience killed them all in the same instant, and the retry wave
  right after answered in 60–261 ms, so a processed host's relay
  that goes silent in BOTH directions for the new `StallTimeout`
  (default 15 s; negative disables, restoring the previous
  behavior) is now closed engine-side — both ends, the answer set
  demoted (the M8-o loop), the error counted, and a new report line
  `tunnel stall killed idle=Nms` — so the app retries onto a fresh
  origin (with its own budget + salvage) in seconds instead of a
  minute; one-direction silence never fires (a pure download or
  upload is a healthy relay shape), and unrouted relays keep the
  plain shape byte-for-byte (D46). Both-ways proven: with the
  engine change stashed, the stall test, the two race tests and the
  updated salvage test all FAIL exactly as designed; with it, all
  sixteen first-byte/salvage/stall tests PASS and the guards pass
  both ways (the streaming-download guard caught an inverted-stamp
  bug mid-round: the watch must measure silence from the NEWEST
  activity clock, or a download whose client never writes dies
  mid-stream). D102 (retrying an exhausted-salvage flow with the
  NEXT preset) stays Ali's ruling — this report's evidence makes it
  live: terminal first-byte failures still cluster under
  method=strong.

### Fixed (2026-10-18/19 — M8-t; shipped in `v1.6.6-android-debug.13`, versionCode 12)

- **The per-video start wait, round two: dead first-byte origins are
  now retried INSIDE the flow instead of being handed back to the
  app's own wave-after-wave retries.** Ali's third pasted diagnostics
  report (debug.12, versionCode 11) proved the previous round's
  budget on-device — the expected `tunnel first-byte failed
  reason=timeout` closes arrived at ~8.3 s (22 of them) alongside
  fast ~0.5 s `reason=reset` closes (18) — and named what remained:
  roughly half of every video-start wave still died at first byte
  (counters: 38 errors / 61 conns), and the app re-opened wave after
  wave until one landed (~35 s from the first wave to the first
  video bytes). Two changes (ADR-0019), both the
  failure-detection/hygiene species, never steering: **(1)
  FirstByteTimeout retuned 8 s → 2.5 s** — every budget-killed flow
  in the report was 100 % silent (`down=0 B`; the same flows sat out
  the app's full 60 s patience in the previous report: dead nodes,
  not slow ones), while healthy routed origins answered in 420–508 ms
  (one 2399 ms outlier that also carries the ~0.1–0.2 s
  fragment-pause write), so 2.5 s never trips a healthy origin and a
  dead attempt costs 3.3x less; **(2) the salvage re-dial** — when a
  processed host's origin dies at first byte, the engine closes only
  the dead origin (never the client side), demotes the answer set
  (the M8-o loop, per dead attempt), re-resolves while skipping the
  IPs whose origins already died this flow, re-dials the SAME host
  with the SAME preset (captured once per flow — even a mid-flight
  method change cannot vary a flow's wire shape), and replays the
  client's buffered bytes through a fresh fragment conn, byte-exact
  (the first record re-fragmented identically; early data captured
  during the wait rides after it). Up to `FirstByteSalvageDials`
  re-dials (default 2 → 3 total attempts, bounded); the app never
  sees the failure, so its serial retry chain collapses into
  in-engine attempts of ~0.5–2.5 s each. When the attempts run out
  the flow fails fast exactly as at M8-s. New report lines:
  `tunnel first-byte retry reason=… attempt=N` (an absorbed dead
  attempt) and `tunnel first-byte salvaged dials=N` (a flow the
  salvage saved). Per-flow METHOD variation is still NOT done — that
  is D102, Ali's ruling. Everything not on the unblock list keeps
  the old relay behavior byte-for-byte. Host-proven both ways: with
  the salvage stashed (the M8-s shape), the three new tests fail —
  the client sees the budget close, no retry line, no echo; with it,
  they pass (ADR-0019).

### Fixed (2026-10-18/19 — M8-s; shipped in `v1.6.6-android-debug.12`, versionCode 11)

- **The per-video start wait: silent origins now fail fast instead
  of hanging to the app's own ~60 s patience.** Ali's second pasted
  diagnostics report (debug.11) proved the previous round's fixes
  on-device (state on, a heal sweep that found its method) and gave
  the remaining stall its exact shape: at each video start the app
  opens a wave of parallel connections; some complete in ~0.5 s
  while others connect fine, send the ~1.7 KB TLS hello, and never
  receive one byte — the quick ones get reset (~0.5 s, `down=0 B`),
  the rest hang for EXACTLY ~60 s (`down=0 B`). Read back against
  the engine: nothing in it closes a flow at 60 s (the relay idle
  timeout is 300 s; the TUN keepalive only probes) — that closer is
  the app's own patience, and the engine amplified it by holding
  the dead flow open; and because the dial SUCCEEDED, the DNS
  demotion loop (M8-o) never fired, so the suspect address set
  stayed fresh and every retry wave could land on the same dead
  node. **A processed host's origin must now answer within 8
  seconds** of the app's first message (FirstByteTimeout; >12x the
  worst healthy first byte observed, 619 ms), or the flow fails
  fast: both ends closed immediately (the client sees the reset and
  retries at once, as it already retries the ~0.5 s resets), the
  failure is counted, the reason lands in the report's log
  (`tunnel first-byte failed reason=timeout|reset|closed`), and the
  host's cached answer set is demoted exactly like a total dial
  failure — the M8-o loop extended to origins that connect but
  never speak. Everything not on the unblock list keeps the old
  relay behavior byte-for-byte. Host-proven both ways: with the
  fix reverted, the new tests fail at ~5.0 s into the report's
  exact signature; with it, the flow closes in ~250 ms (ADR-0018).

### Fixed (2026-10-18, late night — M8-r; shipped in `v1.6.6-android-debug.11`, versionCode 10)

- **The false "Couldn't fix it": the heal sweep no longer poisons
  itself on the check that triggered it.** Ali's first pasted
  diagnostics report (debug.10) showed the engine degrading 21
  seconds after Start — `state: degraded`, `last failed check: dns
  (DNS_UNREACHABLE)`, "heal sweep found no working method",
  `method: legacy` — while the same report's log showed the
  network recovering and real video data flowing. The
  reconstruction (host-reproduced into the exact same signature,
  ADR-0017): a secure-DNS blip at start failed all three servers
  once (~170 ms each), which negative-caches `www.youtube.com` for
  15 seconds; the automatic heal sweep triggered by that failure
  then probed the SAME name for every method, and every probe
  failed instantly on the negative entry — no server was ever
  asked again inside the window, so a recovered network was
  declared broken and Ali got the "Couldn't fix it. Open
  Younblock." notification on a working connection. **A heal sweep
  now resets the DNS resolver before probing** (the same
  fresh-start semantics the app already applies on a real network
  change): one blip at start can no longer make every method look
  dead. A genuinely dead network still fails honestly (the
  degraded state and the recorded dns failure are unchanged).
- **A failed repair sweep no longer silently switches your
  connection method.** The sweep applies each fragmentation preset
  live as it probes; when NOTHING passed, the last-tried preset
  (legacy) stayed live — the report's `method: legacy` was exactly
  that, dragged off the user's saved choice by the sweep that
  found nothing. A failed sweep now restores the method that ran
  before it.
- **The diagnostics report now says WHY a secure-DNS lookup
  failed.** The report's log ring keeps only the message part of
  each line (by design); the resolver's failure lines carried
  their server and reason as structured attributes, so the pasted
  report showed three bare "dns server failed" lines that could
  not say timeout vs reset vs bad answer. Those lines now bake the
  evidence into the message (`dns server failed server=#N
  reason=…`, the fallback line `…failed=#N now=#M`, and `reason=`
  on the serve-stale and tunnel-lookup lines) — the next pasted
  report names the failure kind, the missing piece for the
  open stall row. No hostnames or user-chosen strings enter the
  report (the server travels as its 1-based index).

### Fixed (2026-10-18, late — M8-q; shipped in `v1.6.6-android-debug.10`, versionCode 9)

- **The diagnostics button now exists on the screen your phone
  actually shows.** Ali's debug.9 round: "I still dont see
  diagnostic button" — exact, because the debug.9 build shipped the
  Diagnostics screen into the app's CLASSIC interface while the APK
  bakes the redesigned (pro) interface (D66's build-time variant
  default; only the Windows/Linux desktops bake classic). On the
  phone, the only diagnostics was a section at the very bottom of
  the menu — below Help — and its Copy could silently fail inside
  the WebView. This build ports the whole surface into the served
  screen (ADR-0016, and D101's standing rule: a surface ships in
  the variant the edition serves): **Diagnostics is now the FIRST
  row of the menu** (open the ⋯ menu — it is right there, no
  scrolling, every edition), opening the full report view with
  **Copy report** (the honest clipboard ladder: the Android-native
  bridge first), **Share** (the system share sheet), **Refresh**,
  and the selectable report text as the path of last resort. The
  old buried section is gone. Verified end-to-end through the real
  webui server — the same bundle and dispatcher the APK serves —
  before release: menu → Diagnostics first row → the report loads →
  Copy → "Copied". The engine report itself (TUN counters, build
  identity, 200-line window) is unchanged from debug.9 and now
  actually reachable on the device.

### Added (2026-10-18, night — M8-p; shipped in `v1.6.6-android-debug.9`, versionCode 8)

- **The in-app Diagnostics screen — "copy paste logs and diagnostics"
  is now one reachable button (Ali's D100 directive).** His debug.8
  round: the stall persisted AND the promised evidence path did not
  exist ("there is no diagnostic option in the apk app setting" — the
  copy button lived buried in Help and could silently fail inside the
  Android WebView, toasting "Copied" over an empty clipboard). This
  build ships the evidence-collection surface itself (ADR-0015): a
  Diagnostics screen in the menu AND in Settings, showing the full
  report with Copy (an honest three-path clipboard ladder — the
  Android-native bridge first), Share (the system share sheet, APK
  only), and the selectable text itself as the path of last resort.
  Help's copy button joins the same ladder and stops claiming false
  success. Every edition gets the screen; the desktops and the Linux
  web edition just don't show Share.
- **The report now carries the evidence only the APK can see.** The
  TUN counters (flows opened/reset-unmapped/over-cap, door failures,
  DNS queries, dropped UDP — the one surface that names the "flow
  never reached the door" case and the QUIC/UDP row's suspect) and
  the exact build identity ("android shell: 1.6.6 (versionCode 8)" —
  a pasted report names its own release). The log window doubled
  from the last 100 to the last 200 lines, so a stalled video's
  whole retry story survives until the copy. Counters only — the
  report's privacy redaction (no hostnames, no addresses) extends to
  the new sections and is pinned by test.
- **No third guess on the stall.** Two root-caused fixes
  (debug.7/ADR-0013, debug.8/ADR-0014) are disproven on-device; the
  next root-cause waits for a real pasted report (doc 10's circle
  row 3 stays open). After a stall: Menu → Diagnostics → Copy, and
  paste the report in the chat.

### Fixed (2026-10-18, later evening — M8-o; shipped in `v1.6.6-android-debug.8`, versionCode 7)

- **Every video, not just the first: the per-video stall loop is
  closed.** Ali's debug.7 round: the feed and UI stayed instant but
  EVERY video took 1–2 minutes to start and re-buffered during
  playback, while the Windows app on the same ISP was instant. The
  M8-n racing dialer cannot help when the WHOLE address set of a
  host is blackholed — each rr*.googlevideo.com hostname carries
  the few addresses of one cache node, so one filtered node poisons
  the entire set. The app retried every few seconds, every retry
  re-dialed the same cached addresses and failed, and the DNS cache
  (a 120 s minimum TTL) kept serving that suspect answer until it
  rotated — the observed per-video wait, and (video URLs rotate rr
  hostnames mid-play) the mid-play re-buffering. The fix (ADR-0014):
  when a door dials every address of a host's answer set and none
  connect, the cached answer is demoted to stale immediately — the
  app's NEXT retry fetches a fresh answer from the DNS servers
  (serve-stale preserved: if the servers then fail, the old answer
  still serves). The resolver's own bootstrap dial now also races
  its addresses (~0.75 s failover instead of a full 3 s per dead
  bootstrap). One mechanism, wired identically on the browser doors
  and the Android SOCKS5 door. Host-proven by seven new tests; the
  on-device confirmation rides Ali's debug.8 round (doc 10's circle
  row 3 stays open until then).

### Changed (release process — D99, 2026-10-18)

- **The release APK is now named with the version.** From
  `v1.6.6-android-debug.8` on, the APK asset on every release page
  carries the version name — `v1.6.6-debug.apk` — per Ali's ruling,
  so the build is identifiable without opening the release page.
  D98 (every release carries the APK) is unchanged; this names what
  it attaches.

### Fixed (2026-10-18, evening — M8-n; shipped in `v1.6.6-android-debug.7`, versionCode 6)

- **Video start on Android no longer waits minutes on the first
  video.** Ali's debug.6 round: the feed loaded fast but the FIRST
  video took 2–3 minutes to actually play, while the Windows app on
  the same network was instant. The root cause was in the code, not
  the network (ADR-0013): the Android tunnel aborted every
  connection whose setup exceeded 10 s — a budget shorter than the
  engine's own honest resolve+dial time — so on filtered networks
  (where googlevideo answers carry silently-dead addresses) the
  phone's connections were killed mid-setup, retried into the same
  wall (the DNS answer stays cached ~2 minutes), and the player's
  retry ladder stacked the visible wait; the desktop has no such
  abort, which is why Windows was instant. Two changes: the engine's
  dialer now fails over between a site's addresses in ~0.75 s
  instead of 5 s per dead address (the Windows/Linux browsers
  benefit too), and the Android tunnel's handshake budget now
  covers the engine's full honest setup, so a flow is never aborted
  mid-setup. Host-proven by timing tests; the on-device
  confirmation rides Ali's debug.7 round (doc 10's circle row 3
  stays open until then).

### Changed (release process — D98, 2026-10-18)

- **Every release page now carries the debug APK.** From
  `v1.6.6-android-debug.7` on, each release in the repo attaches the
  installable debug APK (the same pinned-certificate build the test
  circle installs), per Ali's ruling — no more APK-less release
  pages like debug.4's. versionCode-bumping (D84), the pinned debug
  certificate (D85) and the regular-release discipline (D89) are
  unchanged.

### Cancelled (the planned login gate — announced D94, cancelled D97 the same day; nothing was ever built)

- **The login authorization gate is cancelled (D97, 2026-10-18).**
  The gate announced at D94 (members sign in with a Younblock
  account before they can connect — a login page, a small
  authentication backend with a user database, an admin surface for
  accounts), ruled admin-only at D95 and split into members/free
  editions at D96, was cancelled before a line of it existed:
  "cancel it altogether, no auth needed anymore." Younblock stays
  ONE free-for-all app: no account, no login, no server, no
  editions. Nothing was ever built, so nothing is removed — the
  design-first rule held to the end — and today's "no account, no
  data collection" privacy claims stay literally true, permanently.
  The six-document design pack survives under `docs/auth/` as the
  bannered record of the cancelled feature; D95/D96 remain in
  DECISIONS.md as superseded history; all auth open questions
  closed as moot, never ruled on their merits.

### Fixed (host-side, 2026-10-18 — A0 session; no app-facing change, no new build)

- **`GOOS=windows go build ./...` now compiles the whole tree.** A
  build constraint was missing from `mobile/nonblock_unix.go` since
  its birth (M4), so on Windows it collided with the non-Unix stub
  file and the full cross-compile sweep failed — a defect invisible
  to the shipped apps (every Android ABI is unix; the desktop shell
  always compiled) but real for the verification gate that claims
  "compiles everywhere". One line (`//go:build unix`) fixes it; the
  baseline was re-verified green after (18/18 twice, the Windows
  sweep exit 0). No release follows (nothing app-facing changed).

### Changed (the desktop close ruling, M8-l / D93 — unreleased desktop builds)

- **Closing the app's window now stops it completely, by default.**
  The app shuts down, leaves the taskbar, and puts the Windows
  proxy setting back first — exactly like choosing Quit. (Before,
  closing the window always hid the app into the tray and it kept
  running in the background.) Nothing remains anywhere unless you
  ask for it.
- **New setting — "Keep running in the tray when closed"**
  (the menu's new Settings section, and Settings in the classic
  edition): turn it on to get the old behavior back — closing the
  window keeps Younblock running as an icon near the clock, with
  the tray menu's Quit stopping it fully. The row appears only on
  Windows, where a tray exists; the first-run welcome and the Help
  copy now state the new default honestly.

### Added (the cross-platform UI unification, M8-k / D92 — in the debug.6 test build)

- **Data usage, on every platform (round-3):** while connected the
  screen shows the session's data — "2.10 GB used" with the ↓/↑
  split under the timer, counting up live; after a disconnect,
  "Last session: size · length" sits under Not Connected; and the
  menu's new **Data usage** section shows Today / This month /
  All time, the last sessions, and a confirmed Clear history
  (Delete vs Keep — only Delete clears). The history is one local
  file (`usage.json` beside the settings — times and byte totals
  only, never a website name, never sent anywhere), capped at 500
  sessions, autosaved every 15 s and recovered after a crash or
  power loss. Counted once, in the engine, from the relay counters
  the tunnels have always fed — so the desktop, the Android app
  and the web edition cannot disagree.
- **The "Other websites" honesty section** in the menu: some other
  blocked websites open too, but that is still being tested — no
  site is named as working until it passes the two-day test plan.
- **A ping verdict:** "Ping 35 ms · Good" (Good ≤ 120 ms,
  OK ≤ 400 ms, Slow above) — a bare number means nothing to people.
- **"Created by ChitaCodes"** rides the About version line.

### Changed (the round-2 audit fixes — one UI across devices, D92)

- **Real Lucide icons everywhere** (the power mark, the "…" menu
  button, the pill's chevrons, the usage arrows, the sheet's close)
  through one file, `src/lib/icons.ts`; the drawn four-dot glyph is
  gone.
- **The menu button sits top-right at 44 px**; the menu sheet has a
  proper header — title left, neutral 44 px X right — with the body
  scrolling underneath, replacing the red circle that floated half
  outside the sheet.
- **The countdown digits sit in 0.80 em boxes** — wider than the
  font's widest digit, so they can never touch.
- **The connected ring family stops clear above the hint**, which
  itself grew to 12 px at 82 % white — the dots no longer run
  through the text.
- **The live Mb/s row is off by default** (the session total is
  what matters; `SHOW_SPEEDS` in App.svelte brings it back), and
  speeds the engine cannot measure are hidden (-1), never faked
  as "0 Kb/s".
- **The desktop window is 360×640, resizable, floored at 320×560.**

### Added (Android port, M6 — not yet a released build)

- **The per-app allow-list is a setting (schema v2, D79)**:
  `settings.json` gains the `apps` field; v1 files migrate through the
  existing version mechanism (the built-in default list — Chrome +
  YouTube — fills in). Normalize validates package shapes, dedupes,
  caps the list at 32 and restores the built-in defaults when the list
  is empty or all-invalid — an empty allow-list would route EVERY
  app, the opposite of the product scope, so that state can never
  persist. The same normalized list now feeds three doors: the file,
  the settings API the WebView edits, and the new facade binding
  `RoutedApps()` the VPN service reads when it builds the TUN.
  Changes apply on the next connect (the list is baked into the TUN at
  establish; the UI says so).
- **The mobile settings/UX surfaces (D80)**: the Android menu sheet
  gains the Apps-to-unblock row (friendly labels for the known two,
  package names for anything else, add/remove with the engine's saved
  response as the authority), the honest VPN-slot/key-icon About
  lines (still not a VPN: no IP hiding, no added encryption), the
  privacy disclosure (doc 07 §5: no analytics, no Younblock-run
  server, DNS to the user-configured DoH providers, local logs), and
  a diagnostics view over the existing redacted report. All of it is
  gated on the host's reported platform/capabilities — the desktop
  sheet renders its exact prior copy.
- The M6 debug APK (35,194,757 B, SHA-256 5669efd1…02dc) with the
  settings-driven allow-list and the new sheet is recorded in
  docs/android-port/05–06; on-device observation gates (live
  rotation/font-scale/TalkBack, the apps row on a real screen, plus
  the standing M4+M5 checklist) remain blocked by the sandbox (no
  /dev/kvm, no device) and are carried, never claimed.

### Added (Android port, M5 — not yet a released build)

- **The connection lifecycle is host-verified**: the engine's connect
  run now waits at an optional platform start gate (D77) before its
  listener — on Android, the VPN consent plus the established TUN —
  so an unpermitted engine holds at "connecting" (outside the start
  budget) and can never report "on" while nothing routes. The Kotlin
  consent flow completes the M4 wiring: first-run explanation →
  system dialog → grant starts the foreground service and opens the
  gate; denial stops the held engine and shows honest copy. Consent
  and service starts are single-flight; MainActivity re-derives the
  state on resume; a dying service stops the engine (no lying
  "Connected", no zombie connect); onRevoke is a full disconnect with
  re-consent on the next connect.
- **The honest routing report**: `VpnState()` (D78) answers the
  product's "Connected" question in one call — routing requires the
  engine serving AND a TUN stack bound AND that stack's door being
  the live server's door (a stack left on a dead door is reported
  stale for reconciliation, never as routing). The VPN notification's
  text is derived from this same report: active only when actually
  routing, degraded when degraded, connecting otherwise.
- The M5 debug APK (34,711,941 B, SHA-256 c46026a7…bf26) with the
  lifecycle shell is recorded in docs/android-port/05–06; on-device
  observation gates remain blocked by the sandbox (no /dev/kvm, no
  device) and are carried, never claimed.

### Fixed (Android port, M5)

- The facade's SOCKS5-door refusal paths closed the TUN fd leak they
  documented but did not perform (D76's single-owner rule now holds
  on every StartTun return).
- The mobile test harness now writes the product's default port
  behavior (AutoPort=true) instead of the accidental fixed-port
  setting that only held up in single-package runs.

### Added (Android port, M4 — not yet a released build)

- **The routing path exists and is host-verified**: a gVisor-netstack
  userspace stack (`internal/tunstack`) turns the VpnService TUN's IP
  packets back into TCP flows and hands each one to the engine's
  SOCKS5 front door — the same tunnel path (DoH resolve, protect()ed
  dial, first-record fragmentation for routed domains) the browser
  door uses. DNS from routed apps is answered by a virtual resolver
  (fake-pool A records, empty AAAA, other UDP dropped), so apps stay
  IPv4 and domains cross the door as domains. `YounblockVpnService`
  (foreground, per-app allow-list defaulting to Chrome + YouTube,
  fail-closed when no target is installed) plus the facade's
  StartTun/StopTun lifecycle are compiled into a verified debug APK;
  the engine's TUN contract and protocol policy are proven by
  host tests against the real proxy server and engine, including the
  protect() call-counting proof extended through the door. The
  on-device gates (real DoH through the stack, QUIC fallback,
  protect() on real sockets) remain honestly ⛔ — no emulator/device
  in this environment (docs/android-port 05 M4).
- Decisions D75 (gVisor-direct stack selection) and D76 (the TUN fd
  contract: non-blocking before handover, single-owner) in
  DECISIONS.md; gVisor vendored (+6 MB) per the vendor-means-vendor
  rule.

## [v0.7.6] — 2026-10-16 (app 1.6.6)

A feel pass on the redesigned connect screen: the state change is now a
choreography instead of a set of hard cuts, the drag answers "what happens
if I let go" before you let go, and the About sheet can no longer lie about
its version number.

### Fixed

- **The offline packs could not build the Windows apps.** Since the
  pack was born (v0.7.4), its `vendor\` folder has been missing the
  WebView2 bootstrapper (`MicrosoftEdgeWebview2Setup.exe`, 1.8 MB)
  that the vendored wails embeds into every Windows build — so on a
  fresh machine `wails dev` and `wails build` died with
  `no matching files found` before a single line of the app compiled,
  defeating the pack's whole "no internet" promise. The bootstrapper
  is vendored in the repo now (a `.gitignore` exception un-hides it
  from the `*.exe` rule), the pack builder refuses to zip without it,
  and any fresh clone can build for Windows out of the box.
- **Fresh clones couldn't build the native Linux packages.**
  `packaging/native/fetch-stack.sh` — which `build.sh` calls directly —
  was committed without its executable bit, so on a fresh clone the
  build silently skipped the native-window apps and shipped only the
  web-edition packages (the release machine had a local `chmod`, which
  is why the v0.7.5 artifacts came out fine). The bit is in the repo
  now; `build-offline-pack.sh` gets its bit back too. Mode-only
  change, no script text touched.
- **The About sheet showed a stale version** — "1.6.3" in a 1.6.5 app
  (the same bug the v0.7.3 pass fixed when it was "1.5.0"). The number is
  no longer typed into the frontend at all: the sheet asks the backend for
  it (the `GetAppInfo` binding on Windows, `/api/appinfo` on the Linux
  web edition), and in a plain browser preview — which has no host — the
  version line is simply hidden rather than inventing a number.

### Changed

- **The connected/disconnected flip is one motion now.** The dotted rings
  used to pop (one red ring vanished, five green arcs appeared in the same
  frame); both sets now cross-fade over 250 ms while the family drifts to
  its new center, and the five arcs pulse as an outward ripple instead of
  one flat group pulse. The big status word ("Not Connected" →
  "Connecting…" → "Connected") rises in softly instead of snapping.
- **Dragging the pill past halfway now previews the outcome.** The LED on
  the thumb lights green while a connect is armed (or dims back to red
  while a disconnect is), and the chevron in the direction of travel
  brightens and pauses its idle nudge — release-to-commit is visible
  before you commit. The thumb also presses down (scale 0.98) under a
  finger or mouse, and the chevrons breathe gently toward the swipe
  direction while idle.
- The error detail line ("Could not connect. Try again.") is now the
  approved-for-text red (`--red-400`) instead of secondary grey — red is
  for real errors, per the design language.

### Added

- The menu sheet now behaves like a proper modal: focus moves in when it
  opens (already true), **Tab and Shift+Tab wrap inside it** (previously
  Tab leaked to the dimmed screen behind), and focus returns to the menu
  button on close (already true).

All new motion stops under `prefers-reduced-motion`, verified: every
animation collapses to 0.01 ms and the app remains fully usable.

## [v0.7.5] — 2026-10-16 (app 1.6.5)

The "offline means offline" release: the offline pack from v0.7.4
promised "no internet needed" but died on machines without access to
Go's module proxy — this one keeps the promise.

### Fixed

- **The offline pack now actually works offline.** `wails dev` and
  `wails build` always run `go mod tidy` before compiling, and tidy
  ignores the packed `vendor\` folder — it downloads the dependency
  list from the internet first. On a filtered connection that meant
  `403 Forbidden` from proxy.golang.org and the app never started.
  Every wails call in the setup and build scripts now passes `-m`
  (wails' "skip mod tidy" flag): the dependencies ship inside the
  pack, so there is nothing to download, ever.
- **`'powershell' is not recognized`** in the offline setup: the
  optional "remember the tools for future windows" step called bare
  `powershell`, which fails on systems whose PATH lacks the
  WindowsPowerShell folder. It now calls PowerShell by its full
  Windows path, silently, and can never stop the setup.
- The offline setup's "The app window - 360 x 760" message was stale:
  it now says 360 x 628, the window the redesigned screen actually
  opens since v0.7.3.

### Changed

- `build.bat`, `build.sh` and `setup.ps1` build with `-m` too — the
  tidy step added a network round-trip and zero value (go.mod and
  go.sum are complete and vendored), so builds are faster and never
  surprise-download on filtered networks.

## [v0.7.4] — 2026-10-16 (app 1.6.4)

The two-editions release: every platform now ships **Younblock** (the
redesigned connect screen) and **Younblock Classic** (the simple v0.6.0
UI you knew) side by side — and on Linux the AppImage and .deb finally
**open as their own desktop window** instead of a browser tab.

### Added

- **Younblock Classic editions** (D66): `younblock-classic.exe` (64/32-bit
  Windows) and `younblock-classic_<ver>_amd64.deb` /
  `Younblock-Classic-<ver>-x86_64.AppImage` (Linux). Same engine, same
  settings, ~8.5 MB downloads — the small simple version is back. The
  browser (single-executable) editions carry both screens: add
  `?ui=classic` to the URL to switch.
- **Native window editions for Linux** (D67): the 64-bit .deb and AppImage
  are now real GTK + webkit2gtk desktop apps with their own taskbar icon
  and window — no browser tab. The .deb pulls webkit from your Mint
  repos (apt keeps it updated, ~10 MB download); the AppImage bundles
  the whole webkit runtime so it runs on any glibc ≥ 2.35 desktop even
  with nothing installed. Built against Ubuntu 22.04 webkit2gtk-4.1, so
  every Mint since 21 is covered.
- The web edition's `?ui=classic|pro` switch (both screens are in the
  bundle; the URL picks).

### Changed

- The window's Linux close button now quits the app (stopping the engine
  and restoring the proxy first) — there is no tray to hide into there;
  on Windows the close-to-tray behavior is unchanged.
- Opening the logs folder uses the desktop's file manager on Linux
  (xdg-open) instead of trying Windows Explorer.
- build.bat now produces the classic exes too; build.sh builds the full
  two-edition matrix for both platforms.

### Fixed

- The AppImage on Linux Mint no longer opens in a web browser — it opens
  as an application window (the single-executable 32-bit line keeps the
  browser mode for old 32-bit Mint, where the native stack cannot run).

### Compatibility notes

- The 32-bit Linux editions (.deb i386, i686 AppImage, younblock-linux-386)
  remain browser editions: old 32-bit Mint (glibc 2.31) cannot run the
  native stack. 64-bit Mint 21/22/23 users get native windows.
- The Flatpak stays at 1.6.3 this release (the sandbox lost its flatpak
  stack again; the app code it wraps did not change). It keeps the
  browser mode for now; a native flatpak is on the list.

## [v0.7.3] — 2026-10-16 (app 1.6.3)

The proportion pass: the window is shorter and better balanced — the
same connect screen, the same engine, ~17% less height. Nothing about
how it works changed.

### Changed

- **The window is now 360×628 inside (≈388×650 with the title bar),
  down from 360×760 (≈388×782).** The screen keeps its dark theme,
  red START button, YouTube branding, dotted background and rounded
  shapes — every font size is unchanged; only the spacing and the two
  big shapes were retuned.
- **A deliberate vertical rhythm instead of stacked islands** (D65):
  tighter top padding above the header (50→38 px), the YouTube play
  illustration 15% smaller, the status group pulled closer to it, the
  power-button assembly 12% shorter (track 228→200 px, thumb 124→110
  px — the 82 px width and the touch target are untouched), and the
  dotted ring rescaled to hug it (124→106 px radius; the five
  connected arcs −17%).
- **The bottom hint ("Click or swipe down to connect") moved up** to a
  consistent 26 px below the button instead of floating 72 px lower,
  pushed toward the window edge. Horizontal padding is now a uniform
  20 px.
- `PowerPill` now reads the swipe travel from `tokens.css` (one
  source of truth) instead of hard-coding it beside the token.

### Fixed

- **Opening the menu could shove the whole screen upward for a
  moment.** The sheet lives inside the scaled stage, and focusing its
  close button while the sheet was still sliding up made the browser
  scroll the stage to chase the button. The focus call now explicitly
  forbids that scroll. (Visible at the new window height; barely
  visible before.)
- The About sheet showed "Younblock 1.5.0" since the new screen
  landed — the version string was hard-coded and never bumped with
  the app. It now says 1.6.3.

## [v0.7.2] — 2026-10-15 (app 1.6.2)

The "filtered network" release: the one-click setup survives blocked
download sites, and there is now a fully offline pack (D63, D64). The
app itself is unchanged from 1.6.0 — same connect screen, same engine.

### Added

- **Mirrors for every dependency.** The one-click scripts try several
  mirrors per download and use the first one that answers: three Go
  module proxies (proxy.golang.org, goproxy.io, goproxy.cn — the chain
  is remembered so `build.bat` benefits too), two Go toolchain mirrors
  (Aliyun, go.dev) when winget cannot deliver Go, two Node.js mirrors
  (nodejs.org, npmmirror) when winget cannot deliver Node, and two npm
  registries (npmjs, npmmirror — the mirror is the automatic retry).
  winget is still tried first for everything it can install.
- **The code has three ways to arrive**: a normal git clone (GitHub's
  browser login); the same clone with a GitHub token pasted into the
  `$GH_TOKEN` line at the top of `setup.ps1` (no prompts at all, and
  the token never lands in `.git/config`); or, when git itself cannot
  be installed, a direct zip download from the GitHub API — the script
  offers the token prompt at that point.
- **`Younblock-offline` — the no-internet pack.** One zip carrying the
  app, its Go dependencies, the window building blocks, the Go
  toolchain, Node.js and the Wails CLI. Extract it, double-click
  `setup-offline.bat`, done — nothing is downloaded, ever. It also
  builds the exes (`setup-offline build`).
- The Go dependencies are now **vendored in the repository**
  (`vendor/`), so building and testing never touch a module proxy.

### Changed

- The setup became a pair: the `.bat` files you double-click, and one
  `setup.ps1` holding the whole brain — they travel together in the
  new `younblock-setup` zip on the Releases page (D62 refined).
- The release ships 12 files: the usual ten builds, the
  `younblock-setup` zip and the offline pack.

## [v0.7.1] — 2026-10-15 (app 1.6.1)

The "one-click" release: two scripts that set up a Windows PC by
themselves (D62). The app itself is unchanged from 1.6.0 — same connect
screen, same engine; only its version number moved.

### Added

- **`setup-dev.bat`** — double-click it on any Windows 10 1709+ / 11 PC
  and it installs whatever is missing (Git, Go, Node.js LTS, the
  WebView2 part of Windows, the Wails CLI), fetches the code, and
  starts the app straight from source (`wails dev`) — the 360×760
  window opens by itself. Safe to run again any time: it skips what
  is already installed and updates the rest.
- **`setup-build.bat`** — the same one-click checks, then it builds
  every `.exe` (the app and the headless engine, 64- and 32-bit),
  prints each file's SHA-256 and opens the `dist` folder with the
  finished files.
- Both scripts also fix the usual first-run problems by themselves:
  the search path is reloaded right after an install (no "open a new
  window and try again" step), a too-old Node.js or Go is refused
  with a plain fix-it note, and the private-repo login is explained
  when the code is first downloaded.
- `.gitattributes` now forces Windows (CRLF) line endings on `.bat`
  files, so both scripts always arrive exactly double-clickable.

## [v0.7.0] — 2026-10-15 (app 1.6.0)

The "new face" release: the main screen is replaced by the new connect
screen — one button, one glance (D60, D61).

### Changed

- **The whole app is now the connect screen** (D60): a single power
  pill (START / CANCEL / STOP / WAIT) on a 360×760 stage — a red ring
  while off, five green arcs while on — with the connection timer, live
  download/upload speeds and the ping beside it. The pill can be
  clicked, dragged past halfway to connect or disconnect, or driven
  with ArrowDown / ArrowUp / Enter; pressing it while "Connecting…"
  cancels. The window grew from 360×560 to give the new screen its
  stage (the D48 client-size fix travels with it).
- **The menu is a small sheet now**: About, How it works and Help live
  behind the menu button; Escape closes it. The separate Settings,
  Help, About and First-run screens — and the i18n layer only they
  used — are gone (recoverable at tag v0.6.0). The engine still reads
  the saved settings file, and the Linux JSON API still exposes them;
  only their editing screen is gone.
- **The tray and connect-on-launch now route through the new state
  bridge**, so the tray icon, the window and the screen can never
  disagree about whether the app is on.
- **The Linux editions keep their real engine** (D61): the new screen's
  one backend seam (`backend.ts`) gained back the localhost JSON +
  Server-Sent-Events transport, so `younblock-linux-*`, the .debs, the
  AppImages and the Flatpak drive the same engine as before, with the
  same state mapping as the Wails bridge (degraded counts as connected;
  PORT_BUSY and friends surface as "Can't Connect" with a retry). The
  ping is re-read once a second while connected — the same cadence as
  the Windows side.

### Added

- The Sora variable font ships with the app (the new screen's
  typeface); no flags, no "Go Premium", no location chips anywhere.

## [v0.6.0] — 2026-10-15 (app 1.5.0)

The "polish" release: a dedicated UI/UX and look-and-feel pass over the
whole 360×560 window — every screen and all six states — with no engine
or network changes (D56–D59).

### Changed (the UI/UX and look & feel pass — D56–D59)

- **One screen chrome**: Settings, Help, About and FirstRun mount on a
  shared stylesheet (header, back button, scroll body, section captions,
  cards, reveal) instead of three drifted copies; a real type ramp
  (11 px overline captions, 17 px view titles, mono for addresses and
  URLs) and a duration scale so every motion in the app shares one clock.
- **One control standard**: the Advanced radios and the FirstRun
  checkbox are drawn in the brand style (a ring filling with the
  padlock red) instead of native accent-colored dots beside the custom
  switch; every touch target is ≥44 px (the Add-provider chips and the
  copy-address button were under); the port field gets hover/focus
  states; "tried first" is a small gold badge; secure-DNS URLs and the
  domain list use a mono face.
- **Motion, everywhere, still honest**: screens enter with one
  rise-and-fade, the welcome staggers in, the menu unfolds from its
  corner, the close-notice sheet rises, Advanced / licenses / the Help
  accordion expand with the same smooth reveal, Run-test carries a
  spinner and the running test shows a quiet "Checking…" row. Purely
  decorative motion switches off under reduced-motion settings;
  functional spinners keep turning (D45).
- **The stopping state says what it is doing** ("Putting the internet
  setting back…"), the state subtitles come from the copy deck like
  every other word, and the degraded state's button shrinks the same
  step the error state always had — its "Fix connection" stack now
  keeps comfortable room above the footer instead of landing exactly on
  the minimum.

### Fixed

- **The open menu could be unclickable** right after a screen appeared:
  a filled entrance animation left a lingering (identity) transform on
  the header, which silently repainted the menu panel under the page
  content. One-shot animations now end without keeping transforms.
- **Keyboard focus reshaped pill buttons**: the global focus ring
  carried a border-radius that squared the Fix-connection pill and the
  chips for as long as the ring showed; the ring now only paints.
- **Hovering the On button grayed its rim**: the hover rule outranked
  the lit state's border. Hover now applies to the resting states.
- **The About diagram was below the fold** — under the numbered steps
  at exactly the window's bottom edge, where nobody met it. The picture
  (browser → Younblock → YouTube) now leads the card.

## [v0.5.1] — 2026-10-14 (app 1.4.1)

The "install it properly" release: the three standard Linux Mint package
formats, wrapping the same static binary as the plain-file edition (D53).

### Added

- **.deb packages** (amd64 + i386): `/usr/bin/younblock`, menu entry, nine
  hicolor icon sizes, `Depends: libglib2.0-bin` + `Recommends: libnotify-bin,
  xdg-utils`, postinst/postrm cache refresh, md5sums.
- **AppImages** (x86_64 + i686): single-file, portable, download-and-launch;
  AppRun forwards to the binary with `--web`; built FUSE-free.
- **Flatpak bundle** (x86_64): manifest + metainfo for
  `io.github.assadigit.Younblock` on `org.gnome.Platform/48`.
- Native icon set regenerated from the logo at 4096 px and exported to all
  sizes 16–512 under both icon names.
- `packaging/` tree with one build script per format; `build.sh` now cuts
  all six binaries and all four packages in a single run.
- Autostart resolution per format (D54): `$APPIMAGE` names the AppImage
  file, `$FLATPAK_ID` runs `flatpak run <id>`, passwd home under Flatpak;
  four new unit tests.

### Fixed

- Autostart entries written from inside an AppImage or Flatpak now survive
  the package's own launch mechanism (previously they pointed at the
  extracted temporary path).

### Known limits (honest list)

- `flatpak run` could not be executed in the build container (nested pidns
  restriction); the bundle's contents, install, live API and autostart entry
  were verified, and launching on real Mint is README manual-check step 8.
- Package wrapper files embed build timestamps; only the inner binaries are
  byte-reproducible.

## [v0.5.0] — 2026-10-14 (app 1.4.0)

The "same app for everyone" release: 32-bit Windows and the Linux Mint
edition, all built from the one source tree (D52).

### Added

- **32-bit Windows editions**: `younblock-x86.exe` and
  `younblock-core-x86.exe` (PE32 i386).
- **Linux edition**: `younblock-linux-amd64` / `-386`, static ELF with the
  UI embedded (`go:embed`), loopback-only JSON API + SSE events, `--web`
  mode on port 17500 with free-port fallback; works on Windows too.
- Linux platform adapter: gsettings system proxy with backup/restore,
  EnsureAttached anti-clobber re-assert, autostart `.desktop`, notify-send
  notices, capability probing (D50, D53 groundwork).
- The frontend auto-detects its edition (Wails → HTTP → mock) and adapts
  copy ("Linux" word swap, no-tray first-run text, capability-gated Help
  rows for Chrome-session features).

### Changed

- **StatusBlock error state redesign (D49)**: the failure reason is now the
  subtitle directly under the state label, where the eye lands first; the
  old duplicate low paragraph and its dead gap are gone.

### Fixed

- 32-bit build correctness: `parseDword` 1<<31 signed-int overflow replaced
  with unsigned arithmetic.

## [v0.4.1] — 2026-10-14 (app 1.3.1)

Fixed the "white space" and "Fix connection is on the edge" reports.

### Fixed

- **Window sizing under DPI scaling (D48)**: the outer window is now grown
  at startup so the client area is a true 360x560 logical pixels at any DPI
  (previously ~336x522 at 125%).
- **Overflowing warning states (D48)**: the footer is pinned with real
  margins, the fix-connection button always sits ≥15 px above it, the timer
  block collapses smoothly (0fr↔1fr grid) in states that never show it, and
  long error text gets its own 100 px scroll cap. Header slimmed 52→44 px;
  response-ladder (button scale-down, then zoom) for very small viewports.

## [v0.4.0] — 2026-10-14 (app 1.3.0)

The "every browser really uses it" release, plus the logo palette.

### Changed

- **System proxy rewritten to manual values (D46)**: ProxyEnable /
  ProxyServer / ProxyOverride with a v2rayN-style bypass list instead of the
  registry PAC (AutoConfigURL), which real-world browsers and tools quietly
  defeat. Any AutoConfigURL is cleared while active; legacy orphaned PAC
  entries cleaned at startup; WM_SETTINGCHANGE broadcast; watchdog
  EnsureAttached re-assert with a PROXY_REAPPLIED notice.
- **Logo palette everywhere (D47)**: warm charcoal neutrals, padlock-red
  primary/on-state, lightning-gold accents (warnings, focus, echo rings, OK
  checks); danger keeps its distinct coral. No green anywhere; tray icons
  regenerated to match.
- YouTube mode fragments only routed hosts (unfragmented relay otherwise);
  plain-http to non-routed hosts is forwarded as a real forward proxy so
  http-only sites keep working.

## [v0.3.0] — 2026-10-14 (app 1.2.0)

The power-button animation fix and professional UI polish.

### Fixed

- **Animation freezing under reduced motion (D45)**: Windows mirrors
  "system animations off" (and RDP) into WebView2; the blanket kill-rule
  froze the connecting arc mid-circle. Replaced with a per-component policy
  — decoration off, functional progress indicators and color fades stay.

### Changed

- Orchestrated turn-on: spring pop, staggered echo rings, crossfaded lit
  plate, slow glow breath; press-release hardening; refined connecting arcs.
- StatusBlock: state-colored status dot, typographic rhythm, amber ghost
  "Fix connection" pill (reads as a warning repair, not a second on-button).
- Connect footer badge chip + "works with every browser" micro-caption;
  copy updated across First-run / Help / About.

## [v0.2.1] — 2026-10-14 (app 1.1.1)

Hotfix after a "didn't even connect" report on 1.1.0.

### Fixed

- Degraded-from-start now actually starts the stats loop, watchdog and
  network monitor — the timer runs and the app self-recovers (R14).
- The state JSON carries `fail` (step + code, root-cause ordered); the
  degraded subtitle says which part failed; "last failed check" added to
  the diagnostic report.
- A changed secure-DNS list swaps the resolver live (previously kept until
  restart).

### Added

- Editable Advanced DoH list: remove buttons, one-tap add of
  AdGuard/Quad9/dns.sb/defaults, new provider tried first, keep-at-least-one
  guard.

## [v0.2.0] — 2026-10-14 (app 1.1.0)

Ali's five post-test requests; all default-off or additive.

### Added

- Route-all ("All websites") switch surfaced as a top-level setting.
- Dedicated Chrome session option (`openBrowserOnConnect`, default off) —
  neutral browser package, Windows shell launcher with registry +
  standard-path lookup, session ends on off/error/quit.
- Background texture: two soft brand glows + subtle SVG noise grain, inline.
- Response time ("ping"): YouTube round-trip shown under the timer and in
  the diagnostic report; resets on stop.
- Ali's icon.ico installed as the exe/window icon.

## [v0.1.0] — 2026-10-13 (app 1.0.0)

First Windows test build, published to GitHub Releases.

### Added

- Headless engine: fragmenting CONNECT proxy (routed hosts only), secure
  DNS-over-HTTPS resolver with cache, health self-test with self-healing
  watchdog, ring + file logging, JSON settings.
- Windows adapters: system proxy with backup/restore state machine,
  autostart registry entries, toast notices.
- Wails app: 360x560 window, tray with state icons, First-run, Connect /
  Settings / Help / About screens, i18n-ready copy.
- `younblock-core.exe` headless CLI (`--selftest`, `--version`) and the
  diagnostic report generator.

[Unreleased]: https://github.com/assadigit/Younblock/compare/v0.7.0...HEAD
[v0.7.0]: https://github.com/assadigit/Younblock/releases/tag/v0.7.0
[v0.6.0]: https://github.com/assadigit/Younblock/releases/tag/v0.6.0
[v0.5.1]: https://github.com/assadigit/Younblock/releases/tag/v0.5.1
[v0.5.0]: https://github.com/assadigit/Younblock/releases/tag/v0.5.0
[v0.4.1]: https://github.com/assadigit/Younblock/releases/tag/v0.4.1
[v0.4.0]: https://github.com/assadigit/Younblock/releases/tag/v0.4.0
[v0.3.0]: https://github.com/assadigit/Younblock/releases/tag/v0.3.0
[v0.2.1]: https://github.com/assadigit/Younblock/releases/tag/v0.2.1
[v0.2.0]: https://github.com/assadigit/Younblock/releases/tag/v0.2.0
[v0.1.0]: https://github.com/assadigit/Younblock/releases/tag/v0.1.0

### Added (Android port, M7 — not yet a released build)

- **The NO_TARGET_APPS typed error (D81)**: when none of the
  allow-list's apps is installed, the connect attempt now fails
  honestly instead of silently returning to "Not Connected" — the
  engine lands in its error phase with the typed code and the page
  shows the reason ("None of the apps on your list is installed.
  Install one, or edit the list in the menu."). The refusal travels
  the existing start gate with atomic delivery (the shell's re-arm
  can never claw it back) and a whitelisted code vocabulary.
- **Run generations (D82)** — a hardening fix found by the new tests:
  a connect goroutine that lost the race with its own Stop could
  previously register a fresh uncancellable context and resurrect a
  phase after the cancellation. Every cancel now supersedes spawned
  runs; one existing test had been passing through exactly this bug
  and was corrected to its documented intent.
- **The -race lane**: `go test -race` runs clean on the seven key
  packages (engine, android adapter, mobile, tunstack, proxy, webui,
  config).
- **Audit + attribution**: the AAR's dependency closure verified
  (gVisor netstack Apache-2.0, gomobile BSD-3, golang.org/x BSD-3,
  google/btree Apache-2.0 — all permissive, licenses vendored); the
  About sheet credits the open-source components. The built APK's
  permission set verified to be exactly the documented inventory.
- The v1 release-notes draft (docs/android-port/10-release-notes.md)
  with the truthful-claims review and the open device gates listed.
- The M7 debug APK (35,298,149 B, SHA-256 f27b7eef…491) is recorded in
  docs/android-port/05–06; battery/behavior, reboot/revocation and all
  live-observation gates remain ⛔ (no /dev/kvm, no device) and are
  carried, never claimed.

### Changed (Android port, M8-a — Ali's two product rulings, D83 — not yet a released build)

- **The apps allow-list has NO default (Ali's ruling)**: the list
  starts empty and nothing is routed until the user adds an app. The
  M6 "empty restores the built-in Chrome+YouTube default" rule is
  gone — the M7 NO_TARGET_APPS refusal door is what makes the empty
  list safe, and it now also covers the empty state (the connect
  fails with the typed reason and an honest "add an app" message,
  instead of the old silent stop that treated an empty list as a
  bridge bug). The UI drops the last-entry guard and shows an
  empty-state line. Existing saved lists are kept — the ruling
  changes the default, never user data.
- **Websites ending in .ir never touch the app (Ali's ruling)**: in
  every mode and both editions, .ir hosts are never fragmented, never
  redirected — resolved and dialed direct, byte-transparent. The
  desktop PAC answers DIRECT for them first (youtube mode already
  behaved this way by fallthrough; **"All websites" mode is a real
  behavior change** — it previously sent .ir through the fragmenting
  path). The Android SOCKS5 door makes the same decision, and the
  rule wins even over a .ir entry on the user's own domain list.
  Honest limit, stated in-app: on Android, .ir name lookups still
  resolve through the configured secure-DNS providers (the app has no
  other resolver there).
- **The manifest's `<queries>` block broadened to the launcher-intent
  form** — a latent bug the ruling exposed: the old block listed only
  the two former default packages, so any user-added package would
  have been invisible to PackageManager (Android 11+ package
  filtering) and refused as "not installed" even when installed. Every
  launchable app is now visible; non-launchable packages cannot be
  verified and are refused honestly.
- The M8-a debug APK (35,567,561 B, SHA-256 ccf46a2e…7b64) with both
  rulings applied is recorded in docs/android-port/05–06; the standing
  device gates remain ⛔ (no /dev/kvm, no device) and are carried,
  never claimed.

### Added (Android port, M8 — package, verify, hand off, D84 — the debug test circle)

- **The first distributable Android build** — the debug-APK-only test
  circle (Ali's keystore ruling, D84: "keep it debug version… we're
  not releasing yet. but release in github for me to download and
  debug"): no signed variant exists at this stage; the debug APK is
  published as the GitHub prerelease **v1.6.6-android-debug.1** on the
  private repo, downloadable with Ali's own GitHub account for device
  testing. The artifact is the M8-a clean-build output — M8 changed no
  application code — re-verified on disk before distribution:
  `app-debug.apk`, 35,567,561 B, SHA-256 ccf46a2e…7b64; apksigner
  v2-scheme OK (debug certificate, digest recorded); badging
  io.github.assadigit.younblock 1.6.6 (versionCode 1), minSdk 26 /
  target 36, native-code arm64-v8a/armeabi-v7a/x86_64; the manifest's
  four-permission inventory and launcher-intent `<queries>` verified
  in the shipped file.
- **The release carries the device test circle** (doc 10): the
  checklist of everything the build environment could never verify
  (no /dev/kvm, no device — install/first-launch, the live consent
  dialog, notification behavior, screen-off/Doze, real DoH, the
  YouTube app's QUIC→TCP fallback, protect() on real sockets, the
  .ir direct path on a real radio, stop/reboot paths, battery) —
  handed to Ali's device through the release notes, never claimed
  here. His observations are the feedback loop that closes those
  gates and drives any v1.6.6-android-debug.2 fix build.
- M8's verification record: the baseline gate (vet clean, 17/17
  packages twice, svelte-check 0/0, vite clean, webui dist synced),
  the dependency-closure re-run (the same audited permissive set),
  the platform-requirements recheck against developer.android.com
  primary sources, three of doc 07 §9's release-blocking rows closed,
  and the README's Android sideload section (the unknown-apps + Play
  Protect + built-for-testing warnings, all expected for this circle).
  **Every future debug build bumps versionCode** so it installs over
  the previous one (same debug certificate) — the D84 rule.

### Added (Android port — the device-test feedback loop, session 1, D85)

- **Ali's first device observation ingested**: "Android didn't let me
  install it" from v1.6.6-android-debug.1. Diagnosed before any fix:
  the served release bytes re-downloaded and verified byte-identical
  to the record (sha256, apksigner v2, badging, manifest — no
  `testOnly`, `debuggable` is normal for debug; zip intact), and the
  asset's download_count was 0 — pointing at an interrupted download
  from the Iranian network (release assets are served from
  release-assets.githubusercontent.com) rather than an APK defect.
- **The install-blocked troubleshooting path is now on the live
  release body** (PATCHed via the API) and doc 10 §1: check the file
  size first (≈ 35,567,561 B; a few-KB file is an error page), retry
  on Wi-Fi or download on a PC + USB; then the install-unknown-apps
  gate; then Play Protect's "install anyway"; then the parse-error and
  storage branches.
- **The debug certificate is pinned in the repo (D85)**:
  `android/app/debug.keystore` — the exact key that signed
  v1.6.6-android-debug.1 — is vendored and wired as the debug
  signingConfig, so successive debug builds keep installing over the
  previous one even after sandbox wipes regenerate the machine-local
  ~/.android/debug.keystore. Verified: clean assembleDebug signs with
  the same certificate digest (c76bb7c6…8551d) and is byte-identical
  to the published release. No debug.2 yet — the artifact itself is
  unchanged; the next release rides on the next code-changing
  observation.

### Changed (Android port — the device-test feedback loop, session 2, D86 — v1.6.6-android-debug.2)

- **The YouTube app is the default allow-list entry (D86, Ali's
  ruling)**: a fresh install routes the YouTube app out of the box —
  debug.1's born-empty list (D83) is superseded. A saved empty list
  still means empty: removing the last entry is a user choice that
  persists, and the connect refuses with the honest NO_TARGET_APPS
  reason instead of restoring anything.
- **One-time settings migration (schema v3)**: a v1/v2 settings file
  whose apps list is absent or empty migrates to the YouTube default —
  that empty was the shipped default of the debug.1 era, never a user
  choice (its entry path refused friendly names, so "empty" almost
  always meant "never successfully configured" — the exact state
  behind Ali's "the first version didnt work, whether in youtube app
  or youtube in chrome"). Custom lists survive verbatim; a v3 empty
  list is the user's own and stays. Updating installs over debug.1
  (versionCode 2, same pinned debug certificate) with settings intact.
- **Adding apps is now usable**: the Apps-to-unblock row gains one-tap
  quick-add chips for the known apps (+ YouTube, + Chrome) and the
  type-in field accepts friendly names — "youtube" and "chrome"
  resolve to their packages; other apps still need their Android
  package name. The engine's saved response remains the validation
  authority.
- The honest copy updated wherever it claimed the list "starts empty"
  (menu sheet, How-it-works, the refusal message, README, release
  notes), and the release notes' stale default DoH provider list
  corrected to the code truth (RethinkDNS, Cloudflare, Google).
- Root-cause record: the engine's routing chain (virtual DNS, fake-IP
  reversal, the SOCKS5 door, per-app TUN establish) was audited
  end-to-end and found sound — the failure debug.1 showed on-device was
  the empty list plus the package-name-only entry path, not the engine.

### Changed (Android port — the device-test feedback loop, session 3, D87 — v1.6.6-android-debug.3)

- **The default allow-list is five apps (D87, Ali's ruling — "Add
  chrome to the default addlist as well, also firefox, opera, brave
  browser")**: the YouTube app plus the Chrome, Firefox, Opera and
  Brave browsers (the stable mainline package names:
  com.android.chrome, org.mozilla.firefox, com.opera.browser,
  com.brave.browser). A fresh install routes every installed one of
  them out of the box. Entries whose apps are not installed are
  skipped at TUN establish — a missing browser can never break the
  connect (audited and recorded before shipping; the establish loop
  has caught NameNotFoundException per entry since M6). A saved empty
  list still means empty, as before.
- **One-time settings migration (schema v4)**: a v1/v2 born-empty
  file fills the five-app default directly; a v3 file still holding
  exactly the debug.2 default [com.google.android.youtube] — the
  shipped state of that build, never curated — upgrades to the five
  apps; a v3 empty list (user agency) and every custom list survive
  verbatim (including a v2 file whose [youtube] entry was hand-typed
  under debug.1's refusing entry path). Updating installs over
  debug.1/debug.2 (versionCode 3, same pinned debug certificate) with
  settings intact.
- **The quick-add chips cover all five defaults** (+ YouTube /
  + Chrome / + Firefox / + Opera / + Brave — rendered when not
  already listed), and the friendly-name aliases extend to
  "firefox", "opera", "brave" (and "brave browser"); other apps
  still need their Android package name, and the engine's saved
  response remains the validation authority.
- The honest copy updated everywhere the default list is named
  (menu intro, empty state, note, How-it-works, the NO_TARGET_APPS
  message, README, doc 07 §1, doc 10 / the release notes) — and the
  release guide's expected file size updated to this build's exact
  artifact size.

### Added (Android port — the release-signing helper, D88; no app change, v1.6.6-android-debug.3's APK unchanged)

- **`make-release-keystore.bat` — the local release-keystore ceremony
  in one double-click** (Ali's ask; the security boundary kept: the
  password is generated on his machine with Windows' own crypto RNG
  and never transits chat, agent or repo): locates keytool, generates
  a 32-character zero-bias password, creates the keystore with every
  keytool prompt pre-filled (RSA 4096, PKCS12, 30-year validity — the
  "serial number and etc" interactive ceremony eliminated), verifies
  it by reading it back, and writes a
  `younblock-keystore-backup_<date>.md` recording the password, the
  alias, the certificate serial number, the SHA-256 fingerprint and
  the full keytool record. Refuses to overwrite an existing keystore;
  warns when run inside a git repo; fully offline; the .gitignore
  guards the generated files. The keytool flow was verified against
  the real JDK 21 keytool on the build host; the Windows batch
  execution itself stays ⛔ there (no Windows) — the script
  self-verifies on its first run. Attached to the
  v1.6.6-android-debug.3 release as a second asset and committed at
  the repo root; the README's Android section now documents the
  signed-lane steps it automates.

### Changed (Android port — the release-flag ruling, D89; no app change, the APK bytes untouched)

- The three android debug releases (v1.6.6-android-debug.1/.2/.3)
  are now regular GitHub releases, not prereleases (Ali's ruling: "Do
  not prerelease, just release, the repo is private" — the flag
  carried no public meaning in a private repo, and the Latest pointer
  now lands on the current test-circle build, debug.3). Switched and
  verified via the API; the debug.3 release notes gained one
  clarifying paragraph so the flag cannot be misread as a stability
  claim — it is still the debug-signed device-test build,
  collaborator-visible only; nothing about the artifacts changed.

### Fixed (Android port — the keystore helper survives PowerShell-less PATH machines, D90; no app change, the APK bytes untouched)

- `make-release-keystore.bat` no longer depends on the PATH variable
  finding PowerShell (Ali's first field report: "'powershell' is not
  recognized" — the ceremony stopped at password generation, exit
  code 4). PowerShell is now located by its full standard install
  path first — the same fix the desktop setup scripts took at v0.7.5
  — then through where.exe; and if PowerShell cannot be found at
  all, a pure-cmd fallback generator builds the password (six RANDOM
  draws, digits and dashes, ~90 bits) so the ceremony always
  completes. The .md backup and the final summary record which
  generator produced the password. The fixed script ships on a new
  helper-maintenance release, v1.6.6-android-debug.4 (a regular
  release, D89) — it carries only the .bat; the app is unchanged
  and debug.3's APK remains the current build (D84: identical bytes
  are never re-released), with a pointer note added to debug.3's
  page superseding its older helper asset.
- Also recorded: the answer to "can't I use an online keyring
  generator instead?" — no. An online generator would hand the app's
  signing identity and password to a third party; the D88 boundary
  stands (signing secrets are generated where they are used). A
  one-line manual keytool alternative is documented in the release
  notes for machines without PowerShell.

### Fixed (Android port — the first device feedback answered, D91; v1.6.6-android-debug.5, versionCode 4)

- **The connect screen's numbers get their spacing back** (Ali's
  device report: "the numbers in the app are too near each other, they
  need regular spacings"). Root cause: the redesigned screen's stage
  was a fixed 22.5 rem wide — correct for a 360 px viewport at the
  default font scale, but on narrower screens or with Android's
  font-size setting turned up it grew past the screen edge and pushed
  the timer and the two speed values against it (measured: 495 px of
  stage inside a 320 px viewport at 1.4× scale, the two speed values
  sitting 24 px apart with 29 px to the screen edges). The stage now
  never exceeds the viewport, the two speed values carry wider
  spacing that can never wrap or jam, and the timer's digits get a
  touch more air with tabular figures where the typeface offers
  them. Verified against the rendered app at both the worst case
  (320 px / 1.4×) and the designed case (360 px / 1×) before
  shipping.

### Added (Android port — per-flow tunnel timing in the diagnostics report, D91; same release)

- **Every tunnel now records its own phases in the log** so a field
  report can name WHERE time goes instead of guessing: `tunnel up
  dns=…ms dial=…ms` when a flow is established, `tunnel first-byte
  …ms` when the origin's first response byte arrives (the TLS
  handshake and response start, through the fragment path), and
  `tunnel done dur=…ms up=… down=…` with the flow's own byte totals
  when it ends. These lines ride the in-memory log ring into the
  redacted diagnostics report (Menu → Diagnostics → copy), with
  hostnames appearing only when the log-domains privacy setting is
  on — the same rule the failure-path logs already follow. This is
  the instrument for the round's open question: Ali's device shows
  the YouTube feed refreshing fast while video playback takes
  30 s–1 min+ to first show; the host-side suspects were each
  eliminated with evidence (including a new test pinning that
  non-DNS UDP gets an immediate ICMP port-unreachable — the QUIC
  fast-fail contract routed apps rely on), so the remaining suspects
  are device/network-side and the next diagnostics copy from a
  slow-video moment will name the phase.
- Also in this release: the Play Protect popup observed at install
  is documented as expected behavior for sideloaded debug-signed
  APKs (no defect, no change), and the release keystore Ali created
  is explicitly NOT used for the test circle — signing a test build
  with it would break the installs-over-the-previous update chain;
  it waits for the signed public release.
