# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-x (2026-10-19, later still — **Ali's SEVENTH
pasted report round: "I think the problem is worse now, because
videos used to have lag for starting to play, but after that you
could easily go next (watch next few minutes) instantly without
buffer, but now even going next and back in the same video lags a
lot" + a report generated 2026-10-19T11:43:34Z whose "android
shell: 1.6.6 (versionCode 13)" line STILL names debug.14 — the
device never ran debug.15. The report's log is the M8-v stall
watch's smoking gun: thirteen "tunnel stall killed idle=15xxx ms"
lines in ~70 s, every one a connection that had ALREADY carried
real data (dur=17264 down=2.1 MB, dur=16756 down=1.6 MB, 723.6 KB,
633.7 KB, 162.6 KB, plus 20.9/23.4/9.2/7.4/5.3/4.4/3.6/2.8 KB) —
the player's POOLED video connections murdered at exactly the
lean 15 s budget mid-pacing, each kill also demoting a proven-good
answer set and counting an error (errors 70/122 ≈ 57 %; the same
log shows a fresh dial's cost: reset terminals ~1.2 s, timeout
terminals ~5.4 s). The method line flipped to balanced (the heal
sweep settling after DNS false-failures — the M8-w ClearNegative
attacks the cause). The round shipped ADR-0022 (the tiered stall
watch: rich = ≥32 KB payload or one survived ≥3 s idle gap →
closed only at 90 s, quietly — no demotion, no error; lean keeps
the exact M8-v death; the stall line now carries "down=N
tier=rich|lean") as v1.6.6-android-debug.16, versionCode 15, asset
v1.6.6-debug16.apk — carrying debug.15's never-run cargo (the
first-dial race + the honest dns probe) forward.**
Earlier: M8-w (ADR-0021, the first-dial race + honest dns probe,
debug.15 — never run on-device), M8-v (ADR-0020, the salvage race +
the stall watch, debug.14), M8-u (D103, the no-build round), M8-t
(ADR-0019, the salvage re-dial, debug.13), M8-s (ADR-0018, the
first-byte budget, debug.12), M8-r (ADR-0017, the sweep probes from
zero, debug.11), M8-q/p (the diagnostics surface, ADR-0015/0016 +
D100/D101), M8-o/n (the stall fixes, ADR-0014/0013 + D99/D98), A0
rounds — **the account gate is CANCELLED (D97), nothing of it was
ever built.** This file is the live handoff; keep the fields in
§"Current handoff fields" accurate at every session boundary.

## If starting Phase 2 (implementation) — CURRENT

1. Read every spec file; they are now evidence-backed, not baseline
   templates. The audit (doc 01) and ADRs (doc 08) are the ground
   truth; verify anything that looks stale against the repository.
2. Inspect the working tree; preserve unrelated/uncommitted changes.
3. Begin at **M3** (doc 05) — the Android toolchain first, then the
   Go-side additions (platform adapter, SOCKS5 door, mobile facade)
   which are host-testable even without the SDK.
4. Small changes; run the relevant tests after each; keep the desktop
   baseline (doc 01 §7) green on shared-tree edits.
5. Stop and document (ADR update) if a critical assumption fails; do
   not fake functionality or silently narrow the product scope.
6. Build the debug APK once the approved path is implementable; verify
   its actual existence/metadata; report tests honestly.
7. Update the specs with implementation reality, exact commands,
   artifact path/checksum, and commit/push status.
8. Keep the worklog discipline: read `/home/z/my-project/worklog.md`
   first; append your entry (Task ID + template) when done.

## Phase 1 record (completed 2026-10-16)

- Full source audit with symbols and line references (doc 01).
- Feasibility matrix scored; architecture accepted (docs 03, 08).
- Baseline green: go vet clean; go test 13/13; svelte-check 0/0;
  vite build clean; Android compile matrix 4/4 ABIs.
- Documentation-only phase: **zero application code changed**; the
  only repository additions are the completed specs themselves (and
  the extraction of this pack from the zip Ali uploaded).

## Environment / execution honesty (updated 2026-10-16, end of M3)

