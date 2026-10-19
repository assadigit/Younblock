# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-w (2026-10-19, late — **Ali's SIXTH pasted
report round: "The problem persisted" + a report generated
2026-10-19T01:10:10Z whose "android shell: 1.6.6 (versionCode 13)"
line names debug.14 — the salvage-race + stall-watch build — the
FIRST report from that build itself. The report PROVED ADR-0020
on-device: the "tunnel first-byte retry" lines batch at the same
millisecond (the re-dials race as designed), "tunnel stall killed
idle=15059-15315 ms" lines kill exactly at the 15 s budget (the
60-second mid-stream hangs are gone), saves land at 738-848 ms —
and it named the two remaining costs with new precision: (1) the
SERIAL FRONT — six terminal reason=timeout dones at 5320-5576 ms
(attempt 0 still walked its 2.5 s budget ALONE before the race
launched) while CONCURRENT flows to the SAME hosts answered at
72-162 ms in the same seconds (the dirt is per-origin, not
per-moment); (2) the DNS degraded state — dns fail 16 with
DNS_UNREACHABLE while the network flowed (a dirty-window DoH blip
negative-cached the probe name; the periodic check read the 15 s
memory). The round shipped ADR-0021 (the first-dial race + the
honest dns probe) as v1.6.6-android-debug.15, versionCode 14,
asset v1.6.6-debug15.apk.**
Earlier: M8-v (ADR-0020, the salvage race + the stall watch,
debug.14), M8-u (D103, the no-build round), M8-t (ADR-0019, the
salvage re-dial, debug.13), M8-s (ADR-0018, the first-byte budget,
debug.12), M8-r (ADR-0017, the sweep probes from zero, debug.11),
M8-q/p (the diagnostics surface, ADR-0015/0016 + D100/D101),
M8-o/n (the stall fixes, ADR-0014/0013 + D99/D98), A0 rounds —
**the account gate is CANCELLED (D97), nothing of it was ever
built.** This file is the live handoff; keep the fields in
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

- **Phase completed:** M8-w (2026-10-19, late — the TWENTIETH
  feedback-loop exchange; the SIXTH pasted-report round, the first
  from the salvage-race + stall-watch build). Ali's input: "The
  problem persisted" + a report whose "android shell: 1.6.6
  (versionCode 13)" line names debug.14, and whose log carries that
  build's own lines. The report PROVED ADR-0020 on-device (the retry
  lines batch at the same millisecond — the re-dials race; "tunnel
  stall killed idle=15059-15315 ms" — the watch kills at exactly the
  15 s budget; saves at 738-848 ms; the 60-second hangs are gone)
  and named the two remaining costs: (1) the SERIAL FRONT — six
  terminal reason=timeout dones at 5320-5576 ms (the two-budget
  chain: attempt 0 walked its budget ALONE, then the race) and reset
  terminals at 1131-1292 ms, while CONCURRENT flows to the SAME
  hosts answered at 72-162 ms in the same seconds (five sub-250 ms
  first-bytes right between two timeout terminals — the dirt is
  per-origin, not per-moment); (2) the DNS degraded state — dns
  fail 16 / DNS_UNREACHABLE with the network flowing: the periodic
  check read the 15 s negative cache its own app flows had written.
  Counters otherwise healthy: 124 conns / 69 errors (≈56 %, the
  dirty window this report caught), rstUnmapped=0, doorFailures=0,
  udpDropped=104.
