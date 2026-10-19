# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-u (2026-10-19, early — **Ali's FOURTH pasted
report round, a NO-BUILD round: "The problem persists" arrived again,
the report generated 23:41:37 UTC — but it names itself versionCode
11, the PREVIOUS build (debug.12). The identity is authoritative
(MainActivity reads it from Android's PackageManager at runtime) and
the log confirms it three independent ways: 21 reason=timeout dones
at 8054–8070 ms (the old 8 s budget — the 2.5 s build cannot produce
an 8 s death), zero "tunnel first-byte retry"/"salvaged" lines in
the whole 200-line window, and the clock — debug.13 was published at
23:16:28 UTC, twenty-five minutes BEFORE the report was generated.
The report carries zero bytes of salvage-build evidence: it is the
third report's signature measured again (21 timeouts + 17 resets at
64–527 ms, 67 errors / 114 conns, ~24 healthy first-bytes at 58–548
ms — every one inside the new 2.5 s budget with 4.5x headroom, DNS
ok 118 / fail 0, udpDropped=107). The round's action: NO new build
(D84 — no app change, no bump); debug.13 re-verified end-to-end (the
release asset re-downloaded through the API and hash-matched, the
embedded engine checked to carry both salvage lines in libgojni.so
in all three ABIs, Latest confirmed via /releases/latest); the reply
and the preview page walk Ali through installing
v1.6.6-android-debug.13. The same session then brought Ali's D103
ruling ("add debug13 in the apk's name"): every D99-era APK asset
renamed in place (v1.6.6-debug8.apk … v1.6.6-debug13.apk, bytes
unchanged — the debug.13 asset re-served and hash-matched after
the rename), every affected release body updated, and the naming
rule amended — from debug.14 on the asset carries the build
number, so the downloaded FILE itself names its build.**
Earlier: M8-t (ADR-0019, the salvage re-dial + the 2.5 s budget,
debug.13 — still the CURRENT build), M8-s (ADR-0018, the first-byte
budget, debug.12), M8-r (ADR-0017, the sweep probes from zero,
debug.11), M8-q/p (the diagnostics surface, ADR-0015/0016 +
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

- **Phase completed:** M8-v (2026-10-19, later — the NINETEENTH
  feedback-loop exchange; the FIFTH pasted-report round, the first
  from the salvage build itself). Ali's input: "The problem
  persists" + a report whose "android shell: 1.6.6 (versionCode
  12)" line names debug.13 — the salvage build — and whose log
  carries the salvage's own lines. The report PROVED the salvage
  on-device (retry lines absorbing dead attempts, "tunnel
  first-byte salvaged dials=1/2" saving flows at 0.7–1.4 s, errors
  37/81 ≈ 46 % from 67/114 ≈ 59 %) and named the two remaining
  start-lag sources: (1) the SERIAL salvage chain — 21 terminal
  reason=timeout dones at 8014–8227 ms = three 2.5 s budgets walked
  one after another, + 11 terminal resets at 0.6–1.9 s; (2) the
  mid-stream stall — five video-start flows done in the same
  instant at 00:18:17.2, dur=57290–64620 ms, up=2.4–5.0 KB,
  down=6.6–7.7 KB (one 121.1 KB): first byte plus ~7 KB, then total
  silence until the app's own ~60 s patience killed them; the retry
  wave right after answered in 60–261 ms (the video took ~65 s to
  start). Counters otherwise healthy: DNS ok 150 / fail 0,
  rstUnmapped=0, doorFailures=0, udpDropped=78.
- **What this round shipped — v1.6.6-android-debug.14, versionCode
  13, ADR-0020 (doc 08), the CURRENT build:** (1) the salvage RACE
  — when attempt 0 dies at first byte, the remaining re-dials
  launch IN PARALLEL (re-resolve, dead-IP exclusion, dedupe,
  rotated lists so candidates start on distinct live origins; each
  candidate's DialHost keeps the M8-n stagger failover), the FIRST
  origin to speak wins (bytes delivered, losers closed and reaped,
  one demotion per dead attempt preserved), and a silent-host
  terminal costs ~2x the budget (attempt 0 + the race), never the
  serial chain's 3x; the total attempt count is unchanged (1 + 2);
  the client's bytes during the race wait in the socket buffer and
  flow to the winner in order. (2) The STALL WATCH — new knob
  StallTimeout (default 15 s; 0 → default, negative → disabled):
  a budgeted flow's relay that goes silent in BOTH directions for
  the budget is closed engine-side (both ends so the app's read
  errors NOW, the answer set demoted, the error counted, new report
  line "tunnel stall killed idle=Nms"); one-direction silence
  never fires (a pure download or upload is healthy); unrouted
  relays keep the plain shape byte-for-byte (D46). Both-ways
  proof: with tunnel.go stashed, TestStallKillClosesSilentRelayFlow,
  TestSalvageRaceRedialsInParallel, TestSalvageRaceTerminalIsBounded
  and the updated TestFirstByteSalvageRecoversDeadOrigin all FAIL;
  with the fix all 16 first-byte/salvage/stall tests PASS; guards
  pass both ways (the streaming-download guard caught an
  inverted-stamp bug mid-round — the watch measures from the NEWEST
  activity clock). Lane evidence: baseline green (vet, 18/18 ×2,
  GOOS=windows), compile matrix 4/4 + arm64 full tree, gomobile
  bind with the stall string in all three ABIs, clean assembleDebug;
  APK 35,182,673 B, SHA-256 154d40355bee702fcc751ead595688b5732abe
  b44cf186913dddbc5a99cad5e7, apksigner OK (the pinned
  c76bb7c6...8551d), badging versionCode 13 / versionName 1.6.6 /
  minSdk 26 / targetSdk 36; the stall string also verified in the
  APK's libgojni.so in all three ABIs.
- **Current architecture decision / confidence:** ADR-0020 (doc
  08) — the salvage race + the stall watch, extending the
  ADR-0018/0019 species (failure detection, hygiene, bounded
  recovery; never steering — the preset is still captured once per
  flow and NEVER varies). Live ADRs: ADR-0001..0020. The auth lane
  stays CLOSED (D97).
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.14 round** — the report whose shell line
  reads **versionCode 13** (a versionCode 12 report means the old
  APK is still installed: the D103 filename v1.6.6-debug14.apk
  makes the right build self-identifying): (a) video starts —
  do they open in seconds now? (b) at any remaining stall: menu →
  Diagnostics (the first row) → Copy → paste — the NEW evidence
  lines are "tunnel stall killed idle=Nms" (how many fire, and
  whether starts still stall when they do not — the stall budget's
  tuning material) plus the retry/salvaged-vs-terminal ratio (the
  D102 material); (c) the D102 question for Ali himself, now LIVE:
  terminal first-byte failures still cluster under method=strong
  (21 timeouts + 11 resets, ~40 % of conns) — should a flow that
  exhausts its RACED salvages be retried once with the NEXT preset
  in HealSweepOrder (per-flow method variation)? His ruling, never
  unilateral. Plus the older circle rows (the cross-device UI
  verdict on debug.6, rows 4-8, rotation/TalkBack, the data-usage
  round, D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-w — ingest Ali's results from v1.6.6-android-debug.14 (the
  versionCode 13 build)**: the pasted diagnostics report FIRST —
  verify the "android shell:" line names versionCode 13 (a
  versionCode 12 report means the install never happened: ask
  which FILENAME was downloaded — the D103 name
  v1.6.6-debug14.apk makes the wrong build self-identifying); then
  read the "tunnel stall killed" lines, the retry/salvaged/failed
  ratios and the dur= dones to name the remaining waiting phase;
  root-cause from the evidence; fix at the correct layer; keep the
  D77-D103 honesty rules binding (D101: any new user-facing surface
  ships in the variant(s) the affected editions bake, verified
  through the real webui server). Bump `versionCode` and cut
  `v1.6.6-android-debug.15` through the same regular-release lane
  (D89) WHEN an app code change warrants it — D98: every release
  page carries the debug APK asset; D103 (amending D99): the asset
  is NAMED v<versionName>-debug<N>.apk, N = the tag's build
  sequence (debug.15's asset is v1.6.6-debug15.apk) — push the code
  commit BEFORE creating the release (the full-SHA target_commitish
  422s against an unpushed SHA). The auth Task IDs (A1/A2/A3) stay
  retired (D97). The dormant alternatives when no feedback arrives:
  the desktop release lane or public-release engineering — ask Ali
  which, do not guess.
