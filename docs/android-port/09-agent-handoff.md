# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-t (2026-10-18/19, late — **Ali's debug.12
device round: "The problem persists" + the THIRD pasted diagnostics
report, versionCode 11. The report PROVED the M8-s budget on-device
exactly as designed — the expected "tunnel first-byte failed
reason=timeout" dones at ~8.3 s arrived (22 of them) alongside fast
~0.5 s reason=reset closes (18) — and named the remaining
per-video-start lag: roughly half the video-start waves die at first
byte (38 errors / 61 conns) and the app re-opens wave after wave
(~35 s from the first wave to the first video bytes). Fixed in
`v1.6.6-android-debug.13` (versionCode 12, ADR-0019): the budget
retuned 8 s → 2.5 s from the report's tuning evidence (every
budget-killed flow was 100 % silent — dead nodes, not slow ones;
healthy routed origins answer in 420–508 ms, one 2399 ms outlier),
PLUS the salvage re-dial — a dead first-byte origin is re-dialed
INSIDE the same flow (same host, SAME preset captured once per flow —
per-flow preset variation stays D102, Ali's ruling, never code), the
client's buffered bytes replayed byte-exact through a fresh fragment
conn, dead-IP exclusion on the re-resolve, up to 2 re-dials, so the
app never sees the failure and its serial retry chain collapses into
bounded in-engine attempts. New report lines: "tunnel first-byte
retry reason=… attempt=N" (absorbed) and "tunnel first-byte salvaged
dials=N" (saved).)** Earlier: M8-s (ADR-0018, the first-byte budget,
debug.12), M8-r (ADR-0017, the sweep probes from zero, debug.11),
M8-q/p (the diagnostics surface, ADR-0015/0016 + D100/D101), M8-o/n
(the stall fixes, ADR-0014/0013 + D99/D98), A0 rounds — **the
account gate is CANCELLED (D97), nothing of it was ever built.** This
file is the live handoff; keep the fields in §"Current handoff
fields" accurate at every session boundary.

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

- **Phase completed:** M8-t (2026-10-18/19, late — the SEVENTEENTH
  feedback-loop exchange; the debug.12 device round). Ali's input:
  "The problem persists" + the THIRD pasted diagnostics report (its
  "android shell: 1.6.6 (versionCode 11)" line proved the debug.12
  install). The report proved the M8-s budget fires on-device exactly
  as designed (22 reason=timeout dones at ~8.3 s + 18 reason=reset
  at ~0.5 s — the tuning evidence ADR-0018 asked for) and named the
  remaining lag: roughly half the video-start waves die at first
  byte (counters 38 errors / 61 conns), the app re-opens wave after
  wave (~35 s from the first wave to the first video bytes, down=4.4
  MB once a flow survived). Every budget-killed flow was 100 %
  silent (down=0 B; the same flows sat out the full 60 s in the
  previous report — dead nodes, not slow ones); healthy routed
  origins answered in 420–508 ms (one 2399 ms outlier carrying the
  fragment-pause write).
