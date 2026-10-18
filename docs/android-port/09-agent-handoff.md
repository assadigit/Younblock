# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the A0-R3 round (2026-10-18, the third
exchange of the A0 day — **the account gate is CANCELLED, D97**:
nothing of it was ever built and nothing ever will be; the
`docs/auth/` pack stays as bannered history, ADR-AUTH-0001 is
retired, D95/D96 are superseded, and Younblock stays one
free-for-all app — no login, no server, no editions. Docs-only
round: no code change, debug.6 stays Latest, no release was cut).
Before that, the A0 round delivered the spec pack itself plus one
two-line host-side fix — `GOOS=windows go build ./...` had never
compiled since M4 — **Phase 2 host-complete; the test circle
is on its FIFTH build, published as regular releases in the
private repo, and the keystore helper survives PowerShell-less
PATH machines (D90)**. This file is the live handoff; keep the
fields in §"Current handoff fields" accurate at every session
boundary.

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

- **Phase completed:** the A0 round (2026-10-18, Task ID A0) — **the
  account-gate spec delivered + a baseline defect fixed**. No device
  feedback had arrived (zero issues on the repo, all states; none in
  the session messages), so the session ran the design lane: the
  six-document `docs/auth/` pack (00-start-here with the six open
  questions and recommendations, 01-requirements AUTH-001…031,
  02-architecture, 03-auth-protocol, 04-admin-and-operations,
  05-security-review with the drafted privacy copy), plus
  ADR-AUTH-0001 in doc 08 (engine-enforced gate — grounded in D94
  ruling 1; nothing beyond it, because the six questions await
  Ali's rulings and recording recommendations as if ruled would
  fake his choices). The same session caught and fixed a latent
  baseline defect: `mobile/nonblock_unix.go` (born M4) had no build
  constraint, so `GOOS=windows go build ./...` never compiled — one
  line (`//go:build unix`) fixed it; no Android behavior change by
  construction; no release (D84: nothing app-facing; versionCode
  stays 5, debug.6 stays Latest). Beneath it: M8-m/D94 (the gap
  analysis + the lane opening), M8-l/D93, M8-k/D92, M8-j/D91,
  M8-h/D90, M8-g/D89, M8-e/D88.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.6 / versionCode 5** (built at 078e3a5, APK
  34,951,881 B, sha256 bc4b239e…d1fe8, the pinned D85 debug
  certificate — installs over debug.5 in place): everything
  debug.5 carried PLUS the D92 UI unification and the engine-traced
  data usage. The keystore ceremony is COMPLETE on Ali's machine
  (M8-i-b); all android releases are regular releases in the private
  repo (D89). On top of the shipped state, the account-gate lane ran
  its full design arc and is now CLOSED: A0 delivered `docs/auth/`
  (nothing built), A0-R2 recorded D95 (admin-only accounts) and D96
  (the two-edition direction), and **A0-R3 CANCELLED the whole
  feature (D97, 2026-10-18) before a line of code existed — no
  login, no server, no editions; the pack is kept as bannered
  history; doc 07 §9's row is closed as cancelled; ADR-AUTH-0001
  (doc 08) is retired.** The A0 session also fixed
  `mobile/nonblock_unix.go`'s missing `//go:build unix` (the M4
  defect that kept `GOOS=windows go build ./...` from ever
  compiling) — a host-side fix only; no Android behavior change by
  construction (every android ABI is unix).
- **Branch and commit:** `main`, the A0-R3 commit (the auth
  cancellation — docs only; see `git log -1` at handoff time) on
  top of 6a7bce4 (A0-R2, the D95/D96 pack revision) on top of
  60bfa13 (A0, the auth spec pack + the nonblock fix) on top of
  cf136aa (M8-m); worktree clean after the commit. The release tag
  `v1.6.6-android-debug.6` points at 078e3a5 (M8-k) and is the
  repo's Latest; debug.5 → ad02980, debug.4 → cb28507 (helper-only),
  debug.3 → 2a8b678 (the device-circle APK), debug.2 → 096e8c0,
  debug.1 → f61bfae (superseded; debug.1's page keeps the D85
  troubleshooting guide).
