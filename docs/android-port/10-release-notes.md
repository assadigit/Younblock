# 10 — Release Notes (v1 DRAFT + the shipped debug test circle)

**Status:** updated at M8-af (2026-10-21 — the debug.24 round, the
prove-before-change round). The external engineer reviewed the
debug.23 consultation document and wrote this round's spec: the
cause is still UNPROVEN — the 2.8 KB corpses are probably
TLS-handshake-only replies, but sizes suggest, they don't prove —
so this build measures instead of guessing, and stops the engine
from trusting "handshake succeeded" as proof an address can carry
video. (The debug.23 wire-discipline summary follows the debug.24
section.) The tester's post-debug.22 report was
blunt: "This time, it doesn't even open video after a while. But
the feed refreshes. this is probably one of the worst versions."
And then the most useful thing anyone has sent this whole circle:
two links — GFW-knocker/gfw_resist_HTTPS_proxy and
mahdisml/WebImmortalGuards — with "maybe the instructions of this
would help you, knowing how this system works." They did. Reading
them against our engine showed the bug that has been growing since
debug.13: every recovery mechanism we shipped (racing, probing,
substitution, churn kicks) multiplies the number of googlevideo
hellos the phone emits, and the censor flags noisy sources PER
SERVICE NAME. Early in a session the source is clean (everything
works — "after a while"); once the flag trips, every googlevideo
connection is reset while youtube.com — a different name, and one
we send far less volume to — keeps serving: the feed refreshes,
video never opens, and our own retry machinery keeps re-feeding
the flag forever. The reference implementations never trip it
because they send ONE fragmented hello per connection and NOTHING
in the background, ever. This round makes the engine match that
volume discipline while keeping the proven intelligence (the
substitution pool, flow-fed marks, the box rehabilitation):

- **A connection to an origin we've already seen speak now sends
  ONE hello.** Not four in parallel (the "race") — one, exactly
  like backend.py. The racing fan-out returns only when the origin
  is unknown or the session is young, which is when racing
  actually pays.
- **The background verification is a trickle, not a scanner.** At
  most 10 handshakes every 15 minutes (was up to 48 every 5
  minutes, plus an immediate 48-handshake burst after every
  failed flow, plus a full burst at app start — all deleted).
  A fresh app start now sends ZERO googlevideo packets before
  your first video plays.
- **When googlevideo keeps dying, the engine goes SILENT instead
  of trying harder.** Three completely-failed video connections
  inside 30 seconds → for the next ~45-60 seconds, googlevideo
  requests fail instantly with no packets sent at all, while
  everything else (the feed, search, thumbnails) keeps working
  normally. This is deliberate: in that quiet window the player
  re-fetches the video and rotates to a NEW googlevideo hostname
  — the same recovery your manual refresh always triggered, and
  exactly what the reference implementations rely on the
  browser to do — and any per-source flag decays because we
  stopped feeding it. After the window, the next connection
  just works again.
- **The report now measures our own volume:** a new line
  `gvideo discipline: hellos=N cooldowns=M`. hellos is every
  googlevideo hello we put on the wire (connections, retries,
  background checks). Compare it to the routedFlows number on
  the counters line: ~1 hello per flow is the reference
  implementations' shape (that's the goal); debug.22 ran ~4 per
  flow plus bursts. cooldowns>0 means the silence circuit caught
  a flag state that debug.22 would have ridden into "never opens
  video again."

**v1.6.6-android-debug.23 (versionCode 21+1=22) is the CURRENT
build** — the wire-discipline round. What changed, in plain words:

- **"It doesn't even open video after a while. But the feed
  refreshes."** That split — video dead, feed alive — was the
  clue. Both go through the same proxy, the same fragmented
  hellos, the same DoH. What's different is VOLUME: for every
  video connection the engine opened 4 parallel connections
  (each sending a 299-piece fragmented hello), and on failure it
  retried, re-raced, and fired background bursts of up to 48
  more handshakes to googlevideo servers — while the feed's
  connections to youtube.com are one-at-a-time and quiet. A
  censor that flags noisy sources per service name slowly puts
  your line on the googlevideo list: everything works early
  ("after a while" = the flag accruing), then every googlevideo
  connection is reset on sight — permanently, because our retry
  machinery never stopped feeding it. The two projects you sent
  (gfw_resist_HTTPS_proxy — the family backend.py comes from —
  and WebImmortalGuards) never trip this because they emit ~1
  hello per connection and zero background traffic. That's the
  shape this round adopts.
- **The numbers to read next report:** `gvideo discipline:
  hellos=N cooldowns=M` next to `routedFlows` on the counters
  line. If hellos settles near routedFlows (≈1 per flow), the
  engine is finally as quiet as the Python. If cooldowns>0
  appears during an evening test and video comes BACK after
  ~45-60 s pauses instead of dying for the session, the silence
  circuit is doing its job — note whether it fires.
- **One ask:** watch whether the FIRST video after a fresh app
  start plays (the startup background burst is gone — the pool
  file + your first video's own connections now carry the cold
  start), and if video ever wedges again, note whether it
  recovers by itself within about a minute (the circuit) or
  needs a manual refresh (tell us — that changes the next
  round's tuning).

**v1.6.6-android-debug.24 (versionCode 22+1=23) is the CURRENT
build** — the prove-before-change round. What changed, in plain
words:

- **This build is mostly a measuring instrument.** Six rounds fixed
  many things and the video starvation never changed, because we
  could never see WHERE the data is lost. Now every video
  connection's report line says what the KERNEL knows: whether the
  request we sent was even acknowledged by the server (if not —
  our request packets are being dropped on the way out), or whether
  it was acknowledged but the answer is being dropped or slowed on
  the way back. That one line answers the question every previous
  round had to guess at. A second new line shows the exact TLS
  record timeline of each video connection — so the "2.8 KB
  corpse" question (was it just the handshake? or a small error
  reply?) becomes a measured fact, not an inference.
- **A "Run network test" button** in Diagnostics. One press probes
  the video servers your phone is actually being given, under all
  four fragmentation methods, in the same minute — the
  same-minute comparison we never had (every method comparison so
  far happened on a different build at a different hour of the
  day). It also tries the same probe with a different name and with
  no name at all (does the block care about the video name?), a
  control probe to a thumbnail server (does the test itself work?),
  and counts the packets each method emits. The whole table lands
  in the report. It takes up to a few minutes on a fully dead
  network — usually well under a minute.
- **Experiments you can switch mid-session** (Diagnostics →
  Experiments): the fragmentation method (Legacy / Strong /
  Balanced / Light), the substitution toggle, and the
  emission-breaker toggle. Every report says which position they
  were in. Substitution is now OFF for video servers until an
  address has PROVEN it can carry video (an actual HTTP reply from
  a video request) — your debug.23 report showed 18 substitution
  "wins" that each delivered under 12 KB: the pool was full of
  false positives and this stops it.
- **The silence circuit finally sees the right deaths.** Your
  debug.23 report's video connections all died the "request sent,
  nothing ever came back" way — 13 of them — while the circuit
  that should have gone quiet counted ZERO (it was watching for a
  different death shape). Fixed: it now sees exactly these. If
  `cooldowns>0` shows up in your next report, the quiet windows
  finally ran — note whether video recovers by itself within about
  a minute.
- **The report now names your phone and your operator** (model and
  carrier name only — no identifiers, nothing new is read from the
  phone). The public tuning tables for this ecosystem are per-ISP
  (MCI needs 80-250 fragments, Irancell 10-40) and we never knew
  which one you were riding.

**The debug.24 tester protocol — do this in ONE sitting, with the
phone in the same place:**

1. Open Younblock → Diagnostics. Press **Run network test** and
   wait for it to finish (up to a few minutes on a fully dead
   network — usually under a minute).
2. In the YouTube app, open a video and let it try for 2 minutes.
   Press **I pressed play** before you start and **First frame**
   if it ever plays.
3. Switch the method to **strong** (Diagnostics → Experiments →
   Strong), press **Run network test** again, and play another
   video for 2 minutes.
4. Send the full report (Copy report).
5. Repeat steps 1-4 once in the MORNING (before 9:00 Tehran time)
   and once in the EVENING, and say each time whether you were on
   mobile data or Wi-Fi.
6. If a laptop with `backend.py` is available on the same network,
   play any video through it at the same time and say whether it
   plays fully.

**The debug.24 artifact** (built this round, verified before
packing): `v1.6.6-debug24.apk`, versionCode 23 / versionName
1.6.6 / minSdk 26 / targetSdk 36 (aapt2 badging), signed by the
pinned test-circle debug certificate (SHA-256 `c76bb7c6…8551d`,
apksigner verify — installs over every earlier debug build, D85).
[The build's size, SHA-256, commit and the verified libgojni.so
signatures are recorded in the docs follow-up commit — the house
ritual.]

**v1.6.6-android-debug.23 (versionCode 21+1=22) — superseded by
debug.24** — the wire-discipline round. What changed, in plain
words (kept for the record):

- **"It doesn't even open video after a while. But the feed
  refreshes."** That split — video dead, feed alive — was the
  clue. Both go through the same proxy, the same fragmented
  hellos, the same DoH. What's different is VOLUME: for every
  video connection the engine opened 4 parallel connections
  (each sending a 299-piece fragmented hello), and on failure it
  retried, re-raced, and fired background bursts of up to 48
  more handshakes to googlevideo servers — while the feed's
  connections to youtube.com are one-at-a-time and quiet. A
  censor that flags noisy sources per service name slowly puts
  your line on the googlevideo list: everything works early
  ("after a while" = the flag accruing), then every googlevideo
  connection is reset on sight — permanently, because our retry
  machinery never stopped feeding it. The two projects you sent
  (gfw_resist_HTTPS_proxy — the family backend.py comes from —
  and WebImmortalGuards) never trip this because they emit ~1
  hello per connection and zero background traffic. That's the
  shape this round adopts.
- **The numbers to read next report:** `gvideo discipline:
  hellos=N cooldowns=M` next to `routedFlows` on the counters
  line. If hellos settles near routedFlows (≈1 per flow), the
  engine is finally as quiet as the Python. If cooldowns>0
  appears during an evening test and video comes BACK after
  ~45-60 s pauses instead of dying for the session, the silence
  circuit is doing its job — note whether it fires.