- **What shipped (ADR-0019):** (1) FirstByteTimeout retuned 8 s →
  2.5 s (the CONFIGURATION block; ~5x the worst healthy routed
  wait, covers the outlier, a dead attempt costs 3.3x less); (2) the
  salvage re-dial — when a processed host's origin dies at first
  byte, the engine closes ONLY the origin, demotes the answer set
  (the M8-o loop, per dead attempt), re-resolves excluding the
  flow's dead IPs, re-dials the SAME host with the SAME preset
  (captured once per flow — a mid-flight method change cannot vary a
  flow's wire shape), and replays the client's buffered bytes
  byte-exact through a fresh fragment conn (first record
  re-fragmented identically; early data captured during the wait
  rides after). Up to FirstByteSalvageDials re-dials (default 2 → 3
  total attempts, bounded); the app never sees the failure. Exhausted
  or un-dialable → the M8-s terminal exactly. Mechanically: the
  budgeted shape owns the first record's write; one client pump
  reads the client for the whole first-byte phase (tee into a 64 KB
  replay buffer + live writes through a mutex-guarded swap-able
  origin link); after the origin speaks, the plain two-goroutine
  relay takes over byte-for-byte; tunnelCloser.setB keeps Stop's
  sweep on the live origin. New report lines (reason-in-message, the
  M8-r lesson): "tunnel first-byte retry reason=… attempt=N" (Warn)
  and "tunnel first-byte salvaged dials=N" (Info). Unrouted relays
  never enter the shape (the D46 scoping).
- **The round's proof (both ways):** internal/proxy/firstbyte_test.go
  — TestFirstByteSalvageRecoversDeadOrigin (the client NEVER sees a
  close; retry + salvaged lines; exactly 2 dials),
  TestFirstByteSalvageExhaustsToFailFast (exactly 1+2 dials, then the
  M8-s terminal signature: reason line, done down=0 B, one demotion
  per dead attempt), TestSalvageReplayIsByteExact (the fresh origin
  receives record+early-data byte-exact) — each FAILS with the
  tunnel.go salvage stashed (the M8-s shape at HEAD, call sites
  reverted) and PASSES with it. The guards
  (TestFirstByteSalvageSparesUnroutedHosts,
  TestFirstByteSalvageDisabledByConfig) pass both ways; the two M8-s
  tests now pin the terminal shape with FirstByteSalvageDials = -1.
- **Baseline green:** frontend untouched (the vite rebuild
  byte-identical, dist re-sync left the tree clean); go vet clean;
  go test 18/18 x2; GOOS=windows exit 0; the Android compile matrix
  in its exact form (arm64 full tree + the non-main set 4/4 ABIs,
  CGO_ENABLED=0, -mod=vendor).
- **The build lane:** versionCode 11 -> 12 (the D84 rule); run as
  FOREGROUND CHUNKS this round (this sandbox's background killer
  killed BOTH detached jobs — the sdkmanager install AND the detached
  build-android.sh lane; the caches carried the progress between
  chunks, each exit 0); the AAR 25,101,194 B, its embedded engine
  VERIFIED ("tunnel first-byte salvaged" + "tunnel first-byte retry"
  inside libgojni.so in all three ABIs); app-debug.apk
  **35,130,385 B**, SHA-256 **18b4ea648b0aa02d7eebf84f2dc596d13894**
  **093a1469dfc4f111ddc5b286b518**; apksigner OK (the pinned
  c76bb7c6...8551d); badging versionCode 12 / versionName 1.6.6 /
  minSdk 26 / targetSdk 36. The standing ⛔ unchanged: no /dev/kvm,
  no device.
- **The environment was WIPED a sixth time before this round** (~/sdk
  and ~/go erased; the repo INTACT at f4e2091, clean, with the
  untracked build state surviving — node_modules, dists, the old
  AAR). Reinstalled per doc 06 §1 and verified: Go 1.27.2, javac
  21.0.12.1, sdkmanager 12.0 (the versioned 11076708 zip — the alias
  URL stays dead), platform-tools 37.0.1, platforms 26+36,
  build-tools 36.1.0, NDK 28.2.13676358, gomobile/gobind at the
  pinned x/mobile. Disk: 4.0 GB free after; the Go build cache was
  cleared mid-round (before the SDK install) and rebuilt by the
  bind.
- **Current architecture decision / confidence:** ADR-0019 (doc 08)
  — the salvage re-dial + the budget retune, same species as
  M8-n/o/s (failure detection, answer-set hygiene, bounded recovery
  — NOT steering; the preset is captured once per flow and NEVER
  varies). Confidence: the tuning evidence was read FROM Ali's
  report (the 8.3 s dones are the budget firing; the 100 % silence
  is the dead-node proof) and the salvage was reproduced on the host
  BEFORE shipping (the both-ways proof above). What remains OPEN:
  the per-node silence's ORIGIN (ISP-side, unobservable — no third
  guess) and the METHOD question (D102 below). ADR-0018/0017/0014/
  0013 stand host-proven. The auth lane stays CLOSED (D97). Live
  ADRs: ADR-0001..0019. Next decision ID: **D102 (still reserved
  for Ali — the method question, now SHARPER: the salvage re-flips
  the same coin up to twice more per flow; if terminal first-byte
  failures still cluster under one method in the next report, the
  NEXT-preset retry is his ruling to make)**.
- **Blocking questions:** none blocking the loop. The standing input
  is **Ali's debug.13 round**: (a) video starts should open in
  seconds — the dead-origin coin flips now happen inside the flow;
  (b) at any remaining stall, menu → Diagnostics (the first row) →
  Copy → paste the report — the expected NEW evidence is "tunnel
  first-byte retry reason=… attempt=N" lines (absorbed dead
  attempts), "tunnel first-byte salvaged dials=N" lines (flows the
  salvage saved), and their ratio against the terminal "first-byte
  failed" lines — that ratio is the tuning evidence for the budget
  AND the D102 material; (c) **the D102 question for Ali himself,
  sharpened**: the salvage retries with the SAME preset (never a
  method change); should a flow that exhausts its salvages under one
  preset be retried once with the NEXT preset in HealSweepOrder
  (per-flow method variation)? His ruling, never unilateral. Plus
  the older circle rows (the cross-device UI verdict on debug.6,
  rows 4-8, rotation/TalkBack, the data-usage round, D93's desktop
  check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-u — ingest Ali's results from v1.6.6-android-debug.13**: the
  pasted diagnostics report FIRST (verify the "android shell:" line
  names versionCode 12; a versionCode 11 report means the old APK is
  still installed and the round starts with the install); read the
  retry/salvaged/failed line ratios + the dur= dones + the tun: line
  to name the waiting phase; root-cause from the evidence; fix at
  the correct layer; keep the D77-D101 honesty rules binding (D101:
  any new user-facing surface ships in the variant(s) the affected
  editions bake, verified through the real webui server). Bump
  `versionCode` and cut `v1.6.6-android-debug.14` through the same
  regular-release lane (D89) WHEN an app code change warrants it —
  D98: every release page carries the debug APK asset; D99: the
  asset is NAMED v<versionName>-debug.apk — push the code commit
  BEFORE creating the release (the full-SHA target_commitish 422s
  against an unpushed SHA). The auth Task IDs (A1/A2/A3) stay
  retired (D97). The dormant alternatives when no feedback arrives:
  the desktop release lane or public-release engineering — ask Ali
  which, do not guess.