- **What this round shipped — v1.6.6-android-debug.15, versionCode
  14, ADR-0021 (doc 08), the CURRENT build:** (1) the FIRST-DIAL
  RACE — new knob FirstByteRaceDials (default 2; 0 → default,
  negative → disabled, the exact M8-v shape): a processed host's
  budgeted flow races its primary origin against 2 extra candidates
  FROM THE FLOW'S START (each candidate dials its rotated start of
  the fresh answer set, replays the client's first record
  byte-exact through a fresh fragment conn — the same preset, once
  per flow, D102 — and waits one budget; the primary's dial time is
  its head start), the FIRST origin to speak wins, the losers are
  closed and reaped, an all-dead set terminals at ~1 budget instead
  of the M8-v 2x, and a flow the race SAVED logs the new report
  line "tunnel first-byte raced dials=N" (a clean primary stays
  quiet — the race costs a healthy flow nothing observable); the
  client stays frozen for the round (the M8-v salvage race's own
  proven semantics — bytes beyond the first record wait in the
  socket and flow to the winner in order); a candidate's death is
  classified before it demotes (teardown-caused deaths never demote;
  a budget-timeout or reset demotes whenever it happens — the M8-o
  loop); with racing enabled the salvage rounds never run (the
  budget is spent at the start). (2) The HONEST DNS PROBE —
  Resolver.ClearNegative drops exactly the probe name's
  negative-cache entry before the health lookup (success entries
  stay; optional capability; the heal sweep keeps its full Reset) —
  the watchdog's dns step now asks what the servers say NOW instead
  of reading the 15 s memory (the debug.14 degraded state's
  signature: DNS_UNREACHABLE in 0 ms). Both-ways proof: with the
  raced round neutered, TestFirstDialRaceSavesDirtyPrimary and
  TestFirstDialRaceTerminalIsBounded FAIL; with the ClearNegative
  call neutered, TestSelfTestDNSProbesPastNegativeCache FAILS; with
  the fixes all PASS; guards pass both ways (byte-exactness, the
  quiet primary, the unrouted host, the disabled knob). Lane
  evidence: baseline green (vet, 18/18), compile matrix 4/4 + arm64
  full tree, gomobile bind with the raced string in all three ABIs;
  one lane defect caught by the artifact check itself (the first
  assembleDebug packed the STALE libs/younblock.aar — the raced
  string absent from the APK — plus 12 MB of incremental-repack zip
  slack; the re-bind into android/app/libs/ + a CLEAN assembleDebug
  produced the tight artifact); APK 35,233,585 B, SHA-256
  817c5e2f7f553afcfc65cc684abfecb610c39c84ffdd875c0e3c169fc21ef68e,
  apksigner OK (the pinned c76bb7c6...8551d), badging versionCode
  14 / versionName 1.6.6 / minSdk 26 / targetSdk 36; the raced +
  stall strings verified in the APK's libgojni.so in all three ABIs.
- **Current architecture decision / confidence:** ADR-0021 (doc
  08) — the first-dial race + the honest dns probe, extending the
  ADR-0018/0019/0020 species (failure detection, hygiene, bounded
  recovery; never steering — the preset is still captured once per
  flow and NEVER varies, on every candidate). Live ADRs:
  ADR-0001..0021. The auth lane stays CLOSED (D97).
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.15 round** — the report whose shell line
  reads **versionCode 14** (a versionCode 13 report means the old
  APK is still installed: the D103 filename v1.6.6-debug15.apk
  makes the right build self-identifying): (a) video starts — do
  they open in seconds now? (b) the NEW evidence line "tunnel
  first-byte raced dials=N" is the round's primary material: every
  raced line is a dirty primary the flow never paid a budget for;
  the raced-line count vs clean first-bytes vs terminals says
  whether the candidate count (2) should grow or shrink; (c) the
  dns fail count — does the degraded state still fire (it should
  now clear itself in one probe cycle)? (d) the D102 question for
  Ali himself, still LIVE: terminal first-byte failures still
  cluster under method=strong — should a flow that exhausts its
  raced round be retried once with the NEXT preset in
  HealSweepOrder (per-flow method variation)? His ruling, never
  unilateral. Plus the older circle rows (the cross-device UI
  verdict on debug.6, rows 4-8, rotation/TalkBack, the data-usage
  round, D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-x — ingest Ali's results from v1.6.6-android-debug.15 (the
  versionCode 14 build)**: the pasted diagnostics report FIRST —
  verify the "android shell:" line names versionCode 14 (a
  versionCode 13 report means the install never happened: ask
  which FILENAME was downloaded — the D103 name
  v1.6.6-debug15.apk makes the wrong build self-identifying); then
  read the "tunnel first-byte raced" lines (the save rate — the
  candidate count's tuning material), the terminal ratios (the
  D102 material) and the dur= dones to name the remaining waiting
  phase; root-cause from the evidence; fix at the correct layer;
  keep the D77-D103 honesty rules binding (D101: any new
  user-facing surface ships in the variant(s) the affected
  editions bake, verified through the real webui server). Bump
  `versionCode` and cut `v1.6.6-android-debug.16` through the same
  regular-release lane (D89) WHEN an app code change warrants it —
  D98: every release page carries the debug APK asset; D103
  (amending D99): the asset is NAMED v<versionName>-debug<N>.apk,
  N = the tag's build sequence (debug.16's asset is
  v1.6.6-debug16.apk) — push the code commit BEFORE creating the
  release (the full-SHA target_commitish 422s against an unpushed
  SHA). The auth Task IDs (A1/A2/A3) stay retired (D97). The
  dormant alternatives when no feedback arrives: the desktop
  release lane or public-release engineering — ask Ali which, do
  not guess.