- **Files changed this phase (A0-R3, docs only):** `DECISIONS.md`
  (D97 — the cancellation ruling, verbatim quote + supersession +
  bounds), `docs/auth/` (the dated CANCELLED banner on all six
  files; full notice in 00, one-line banners in 01–05; the pack
  body text untouched), doc 06 (the A0-R3 session record), doc 07
  (§9's account-gate row closed as cancelled + the closing
  paragraph), doc 08 (ADR-AUTH-0001 RETIRED + the Status line),
  doc 09 (this update), doc 10 (the A0-R3 status note + the D-range
  fix), `README.md` (the docs-map line), `PROGRESS.md`,
  `CHANGELOG.md` (the Planned section rewritten as cancelled), and
  `SESSION_HANDOFF_PROMPT.md` (rewritten for the single-workstream
  next session). ZERO code files — by construction nothing of the
  gate ever existed to remove. No release lane ran — nothing
  app-facing changed; the D84/D89 discipline held.
- **Tests run and actual results (this session, A0-R3):** docs-only
  round — no code file was touched, so no build/test lane applies;
  `go vet ./...` run on the untouched tree as a health check: clean
  (exit 0). The A0/A0-R2 baselines stand as their own evidence
  (vet clean; 18/18 packages ok three consecutive passes at A0;
  `GOOS=windows go build ./...` exit 0). The standing ⛔ re-confirmed
  as ever: no /dev/kvm, no device — the UI consistency on Ali's
  actual devices and the video-start delay both wait for his round.
- **The environment is RESTORED and READY** (2026-10-18, A0 — the
  fifth wipe): ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1
  (javac), the Android SDK (cmdline-tools 12.0 via the versioned
  11076708 URL, platform-tools 37.0.1, platforms 26+36, build-tools
  36.1.0, NDK 28.2.13676358), and gomobile/gobind at the pinned
  x/mobile version (~/go/bin). Source `~/sdk/env.sh` before any
  build. If a future session finds ~/sdk wiped again, reinstall per
  doc 06 §1 (mind the URL wrinkle) — but VERIFY first; do not
  reinstall over a working toolchain.
- **Current architecture decision / confidence:** the auth lane is
  CLOSED — ADR-AUTH-0001 (doc 08) is RETIRED by D97 (2026-10-18,
  A0-R3): the account gate was cancelled before implementation, so
  no auth architecture exists or will exist; D94/D95/D96 remain as
  history and `docs/auth/` as the bannered design record. The
  live architecture decisions are the Android-port ADRs
  (ADR-0001…0012, doc 08) — unchanged and evidence-backed. Next
  decision ID: **D98**.
- **Blocking questions:** none blocking the feedback loop. The
  account-gate questions (backend location, account creation,
  fail-open/closed, token design, password storage, brute-force
  form, edition packaging) are all MOOT — cancelled with the
  feature by D97, never ruled on their merits. The standing inputs
  are **Ali's cross-device UI check on debug.6** (does the phone now
  match his Windows/Linux builds? the Data usage numbers sane?), the
  still-open **diagnostics report from a slow-video moment** (the
  debug.5 timing lines name the phase; plus phone model + Android
  version + every-video-or-first-only), and the rest of the test
  circle (rows 4–8, rotation/TalkBack). D93's desktop input stands:
  Ali rebuilds the Windows exe and walks README's manual-check
  steps 4–5.
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-n — ingest Ali's results from v1.6.6-android-debug.6**
  (unchanged from the A0 handoff: the cross-device UI verdict, the
  data-usage round, the slow-video diagnostics report with phone
  model + Android version, the rest of doc 10's circle, D93's
  desktop check on the rebuilt exe). Root-cause what the evidence
  names; fix at the correct layer; bump `versionCode` and cut
  `v1.6.6-android-debug.7` through the same regular-release lane
  (D89) WHEN an app code change warrants it — push the code commit
  BEFORE creating the release (the full-SHA target_commitish 422s
  against an unpushed SHA). The A-series (A1 the backend, A2 the
  client gate, A3 hardening) is CANCELLED with the feature (D97) —
  those Task IDs are retired; do not open them unless a future Ali
  ruling re-opens auth (which would supersede D97 and require
  revalidating docs/auth/ against the then-current tree first).
  The environment is READY (verify with `. ~/sdk/env.sh && go
  version` first; expect wipes at any boundary and reinstall per
  doc 06 §1 + the URL wrinkle when they land). Open offer on the
  table, awaiting Ali's ruling: attaching the APK to every release
  page (relaxing D84's no-duplicate discipline) — do NOT do it
  unilaterally. The dormant alternatives when no feedback arrives:
  the desktop release lane (nine rounds under CHANGELOG
  "Unreleased") or public-release engineering — ask Ali which, do
  not guess.