- **The Android toolchain is installed and working now**: JDK 21.0.8
  (Temurin, with javac), Android SDK (cmdline-tools 12.0,
  platform-tools, platforms 26+36, build-tools 36.1.0), NDK
  28.2.13676358, gomobile + gobind (x/mobile v0.0.0-20261011034528
  via a `go get -tool` directive), Gradle 9.8.1 via the committed
  wrapper. `dl.google.com`, `services.gradle.org`, `repo1.maven.org`,
  `proxy.golang.org` and the Adoptium GitHub release were all reachable
  and used this session.
- **What is still ⛔ is runtime verification**: this sandbox has no
  /dev/kvm (verified: `ls /dev/kvm` → No such file or directory), so no
  emulator, and no physical device. The Kotlin-level init/health/stop
  bridge call and every M4 on-device gate wait for a KVM-capable host
  or a real device. The bridge itself is proven at the Go level
  (mobile tests) and its Kotlin side is written and compiled into the
  verified APK.
- The wails `-m` rule (D68) still applies to desktop lanes; the
  Android lane never invokes wails. The sandbox still kills background
  processes when a Bash command ends; long builds ran as foreground
  chunks (go/gradle caches carried the progress).
- Do not claim source inspection, test success, APK creation, upload,
  or push without evidence. Do not commit secrets, signing keys, user
  settings, or private diagnostics. Do not undo unrelated working-tree
  changes to achieve a clean diff.

## Current handoff fields

