# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the A0 round (2026-10-18, the
account-gate spec session: the six-document design pack delivered
under docs/auth/ per D94's design-before-code rule, plus one
two-line host-side fix — `GOOS=windows go build ./...` had never
compiled since M4; nothing app-facing changed, debug.6 stays Latest,
no release was cut) — **Phase 2 host-complete; the test circle
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
  repo (D89). On top of the shipped state, the account-gate lane now
  has its DESIGN: `docs/auth/` (A0) — nothing built; A1 (the auth
  backend) and A2 (the client engine gate) run only after Ali rules
  on the six questions (D95+); the privacy-copy change ships with
  the gate, never silently (doc 07 §9's new open row carries it).
  The A0 session also fixed `mobile/nonblock_unix.go`'s missing
  `//go:build unix` (the M4 defect that kept `GOOS=windows go build
  ./...` from ever compiling) — a host-side fix only; no Android
  behavior change by construction (every android ABI is unix).
- **Branch and commit:** `main`, the A0 commit (the auth spec pack
  + the nonblock fix — see `git log -1` at handoff time) on top of
  cf136aa (M8-m); worktree clean after the commit. The release tag
  `v1.6.6-android-debug.6` points at 078e3a5 (M8-k) and is the
  repo's Latest; debug.5 → ad02980, debug.4 → cb28507 (helper-only),
  debug.3 → 2a8b678 (the device-circle APK), debug.2 → 096e8c0,
  debug.1 → f61bfae (superseded; debug.1's page keeps the D85
  troubleshooting guide).
- **Files changed this phase:** `docs/auth/` (the six new spec files
  — the session's main output), doc 06 (the A0 session record),
  doc 07 (§9's new open row + its closing paragraph), doc 08
  (ADR-AUTH-0001 + the Status line), doc 09 (this update), doc 10
  (the status note), PROGRESS.md, CHANGELOG.md (a Fixed line under
  Unreleased + the Planned note), README.md (the docs map),
  SESSION_HANDOFF_PROMPT.md (rewritten for the next session), and
  ONE code file: `mobile/nonblock_unix.go` (+2 lines: the missing
  `//go:build unix` constraint + the blank line). No release lane
  ran — nothing app-facing changed; the D84/D89 discipline held.
- **Tests run and actual results (this session, on the fixed tree):**
  the environment was RESTORED from the fifth wipe first (toolchain
  reinstalled per doc 06 §1; verified: go1.27.2, javac 21.0.12.1,
  sdkmanager 12.0, gomobile help bind works). Then the full baseline:
  `npm ci` → svelte-check **0 errors 0 warnings**; vite build clean
  (✓ 3.99 s); the webui dist CLEAN-synced (D74 — the fresh clone
  otherwise fails vet on main.go's embed pattern and mobile's
  TestWebUILifecycle on the placeholder index: the documented
  source-only-checkout state); `go vet ./...` clean; `go test
  -count=1 ./...` **18/18 packages ok — three consecutive green
  passes, zero FAIL lines**; gofmt clean on the touched file;
  `GOOS=windows go build ./...` **exit 0 — first time since M4**
  (the defect it exposed is fixed in this commit). The standing ⛔
  re-confirmed as ever: no /dev/kvm, no device — the UI consistency
  on Ali's actual devices and the video-start delay both wait for
  his round.
- **The environment is RESTORED and READY** (2026-10-18, A0 — the
  fifth wipe): ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1
  (javac), the Android SDK (cmdline-tools 12.0 via the versioned
  11076708 URL, platform-tools 37.0.1, platforms 26+36, build-tools
  36.1.0, NDK 28.2.13676358), and gomobile/gobind at the pinned
  x/mobile version (~/go/bin). Source `~/sdk/env.sh` before any
  build. If a future session finds ~/sdk wiped again, reinstall per
  doc 06 §1 (mind the URL wrinkle) — but VERIFY first; do not
  reinstall over a working toolchain.
- **Current architecture decision / confidence:** ADR-AUTH-0001
  (from D94 ruling 1: the gate is engine-enforced, every shell
  inherits; elaborated in docs/auth/02). Confidence: high on the
  architecture; the six open questions are NOT decided — backend
  location, account creation, fail-open/closed, token design,
  password parameters, brute-force form each need Ali's ruling
  (D95+) before A1/A2 implement anything. D94's own framing stands:
  the privacy-claim conflict is named, the copy changes the day the
  gate ships, never silently. Next decision ID: D95.
- **Blocking questions:** none blocking the feedback loop. The
  account-gate lane now waits on **Ali's six rulings** (docs/auth/00's
  table: backend location; admin-only vs open signup; fail-open vs
  fail-closed offline; token/session design; password storage;
  brute-force protection + the login form) — they gate A1/A2, not
  the device round. The standing inputs are **Ali's cross-device UI
  check on debug.6** (does the phone now match his Windows/Linux
  builds? the Data usage numbers sane?), the still-open
  **diagnostics report from a slow-video moment** (the debug.5
  timing lines name the phase; plus phone model + Android version +
  every-video-or-first-only), and the rest of the test circle (rows
  4–8, rotation/TalkBack). D93's desktop input stands: Ali rebuilds
  the Windows exe and walks README's manual-check steps 4–5.
- **Exact next task:** TWO workstreams, the feedback loop still
  first. **(1) Task ID M8-n — ingest Ali's results from
  v1.6.6-android-debug.6** (unchanged from the A0 handoff: the
  cross-device UI verdict, the data-usage round, the slow-video
  diagnostics report with phone model + Android version, the rest
  of doc 10's circle, D93's desktop check on the rebuilt exe).
  Root-cause what the evidence names; fix at the correct layer;
  bump `versionCode` and cut `v1.6.6-android-debug.7` through the
  same regular-release lane (D89) WHEN an app code change warrants
  it — push the code commit BEFORE creating the release (the
  full-SHA target_commitish 422s against an unpushed SHA).
  **(2) Task ID A1 — the auth backend** — runs ONLY after Ali rules
  on the six questions (D95+): a small self-hosted HTTPS service
  (login/verify/refresh/logout + admin), the user DB, the admin
  CLI, per docs/auth/02–04; A2 (the client engine gate) follows.
  The environment is READY (verify with `. ~/sdk/env.sh && go
  version` first; expect wipes at any boundary and reinstall per
  doc 06 §1 + the URL wrinkle when they land). Open offer on the
  table, awaiting Ali's ruling: attaching the APK to every release
  page (relaxing D84's no-duplicate discipline) — do NOT do it
  unilaterally.