- **One ask:** watch whether the FIRST video after a fresh app
  start plays (the startup background burst is gone — the pool
  file + your first video's own connections now carry the cold
  start), and if video ever wedges again, note whether it
  recovers by itself within about a minute (the circuit) or
  needs a manual refresh (tell us — that changes the next
  round's tuning).

**The debug.23 artifact** (built this round, verified before
packing): `v1.6.6-debug23.apk`, versionCode 22 / versionName
1.6.6 / minSdk 26 / targetSdk 36 (aapt2 badging), signed by the
pinned test-circle debug certificate (SHA-256 `c76bb7c6…8551d`,
apksigner verify — installs over every earlier debug build,
D85), APK 55,283,553 B, SHA-256
`c5d48c50a8c56e061b3c002db04d9711aa139668e7eaae848ed9db3a7c718c7c`.
Built from commit a57a215. The gomobile bind ran with the
debug-tester ldflags
(`-X …/internal/config.DebugTesterBuild=true`) and the M8-ae
signatures were verified present in libgojni.so in ALL THREE
ABIs before gradle (the house ritual): the `gvideo discipline:
hellos=` report line, `gvideo emission breaker armed`, `tunnel
gvideo cooldown`, `gvideoHellos`, and `1.6.6-debug.23`; the
uploaded asset re-downloaded and verified byte-identical
(`cmp` + SHA-256).

**v1.6.6-android-debug.22 (versionCode
20+1=21) — superseded by debug.23** — the long-lived-connection round.
What changed, in plain words (kept for the record):

- **The minute-5 bug has three killers now, and none of them is
  you pressing refresh.** Your report: a video plays fine, then at
  ~minute 5 the spinner comes and only a refresh fixes it. That is
  a PROVEN connection dying mid-life (or being throttled to a
  crawl) while the engine's budgets for proven connections were
  sized for polite cleanup, not recovery: an unanswered request
  waited the full 90 s hygiene budget; a silently-dead pooled
  connection waited for an OS keepalive that takes ~2 HOURS; and a
  throttled connection (bytes still arriving, just never enough —
  the same shape as your debug.20 report's 5×-less-down) never
  tripped ANY stall clock, because the clock only starts when
  bytes STOP. Now: a request the origin leaves unanswered on a
  proven connection closes BOTH ends at 10 s (the player's own
  retry then lands on a fresh connection through the machinery
  that works — engine-side exactly what your refresh did by hand);
  every origin connection carries tuned keepalive probes
  (15 s idle, 3×5 s — a blackholed peer surfaces in ~30 s and the
  app's side closes); and a proven flow whose last 12 s delivered
  more than zero but under ~11 KB/s is closed on the spot — TCP
  delivery of video is bursty at line rate, so a sustained crawl
  is per-connection policing, and a FRESH connection to the same
  origin runs at full speed. Your refresh becomes the fallback,
  not the recovery.
- **A killed connection no longer drags the resolver into it.**
  Your debug.20 report's late-session "dns=579-846 ms" and 59 DoH
  dials were partly our own doing: every stall kill forced the next
  request to re-resolve through the (struggling) DoH lane, even
  though the old answer set's IPs were fine and the dead one is
  already skipped by rotation. Kills now leave the cache alone —
  the old Python cached DNS forever, and it was right to.
- **Auto now starts at the old Python's own hello shape.** You
  pointed back at backend.py, and the sharpest idea in it was the
  one we had as a preset but never actually used: 299 tiny cuts at
  1 ms — the fragmentation your network proved for years. Auto has
  been starting at "balanced" (~4-6 pieces of 80-200 bytes) and the
  repair sweep tried legacy LAST, so the strongest shape almost
  never ran. Under a censor that now resets recognizable hellos
  (35 of 77 probe verdicts in your last report), that was backwards
  — auto starts at legacy, and the sweep walks DOWN (legacy →
  strong → balanced → light) only when legacy genuinely fails.
  Costs ~300 ms per handshake, inside the first-byte budget, and
  in parallel across the race's candidates.
- **The report tells us which death shape your network actually
  uses.** The stall line now splits the proven-connection kills by
  cause: `rich=N (pending=Q trickle=T)`. pending>0 = requests dying
  on dead pooled connections (the wedge — now bounded at 10 s);
  trickle>0 = the per-connection throttle (the refresh-fixes-it
  shape, caught engine-side). If minute-5 stalls survive this
  build, that line says which killer to sharpen.
- **One ask for the next report:** you wrote "more in Linux, less
  in Windows" — please note WHICH BROWSER you tested on each OS
  (Firefox/Chrome/other) and whether the desktop build was the
  same version on both. The fix is browser-agnostic, but if the
  asymmetry is a browser's connection pooling rather than the OS,
  the next tuning pass wants to know.

**The debug.22 artifact** (built this round, verified before packing):
`v1.6.6-debug22.apk`, versionCode 21 / versionName 1.6.6 / minSdk 26 /
targetSdk 36 (aapt2 badging), signed by the pinned test-circle debug
certificate (SHA-256 `c76bb7c6…8551d`, apksigner verify — installs
over every earlier debug build, D85), APK 55,218,521 B, SHA-256
`4cba233bb9b17f6215c386fd74b8d4c956c24a6818e0985266f798e99ae04216`.
Built from commit 25b081c (rebased over the tester's own
`backend.py` upload, 3214c71 — the file this round's ADR-0039
answers). The gomobile bind ran with the debug-tester ldflags
(`-X …/internal/config.DebugTesterBuild=true`) and the M8-ad
signatures were verified present in libgojni.so in ALL THREE ABIs
before gradle (the house ritual): `cause=%s` + `trickle` on the
stall line's cause split, `stallRichPending` / `stallTrickle` on the
counters, the report's `rich=%d (pending=%d trickle=%d)` format, and
`1.6.6-debug.22`; the uploaded asset was re-downloaded and verified
byte-identical (`cmp` + SHA-256).

**v1.6.6-android-debug.21 (versionCode 19+1=20) — superseded by
debug.22** — the hostile-network round. What changed, in plain
words (kept for the record):

- **First, the honest part: your two tests ran at very different
  hours.** Your debug.19 test was ~4:17 AM (quiet hours); the
  debug.20 test was 6:04 PM (evening peak). The two reports show the
  blocking itself CHANGED between them — addresses that were
  blackholed in the morning answered in the evening, and connections
  that lived for minutes in the morning got killed mid-flight in the
  evening. Some of "way worse" is the network, not the app. If
  debug.21 still struggles in the evening, **please re-test the next
  morning** and send both reports — that comparison alone tells us
  how much is time-of-day.
- **The engine stops punishing addresses for the new block's
  behavior.** Under the evening regime a connection death says
  nothing about the ADDRESS (your report's own one-shot probe said
  every penalized address ANSWERED). Now: a RESET death only skips
  that address for ~20 s (long enough to spread one retry wave), and
  never demotes it from the verified pool; a full-timeout death keeps
  the old 60 s skip; and the background verification now RE-CHECKS
  boxed addresses every pass — an address that answers is cleared on
  the spot (the report's new `rehabilitated=` count says how often
  the box was wrong).
- **Your traffic now spreads across verified addresses instead of
  piling onto one.** The debug.20 report showed all 17 www.youtube.com
  connections funneling onto a single pinned "verified" address —
  when the killer took it, everything died together. Consecutive
  connections now rotate through the verified list.
- **A connection that stalls with YOUR request unanswered demotes its
  address (a genuinely dead origin); a connection that stalls with
  nothing pending never does** (it might just be the app's idle
  pooled connection — killing it is cheap, blaming the address is
  wrong). The stall lines now say which: `cause=pending` or
  `cause=idle`, and the report splits the counts — **this is the one
  number that tells us, next report, whether the engine is killing
  dead connections (right) or killing your app's healthy pool
  (fixable with a longer budget)**.
- **The background lanes stand down when the DNS resolver itself is
  under attack.** Your report showed the resolver's connections being
  killed 10x faster than the morning (dials 59 vs 6) — while the
  engine's own prewarm/probe bursts kept piling onto the dying lane.
  Now ≥3 resolver replacements inside a minute pauses every
  background lane until the resolver recovers (the report's
  `brownouts=` counts the periods). The 45-second probe cadence is
  gone entirely — the persisted pool from your last session and the
  connections that actually work do its job without the scan-like
  bursts.

**The debug.21 artifact** (built this round, verified before packing):
`v1.6.6-debug21.apk`, versionCode 20 / versionName 1.6.6 / minSdk 26 /
targetSdk 36 (aapt2 badging), signed by the pinned test-circle debug
certificate (SHA-256 `c76bb7c6…8551d`, apksigner verify — installs
over every earlier debug build, D85), APK 55,197,201 B, SHA-256
`bf66bd831e813a74791e74827ebce987699531e1602cb9482acd7cf5d111c368`. Built from commit 6391221. The
gomobile bind ran with the debug-tester ldflags
(`-X …/internal/config.DebugTesterBuild=true`) and the M8-ac
signatures were verified present in libgojni.so in ALL THREE ABIs
before gradle (the house ritual): cause=pending, rehabilitated=,
brownouts=, goodDrops, rehab=, and 1.6.6-debug.21.

**Tester checklist for debug.21 (the 5 steps):**

1. Install debug.21 and open the app. Turn it **On**, then give it
   ~15 seconds (the persisted pool from your last session seeds
   immediately — the `substitution:` line's pool should be non-zero
   from the start).
2. Fully close the YouTube app, then open it fresh.
3. Tap **"I pressed play"** in the Younblock diagnostics screen, then
   play a **new** video in YouTube.
4. When the first frame appears, tap **"First frame"**.
5. Play two more new videos, turn Younblock **Off then On**, play one
   more, then open **Diagnostics** and send the whole report —
   **ideally twice: once in the evening and once the next morning.**

What to look for: the play→first-frame time; the `stall kills:` line's
**pending vs idle split** (pending dominating = the killer is real and
the engine is failing fast correctly; idle dominating = tell us, the
budget changes next round); `rehabilitated=` > 0 (the box
self-correcting); `brownouts=` (the resolver under attack — with the
background lanes paused during them); and the Top-10 failure rates no
longer pinned at 100% on one host. If videos still won't play with
clean-looking numbers (fast first bytes, substitution wins, few
errors), the block is on the video DATA itself — try lower playback
quality (Settings → Data saving in YouTube) so each chunk completes
before the killer strikes, and send the report anyway.

Earlier at M8-ab: **v1.6.6-android-debug.20 (versionCode 19)** was
the escape-the-pool round. What changed, in plain words:

- **A video connection whose own addresses keep dying now ESCAPES to
  an address the engine has already seen speak** (Android-tuned;
  desktops benefit too once their pool fills). Your debug.19 report
  showed the whole remaining wait: five retry waves re-gambling
  fresh unknown addresses of the same blackholed local cache cluster
  — 45 seconds — while 24 verified-good addresses sat in a table no
  connection could reach, until YouTube itself fell back to a foreign
  cache and the video finally played. Google's video edge serves ANY
  googlevideo.com name from any of its caches (this is exactly the
  trick behind the "working YouTube IPs" hosts-file lists Iranians
  share), so when a video connection's raced round would die, one of
  its candidates now dials a VERIFIED-LIVE pool address instead —
  same fragmented hello, same name, different cache. If that cache
  answers, the video starts in one round.
- **The pool now fills from the connections that ACTUALLY WORKED**
  (not just the background probes): every first byte that lands marks
  its address verified-live — the one foreign cache that served you
  this session now arms every later connection. And an address that
  goes black is dropped from the pool the moment a connection dies on
  it (a latent leak the debug.19 code left: dead addresses could
  resurface at the FRONT of the dial lists).
- **The background verification now covers ALL the cache clusters it
  knows, not just the two newest ones, and runs FAR more often while
  it matters**: passes every 45 seconds while the engine is young and
  the pool is thin (your session ran ONE pass in its whole cold
  start), the name list walks every learned cluster in turn, and a
  video connection that dies completely triggers an immediate extra
  verification pass — verified addresses land inside the app's NEXT
  retry wave, not the next 5-minute cycle.
- **The verified pool now PERSISTS across sessions**
  (googlevideo-good-ips.txt, in the app's private data): the first
  video of your next session rides the addresses this session
  verified. A stale entry self-corrects — one dead connection drops
  and blacklists it.
- **The report says whether the escape worked**: a new
  "substitution: attempts=N wins=M pool=G (gvideo V)" line (wins>0 =
  the cross-cache bet paid; attempts>0 with wins=0 = it did not, and
  the next report tells us to change the shape), a "tunnel
  first-byte substituted" log line beside the raced one, and the
  googlevideo-terminal counter the immediate re-verification
  triggers on.

**The debug.20 artifact** (built this round, verified before packing):
`v1.6.6-debug20.apk`, versionCode 19 / versionName 1.6.6 / minSdk 26 /
targetSdk 36 (aapt2 badging), signed by the pinned test-circle debug
certificate (SHA-256 `c76bb7c6…8551d`, apksigner verify — installs
over every earlier debug build, D85), APK 55,148,841 B, SHA-256
`e44efc0d5baeefb3bff019090b309460264e69921bfa3d340406c5a874f8f2ab`. Built from commit 4058f46. The
gomobile bind ran with the debug-tester ldflags
(`-X …/internal/config.DebugTesterBuild=true`) and the M8-ab
signatures were verified present in libgojni.so in ALL THREE ABIs
before gradle (the house ritual): the substitution line, the
substituted first-byte line, gvideoTerminals, the good pool save
line's source string, and 1.6.6-debug.20.

**Tester checklist for debug.20 (the 5 steps):**

1. Install debug.20 and open the app. Turn it **On**, then give it
   about 15 seconds (the fast-start verification passes are filling
   the pool — the "origin probe:" line shows passes climbing).
2. Fully close the YouTube app, then open it fresh.
3. Tap **"I pressed play"** in the Younblock diagnostics screen, then
   play a **new** video in YouTube.
4. When the first frame appears, tap **"First frame"**.
5. Play two more new videos, turn Younblock **Off then On again**
   (this seeds the next session's pool), play one more new video,
   then open **Diagnostics** and send the whole report.

What to look for: the elapsed time between your two taps (target well
under the debug.19 baseline of 45 s — with substitution working, the
first video should land in seconds); the `substitution:` line —
`attempts` >0 means the escape is firing, `wins` >0 means it WORKED
(this is the round's headline number); `origin probe: passes=` should
climb several entries during the first minutes (the fast-start
cadence); and after the Off/On cycle, the LAST video should start
fast too (the persisted pool seeding). If `attempts` >0 but `wins`
stays 0, send the report anyway — that single number says the
cross-cache bet failed on your ISP and the next round changes shape.

Earlier at M8-aa: **v1.6.6-android-debug.19 (versionCode 18)** was
the dead-origin-pool round. What changed, in plain words:

- **The engine now VERIFIES the video hosts' addresses before your
  first play needs them** (Android only; desktops unchanged). Every
  5 minutes — and once the moment the app turns On — it takes the
  addresses DNS returned for the player's API hosts and the video
  cache nodes and asks each one a tiny fragmented hello, exactly the
  shape a real connection would send. An address that ANSWERS is
  remembered as verified-live for 15 minutes and every new connection
  tries it FIRST; an address that stays SILENT is skipped for 10
  minutes — and if it keeps dying, longer (1 strike 60 s, 2 strikes
  5 min, 3+ strikes 30 min). Your debug.18 report showed the first
  video's connections gambling on addresses that connect but never
  speak (IP-blackholed cache nodes) — 38 failed connections, the top
  host retried 13 times. The gamble now happens in the background,
  before the first flow pays for it.
- **A connection whose race all died gets ONE more in-engine rescue
  round** before giving up: if the address list still holds origins
  nobody tried yet, the engine dials exactly those and only fails the
  flow when they die too — your app's own retry wave (its backoff,
  its fresh gamble) is replaced by one fast internal round you never
  see. A host with nothing left to try still fails fast (~2.6 s), so
  hopeless addresses don't burn double.
- **A stalled half-finished response is cut at 8 seconds on Android**
  (was 15): your report carried three connections that each delivered
  a few KB of a response and then went silent, holding the app's
  retry for the full 15 s each — nearly half the minute you waited.
  Proven video connections (the ones that carried real data) keep
  their 90 s budget exactly as before; only the never-proved corpse
  shape is cut sooner.
- **The report now measures the play→video time even if you don't tap
  the buttons**: the engine watches for the connection burst that
  looks like a play press and the data burst that follows, and prints
  the pair ("auto-timeline", labeled a heuristic — the buttons stay
  the honest measurement). New lines also show how many addresses the
  verification pass checked and what it found (answered / silent /
  reset / verified-live / skipped / strikes), how many flows the race
  and the rescue round saved, and the stall-kill tier split with the
  budget that fired.

**The debug.19 artifact** (built this round, verified before packing):
`v1.6.6-debug19.apk`, versionCode 18 / versionName 1.6.6 / minSdk 26 /
targetSdk 36 (aapt2 badging), signed by the pinned test-circle debug
certificate (SHA-256 `c76bb7c6…8551d`, apksigner verify — installs
over every earlier debug build, D85). Built from commit
1143cc6f3894dceb072fbbd30370c13c569fb97f. The gomobile bind ran with the debug-tester
ldflags (`-X …/internal/config.DebugTesterBuild=true`) and the M8-aa
signatures were verified present in libgojni.so in ALL THREE ABIs
before gradle (the house ritual): the origin probe line, the
AUTO-MARK line, the salvage retry line, stall kills leanBudget=, and
the auto-timeline line.

**Tester checklist for debug.19 (the 5 steps):**

1. Install debug.19 and open the app. Turn it **On**, then give it
   about 15 seconds before the next step (the address verification
   pass warms up in the background — the "origin probe:" line in the
   report will show what it did).
2. Fully close the YouTube app, then open it fresh.
3. Tap **"I pressed play"** in the Younblock diagnostics screen, then
   play a **new** video in YouTube. (If you forget, the report's new
   "auto-timeline" line measures it anyway — but the buttons are the
   honest number.)
4. When the first frame appears, tap **"First frame"**.
5. Play two more new videos, then open **Diagnostics** and send the
   whole report.

What to look for: the elapsed time between your two taps (target
under 5 s, ideally 1–3 s); the `origin probe:` line should show
passes≥1 with answered>0 (the pool was verified); `salvagedSaves` and
`racedSaves` may be >0; the lean stall kills (if any) should land at
~8 s with `leanBudget=8s`; and the Top-10's failure counts should be
well below the debug.18 baseline (8/6/6/5/5/4).

Earlier at M8-z: **v1.6.6-android-debug.18 (versionCode 17)** was the
cold-start round. What changed, in plain words:

- **Every routed app's TLS is now fragmented on Android** (a new
  "fragment all routed" behavior, ON by default on Android only —
  desktops unchanged). Before, only the 7 YouTube domains got the
  fragmentation that beats SNI inspection; every OTHER host the
  YouTube app needs at first play (ads, attestation, Google APIs)
  crossed un-fragmented and, when blocked, hung silently until the
  app gave up after about a minute — the suspected cause of the
  1–2 minute first video. The heavy machinery (budgets, re-dials,
  races, the dead-IP box) stays scoped to the YouTube-list hosts
  exactly as before; everything else gets fragmentation plus ONE new
  safety net: **if a TLS host never answers the hello within 8
  seconds, the flow is closed and counted** instead of hanging. A
  ".ir" host is never touched, as always.
- **DoH shares ONE warm connection and hedges its servers**: the
  resolver now speaks HTTP/2 over a single connection it opens at
  Start and keeps alive with pings (a burst of fresh video hostnames
  used to open one new fragmented TLS handshake EACH — the slow cold
  lookups), and when a server is slow the next one is asked after
  300 ms instead of after a full 3 s timeout. Two no-SNI IP-endpoint
  servers (1.1.1.1, 8.8.8.8 by IP) join the end of the list as last
  resorts — a ClientHello with no name in it is very hard to match.
  The video-cache hostnames it learns are pre-resolved every 5
  minutes so the next video's fresh hostname often hits a warm cache.
- **The QUIC fallback is measurable**: every UDP/443 drop is logged,
  and the NEXT TCP flow to the same host logs how long the fallback
  took; the IPv6 discards are counted.
- **The report can prove all of it**: every tunnel line now says
  class=routed or class=plain; a "Top 10 slowest or failed hosts"
  section names the culprits (this DEBUG build includes host names —
  the report's first line says so); per-DNS-failure lines say WHICH
  server and WHICH phase (TCP, TLS or HTTP) failed; a one-shot probe
  tests whether blocked origins answer a different name; and the
  diagnostics screen has two new buttons, **"I pressed play"** and
  **"First frame"**, which stamp the log — the report then measures
  exactly that window.

**The debug.18 artifact** (built this round, verified before packing):
`v1.6.6-debug18.apk`, versionCode 17 / versionName 1.6.6 / minSdk 26 /
targetSdk 36 (aapt2 badging), signed by the pinned test-circle debug
certificate (SHA-256 `c76bb7c6…8551d`, apksigner verify — installs
over every earlier debug build, D85), APK 55,059,385 B, SHA-256
`9bb9707613bac71aa8680c2a3b545c2579145851884d9f321005028f92d4ecd5`.
Built from commit 8b108e5c02b823c4b48fd6d51deec1b8723b39f4. The gomobile bind ran with the debug-tester ldflags
(`-X …/internal/config.DebugTesterBuild=true`) and the M8-z
signatures were verified present in libgojni.so in ALL THREE ABIs
before gradle (the house ritual): the DEBUG BUILD notice line, the
plain kill line, the udp443 fallback line, the MARK line, the warm
line, hedgeWins, and the dead-origin probe.

**Tester checklist for debug.18 (the 5 steps):**

1. Install debug.18 and open the app. Turn it **On**.
2. Fully close the YouTube app, then open it fresh.
3. Tap **"I pressed play"** in the Younblock diagnostics screen, then
   play a **new** video in YouTube.
4. When the first frame appears, tap **"First frame"**.
5. Play two more new videos (do not tap the buttons), then open
   **Diagnostics** and send the whole report.

What to look for: the time between your two taps (the report prints
it — the target is under 5 s, ideally 1–3 s); the Top-10 list should
no longer show class=plain hosts hanging for tens of seconds (they
either work now or die at 8 s and get counted); the dns failure rate
should be far below the old 26%; and the dns lines now say which
phase failed.

Earlier at M8-y: **v1.6.6-android-debug.17 (versionCode 16)** carried
ADR-0023, the NEGATIVE RE-ASK + the ORIGIN-IP PENALTY BOX:
a DNS_UNREACHABLE negative-cache hit re-asks the servers (through
the singleflight, rate-limited to one walk per second per host —
the window starts at the failure's own walk) instead of locking a
host out for the negative's whole 15 s TTL, and every classified
first-byte death (the origin connected and never spoke) marks its
IP for 60 s, every dial list skipping the marked IPs (a preference,
never a verdict — an all-penalized list dials in full), so the
dead-origin knowledge crosses flows AND hostnames; the socks5 door's
DNS failure line now carries reason= in the message. The evidence
lines for the next report: "dns negative re-ask (path failure, not
a name verdict)", "socks5 dns lookup failed reason=…", the dns
fail counter (should drop far below 50/192), and the terminal
cohort's shape.)
Earlier: M8-x (ADR-0022, debug.16 — superseded by debug.17: its
round produced the eighth pasted report, which debug.17 acts on;
its own fix VERIFIED on-device by Ali's words), M8-w (ADR-0021,
debug.15 — superseded: debug.16 carried it forward and the eighth
report proves it ran — "first-byte raced dials=2"), M8-v
(ADR-0020, debug.14 — superseded), M8-u (the no-build round — the
fourth report ran the previous build, versionCode 11; its D103
ruling put the build number in the APK asset name), M8-s
(ADR-0018, debug.12 — superseded by debug.13), M8-r (ADR-0017,
debug.11), M8-q/p (the diagnostics surface, ADR-0015/0016 +
D100/D101), M8-n/M8-o (the stall fixes, ADR-0013/0014 + D98/D99),
A0/A0-R2/A0-R3
— **the account gate is CANCELLED (D97)**, see the closing note
below. This document now holds two things: §1 the **shipped** debug
test-circle releases (`v1.6.6-android-debug.23` is the CURRENT
build — a regular release in the private repo per D89, the same text
lives on the GitHub release itself; beneath it: debug.22 the
long-lived-connection build (superseded by debug.23: its round
produced the doesn't-even-open-video-anymore report, which
debug.23 acts on), debug.21 the
hostile-network build (superseded by debug.22: its round produced
the minute-5 mid-video report, which debug.22 acts on), debug.16 the
tiered-stall-watch build (superseded by debug.17: its round produced
the eighth pasted report, which debug.17 acts on — and its own fix
VERIFIED on-device by Ali's words), debug.15 the
first-dial-race build (superseded by debug.16, which carried it
forward and the eighth report proves it ran), debug.14 the
salvage-race + stall-watch build
(superseded by debug.15: its build produced the seventh pasted
report, which debug.16 acts on), debug.13 the salvage build
(superseded by debug.14), debug.12 the first-byte-budget build,
debug.11 the heal-sweep build, debug.10 the diagnostics-port
build, debug.9 the classic-tree diagnostics build, debug.8 the
per-video-stall-fix build, debug.7 the first-video-fix build,
debug.6 the one-UI-everywhere + data-usage build, debug.5 the
first-device-feedback build, debug.4 the helper-fix maintenance
release, debug.3 the device-circle build whose APK ran on Ali's
phone, and debug.2/debug.1 superseded and kept as history),
and §2 the **public v1 DRAFT**, unpublished until the ⛔ device gates
close or Ali explicitly accepts shipping with them recorded as known
limitations. Every claim traces to implementation evidence (docs
01–09, DECISIONS.md D1–D103, of which D97 cancels the gate D94 had
announced, D98 rules the APK asset onto every release page, D99 names
it with the version, D100 is the diagnostics-surface ruling, and D101
is the variant rule: a surface ships in the variant the edition
serves).
**A0-R3 closing note (D97, 2026-10-18): the gate is CANCELLED — it
will never ship, so none of that future ever happens. The "no
account, no data collection" line in §2 stays verbatim PERMANENTLY;
no release ever changes it for a gate; `docs/auth/` carries dated
cancellation banners and stays as history; doc 07 §9's row is closed
as cancelled; ADR-AUTH-0001 is retired (doc 08). Younblock stays one
free-for-all app.**

## 1. v1.6.6-android-debug.17 — the negative re-ask + the origin-IP penalty box: DNS blips stop locking hosts out and dead origins stop being re-gambled (SHIPPED — a regular release, the CURRENT build)

**What this build is:** versionCode 16, built at the M8-y commit, the
pinned debug certificate — **installs directly over debug.16** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug17.apk** (D103 — the filename
itself names the build). This is the round your EIGHTH diagnostics
report built — the first report from the right build (your shell
line read versionCode 15, and it carried both prior signatures: the
raced line and the tier= stall line). Your words closed the previous
round — next/back in the same video is instant again — and named
what remains: videos still take time to START playing (ADR-0023).

1. **What was wrong (read from YOUR report, not guessed):** two
   things, both measured. First, the DNS LOCKOUT: your report shows
   dns ok 142 / fail 50, and every one of those 50 failures killed a
   connection at the door (doorFailures=50). The failure lines pair
   at the same millisecond — four pairs inside 22 ms — which is the
   signature of the negative cache answering instantly, not the
   servers actually being asked (a real ask takes seconds). One
   momentary DoH blip negative-cached a batch of your video's fresh
   hostnames, and for the next 15 seconds EVERY retry for those names
died instantly at the door — the blip's one-off failure became 50
   dead connections and a multi-second start. Second, the DEAD-ORIGIN
   RE-GAMBLE: ~17 of your flows died at exactly the 2.5 s first-byte
   budget (plus ~8 at ~0.6 s resets), every one through the raced
   round — both origin candidates were blackholed. Each death demoted
   the answer set, the next flow re-asked DNS, got the SAME blackholed
   addresses back (the official DNS doesn't know about the censor),
   and gambled again — and your SECOND video paid the whole discovery
   cost AGAIN because its cache hostname is a NEW name (the dead-IP
   knowledge used to die with each connection).
2. **What changed (ADR-0023, host-proven both ways):** two memories
   added. The NEGATIVE RE-ASK: when a cached DNS failure means "our
   path to the DNS servers failed" (not "this name is bad"), the
   next lookup RE-ASKS the servers instead of reading the 15-second
   failure memory — rate-limited to one real ask per second per name
   (your retry waves collapse into that one ask), so a recovered blip
   costs one round-trip instead of 15 seconds of dead connections.
   The ORIGIN-IP PENALTY BOX: when an origin connects but never
   answers (your 2.5 s timeouts and 0.6 s resets), its address is
   remembered as dead for 60 seconds and every new connection skips
   it — across flows AND across hostnames (the blackholed cache
   servers are shared between your videos' hostnames, so your second
   video starts on the addresses the first video already proved
   live). If every address of a set is remembered-dead, the engine
   still tries them all (the memory is a preference, never a
   refusal). Also: the door's DNS failure line now says WHY it
   failed, in the line itself.
3. **What to verify on the phone:** video STARTS — the first video
   after connecting, and the second one right after it (that one used
   to pay the full discovery cost again). When anything still stalls:
   menu → Diagnostics (first row) → Copy → paste it here. The evidence
   lines this round: "dns negative re-ask (path failure, not a name
   verdict)" (each one is a lockout absorbed), "socks5 dns lookup
   failed reason=…" (the reason finally visible), the dns fail
   counter (your report had 50 — it should collapse to the handful of
   real server walks), and the first-byte terminal count (17+8 last
   time). And check the shell line says **versionCode 16** before
   anything else — a 15 means the old APK is still installed.

## 1a. v1.6.6-android-debug.16 — the tiered stall watch: the pooled video connections stop being murdered at 15 s + (carried forward) the first-dial race and the honest DNS probe (SHIPPED — a regular release, superseded by debug.17: its round produced the eighth pasted report, which debug.17 acts on — and its own fix VERIFIED on-device, Ali's words: next/back in the same video is instant again)

**What this build is:** versionCode 15, built at the M8-x commit, the
pinned debug certificate — **installs directly over debug.14 or
debug.15** in place; settings and usage history survive. The APK asset
on the release page is NAMED **v1.6.6-debug16.apk** (D103 — the
filename itself names the build). **First: your seventh report's
shell line still read versionCode 13 — the phone was still running
debug.14; the filename you download must say debug16.** This round
acted on your own words: "videos used to have lag for starting to
play, but after that you could easily go next (watch next few
minutes) instantly without buffer, but now even going next and back
in the same video lags a lot."

1. **What was wrong (read from YOUR report, not guessed):** the M8-v
   stall watch — built to kill dead origins fast — was murdering the
   connections your player REUSES. Your log shows thirteen
   "tunnel stall killed idle=15xxx ms" lines in ~70 seconds, and
   every one of them had already carried real video data: 2.1 MB,
   1.6 MB, 723.6 KB, 633.7 KB, 162.6 KB downloaded on that very
   connection. The player paces its video connections — buffer a
   stretch, go quiet (often past 15 s), then ask for the next range
   ON THE SAME CONNECTION — that reuse is exactly what made "next"
   instant in the older builds. The 15 s idle watch kept killing
   them mid-quiet, so every next/seek had to dial a fresh connection
   through a ~50 % dirty path (your report's first-byte failures:
   resets at ~1.2 s, timeouts at ~5.4 s; errors 70/122 ≈ 57 %). And
   each kill also demoted a proven-good origin and counted an error —
   the watch was punishing the very servers that had just delivered
   your video.
2. **What changed (ADR-0022, host-proven both ways):** the watch now
   has TWO tiers. A flow that has PROVEN itself — 32 KB or more
   delivered down (real payload, past any handshake+headers), or one
   quiet gap of 3 s or more that ended with new data flowing (the
   pacing rhythm itself) — is only ever closed at 90 s of total
   silence, and that close is quiet: no origin demotion, no error
   count. The lean tier keeps the exact old death for the shape it
   was built for (first byte plus a few KB, never woke, silent):
   closed at 15 s, demoted, counted. The stall line now reads
   "tunnel stall killed idle=Nms down=N tier=rich|lean" so the next
   report shows exactly which tier fired. This build ALSO carries
   everything from debug.15, which your report never ran: the
   first-dial race (a flow's first origin raced against 2 extra
   candidates dialed at the same moment — the first to answer wins,
   so a dirty primary costs no budget) and the honest DNS probe (the
   health check asks the servers what they say NOW instead of reading
   a stale failure memory — your report's method line had flipped to
   balanced after one such false failure fired the heal sweep).
3. **What to verify on the phone:** going next and back in the same
   video — it should be instant again (that is the whole point of
   this build). Video starts should ALSO improve from the carried
   first-dial race. When anything still stalls: menu → Diagnostics
   (first row) → Copy → paste it here. The evidence lines this
   round: "tier=rich" kills should be rare and harmless, "tier=lean"
   kills should be the dead-origin cohort only, "tunnel first-byte
   raced dials=N" lines say the race is saving dirty primaries, and
   the dns fail counter says whether the degraded state still fires.
   And check the shell line says **versionCode 15** before anything
   else — a 13 means the old APK is still installed.

## 1a. v1.6.6-android-debug.15 — the first-dial race: the whole dial budget spent in parallel from the flow's start + the honest DNS probe (SHIPPED — a regular release, superseded by debug.16, which carries it forward UNVERIFIED: the seventh report's shell line still read versionCode 13 — the device never ran it)

**What this build is:** versionCode 14, built at the M8-w commit, the
pinned debug certificate — **installs directly over debug.14** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug15.apk** (D103 — the filename
itself names the build). This is the round your SIXTH diagnostics
report built: the first report from the salvage-race + stall-watch
build itself (your shell line read versionCode 13), it proved both
of those working and named the two things still costing video
starts (ADR-0021).

1. **What was wrong (read from YOUR report, not guessed):** the
   race and the watch both worked — your retry lines batch at the
   same millisecond, your "tunnel stall killed idle=15xxx ms" lines
   kill exactly at the 15 s budget, and the 60-second hangs are
   gone. But two costs remained. First, the SERIAL FRONT: every
   flow still walked its FIRST origin attempt alone through the
   full 2.5 s budget before the race even launched — six of your
   flows died at 5320–5576 ms (the two-budget chain) — while OTHER
   flows to the SAME hosts, opened in the SAME seconds, answered
   in 72–162 ms. The dirt is per-ORIGIN, not per-moment: one node
   in the answer set is dead while its siblings are fine, and each
   new flow still had to pay its primary's budget before trying
   them. Second, the DNS degraded state: dns fail 16 with
   "last failed check: dns (DNS_UNREACHABLE)" while your video was
   downloading 6.7 MB through the tunnel — a momentary DoH blip
   negative-cached youtube.com for 15 s, and the periodic health
   check read that memory instead of asking any server.
2. **What changed (ADR-0021, host-proven both ways):** two changes.
   The FIRST-DIAL RACE (unblock-list hosts only): your flow's first
   origin is now raced against 2 extra candidates DIALED AT THE
   SAME MOMENT (different addresses from the same answer set), the
   client's hello is replayed byte-exact to each, and the first
   origin to answer wins — so a flow whose primary is dirty lands
   on a live sibling at that sibling's 72–500 ms answer instead of
   paying the dead one's budget first, and a fully-dead answer set
   fails at ~one budget (~2.7 s) instead of two (~5.4 s), so your
   app's own retry — which your report shows recovering in
   60–261 ms — starts that much sooner. A raced save logs
   "tunnel first-byte raced dials=N"; a clean primary logs
   nothing (the race costs a healthy flow nothing). The HONEST DNS
   PROBE: the health check now clears the probe name's
   failure-memory entry before probing (a good cached answer
   stays), so it asks the servers what they say NOW — the degraded
   state clears itself in one cycle instead of living out the
   15 s memory. The method on the wire still never changes (D102
   — retrying a dead flow with the NEXT preset — stays your
   ruling; this build does not do it).
3. **What to verify on the phone:** video starts — the raced flows
   should land at their fastest live origin now. When anything
   still stalls, menu → Diagnostics (first row) → Copy → paste it
   here. The NEW line "tunnel first-byte raced dials=N" is this
   round's primary evidence: every one is a dirty primary your
   flow never paid a budget for; its count against clean
   first-bytes and terminals says whether the candidate count (2)
   should grow or shrink. The dns fail counter — does the
   degraded state still fire?

## 1a. v1.6.6-android-debug.14 — the salvage race + the stall watch: dead origins raced in parallel, stalled relays closed in seconds (SHIPPED — a regular release, superseded by debug.15: its round produced the sixth pasted report, which debug.15 acts on)

**What this build is:** versionCode 13, built at the M8-v commit, the
pinned debug certificate — **installs directly over debug.13** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug14.apk** (D103 — the filename
itself names the build). This is the round your FIFTH diagnostics
report built: the first report from the salvage build itself (your
shell line read versionCode 12), it proved the salvage working and
named the two things still costing video starts (ADR-0020).

1. **What was wrong (read from YOUR report, not guessed):** the
   salvage worked — your log shows "tunnel first-byte retry …
   attempt=N" lines absorbing dead attempts and "tunnel first-byte
   salvaged dials=N" saving flows at ~0.7–1.4 s. But two costs
   remained. First, the salvage walked its re-dials ONE AT A TIME:
   21 flows died at 8014–8227 ms — three 2.5 s budgets plus dials,
   serial — before the app could even retry. Second, and bigger:
   five of your video-start connections got their first byte plus
   ~7 KB, then went COMPLETELY silent — and lived 57–64 SECONDS
   until the YouTube app itself gave up on them (they all closed in
   the same instant); the retry right after answered in 60–261 ms.
   The engine had nothing that watched past the first byte, so it
   held those dead flows open through your app's whole minute of
   patience.
2. **What changed (ADR-0020, host-proven both ways):** two engine
   changes, both only for the unblock list's hosts. The salvage
   RACE: when the first origin dies at first byte, the engine now
   dials ALL its remaining re-dial candidates AT ONCE (different
   live addresses, the dead ones excluded), and the first origin to
   answer wins — a flow that would have died at ~8 s now dies at
   ~5 s worst case, and a save lands at the fastest live origin's
   answer. The STALL WATCH: once a flow is relaying, if BOTH
   directions go silent for 15 seconds the engine closes the flow
   itself (a new report line "tunnel stall killed idle=Nms"), so
   your app retries onto a fresh origin in seconds instead of
   waiting a minute — a download that keeps streaming is never
   touched, and neither is any site off the unblock list. The
   method on the wire still never changes (D102 — retrying a dead
   flow with the NEXT preset — stays your ruling; this build does
   not do it).
3. **What to verify on the phone:** video starts. When anything
   still stalls, menu → Diagnostics (first row) → Copy → paste it
   here. The NEW line that tells the stall story is "tunnel stall
   killed idle=Nms" — how many fire, and whether starts still stall
   when they do not, is the tuning evidence for the next round. The
   ratio of "retry/salvaged" lines to terminal "first-byte failed"
   lines is the D102 material.

## 1a. v1.6.6-android-debug.13 — the salvage re-dial: a dead first-byte origin is retried inside the flow (SHIPPED — a regular release, superseded by debug.14: its round produced the fifth pasted report, which debug.14 acts on)

**What this build is:** versionCode 12, built at the M8-t commit, the
pinned debug certificate — **installs directly over debug.12** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug13.apk** (D99 as amended by
D103 — the build number rides in the filename now, Ali's ruling
after the fourth report ran the previous build; renamed in place
2026-10-19, same bytes, same digest). This is the round your THIRD
diagnostics report built: it
proved the debug.12 budget on-device and named what was still
missing (ADR-0019).

1. **What was wrong (read from YOUR report, not guessed):** the
   budget worked — your log now shows "tunnel first-byte failed
   reason=timeout" closes at ~8.3 s (instead of the 60 s hangs) plus
   fast ~0.5 s resets — but roughly HALF of every video-start wave
   still died at first byte, and the YouTube app re-opened wave after
   wave until one landed: from your log, ~35 seconds from the first
   wave to the first video bytes. Each silent origin cost the full 8 s
   budget, and the retry chain itself was the app's — the engine
   could only make each link cheaper, not shorten the chain.
2. **What changed (ADR-0019, host-proven — reproduced exactly, then
   fixed):** two things. First, the budget is retuned 8 s → 2.5 s:
   your report shows every budget-killed flow was 100 % silent
   (zero bytes — the same flows that sat out the full 60 s before:
   dead nodes, not slow ones) while every healthy origin answered in
   420–508 ms, so 2.5 s never trips a healthy origin and a dead
   attempt now costs 2.5 s. Second — the big one — the SALVAGE
   RE-DIAL: when an origin dies at first byte, the engine no longer
   closes the flow. It closes only the dead origin, re-resolves the
   host (skipping the addresses that already died), re-dials the SAME
   host with the SAME method, and replays your app's buffered bytes
   byte-exact — up to 2 re-dials inside the same flow. The app never
   sees the failure: its wave-after-wave retry chain collapses into
   in-engine attempts of ~0.5–2.5 s each, bounded at 3 dials per flow.
   The method on the wire never changes (that question — retrying a
   dead flow with the NEXT preset — is still yours to rule, D102;
   this build does not do it). Everything NOT on the unblock list
   keeps the old behavior byte-for-byte.
3. **What to verify on the phone:** video starts should now open in
   seconds — the dead-origin coin flips get re-flipped inside the
   flow instead of by the app's own retries. When anything still
   stalls, menu → Diagnostics (first row) → Copy → paste it here:
   the NEW lines that tell the story are "tunnel first-byte retry
   reason=… attempt=N" (an absorbed dead attempt) and "tunnel
   first-byte salvaged dials=N" (a flow the salvage saved) — their
   ratio against the terminal "first-byte failed" lines is the
   tuning evidence for the next round (and the D102 material).

## 1a. v1.6.6-android-debug.12 — the first-byte budget: silent origins fail fast (SHIPPED — a regular release, superseded by debug.13: its round produced the third pasted report, which debug.13 acts on)

**What this build is:** versionCode 11, built at the M8-s commit, the
pinned debug certificate — **installs directly over debug.11** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug12.apk** (D99 as amended by
D103 — renamed in place 2026-10-19). This is the round your SECOND
diagnostics report built: it
proved the debug.11 fixes on-device (state on, a sweep that found
its method) and exposed the remaining stall's exact shape
(ADR-0018).

1. **What was wrong (read from YOUR report, not guessed):** every
   video start still lagged, but your report told us WHY it waits:
   at each video start the app opens a handful of parallel
   connections; in your log, some of them complete in half a second
   while others connect fine, send the ~1.7 KB TLS hello, and then
   never receive ONE byte back — the origin (or the path to it)
   stays silent. Some of those silent flows get reset in ~0.5 s
   (fine — the app retries immediately), but the rest hang for
   EXACTLY ~60 s with zero bytes down. We read that number back
   against the engine line by line: nothing in the engine closes a
   flow at 60 s (its own quiet-close timer is 300 s) — the ~60 s is
   the YouTube app's own patience, and our engine was holding the
   dead flow open the whole time, and worse: because the connection
   SUCCEEDED, the engine kept trusting that server's address set,
   so every retry could land on the same dead server until the
   address cache expired.
2. **What changed (ADR-0018, host-proven — reproduced exactly, then
   fixed):** a processed host's origin must now answer within 8
   seconds of the app's first message, or the flow fails FAST —
   both ends closed immediately (your app sees the reset and
   retries at once, exactly like it already retries the ~0.5 s
   resets), the failure is counted and logged with its reason
   ("tunnel first-byte failed reason=timeout/reset/closed"), and
   the server's address set is demoted — the next lookup re-asks
   the DNS servers instead of serving the same suspect set. 8 s is
   >12x the slowest healthy first byte in your report (619 ms), so
   a healthy origin never trips it; a silent one now costs 8 s
   instead of 60. Everything NOT on the unblock list keeps the old
   behavior byte-for-byte.
3. **What to verify on the phone:** video starts should open in
   seconds instead of "after a while" — and when one still stalls,
   menu → Diagnostics (first row) → Copy → paste it here: the
   report should now carry "tunnel first-byte failed reason=…"
   lines and done lines like "dur=~8000ms down=0 B" — that is the
   tuning evidence for the budget (and the material for the method
   question below).

## 1a. v1.6.6-android-debug.11 — the false "Couldn't fix it" fixed: the heal sweep probes from zero (SHIPPED — a regular release, superseded by debug.12: its round produced the second pasted report, which debug.12 acts on)

**What this build is:** versionCode 10, built at the M8-r commit, the
pinned debug certificate — **installs directly over debug.10** in
place; settings and usage history survive. The APK asset on the
release page is NAMED **v1.6.6-debug11.apk** (D99 as amended by
D103 — renamed in place 2026-10-19). This is the round your first
diagnostics report built: it
named the build (versionCode 9 ✓ — the surface worked), and its log
was read back against the engine line by line (ADR-0017).

1. **What was wrong (read from YOUR report, not guessed):** 21
   seconds after you pressed Start, the app told you "Couldn't fix
   it. Open Younblock." and the state read degraded — while the
   same report showed the network recovering and real video data
   flowing (a 2.5 MB download minutes later). The sequence in your
   log: the secure-DNS lookup blipped at start (all three servers
   failed fast, once); the app remembers a failed lookup for 15
   seconds; the automatic fix sweep that immediately followed then
   probed the SAME name and hit that memory every time — so every
   method "failed" without ever asking the servers again, and the
   sweep reported no working method on a network that had already
   recovered. Two smaller defects rode along: the failed sweep left
   the connection method switched to "legacy" (the last one it
   tried) instead of your setting, and the report's DNS failure
   lines carried no reason ("dns server failed" — unable to say
   timeout vs reset).
2. **What changed (ADR-0017, host-proven — reproduced exactly, then
   fixed):** the fix sweep now starts fresh — it clears the DNS
   memory before probing (the same thing the app already does on a
   real network change), so one blip at start can no longer poison
   the whole sweep; a sweep that finds nothing restores your
   previous method instead of silently switching it; and the DNS
   failure lines now carry their evidence in the message
   ("dns server failed server=#N reason=…"), so the NEXT report
   tells us WHAT KIND of failure the startup blip is.
3. **What to verify on the phone:** press Start on your normal
   network — it should reach Connected (or recover on its own if
   your secure-DNS providers genuinely hiccup) WITHOUT the
   "Couldn't fix it" notification from a working network, and the
   method shown in Settings should stay what you set. When a video
   stalls mid-session: menu → Diagnostics (first row) → Copy →
   paste the report here — the report now carries the DNS failure
   reasons, which is the missing piece for the stall row.

## 1. v1.6.6-android-debug.10 — the diagnostics surface on the screen your phone actually serves (SHIPPED — a regular release, superseded by debug.11: its round produced the first pasted report, which debug.11 acts on)

**What this build is:** versionCode 9, built at the M8-q commit, APK
35,053,265 B (SHA-256 4264996a…f9e7), the pinned debug certificate
— **installs directly over debug.9** in place; settings and usage
history survive. The APK asset on the release page is NAMED with the
version (**v1.6.6-debug10.apk**, D99 as amended by D103 — renamed
in place 2026-10-19). It carries ONE theme: **the diagnostics
surface finally rendered where you have been looking all
along** (ADR-0016, D101):

1. **What was wrong (found this round):** the debug.9 build DID ship
   the Diagnostics screen — but into the app's CLASSIC interface,
   while your phone runs the redesigned (pro) interface the APK
   bakes. On your screen, the only diagnostics was a section at the
   very BOTTOM of the menu (below Help), and its Copy button can
   silently fail inside the WebView. A shipped-in-the-wrong-
   interface feature is invisible — your report was exact: the
   button did not exist where you looked.
2. **What changed:** **Diagnostics is now the FIRST row of the menu**
   — open the menu (the ⋯ button, top right) and it is right there,
   no scrolling: "Diagnostics — Copy a support report to paste into
   a chat". It opens the full report view: **Copy report** (the
   native clipboard path first, honest "Copied" only on real
   success), **Share** (the system share sheet — the most reliable
   way to hand the report to a chat app), **Refresh**, and the
   report text itself, selectable (press-and-hold, Select all,
   Copy — the path that cannot fail). The old buried section is
   gone. The report still carries the TUN counters, the exact build
   identity ("android shell: 1.6.6 (versionCode 9)") and the last
   200 log lines from debug.9.
3. **What to verify on the phone (two pastes, please):** (a) open
   the menu → **Diagnostics is the first row** → open it → **Copy**
   → paste the report here — even with nothing stalled: its build
   line proves this round; (b) when a video stalls mid-session,
   Diagnostics → Copy again and paste that report too. The stall
   row stays OPEN until a real pasted report arrives.

## 1a. v1.6.6-android-debug.9 — the classic-tree diagnostics surface (SHIPPED — a regular release, superseded by debug.10: the surface existed but the APK serves the pro screen, so it never rendered on the phone)

**What this build is:** versionCode 8, built at the M8-p commit, APK
35,026,961 B (SHA-256 1a2b2bf7…7833), the pinned debug certificate
— installed directly over debug.8 in place; settings and usage
history survive. The APK asset on the release page is NAMED with the
version (**v1.6.6-debug9.apk**, D99 as amended by D103 — renamed in
place 2026-10-19). It carries ONE theme: **the evidence-collection
surface for the still-open per-video stall
(ADR-0015) — "Menu → Diagnostics → Copy" is now a real, reachable
path on every edition**:

1. **What was wrong (the evidence):** Ali's debug.8 round reported
   the stall PERSISTED — and that the promised evidence input did not
   exist: "there is no diagnostic option in the apk app setting".
   The report's copy path DID exist (Help → Still stuck → Copy
   diagnostic report) but it was buried, and inside the Android
   WebView its clipboard write can silently fail — the button
   toasted "Copied" while the clipboard stayed empty. A diagnostics
   loop cannot run on a broken evidence path.
2. **What changed:** (a) a **Diagnostics screen** — in the menu
   (between Help and About) AND as a row in Settings — showing the
   full report with three ways out: **Copy** (the Android-native
   clipboard bridge first, the browser API second, a legacy fallback
   third — honest "Copied"/"Copy failed" toasts, never a false
   success), **Share** (the system share sheet, the most reliable
   way to hand the report to a chat app), and the selectable text
   itself (the copy path of last resort, always available);
   (b) the report now carries the **TUN counters** (flows, dropped
   UDP, door failures — the one surface that names the "flow never
   reached the door" case and the QUIC suspect) and the **exact
   build identity** ("android shell: 1.6.6 (versionCode 8)" — a
   pasted report names its own release); (c) the report's log window
doubled to **the last 200 lines** (a stalled video's retry loop
   writes ~3 lines per attempt; the wider window keeps the whole
   stall story). The desktops and the Linux web edition get the same
   screen (minus Share) — one UI, every edition.
3. **What to verify on the phone (the row stays open until you
   confirm):** connect, play videos until one stalls, then **Menu →
   Diagnostics → Copy** (or Settings → Diagnostics report) and send
   the report here — paste it in the chat. The tunnel lines
   (dns=/dial=/first-byte=/dur=), the tun: line, and the on-screen
   speed number at the slow moment split the remaining suspects in
   one read. If copy ever says "Copy failed", use Share, or
   press-and-hold the text and copy manually.

## 1a. v1.6.6-android-debug.8 — the per-video stall loop closed (SHIPPED — a regular release, superseded by debug.9)

**What this build is:** versionCode 7, built at the M8-o commit, APK
34,978,761 B (SHA-256 c0c4019f…cd5f6), the pinned debug certificate
— **installs directly over debug.7** in place; settings and usage
history survive. The APK asset on the release page is now NAMED with
the version, and every release after this one follows that form
(D99; amended by D103 at the D99-era releases: the build number
rides in the filename — this release's asset is now
v1.6.6-debug8.apk). It carries ONE theme: **the dial-failure →
DNS-requery feedback loop (ADR-0014) — the per-video stall amplifier
that survived the debug.7 fix**:

1. **What was wrong (the evidence):** on Ali's debug.7 round the
   feed and UI stayed instant but EVERY video took 1–2 minutes to
   start and re-buffered during playback, while the Windows app on
   the same network was instant. Racing past one blackholed address
   (the debug.7 fix) cannot help when the WHOLE address set is
   blackholed: each rr*.googlevideo.com hostname carries the few
   addresses of ONE cache node, so one filtered node poisons the
   entire set. The app retried every few seconds; every retry
   re-dialed the same cached addresses and failed — and the DNS
   cache (120 s minimum) kept serving that suspect answer until it
   rotated. That lock was the per-video wait, and because video
   URLs rotate rr hostnames DURING playback, also the mid-play
   re-buffering. The feed never suffers: its hosts are prewarmed
   and long-lived.
2. **What changed:** when a door dials every address of a host's
   answer set and none connect, the cached answer is demoted to
   stale immediately — the app's NEXT retry fetches a fresh answer
   from the DNS servers instead of waiting out the 120 s cache
   (serve-stale is preserved: if the servers then fail, the old
   answer still serves — the failure path is never worse). The
   resolver's own bootstrap dial also races its addresses now (the
   same ~0.75 s failover the doors got in debug.7). One mechanism,
   wired identically on the browser doors and the Android SOCKS5
   door — the desktop benefits from the same fix.
3. **What to verify on the phone (the row stays open until you
   confirm):** connect, play the first video, then play several
   OTHER videos back to back — each should start within seconds,
   like the Windows app, and keep playing without re-buffering.
   If any video still stalls: let it finish stalling, then Menu →
   Diagnostics → Copy and send the report — the "tunnel" lines
   carry per-flow dns=/dial=/first-byte=/dur= numbers that name
   the waiting phase in one read (nothing logged at all for the
   stalled video = the flow never reached the door, a different
   suspect; fast setup + slow dur = a throughput leg). The phone's
   on-screen speed number during a stall splits it further: near
   zero = the door path is starved; high while the player still
   buffers = the player's own transport.

## 1b. v1.6.6-android-debug.7 — the first-video fix (SHIPPED — a regular release, superseded by debug.8)

**What this build is:** versionCode 6, built at the M8-n commit, APK
34,960,649 B (SHA-256 dc01c041…dd83d), the pinned debug certificate
— **installs directly over debug.6** in place; settings and usage
history survive. It carries ONE theme: **the first-video fix — video
start on Android should now behave like the Windows app** (ADR-0013):

1. **What was wrong (the evidence):** on Ali's debug.6 round the
   feed loaded fast but the FIRST video took 2–3 minutes to actually
   play, while the Windows app on the same network was instant. The
   root cause was a code-level asymmetry between the two doors, not
   the network: the Android TUN path aborted every flow whose door
   dial exceeded 10 s — but the door's own honest setup (secure-DNS
   resolve + the upstream dial, up to ~21 s when a googlevideo
   address is silently blackholed by the ISP) can legitimately take
   longer, and the desktop path has no such abort. Worse, the DNS
   answer is cached for at least 120 s, so every app retry repeated
   the SAME address walk into the SAME 10 s wall — the minutes-long
   wait was that retry loop running until the DNS cache rotated.
2. **What changed:** (a) the engine's dialer now fails over between
   addresses in ~0.75 s instead of 5 s per dead address (the same
   fix benefits the Windows/Linux browsers too — any flow whose
   first address is blackholed used to pay 5 s there as well); and
   (b) the Android tunnel's handshake budget now covers the door's
   full honest setup, so a flow is never aborted mid-setup — it
   either connects or fails with the same timing the desktop sees.
3. **What to verify on the phone (the row stays open until you
   confirm):** connect, open the YouTube app, tap the FIRST video
   right after the feed loads — it should start within a few
   seconds, like the Windows app. If any video still waits,
   Menu → Diagnostics → Copy and send the report with the phone
   model + Android version; the per-flow timing lines from debug.5
   name the phase that waits.

## 1c. v1.6.6-android-debug.6 — one UI on every device + data usage (SHIPPED — a regular release, superseded by debug.7)

> Note added at D93 (2026-10-18): the shared tree has since gained the
> desktop close ruling (closing the window is a real shutdown by
> default; tray-keeping is the opt-in setting) — nothing an Android
> build renders differently (no window close, no tray, the row is
> capability-gated off), so debug.6 stays the current APK; the ruling
> rides the next debug build whenever an app change next warrants one.

**What this build is:** versionCode 5, built at 078e3a5, APK
34,951,881 B (SHA-256 bc4b239e…d1fe8), the pinned debug certificate
— **installs directly over debug.5** in place; settings survive
(but not usage history — it starts fresh with this build's new
file). It carries ONE theme: **Ali's uploaded instruction rounds
(mobile.zip) applied to the one shared screen, so the phone, the
Windows desktops, the Linux editions and the web edition render the
same UI by construction** (D92):

1. **The round-2 audit fixes, everywhere:** real Lucide icons
   through one file (the power mark, the “…” menu button now at
   the top-right at 44 px, the pill's chevrons, the usage arrows,
   the sheet's X); the menu sheet has a proper header — title
   left, neutral 44 px close right — with the body scrolling
   underneath; the countdown digits sit in 0.80 em boxes so they
   can never touch; the connected ring family stops clear above
   the hint, which is bigger and brighter; the ping line gets a
   word (Good / OK / Slow); the About section carries “Created by
   ChitaCodes”; the honest Other-websites section exists; and the
   desktop window became 360×640, resizable, floored at 320×560.
2. **The round-3 data-usage feature, new on every platform:**
   while connected the screen shows **“X used”** with the ↓/↑
   split under the timer, counting up live; after a disconnect,
   **“Last session: size · length”** sits under Not Connected; the
   menu's new **Data usage** section shows Today / This month /
   All time, the last sessions, and a confirmed Clear history
   (Delete vs Keep — only Delete clears). The history is one
   local file — times and byte totals only, no website names,
   never sent anywhere — capped at 500 sessions, autosaved every
   15 s, and recovered after a crash or power loss. The live
   Mb/s row is off by default now (the total is what matters).

**Verified before shipping** (the reference code folder was not in
the upload, so the acceptance items ran the honest way): the full
test baseline green — 18 packages, twice, race detector on the
touched ones; the screen rendered and exercised at 320×560,
356×625 and 360×760 across every state with no overlap and no
scrollbars; the byte path proven on a real engine (a 949 KB
YouTube page through the tunnel = a recorded 980,303-byte session
in the history file after the stop).

**→ What to check on your devices, Ali:** install debug.6 on the
phone AND rebuild the desktops from this tree (build.bat / build.sh)
— the UIs should now match yours exactly; watch the data counter
climb while browsing, look at the Last-session line and the menu's
Data usage after a disconnect, and try Clear history (both
buttons). The video-start diagnostics ask from debug.5 still
stands when a video is slow.

## 1a. v1.6.6-android-debug.5 — the first device feedback answered (SHIPPED — a regular release, superseded by debug.6)

**What this build is:** versionCode 4, built at ad02980, APK
34,831,081 B (SHA-256 5df2acc3…02b4), the pinned debug certificate —
**installs directly over debug.3** in place; settings survive. It
carries TWO changes from the first real device round:

1. **The numbers-spacing fix (Ali's report: "the numbers in the app
   are too near each other").** Root cause: the connect screen's
   stage was fixed at 22.5 rem — right for a 360 px viewport at 1×
   font scale, but on narrower screens or with Android's font-scale
   setting it outgrew the screen and pushed the timer and the two
   speed values against the edges (measured: 495 px of stage in a
   320 px viewport at 1.4×). The stage now never exceeds the
   viewport, the two speed numbers carry wider spacing that never
   wraps, and the timer digits got more air. Verified at the worst
   case AND the canonical case before shipping (D91).
2. **The video-start instrumentation (Ali's report: feed refreshes
   fast, but playing a video takes 30 s–1 min+ to show).** The
   host-side suspects were each eliminated with evidence (D91 —
   including a new test that PINS the QUIC refusal contract: non-DNS
   UDP gets an immediate ICMP port-unreachable, so routed apps fall
   back to TCP fast instead of blackholing). What remains is
   device/network-side, so this build logs every tunnel's phases —
   DNS time, dial time, first-byte time, transfer totals — into the
   log ring the redacted diagnostics report carries.
   **→ When a video is slow on your phone: Menu → Diagnostics →
   Copy, and send it** (plus phone model, Android version, and
   whether it's every video or just the first after connecting).
   The next report names the phase that eats the time.

Also answered in the release body, no code change: the **Play
Protect popup** at install is expected for a sideloaded
debug-signed APK (Android being cautious about unknown
signatures — not a defect), and the **release keystore needs
nothing now** (keep it private; it is NOT used for the test circle —
signing a test build with it would break the installs-over-debug.3
update chain; it waits for the signed public release).

## 1. v1.6.6-android-debug.3 — the device test circle (SHIPPED — a regular release in the private repo, D89)

A debug-signed build for **Ali's own device testing** — "we're not
releasing yet" (D84): this is not a public release. Expect Android's
"built for testing" and Play Protect warnings for unknown developers;
both are expected and honest for this circle.

**The release flag (D89, 2026-10-17).** This build and the two
earlier debug builds are now published as regular releases, not
prereleases — Ali's ruling: "Do not prerelease, just release, the
repo is private." The repo is private, so the flag carried no public
meaning and the releases remain visible to the repo's collaborators
only. Nothing about the build changed: it is still the debug-signed
device-test build this page describes.

**The honest headline.** Younblock for Android helps YouTube load
when the network gets in the way. It routes **only the apps on its
list** — the YouTube app **and the Chrome, Firefox, Opera and Brave
browsers are added by default** (D87), and you add or remove apps in
the menu — through an on-device engine that uses secure DNS and TLS
record splitting, exactly like the desktop edition. It is **not a VPN
in the privacy sense**: it does not hide your IP address, adds no
encryption, and sends nothing to any Younblock-operated server (none
exists). It uses Android's VPN slot as a **local traffic-routing
interface** — that is the key icon you will see while it runs.

**What changed since debug.2** (Ali's ruling: "Add chrome to the
default addlist as well, also firefox, opera, brave browser"):

- **The four browsers join the YouTube app on the default list (D87)**
  — a fresh install routes the YouTube app and every installed one of
  Chrome, Firefox, Opera and Brave out of the box. A browser you
  don't have installed is simply skipped (it can never break the
  connect); install it later and it routes on the next connect.
- **A debug.2 install updates to the new default automatically**: its
  saved [YouTube]-only list was the shipped default of that build,
  never a hand-made choice, so the one-time v4 settings migration
  upgrades it to the five apps. A debug.1 install (born-empty list)
  migrates straight to the five apps. A list YOU edited (added or
  removed apps) is kept exactly as you left it — and the quick-add
  buttons now cover all five defaults, so regaining any of them is
  one tap.
- **Adding apps is still the D86 path, widened**: one-tap quick-add
  buttons for the known apps (YouTube, Chrome, Firefox, Opera,
  Brave) and friendly names in the type-in field — "youtube",
  "chrome", "firefox", "opera", "brave" all resolve; other apps
  still need the package name.
- Same debug certificate as debug.1/debug.2 (pinned in the repo, D85):
  this build **installs over them** as an update — no uninstall
  needed, settings survive.

**The artifact.** `app-debug.apk`, 35,816,233 B, SHA-256
`81515f0dd1b0f1ff2d3fb1a87769c8c6d4df3dc0225efeb975bd9f543257d6a1`;
applicationId `io.github.assadigit.younblock`, versionName 1.6.6,
versionCode 3; Android 8.0+ (minSdk 26), targets API 36; native code
for arm64-v8a, armeabi-v7a and x86_64; the same pinned debug
certificate as debug.1/debug.2 (SHA-256 digest `c76bb7c61ec623892eabc
37ac90ac1bc0041d39490f2d48073676d6e2ed8551d`, at
`android/app/debug.keystore` since D85), apksigner v2-scheme
verified; the four-permission manifest, no testOnly.

**Release-signing helper (added after the APK upload — M8-e / D88;
the APK itself is unchanged).** This release carries a second asset,
`make-release-keystore.bat` (11,527 B, SHA-256
`82e6dded40a8eb415127c6d46b3cb4acb0633ab2cd13ac8b809a8ae8c7c74090`,
pure ASCII, CRLF): the local one-double-click ceremony for the future
signed lane. Run it in a private folder on Windows and it generates
the keystore password **on your own machine** with Windows' crypto
RNG, creates `younblock-release.keystore` with every keytool question
pre-filled (RSA 4096, 30-year validity), verifies the result by
reading it back, and writes a `younblock-keystore-backup_<date>.md`
next to it — the password, the alias, the certificate serial number,
the SHA-256 fingerprint and the full keytool record. Not needed for
this debug build; it exists so the signed lane is ready the day you
decide to publish (doc 07 §8 / D88). The keystore and that backup
never go in the repo.

**The helper-fix maintenance release: v1.6.6-android-debug.4 (D90,
2026-10-17 — no new APK).** Ali's first run of the helper on his
machine stopped at password generation — "'powershell' is not
recognized as an internal or external command": his PATH variable
does not include PowerShell (Windows itself ships it; the desktop
setup scripts took the same fix at v0.7.5). The fixed
`make-release-keystore.bat` (13,683 B, SHA-256
`ee35e4565dadf2a4a1c6e150208b8749aa6e5c3b8aec6303cc57025c06939558`,
pure ASCII, CRLF) resolves PowerShell by its full standard install
path instead of through PATH, and if PowerShell cannot be found at
all the script's own fallback generator builds the password — the
ceremony always completes, and the .md backup records which
generator ran. debug.4 is a regular release (D89) carrying ONLY the
fixed helper: **the app itself is unchanged — keep using debug.3's
`app-debug.apk` above** (D84: identical bytes are never re-released;
debug.3's page carries the pointer note superseding its older helper
asset). Ali's "can't I use an online keyring generator instead?" is
answered NO (D90): the keystore is the app's signing identity —
whoever holds it and its password can sign updates as you — so it
is made once, on your own machine, and never passes through any
server, website or chat. The release body documents a one-line
manual keytool alternative for machines without PowerShell.

**Superseded: v1.6.6-android-debug.2** (versionCode 2, SHA-256
`9cca38c9…811b`) — the D86 build: it put the YouTube app on the
default list and fixed the entry path (quick-add chips + friendly
names); its [YouTube]-only default is what debug.3's v4 migration
upgrades. **Superseded: v1.6.6-android-debug.1** (versionCode 1,
SHA-256 `ccf46a2e…7b64`) — the first test-circle build; shipped with
the born-empty list (D83) whose entry-path gap is what debug.2 fixed.
Its release page carries the install-blocked troubleshooting guide
(D85), which still applies unchanged to downloads from Iranian
networks. Their release flags were switched to regular releases
together with debug.3's (D89, 2026-10-17); their bodies stay exactly
as published — history.

**Install (private repo, doc 07 §8).** Download the APK from this
release with your GitHub account, allow installs from your browser
app, accept the Play Protect warning, install. If debug.1 or debug.2
is already installed, this build updates it in place. Nothing
auto-starts; connecting is always a user action.

### If the install is blocked — read this first (D85)

Ali's first observation: "Android didn't let me install it." The
artifact was exonerated end-to-end first (the served bytes hash to
the recorded sha256; apksigner/badging/manifest clean; no testOnly;
zip intact) — and the asset's download_count was **0**, pointing at
an interrupted download as the probable cause. The ordered path now
lives on the release body itself:

1. **The download didn't complete** (most likely from Iranian
   networks — release assets are served from
   release-assets.githubusercontent.com, frequently slow/unreliable
   there). Check the file size: it must be **about 34 MB
   (35,816,233 bytes)** — a few-KB file is an error page, not the
   APK. Retry on Wi-Fi, or download on a PC and copy via USB (the
   most reliable route).
2. **"For your security, your phone is not allowed to install
   unknown apps from this source."** → Settings on the dialog →
   allow **Install unknown apps** for the opening app (Chrome /
   Files) → back → Install.
3. **Play Protect "Unsafe app blocked"** → More details →
   **Install anyway (unsafe)** — expected for a debug-signed unknown
   developer.
4. **"There was a problem parsing the package"** → usually also a
   truncated download; if the size is right, check the Android
   version — **8.0+ required**.
5. **"App not installed"** at the end → free storage; uninstall any
   older Younblock first.

If none match, the exact on-screen wording + Android version is the
feedback this circle needs.

### Device test circle — what to test and report

The build environment has no emulator and no device (verified — doc
09's standing blocker), so every device-side claim is **unclaimed
until observed on your device**. This circle is the checklist's
closure path; report pass/fail per row plus the in-app diagnostics
report (Menu → Diagnostics → Copy — **shipped in debug.9, ADR-0015;
before that build the path was Help's buried copy button**) and your
device model + Android version. **If anything "doesn't work", the
diagnostics report is the single most valuable thing to send** — it
carries the engine's own view of what happened, redacted for privacy:

**The debug.6 round (D92) adds two checks of its own, above the
rows below: (a) the cross-device UI match — install debug.6 on the
phone AND rebuild the Windows/Linux desktops from the same tree;
the screens should now be identical to Ali's instruction-following
builds (menu top-right "…", the sheet's header close, Lucide
icons, the usage lines); and (b) the data-usage feature — watch
"X used" climb while browsing, the Last-session line after a
disconnect, the menu's Today / This month / All time, and Clear
history with both buttons (Keep must keep, Delete must empty and
remove the Last-session line).**

1. **Install + first launch** — the sideload path works (the D85
   troubleshooting guide covers blocked installs); the app opens on
   the connect screen; rotation, font scaling and TalkBack behave;
   the sheet's Apps-to-unblock row renders **with the five default
   apps already on the list** (YouTube, Chrome, Firefox, Opera,
   Brave — whichever are installed).
   **Observed on Ali's device (debug.3, 2026-10-17): PASS with a
   note** — "it worked on android"; the install completed; a Play
   Protect popup appeared (the expected unknown-developer warning
   for a sideloaded debug APK — D91 documents it; no action). The
   font-scaling half arrived via the spacing complaint (the M6
   gate's first live data — fixed in debug.5); rotation and
   TalkBack remain unreported.
2. **First connect (the default list)** — the YouTube app and the
   installed default browsers route out of the box: the consent
   explanation → the system VPN dialog → Allow; the key icon
   appears; the notification honestly tracks connecting / active /
   degraded; a browser that is NOT installed is skipped without
   breaking anything. (If you emptied the list on purpose, the
   connect refuses with the honest NO_TARGET_APPS message instead.)
   **Observed on Ali's device (debug.3, 2026-10-17): PASS** — "the
   youtube app is routed through this system" and the feed
   refreshes fast while connected.
3. **The product's purpose** — YouTube actually loads in the
   YouTube app (browse + video playback) while connected; the
   self-test and recovery behave when YouTube stops responding.
   **Observed on Ali's device (debug.3, 2026-10-17): PARTIAL** —
   browse/feed fast; **video playback took 30 s–1 min+ to show**
   ("feed quickly refreshed but actual video playing delayed").
   **Re-observed on debug.6 (2026-10-18, M8-n): the delay GREW to
   2–3 minutes for the first video, while the Windows app on the
   same network is instant — that discriminator localized the defect
   to the Android-only layers, and the root cause was found IN THE
   CODE (ADR-0013): the TUN path aborted flows at 10 s, shorter than
   the door's own honest resolve+dial budget, and the sequential
   5 s-per-address dial walk against a 120 s-cached DNS answer turned
   every retry into the same wall — fixed in debug.7 (fast ~0.75 s
   address failover + the budget-true handshake timeout). The row
   STAYS OPEN for the debug.7 device confirmation: first video after
   connecting should start within seconds, like Windows; if not, the
   diagnostics report (Menu → Diagnostics → Copy) at the slow moment
   + phone model + Android version is the expected input.**

   **Re-observed on debug.7 (2026-10-18, M8-o): the stall moved past
   the fix — feed and UI instant, but EVERY video 1–2 minutes to
   start and re-buffering mid-play, Windows instant on the same ISP.
   The surviving amplifier was root-caused in the code (ADR-0014):
   racing cannot help when the WHOLE address set of a host is
   blackholed (rr*.googlevideo.com hostnames carry the few addresses
   of one cache node each), and the 120 s-minimum DNS cache kept
   serving the suspect set to every retry — fixed in debug.8 (a
   total dial failure demotes the cached answer to stale, so the
   next retry re-queries the servers; the resolver's own bootstrap
   dial races its addresses too). The row STAYS OPEN for the
   debug.8 device confirmation: every video, first and later,
   should start within seconds and play without re-buffering; if
   not, the diagnostics report's tunnel lines (dns=/dial=/
   first-byte=/dur=) + the on-screen speed number at the slow
   moment are the expected inputs.**

   **Re-observed on debug.9 (2026-10-18, M8-q): "I still dont see
   diagnostic button" — and the report was exact. The debug.9 build
   carried the ADR-0015 surface, but ONLY in the classic UI tree;
   the APK bakes the pro screen (D66), whose lone diagnostics was
   the M6-era section buried below Help with the WebView-silent
   clipboard copy. A surface shipped into a tree the edition never
   renders is indistinguishable from not shipped (D101). Still NO
   new stall evidence, so still NO root-cause attempt — debug.10
   ports the evidence path into the served variant (ADR-0016:
   Diagnostics is the FIRST row of the menu, opening the full report
   view — the ladder Copy, Share, the selectable text). The row
   STAYS OPEN: after the next stall, open the menu, tap the first
   row, Copy, paste the report here. Two pastes close this loop:
   one now (its build line proves the round), one at the stall.**

   **THE FIRST REPORT ARRIVED (debug.10, 2026-10-18, M8-r): Ali
   pasted the full diagnostics report — versionCode 9 on the build
   line, state degraded, last failed check dns (DNS_UNREACHABLE),
   "heal sweep found no working method" 12.4 s in, method legacy,
   dns ok 58 / fail 16, tun active flows=37 udpDropped=76 — and the
   log read back against the code EXACTLY (ADR-0017): a startup
   secure-DNS blip (three ~170 ms fast-failures) negative-cached
   the sweep's own probe name, every method then failed instantly
   on the 15-second memory, and a RECOVERED network was declared
   broken (degraded + "Couldn't fix it" notification) — reproduced
   on the host into the same signature (degraded, method legacy)
   and fixed in debug.11 (the sweep resets the DNS memory and
   probes from zero; a failed sweep restores the method; the DNS
   failure lines now carry server=#N reason=…). The stall itself
   now has its first real telemetry: five video flows that
   connected, handshook, and then moved ~7 KB in 51 seconds (the
   DPI throttling the direct path to specific googlevideo cache
   nodes) while OTHER flows completed fine on the same session
   (122 KB in ~1 s; 2.5 MB in ~7 s) — ISP-side interference, not an
   app-side defect this time; and udpDropped=76 (the YouTube app's
   QUIC attempts, dropped by design, row 9's measurement) adds
   per-video start latency while the app falls back to TCP. The row
   STAYS OPEN with a sharper question for the NEXT report: with
   debug.11's baked DNS reasons, does the startup blip read
   reason=timeout (filtered) or reason=network (transient) — and do
   the stalled flows correlate with specific cache nodes? No
   steering or policy change is made without Ali's ruling.**
   **THE SECOND REPORT ARRIVED (debug.11, 2026-10-18/19, M8-s): Ali
   pasted it with the round's key observation — "the problem
   persists, but when after a while, video plays, it can be easily
   rewinded and go next, which means the speed is okay, the routing
   is okay... only the problem is each video starting to play which
   lags considerably. IN pc, it's instant." The report PROVED the
   M8-r fixes on-device (state on, a sweep that found strong 3 s in,
   the startup DoH blip still logging its three baked
   reason=DNS_UNREACHABLE lines and self-healing — the sharpened
   question answered: a transient at Start, not a filter) and gave
   the stall its exact shape: video-start waves where some flows
   complete (first-byte 56-619 ms) while others dial fine, send the
   ~1.7 KB hello, and never receive one byte — ~0.5 s closes
   (origin reset) and EXACTLY ~60.0 s hangs, all with down=0 B.
   Nothing in the engine closes at 60 s (the idle timeout is 300 s,
   the TUN keepalive only probes) — that closer is the app's own
   patience, amplified by an engine that held the dead flow open;
   and because the dial SUCCEEDED, the M8-o demotion never fired, so
   the suspect address set stayed fresh for every retry. Fixed in
   debug.12 (ADR-0018, the first-byte budget: silent origins fail
   fast at 8 s, the reason lands in the report, and the suspect set
   is demoted like a total dial failure). The row STAYS OPEN for
   the debug.12 device confirmation with a SHARPENED question — and
   one for ALI himself (D102 material, his ruling, not our code):
   the sweep picks the method from its redirector.googlevideo.com
   probe, but the media flows hit rr*.googlevideo.com cache nodes
   whose tolerance differs per node — this report ran method strong
   while his PC is instant on the same network. Should the engine
   retry a dead routed flow once with the NEXT preset
   (balanced→strong→light→legacy, per-flow method variation)?
   That is a method-policy change: no steering without his ruling.
   The next report's expected shape: "tunnel first-byte failed
   reason=…" lines and dur≈8000ms down=0 B dones.
   **THE THIRD REPORT ARRIVED (debug.12, 2026-10-18/19, M8-t): Ali
   pasted it with "The problem persists" (versionCode 11 verified).
   The expected shape ARRIVED exactly: 22 reason=timeout dones at
   ~8.3 s (the budget firing) + 18 reason=reset dones at ~0.5 s —
   and the counters named the remaining lag: 38 errors / 61 conns —
   roughly half the video-start waves die at first byte, and the
   app re-opens wave after wave (~35 s from the first wave to the
   first video bytes, down=4.4 MB once a flow survived). The
   tuning evidence ADR-0018 asked for: every budget-killed flow was
   100 % silent (down=0 B — dead nodes, not slow ones; the previous
   report showed the same flows silent for the full 60 s), while
   healthy routed origins answered in 420–508 ms (one 2399 ms
   outlier carrying the fragment-pause write). Fixed in debug.13
   (ADR-0019): FirstByteTimeout 8 s → 2.5 s, plus the salvage
   re-dial — a dead first-byte origin is re-dialed INSIDE the same
   flow (same host, SAME preset, the client's buffered bytes
   replayed byte-exact, up to 2 re-dials), so the app never sees
   the failure and its serial retry chain collapses into bounded
   in-engine attempts; dead-IP exclusion steers re-dials off the
   corpses; new report lines "tunnel first-byte retry reason=…
   attempt=N" (absorbed) and "tunnel first-byte salvaged dials=N"
   (saved). The row STAYS OPEN for the debug.13 confirmation —
   and the D102 question for Ali SHARPENS: the salvage re-flips the
   same coin (same preset) up to twice more per flow; if the next
   report still shows terminal first-byte failures clustering under
   one method, the NEXT-preset retry (per-flow method variation)
   remains his ruling to make, never unilateral. udpDropped=88
   (row 9) keeps measuring; the QUIC path already fast-fails via
   ICMP port-unreachable, so it is not a startup-lag contributor.)
   **THE FOURTH REPORT ARRIVED (2026-10-19, early — M8-u, a
   NO-BUILD round): "The problem persists" again — but this report
   is from the PREVIOUS build. Its "android shell: 1.6.6
   (versionCode 11)" line names debug.12 (the identity is read from
   Android's PackageManager at runtime, so it names the installed
   build); every timeout done sits at 8054–8070 ms — the old 8 s
   budget, which the 2.5 s build cannot produce; not one retry/
   salvaged line in the 200-line window; and the release clock
   closes the case: debug.13 published 23:16:28 UTC, the report
   generated 23:41:37 UTC — the phone never received the fix. The
   report is the third report's signature measured again (21
   timeouts + 17 resets at 64–527 ms, 67 errors / 114 conns, ~24
   healthy first-bytes at 58–548 ms, DNS ok 118 / fail 0,
   udpDropped=107 — the storm still costing ~80 s to first video
   data). No new build: zero bytes of the report describe the
   salvage build's behavior, so there is nothing to tune (D84). The
   row STAYS OPEN, unchanged, for the ACTUAL debug.13 report —
   first check: the shell line reads versionCode 12; the APK filename
   itself now names the build (D103): download v1.6.6-debug13.apk —
   if the filename says anything else, it is the wrong build.)
   **THE FIFTH REPORT ARRIVED (2026-10-19, later — M8-v): "The
   problem persists" — and this time it IS the salvage build (shell
   line versionCode 12, retry/salvaged lines present in the log).
   The salvage is PROVEN on-device: retry lines absorbing dead
   attempts the app never saw, "tunnel first-byte salvaged dials=N"
   saving flows at 0.7–1.4 s, errors down to 37/81 ≈ 46 % (from 59 %).
   The report names the two REMAINING start-lag sources: (1) the
   serial salvage chain — 21 terminal reason=timeout dones at
   8014–8227 ms = three 2.5 s budgets plus dials walked one after
   another (+ 11 terminal resets at 0.6–1.9 s); (2) the mid-stream
   stall — five video-start flows done in the same instant at
   00:18:17.2, dur=57290–64620 ms, up=2.4–5.0 KB, down=6.6–7.7 KB
   (one 121.1 KB): first byte plus ~7 KB, then TOTAL silence until
   the app's own ~60 s patience killed them all — and the retry
   wave right after answered in 60–261 ms first-byte (the video
   took ~65 s to start). Fixed in debug.14 (ADR-0020): the salvage
   RACE (all remaining re-dials launched in parallel on the rotated
   live set, first speaker wins, a silent-host terminal costs ~2x
   the budget, never 3x) + the stall watch (both directions silent
   for StallTimeout=15 s closes the flow engine-side, the answer set
   demoted, new report line "tunnel stall killed idle=Nms", so the
   app retries onto a fresh origin in seconds instead of a minute;
   one-direction silence — a pure download or upload — never fires).
   The row STAYS OPEN for the debug.14 confirmation with TWO named
   evidence lines: the "tunnel stall killed" lines (how many fire,
   and whether starts still stall when they do not) and the
   retry/salvaged-vs-terminal ratio. The D102 question for Ali is
   now LIVE with this report's evidence: terminal first-byte
   failures STILL cluster under method=strong (21 timeouts + 11
   resets, ~40 % of conns) — whether a flow that exhausts its raced
   salvages should be retried once with the NEXT preset remains his
   ruling, never unilateral. udpDropped=78 (row 9) keeps measuring.)
   **THE SIXTH REPORT ARRIVED (2026-10-19, late — M8-w): "The
   problem persisted" — and it IS the salvage-race + stall-watch
   build (shell line versionCode 13, the retry lines batching at
   the same millisecond, the stall lines present). ADR-0020 is
   PROVEN on-device: "tunnel stall killed idle=15059-15315 ms"
   kills at exactly the 15 s budget (the 57-64 s mid-stream hangs
   are GONE — the watch closed every one of this report's silent
   relays at the budget), the raced re-dials save at 738-848 ms,
   and the report's error rate (69/124, a dirty window it caught
   mid-flight) no longer contains a single 60 s done. The report
   names the two REMAINING costs: (1) the SERIAL FRONT — six
   terminal reason=timeout dones at 5320-5576 ms (attempt 0 walked
   its 2.5 s budget ALONE before the race launched: the
   two-budget chain) and reset terminals at 1131-1292 ms, while
   CONCURRENT flows to the SAME hosts answered at 72-162 ms in the
   same seconds (five sub-250 ms first-bytes at 01:09:49-50 right
   between two timeout terminals) — the dirt is per-origin, not
   per-moment; (2) the DNS degraded state — dns fail 16 with
   "last failed check: dns (DNS_UNREACHABLE)" while the network
   flowed (6.7 MB down): a dirty-window DoH blip negative-cached
   the probe name and the periodic check read the 15 s memory
   instead of asking any server (the M8-r lesson recurring on the
   watchdog path). Fixed in debug.15 (ADR-0021): the FIRST-DIAL
   RACE (the primary plus FirstByteRaceDials=2 extra candidates
   dial in parallel from the flow's start, the first record
   replayed byte-exact through each, first speaker wins, an
   all-dead set terminals at ~1 budget — new report line "tunnel
   first-byte raced dials=N", a clean primary stays quiet) + the
   honest dns probe (ClearNegative before the dns step: the probe
   asks the servers NOW; success entries stay). The row STAYS OPEN
   for the debug.15 confirmation with THREE named evidence lines:
   the "tunnel first-byte raced" lines (the save rate — the
   candidate count's tuning material), the terminal ratio (the
   D102 material), and the dns fail counter (does the degraded
   state still fire?). udpDropped=104 (row 9) keeps measuring.)
   **THE SEVENTH REPORT ARRIVED (2026-10-19, later still — M8-x):
   "I think the problem is WORSE now, because videos used to have
   lag for starting to play, but after that you could easily go
   next (watch next few minutes) instantly without buffer, but now
   even going next and back in the same video lags a lot" — and
   the shell line STILL read versionCode 13: the phone NEVER RAN
   debug.15 (the D103 accident class proven real; the wrong-build
   guard this time is the filename itself: download
   v1.6.6-debug16.apk). The report's log is the M8-v stall watch's
   smoking gun and the first user-visible REGRESSION of the loop:
   thirteen "tunnel stall killed idle=15xxx ms" lines in ~70 s,
   every one a connection that had ALREADY carried real data —
   dur=17264 ms down=2.1 MB, dur=16756 ms down=1.6 MB, 723.6 KB,
   633.7 KB, 162.6 KB, plus 20.9/23.4/9.2/7.4/5.3/4.4/3.6/2.8 KB —
   the player's POOLED video connections (buffer, quiet past 15 s,
   next range on the SAME connection) murdered at exactly the lean
   budget mid-pacing, each kill ALSO demoting a proven-good answer
   set and counting an error (errors 70/122 ≈ 57 %; a fresh dial
   cost reset ~1.2 s / timeout ~5.4 s — the pre-watch builds'
   instant next/seek was exactly the pooled reuse the watch was
   killing). Fixed in debug.16 (ADR-0022, the TIERED STALL WATCH):
   a flow that proved itself — ≥ StallRichBytes=32 KB delivered
   down, or one ≥ 3 s quiet gap that closed with fresh data (the
   pacing proof; the shapes overlap in bytes but differ in RHYTHM:
   the debug.13 dead cohort never woke) — is closed only at
   StallRichTimeout=90 s and QUIETLY (no demotion, no error count;
   the stall line now carries "down=N tier=rich|lean"), while the
   lean tier keeps the exact M8-v death for the never-proved shape.
   The build carries debug.15's never-run cargo forward unchanged.
   The row STAYS OPEN for the debug.16 confirmation with FOUR named
   evidence lines: the tier= split on the stall lines (rich kills
   rare and harmless, lean kills the dead cohort only), the raced
   lines (the save rate), the terminal ratio (the D102 material —
   now clustered under balanced too: the sweep dragged the method
   after DNS false-failures), and the dns fail counter. And the
   FIRST check of all: the shell line reads versionCode 15 —
   anything else is the install, not the fix.)