- **Phase completed:** M8-x (2026-10-19, later still — the TWENTY-FIRST
  feedback-loop exchange; the SEVENTH pasted-report round). Ali's
  input: "I think the problem is worse now, because videos used to
  have lag for starting to play, but after that you could easily go
  next (watch next few minutes) instantly without buffer, but now
  even going next and back in the same video lags a lot" + a report
  generated 2026-10-19T11:43:34Z whose "android shell: 1.6.6
  (versionCode 13)" line STILL names debug.14 — the device never ran
  debug.15. The report's log root-caused the regression exactly:
  thirteen "tunnel stall killed idle=15xxx ms" lines in ~70 s, every
  one a connection that had ALREADY carried real data (2.1 MB,
  1.6 MB, 723.6 KB, 633.7 KB, 162.6 KB, plus 20.9/23.4/9.2/7.4/
  5.3/4.4/3.6/2.8 KB) — the player's POOLED video connections
  (buffer, quiet past 15 s, next range on the SAME connection)
  murdered at exactly the lean M8-v budget mid-pacing, each kill
  ALSO demoting a proven-good answer set and counting an error
  (errors 70/122 ≈ 57 %; fresh dials cost reset ~1.2 s / timeout
  ~5.4 s). Counters otherwise: dns ok 176 / fail 4 (down from
  16 — the sixth report's ClearNegative target), rstUnmapped=0,
  doorFailures=0, udpDropped=73, method balanced (was auto→strong:
  the heal sweep settled after DNS false-failures).
- **What this round shipped — v1.6.6-android-debug.16, versionCode
  15, ADR-0022 (doc 08), the CURRENT build:** the TIERED STALL
  WATCH. A flow PROVES itself in either of two ways — payload
  (flow.down ≥ StallRichBytes, default 32 KB: past handshake +
  headers into real payload; the murdered cohort started at 162.6 KB,
  the debug.13 dead cohort topped out ~7 KB) — or pacing (the watch
  observes an activity gap ≥ StallTimeout/5 = 3 s that CLOSED: fresh
  bytes flowed after it — the pooled-connection rhythm; the shapes
  overlap in bytes but differ in RHYTHM: the dead cohort never woke).
  A proved flow's stall budget escalates to StallRichTimeout
  (default 90 s — well past every real pacing gap, well under the
  300 s idle timeout) and its close is QUIET: both ends closed but
  NO answer-set demotion (an origin that just carried megabytes is
  the opposite of suspect) and NO error count. The LEAN tier keeps
  the exact M8-v semantics (demotion + error + the fast 15 s close)
  for the never-proved shape. The stall line is now "tunnel stall
  killed idle=Nms down=N tier=rich|lean" (the prefix stable — the
  next report's primary tuning evidence). Knobs StallRichTimeout
  (negative or ≤ StallTimeout → no escalation, the exact M8-v
  shape) and StallRichBytes (negative → only a survived gap
  escalates), wired through Config normalization + the engine.
  The build carries debug.15's never-run cargo UNCHANGED: the
  first-dial race (ADR-0021) and the honest dns probe
  (ClearNegative) — the next report must carry BOTH signatures
  (raced lines + tier lines). Both-ways proof: with the tiering
  stashed, TestStallRichFlowSurvivesLeanBudget FAILS (the payload
  flow closed at the lean budget — the regression reproduced in
  test) and TestStallWakeEscalatesBudget FAILS (the pacing flow
  closed at lean right after its second flight); the lean guards
  fail only on the new line format pre-fix (behavior identical both
  ways); with the fix 4/4 new + 4/4 M8-v pins PASS. Lane evidence:
  baseline green (vet, 18/18 ×2, Windows exit 0, matrix 4/4 + arm64
  full tree), gomobile bind with the tier= string verified in
  libgojni.so in all three ABIs BEFORE gradle, CLEAN assembleDebug;
  APK 35,235,345 B, SHA-256
  e107e337245b89e048e7025e1a6daaab083a9da170d8660f98b55874bdddac5f,
  apksigner OK (the pinned c76bb7c6...8551d), badging versionCode
  15 / versionName 1.6.6 / minSdk 26 / targetSdk 36; the tier= +
  raced + stall strings verified in the APK's libgojni.so in all
  three ABIs.
- **Current architecture decision / confidence:** ADR-0022 (doc
  08) — the tiered stall watch, extending the ADR-0018..0021 species
  (failure detection, hygiene, bounded recovery; never steering).
  Live ADRs: ADR-0001..0022. The auth lane stays CLOSED (D97).
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.16 round** — the report whose shell line
  reads **versionCode 15** (13 or 14 means the old APK is still
  installed: the D103 filename v1.6.6-debug16.apk makes the right
  build self-identifying — this round's report proved the accident
  class is REAL): (a) going next and back in the same video —
  instant again? (that is this build's whole point); (b) the stall
  lines' tier split — "tier=rich" kills should be rare and harmless,
  "tier=lean" kills the dead-origin cohort only (their counts tune
  32 KB / 90 s / the wake gap); (c) the "tunnel first-byte raced
  dials=N" lines (debug.15's never-run race — the save rate is the
  candidate count's tuning material); (d) the dns fail count and the
  method line (does the degraded state still fire; did the sweep
  stop dragging the method); (e) the D102 question for Ali himself,
  still LIVE: terminal first-byte failures cluster under both strong
  and balanced now — should a flow that exhausts its raced round be
  retried once with the NEXT preset in HealSweepOrder? His ruling,
  never unilateral. Plus the older circle rows (the cross-device UI
  verdict on debug.6, rows 4-8, rotation/TalkBack, the data-usage
  round, D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-y — ingest Ali's results from v1.6.6-android-debug.16 (the
  versionCode 15 build)**: the pasted diagnostics report FIRST —
  verify the "android shell:" line names versionCode 15 (13 or 14
  means the install never happened: ask which FILENAME was
  downloaded — the D103 name v1.6.6-debug16.apk makes the wrong
  build self-identifying); then read the tier= split on the stall
  lines (the tuning material), the raced lines (the save rate), the
  terminal ratios (the D102 material) and the dur= dones to name
  the remaining waiting phase; root-cause from the evidence; fix at
  the correct layer; keep the D77-D103 honesty rules binding (D101:
  any new user-facing surface ships in the variant(s) the affected
  editions bake, verified through the real webui server). Bump
  `versionCode` and cut `v1.6.6-android-debug.17` through the same
  regular-release lane (D89) WHEN an app code change warrants it —
  D98: every release page carries the debug APK asset; D103
  (amending D99): the asset is NAMED v<versionName>-debug<N>.apk,
  N = the tag's build sequence (debug.17's asset is
  v1.6.6-debug17.apk) — push the code commit BEFORE creating the
  release (the full-SHA target_commitish 422s against an unpushed
  SHA). The auth Task IDs (A1/A2/A3) stay retired (D97). The
  dormant alternatives when no feedback arrives: the desktop
  release lane or public-release engineering — ask Ali which, do
  not guess.
