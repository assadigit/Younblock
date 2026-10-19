# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-y (2026-10-19, evening — **Ali's EIGHTH
pasted report round, the FIRST from the right build: the shell line
reads versionCode 15 = the debug.16 build, both prior signatures
present (the raced line and the tier= stall line), and Ali's own
words closed ADR-0022: "The original problem (videos taking time
to play) persist, but the problem of easily going next and back in
same video instantly is fixed"**. The report measures the start
lag's two remaining sources: (1) the DNS negative-cache lockout —
dns ok 142 / fail 50 with doorFailures=50, every resolver failure a
dead door dial, the failure lines pairing at the same millisecond
(four pairs inside 22 ms = instant negative hits, not walks): one
DoH blip's tail amplified through the 15 s TTL on fresh per-video
hostnames; (2) the dead-origin re-gamble — ~17 timeout terminals at
2892–4304 ms + ~8 resets, all through the raced round, the SECOND
video's start paying the full discovery cost again (a new rr
hostname; the per-flow dead-IP map dying with each flow; udpDropped=
99 is the M8-j QUIC fast-fail working). The round shipped ADR-0023
(the negative re-ask + the origin-IP penalty box + the baked socks5
reason) as v1.6.6-android-debug.17, versionCode 16, asset
v1.6.6-debug17.apk.**
Earlier: M8-x (ADR-0022, the tiered stall watch, debug.16 — its fix
VERIFIED on-device by the eighth report's words), M8-w (ADR-0021,
the first-dial race + honest dns probe, debug.15 — proven run by the
eighth report's raced lines), M8-v (ADR-0020, the salvage race +
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

- **Phase completed:** M8-y (2026-10-19, evening — the TWENTY-SECOND
  feedback-loop exchange; the EIGHTH pasted-report round, the first
  from the right build). Ali's input: "The original problem (videos
  taking time to play) persist, but the problem of easily going next
  and back in same video instantly is fixed" + a report generated
  2026-10-19T12:53:37Z whose "android shell: 1.6.6 (versionCode
  15)" line names the debug.16 build (both prior signatures
  present: "tunnel first-byte raced dials=2" and
  "tier=lean" stall lines — ADR-0022 verified on-device by Ali's
  own words). Counters: conns total 130, live 14, errors 80, dns
  ok 142 / fail 50, doorFailures=50 (every resolver failure a dead
  door dial — the pairs at the same millisecond are instant
  negative-cache hits, one DoH blip's tail through the 15 s TTL on
  fresh per-video hostnames), rstUnmapped=0, overCap=0, udpDropped=99
  (the M8-j QUIC fast-fail working — no action), method strong.