4. **youtube.com in a browser** — Chrome, Firefox, Opera and Brave
   are on the default list now (D87): youtube.com loads inside each
   installed one of them after connecting. For a browser you removed
   from the list, tap its **+** chip in the menu, reconnect — this
   row also re-tests the one-tap add path.
5. **.ir stays direct** — with a routed app (e.g. one of the default
   browsers), any `.ir` site loads as if Younblock were not there: no
   unblocking, no fragmentation, always direct (D83/D84's ruling;
   their name lookups still use your configured secure-DNS
   providers — the stated limit).
6. **Stop paths** — disconnect from the app; disconnect from the
   system's VPN settings (onRevoke); reboot (nothing self-starts);
   reconnect works.
7. **Persistence + battery** — screen-off/Doze behavior while
   connected; how the notification channel behaves; the battery
   feel vs. the notification-disclosed budget.
8. **Network switch** — Wi-Fi ↔ mobile data while connected.
9. **QUIC fallback** — whether the YouTube app falls back from
   QUIC/UDP to TCP as expected (UDP beyond DNS is dropped by
   design; routed apps must fall back, never silently leak).
   **Host half PINNED by test (D91)**: the stack answers non-DNS
   UDP (443) with an immediate ICMP port-unreachable
   (TestUDP443FastFailICMP) — the phone kernel turns that into an
   instant refusal on the app's socket, the fast-fail path. The
   on-device observation (does HIS YouTube app actually fall back
   fast) rides with the video-start timing data.

