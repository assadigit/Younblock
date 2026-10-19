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

- **Phase completed:** M8-u (2026-10-19, early — the EIGHTEENTH
  feedback-loop exchange; the FOURTH pasted-report round, a NO-BUILD
  round). Ali's input: "The problem persists" + a fourth diagnostics
  report generated 23:41:37 UTC — but its "android shell: 1.6.6
  (versionCode 11)" line names the PREVIOUS build (debug.12), the
  one debug.13 replaces. Identity proven three ways (the
  PackageManager-read shell line; 21 reason=timeout dones at
  8054–8070 ms — impossible on the 2.5 s budget; zero retry/salvaged
  lines), plus the clock: debug.13 published 23:16:28 UTC, the
  report generated 23:41:37 UTC — the phone never received the fix.
  The report is the third report's signature measured again: 21
  timeouts + 17 resets at 64–527 ms, 67 errors / 114 conns, ~24
  healthy first-bytes at 58–548 ms, DNS ok 118 / fail 0,
  udpDropped=107, first video-data responses ~80 s into the window.
- **What this round did (no app change — D84: no versionCode bump,
  no release cut):** (1) debug.13 re-verified end-to-end: the
  release asset re-downloaded through the API and hash-matched
  (SHA-256 18b4ea648b0aa02d7eebf84f2dc596d13894093a1469dfc4f111ddc
  5b286b518, 35,130,385 B); the embedded engine checked to carry
  BOTH salvage lines ("tunnel first-byte retry", "tunnel first-byte
  salvaged") in libgojni.so in all three ABIs (arm64-v8a,
  armeabi-v7a, x86_64); Latest confirmed via /releases/latest; the
  HEAD knobs re-read (FirstByteTimeout 2.5 s, FirstByteSalvageDials
  2). (2) The reply + the preview page (src/app/page.tsx in the
  Next.js surface) walk Ali through the install: open the
  v1.6.6-android-debug.13 release page and download
  v1.6.6-debug13.apk — the filename itself names the build now
  (D103, Ali's ruling this round; if the filename says anything
  else, it is the wrong build); install over debug.12 in place
  (same pinned certificate, higher versionCode — settings and usage
  survive), reconnect, then verify the report header reads
  versionCode 12. (3) If a stall remains: menu → Diagnostics → Copy
  → paste — the retry/salvaged line ratios against terminal
  first-byte failed lines are the next round's evidence.
- **The M8-t carryover (unchanged, still the CURRENT build):**
  v1.6.6-android-debug.13, versionCode 12, ADR-0019 —
  FirstByteTimeout 8 s → 2.5 s + the salvage re-dial (a dead
  first-byte origin re-dialed INSIDE the same flow: same host, SAME
  preset captured once per flow, the client's buffered bytes
  replayed byte-exact through a fresh fragment conn, dead-IP
  exclusion on the re-resolve, up to FirstByteSalvageDials=2
  re-dials, the app never sees the failure). Both-ways proof at
  internal/proxy/firstbyte_test.go (recovers / exhausts-to-the-M8-s-
  terminal / byte-exact replay, each FAILING with the salvage
  stashed; two guards pass both ways). APK 35,130,385 B, apksigner
  OK (the pinned c76bb7c6...8551d), badging versionCode 12 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36.
- **Current architecture decision / confidence:** ADR-0019 (doc 08),
  unchanged — the salvage re-dial + the budget retune, the
  failure-detection/hygiene/bounded-recovery species, never steering
  (the preset is captured once per flow and NEVER varies). What
  remains OPEN: the per-node silence's ORIGIN (ISP-side,
  unobservable — no third guess), the METHOD question (D102 below),
  and now the INSTALL question — the fourth report proved the fix
  can ship while the phone still runs the old build, so every future
  report's FIRST check is its shell line. The auth lane stays CLOSED
  (D97). Live ADRs: ADR-0001..0019. Next decision ID: **D102 (still
  reserved for Ali — the method question, SHARPENED: the salvage
  re-flips the same coin up to twice more per flow; if terminal
  first-byte failures still cluster under one method in the actual
  debug.13 report, the NEXT-preset retry is his ruling to make)**.
- **Blocking questions:** none blocking the loop. The standing input
  is **Ali's ACTUAL debug.13 round** — the report whose shell line
  reads versionCode 12 (a versionCode 11 report means the old APK is
  still installed and the round is the install again): (a) video
  starts should open in seconds — the dead-origin coin flips now
  happen inside the flow; (b) at any remaining stall, menu →
  Diagnostics (the first row) → Copy → paste — the expected NEW
  evidence is "tunnel first-byte retry reason=… attempt=N" lines
  (absorbed dead attempts), "tunnel first-byte salvaged dials=N"
  lines (flows the salvage saved), and their ratio against the
  terminal "first-byte failed" lines; (c) the D102 question for Ali
  himself, sharpened: should a flow that exhausts its salvages under
  one preset be retried once with the NEXT preset in HealSweepOrder
  (per-flow method variation)? His ruling, never unilateral. Plus
  the older circle rows (the cross-device UI verdict on debug.6,
  rows 4-8, rotation/TalkBack, the data-usage round, D93's desktop
  check).
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-v — ingest Ali's results from v1.6.6-android-debug.13 (the
  versionCode 12 build)**: the pasted diagnostics report FIRST —
  verify the "android shell:" line names versionCode 12 (a
  versionCode 11 report means the install failed again: the round is
  the install, exactly as M8-u was — ask which FILENAME was
  downloaded: the D103 names (v1.6.6-debug13.apk) make the wrong
  build self-identifying); then
  read the retry/salvaged/failed line ratios + the dur= dones + the
  tun: line to name the waiting phase; root-cause from the evidence;
  fix at the correct layer; keep the D77-D101 honesty rules binding
  (D101: any new user-facing surface ships in the variant(s) the
  affected editions bake, verified through the real webui server).
  Bump `versionCode` and cut `v1.6.6-android-debug.14` through the
  same regular-release lane (D89) WHEN an app code change warrants
  it — D98: every release page carries the debug APK asset; D103
  (amending D99): the asset is NAMED v<versionName>-debug<N>.apk,
  N = the tag's build sequence (debug.14's asset is
  v1.6.6-debug14.apk) — push the code commit
  BEFORE creating the release (the full-SHA target_commitish 422s
  against an unpushed SHA). The auth Task IDs (A1/A2/A3) stay
  retired (D97). The dormant alternatives when no feedback arrives:
  the desktop release lane or public-release engineering — ask Ali
  which, do not guess.
