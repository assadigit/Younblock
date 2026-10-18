# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-s (2026-10-18/19 — **Ali's debug.11 device
round: "the problem persists, but when after a while, video plays...
IN pc, it's instant" + the SECOND pasted diagnostics report — the
M8-r evidence loop working exactly as designed. The report PROVED the
debug.11 fixes on-device (state on, a heal sweep that found its
method 3 s in, the startup DoH blip logging its baked
reason=DNS_UNREACHABLE lines and self-healing — the sharpened
question answered: a transient at Start, not a filter) and gave the
remaining per-video-start stall its exact shape: video-start waves
where some flows complete (first-byte 56-619 ms) while others dial
fine, send the ~1.7 KB hello, and never receive one byte — ~0.5 s
closes (origin reset) and EXACTLY ~60.0 s hangs, all down=0 B;
nothing in the engine closes at 60 s (the relay idle timeout is 300
s, the TUN keepalive only probes) — the app's own patience,
amplified by an engine holding the dead flow open, with the M8-o
demotion never firing because the dial SUCCEEDED. Fixed in
`v1.6.6-android-debug.12` (versionCode 11, ADR-0018 — the first-byte
budget: FirstByteTimeout=8 s on processed hosts; silent origins fail
fast, the reason lands in the report, and the suspect answer set is
demoted like a total dial failure)**). The method-policy question was
SURFACED for Ali (D102 material, his ruling, not our code): the sweep
picks the preset from its redirector.googlevideo.com probe while the
media flows hit rr*.googlevideo.com nodes whose tolerance differs per
node (method strong on his phone, PC instant on the same network) —
should a dead routed flow be retried once with the NEXT preset? No
steering without his ruling.** Earlier: M8-r (ADR-0017, the first
pasted report — the sweep resets the resolver, a failed sweep
restores the method, the DNS reasons baked into the messages),
M8-q/p (the diagnostics surface, ADR-0015/0016 + D100/D101), M8-o/n
(the stall fixes, ADR-0014/0013 + D99/D98), A0 rounds — **the
account gate is CANCELLED (D97), nothing of it was ever built.**
This file is the live handoff; keep the fields in §"Current handoff
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