## 2. The public v1 DRAFT (UNPUBLISHED)

Nothing here may be published until the ⛔ device gates (§"Open
before release" below) close or Ali explicitly accepts shipping with
them recorded as known limitations. M8 performed the final copy
review against the shipped debug artifact.

### What it does

- Routes the TCP traffic of the apps on the allow-list (the YouTube
  app and the Chrome, Firefox, Opera and Brave browsers are the
  default entries — D87; entries whose apps are not installed are
  skipped at establish; the list is editable in-app,
  changes apply on the next connect; an emptied list or
  all-uninstalled entries refuse the connect with the honest
  reason).
- Resolves the routed apps' DNS through the secure-DNS (DoH) providers
  you configure, tried in order (defaults: RethinkDNS, Cloudflare,
  Google — matching internal/config's defaultDoHServers; this line
  previously named a stale set and was corrected at D86).
- Splits the first TLS record for the routed YouTube domains into
  smaller pieces — the same fragmentation the desktop edition uses.
- Leaves every website ending in .ir completely untouched — no
  unblocking, no fragmentation: always a direct connection, exactly as
  if Younblock were not running (D83; their name lookups still use
  your configured secure-DNS providers).
- Keeps a persistent, honest status notification while routing: active
  only when traffic actually routes, degraded when partially working,
  connecting otherwise.
- Carries the desktop edition's self-test, automatic recovery, and
  redacted diagnostics report.

### What it does NOT (the truthful-claims review, doc 07 §1)

- No IP hiding, no added encryption — YouTube's own HTTPS protects the
  content.
- No remote VPN server; nothing leaves the device except the routed
  apps' own traffic and their DNS queries to your configured DoH
  providers.
- No analytics, no crash reports, no account, no data collection.
- No claim to work with "every app": only the allow-listed apps' TCP
  and DNS are processed; other apps and protocols are untouched.

### Scope and limitations (doc 02 §4 — the release-notes obligations)

- **IPv4 only for DNS resolution**: AAAA records are answered empty so
  routed apps stay on IPv4 (the desktop default). IPv6 traffic from
  routed apps is routed into the tunnel and dropped (fail-safe —
  visible breakage for that flow, never a silent leak past the engine).
- **UDP beyond DNS is dropped**: routed apps fall back to TCP. The
  YouTube app's QUIC-to-TCP fallback behavior is a device gate (§Open)
  — reported when measured, not promised here.
- **Supported path**: launchable Android apps over TCP (added by you;
  the tested circle is Chrome and the YouTube app), with the
  configured DoH providers. No support claims beyond the per-app
  allow-list and the tested matrix.
- **.ir websites are always direct** (D83) — never routed, fragmented
  or redirected; non-launchable packages cannot be verified and are
  refused as not-installed (the launcher-visibility limit, doc 07 §6).
- **Per-app list changes apply on the next connect** (the list is
  baked into the TUN at establish).
- **Nothing auto-starts after reboot** (AND-028): connecting is always
  a user action.
- Requires Android 8.0+ (minSdk 26); targets Android 16 (API 36).
- Distributed as a sideloaded APK — debug-signed for the current test
  circle (D84); release signing per doc 07 §8 when a public release
  happens.

### Open before release (⛔ — the device gates)

The build environment cannot run them (no /dev/kvm, no device — doc
09's standing blocker); **never claimed, never faked**. Since D84
they have their closure path — Ali's own device, through the debug
test circle above; his observations become the recorded evidence:

- The live consent dialog flow, notification channel behavior, and
  screen-off/Doze persistence.
- Real DoH resolution through the full stack; the YouTube app's actual
  QUIC→TCP fallback; protect() on real sockets; loop prevention
  on-device.
- Network switching on a real radio; stop from system VPN settings
  (onRevoke); reboot (nothing self-starts).
- Live WebView observation: rotation, font scaling, TalkBack, the
  Apps-to-unblock row on a real screen.
- Battery/behavior measurement (the notification-disclosed budget).

### Install (doc 07 §8)

Sideload only: allow installs from the chosen source, accept the
Play Protect warning for unknown developers, install the APK. When
the signed lane returns: losing the release keystore means
uninstall-before-update for everyone who installed the signed build
(README documents backup steps).

### Open-source components

gVisor netstack (Apache-2.0), gomobile (BSD-3), and other permissive
Go libraries — the full audited set in doc 07 §7 (re-verified against
the final artifact at M8). Attribution in-app: the About sheet.