- **What this round shipped — v1.6.6-android-debug.17, versionCode
  16, ADR-0023 (doc 08), the CURRENT build:** (1) the NEGATIVE
  RE-ASK — a DNS_UNREACHABLE negative-cache hit (evidence about the
  resolver's own path, not a verdict about the name) re-asks the
  servers through the singleflight instead of failing instantly,
  rate-limited per host to one WALK per DNSCacheNegativeReask=1 s
  with the failure's own walk counting as the window's first probe
  (every negative hit is instant or a genuinely fresh walk, never a
  redundant one); server-verdict negatives (BLOCKED_ANSWER / no
  usable answer) keep the instant fail; new log line "dns negative
  re-ask (path failure, not a name verdict)" — its count is the
  lockouts-absorbed evidence. (2) The ORIGIN-IP PENALTY BOX — every
  classified first-byte death (the origin connected or reset at the
  hello write and never spoke; writeReplay failure or read death,
  natural only) marks its origin IP for DeadIPPenalty=60 s; every
  dial list (both doors, the raced extras, the salvage candidates,
  the plain-http forward) skips the marked IPs; a preference never a
  verdict — an all-penalized list dials in full; NOT marked: stall
  kills, client-side deaths, teardown-canceled deaths, dial
  failures; bounded 512 FIFO; the knowledge crosses flows AND
  hostnames (the blackhole is per cache-node IP, shared across rr
  names — the second video's start dials only proven-live
  addresses). (3) The baked reason — the socks5 door's DNS line now
  carries reason= in the message (the 50 bare lines of the report
  could not say why). Knobs DNSCacheNegativeReask (doh.Config;
  negative → the exact M8-w shape) and DeadIPPenalty (proxy.Config;
  negative → the exact M8-x shape), both wired through the engine.
  Both-ways proofs: the re-ask neutered →
  TestNegativeReaskRecoversAfterBlip FAILS with the report's exact
  signature ("lookup failed recently: DNS_UNREACHABLE"),
  TestNegativeReaskRateLimited FAILS, the rewritten TestCacheNegativeTTL
  FAILS at its recovery assertion (the old pin WAS the lockout);
  tunnel.go marking stashed → TestDeadIPPreferenceSparesTheNextFlow
  FAILS (flow 2 EOF on the corpse) and TestRacedDeathsMarkTheIP
  FAILS (corpse accepts 5 ≠ 3); all guards pass both ways. Lane
  evidence: baseline green (vet, 18/18 ×2, Windows exit 0, matrix
  4/4 ABIs CGO+NDK + arm64 full tree), gomobile bind with the
  re-ask + reason strings verified in libgojni.so in all three ABIs
  BEFORE gradle, CLEAN assembleDebug; APK 35,279,565 B, SHA-256
  f40d03a09a896a6ea3cfcad4e081ba36e3e62430e55b23b69640b4a9f6577018,
  apksigner OK (the pinned c76bb7c6...8551d), badging versionCode
  16 / versionName 1.6.6 / minSdk 26 / targetSdk 36; the strings
  verified in the APK's libgojni.so in all three ABIs.
- **Current architecture decision / confidence:** ADR-0023 (doc
  08) — the negative re-ask + the origin-IP penalty box, extending
  the ADR-0018..0022 species (failure detection, hygiene, bounded
  recovery; memory as preference, never verdict; never steering).
  Live ADRs: ADR-0001..0023. The auth lane stays CLOSED (D97).
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.17 round** — the report whose shell line
  reads **versionCode 16** (15 means the old APK is still
  installed: the D103 filename v1.6.6-debug17.apk makes the right
  build self-identifying): (a) video STARTS — the first video after
  connecting and the second right after it (the second used to pay
  the full dead-origin discovery cost again); (b) the "dns negative
  re-ask (path failure, not a name verdict)" count — each line is a
  lockout absorbed; (c) the dns fail counter (was 50 — should
  collapse to the handful of real server walks) and the "socks5 dns
  lookup failed reason=…" lines (the reason finally visible — it
  names which DoH leg fails); (d) the first-byte terminal count
  (was 17 timeouts + 8 resets) and the raced save rate; (e) the
  D102 question for Ali himself, still LIVE: terminal first-byte
  failures cluster under both strong and balanced — should a flow
  that exhausts its raced round be retried once with the NEXT
  preset in HealSweepOrder? His ruling, never unilateral. Plus the
  older circle rows (the cross-device UI verdict on debug.6, rows
  4-8, rotation/TalkBack, the data-usage round, D93's desktop
  check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-z — ingest Ali's results from v1.6.6-android-debug.17 (the
  versionCode 16 build)**: the pasted diagnostics report FIRST —
  verify the "android shell:" line names versionCode 16 (15 means
  the install never happened: ask which FILENAME was downloaded —
  the D103 name v1.6.6-debug17.apk makes the wrong build
  self-identifying); then read the re-ask lines (lockouts
  absorbed), the dns fail counter + the baked reasons (which leg
  fails), the first-byte terminal cohort (was 17+8), the raced save
  rate, and the tier= split; root-cause from the evidence; fix at
  the correct layer; keep the D77-D103 honesty rules binding (D101:
  any new user-facing surface ships in the variant(s) the affected
  editions bake, verified through the real webui server). Bump
  `versionCode` and cut `v1.6.6-android-debug.18` through the same
  regular-release lane (D89) WHEN an app code change warrants it —
  D98: every release page carries the debug APK asset; D103
  (amending D99): the asset is NAMED v<versionName>-debug<N>.apk,
  N = the tag's build sequence (debug.18's asset is
  v1.6.6-debug18.apk) — push the code commit BEFORE creating the
  release (the full-SHA target_commitish 422s against an unpushed
  SHA). The auth Task IDs (A1/A2/A3) stay retired (D97). The
  dormant alternatives when no feedback arrives: the desktop
  release lane or public-release engineering — ask Ali which, do
  not guess.