- **Phase completed:** M8-s (2026-10-18/19 — the SIXTEENTH
  feedback-loop exchange; the debug.11 device round). Ali's input:
  the SECOND pasted diagnostics report + the round's discriminators
  ("video plays after a while, then smooth — rewind/next instant;
  only each video START lags; PC instant"). Its "android shell:
  1.6.6 (versionCode 10)" line proved the debug.11 install; state
  on, method strong (the M8-r fixes proven: a sweep that found its
  method 3 s in, no false degraded); the startup DoH blip still
  logging its three baked reason=DNS_UNREACHABLE lines (~170 ms
  each, self-healed — the sharpened question answered: transient);
  and the stall's exact shape (the full reconstruction is
  ADR-0018's Context): video-start waves where some flows complete
  (first-byte 56-619 ms) while others dial fine, send the ~1.7 KB
  hello, and never receive one byte — ~0.5 s closes and EXACTLY
  ~60.0 s hangs, all down=0 B. Nothing in the engine closes at 60 s
  (idle 300 s, the TUN keepalive only probes) — the app's own
  patience, amplified by an engine holding the dead flow open; and
  because the dial SUCCEEDED, the M8-o demotion never fired, so the
  suspect answer set stayed fresh for every retry.
- **What shipped (ADR-0018):** the first-byte budget —
  `FirstByteTimeout = 8 s` (internal/config, the CONFIGURATION
  block; >12x the worst healthy first byte observed, 619 ms): after
  the first client record is written upstream, a processed host's
  origin must answer within the budget or the flow fails FAST —
  both ends closed immediately (the client sees the reset and
  retries, as it already retries the ~0.5 s reset waves), the error
  counted, "tunnel first-byte failed reason=timeout|reset|closed"
  baked into the message (the ring drops attrs — M8-r's lesson),
  and the host's cached answer set demoted exactly like a total
  dial failure (the M8-o loop extended to origins that connect but
  never speak). Unrouted relays keep the plain two-goroutine shape
  byte-for-byte (the D46 scoping: the budget is part of the
  processing of the hosts the app processes, not of the relay).
- **The round's proof (both ways):** internal/proxy/firstbyte_test.go
  — with the tunnel.go restructure stashed,
  `TestFirstByteTimeoutFailsSilentRoutedTunnelFast` and
  `TestFirstByteFailureInvalidatesSuspectSet` FAIL at ~5.0 s (the
  silent processed flow hangs to the idle timeout — the device's 60
  s signature compressed); with the fix, ~250 ms close, the reason
  line, the done line (down=0 B), the demotion recorded. The two
  guards (`...SparesUnroutedHosts`, `...WithinBudgetFlowsNormally`)
  pass both ways, as guards must.
- **Baseline green:** frontend untouched (the committed dist intact,
  the tree clean); go vet clean; go test 18/18 x2; GOOS=windows
  exit 0; the Android compile matrix in its exact form (arm64 full
  tree + the non-main set 4/4 ABIs, CGO_ENABLED=0, -mod=vendor);
  disk managed (Go build cache cleared before the bind).
- **The build lane:** versionCode 10 -> 11 (the D84 rule); the full
  build-android.sh lane exit 0 (run detached — the double-fork
  setsid pattern); the AAR's embedded engine VERIFIED ("tunnel
  first-byte failed" inside libgojni.so in all three ABIs);
  app-debug.apk **35,076,497 B**, SHA-256 **ce25f9cdd9a103f26a1286
  1c21c989e0ecb451a2dfe9dd2251bc421beeab261c**; apksigner OK (the
  pinned c76bb7c6...8551d); badging versionCode 11 / versionName
  1.6.6 / minSdk 26 / targetSdk 36. The standing ⛔ unchanged: no
  /dev/kvm, no device.
- **The environment is INTACT** (no wipe — verified at open: Go
  1.27.2, javac 21.0.12, platforms 26/36, the pinned gomobile).
  Disk was managed by clearing the Go build cache before the bind
  (~9.9 GB box; expect that again after big builds). Wipes happen
  at ANY boundary — VERIFY first (`. ~/sdk/env.sh && go version`),
  reinstall per doc 06 §1 only if actually wiped (the cmdline-tools
  `12.0` alias URL 404s — use the versioned
  commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0018 (doc
  08) — the first-byte budget, same species as M8-n's handshake
  budget and M8-o's demotion: failure detection and answer-set
  hygiene, NOT steering. Confidence: the root cause was read FROM
  Ali's report (the ~60 s elimination list against every engine
  deadline) and reproduced on the host BEFORE the fix. What remains
  OPEN: the per-node silence's ORIGIN (ISP-side, not observable
  from the app — no third guess) and the METHOD question (below).
  ADR-0017/0014/0013 stand host-proven on the working path. The
  auth lane stays CLOSED (D97). Live ADRs: ADR-0001..0018. Next
  decision ID: **D102 (reserved for Ali's rulings — the method
  question below is its live material)**.
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.12 round**: (a) video starts should open
  in seconds instead of "after a while"; (b) at any remaining
  stall, menu → Diagnostics (the first row) → Copy → paste the
  report — the expected new evidence is "tunnel first-byte failed
  reason=..." lines and done lines like "dur=~8000ms down=0 B"
  (the tuning evidence for the budget); (c) **the D102 question for
  Ali himself**: the sweep picks the method from its
  redirector.googlevideo.com probe while the media flows hit
  rr*.googlevideo.com cache nodes whose tolerance differs per node
  (his report ran method strong; his PC is instant on the same
  network) — should the engine retry a dead routed flow once with
  the NEXT preset in HealSweepOrder (per-flow method variation)?
  That is a method-policy change: his ruling, never unilateral;
  plus the older circle rows (the cross-device UI verdict on
  debug.6, rows 4-8, rotation/TalkBack, the data-usage round,
  D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-t — ingest Ali's results from v1.6.6-android-debug.12**:
  the pasted diagnostics report FIRST (verify the "android shell:"
  line names versionCode 11; a versionCode 10 report means the old
  APK is still installed and the round starts with the install);
  read the reason= lines + the dur= dones + the tun: line to name
  the waiting phase; root-cause from the evidence; fix at the
  correct layer; keep the D77-D101 honesty rules binding (D101:
  any new user-facing surface ships in the variant(s) the affected
  editions bake, verified through the real webui server). Bump
  `versionCode` and cut `v1.6.6-android-debug.13` through the same
  regular-release lane (D89) WHEN an app code change warrants it —
  D98: every release page carries the debug APK asset; D99: the
  asset is NAMED v<versionName>-debug.apk — push the code commit
  BEFORE creating the release (the full-SHA target_commitish 422s
  against an unpushed SHA). The auth Task IDs (A1/A2/A3) stay
  retired (D97). The dormant alternatives when no feedback
  arrives: the desktop release lane or public-release engineering
  — ask Ali which, do not guess.