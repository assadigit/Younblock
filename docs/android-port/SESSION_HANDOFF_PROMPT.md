# Prompt — Continue Younblock: the device-test feedback loop + the account-gate implementation lane (after the A0 spec round)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the A0 round close, 2026-10-18)

- **Phase 2 is host-complete. M0–M8 are all done**, and the account-gate
  lane now has its DESIGN: the six-document spec pack under `docs/auth/`
  (delivered at A0, 2026-10-18), plus ADR-AUTH-0001 in doc 08 (the gate
  is engine-enforced, from D94 ruling 1).
- **A0 (2026-10-18): the account-gate spec round — delivered.** The
  session ran it because NO device feedback had arrived (zero issues on
  the repo, all states; none in the session messages). It produced
  `docs/auth/00-start-here.md` (the consolidated six questions +
  recommendations), `01-requirements.md` (AUTH-001…031), `02
  -architecture.md` (the engine-level gate on the real seams: Platform/
  Capabilities/StartGater D77/PlatformRefusalError D81; the typed-code
  road NO_TARGET_APPS travels; capabilities gain `auth: true`), `03
  -auth-protocol.md` (endpoints, token design, argon2id, brute-force),
  `04-admin-and-operations.md` (CLI-first admin, lifecycle table,
  backups), and `05-security-review.md` (the honest threat model + the
  drafted privacy copy). **Nothing is built — no login code, no server,
  no database exists anywhere.** The same session caught and fixed a
  latent M4 defect: `mobile/nonblock_unix.go` had no build constraint,
  so `GOOS=windows go build ./...` had NEVER compiled since M4 (the
  scoped shell builds always did — that is what "the whole shell"
  meant); one line (`//go:build unix`) fixed it; baseline re-verified
  green after; no release followed (nothing app-facing changed).
- **A0-R2 (2026-10-18, later the same day — docs only, no code): Ali
  replied to the A0 wrap-up and the session recorded it.** His ruling
  on (b) — "I create account - admin-only" — is **D95**: admin-only
  creation, no open signup, no approval queue (the surface FORM stays
  a recommendation riding (f)'s sibling). His new direction — "Also I
  want two apk version, one with auth, one without (free for all)" —
  is **D96**: the two-edition split (members carries the full gate;
  free contains zero auth code and no server dependency; build-time
  only — Go build tags + Gradle product flavors; the privacy copy
  splits per edition and both copies disclose together at the first
  split release). It opened **(g)** with sub-rulings g1–g4 (install
  identity, distribution, scope, naming) — none a decision yet. His
  hosting question (Cloudflare Workers / GitHub Pages / Vercel /
  Netlify) is NOT a ruling; the honest platform matrix is in
  `docs/auth/02` §6, and the recommendation stays the small always-on
  host. The whole auth pack was revised in place with dated A0-R2
  notes (00's table: b RULED, row g added; 01 §0 + new §6
  AUTH-040…044; 02 §1/§6/§7; 03 §5; 04 §2; 05 §2/§6).
- **The remaining open questions — each needs Ali's ruling before
  A1/A2 starts; D97+ is reserved for them:**
  (a) backend location — the platform matrix is in docs/auth/02 §6
  (recommendation: a small always-on host; the strictly-$0
  alternative is Cloudflare Workers + D1 with named trade-offs); (c)
  fail-open vs fail-closed when the auth server is unreachable
  (recommendation: fail-closed with a bounded offline grace, ~72 h);
  (d) token/session design and expiry (recommendation: opaque
  server-side session tokens, ~30-day sliding; open sub-item: single
  active session per account);
  (e) password storage parameters (recommendation: argon2id
  m=64 MiB t=3 p=4 — now also rides (a): full strength presumes an
  always-on host; serverless CPU budgets force a re-tune down);
  (f) brute-force protection + the login form + the admin surface
  form (recommendation: per-username and per-IP limits with
  admin-clearable lockout; the form is the shared screen's signed-out
  state; CLI first);
  (g) the edition packaging — g1 side-by-side install (recommended:
  an `.auth` applicationId suffix) vs one replacing package; g2
  distribution channel per edition; g3 Android-only first
  (recommended) vs desktop too; g4 the two names.
  Recommendations are NOT decisions — never record one as if ruled.
- **The open slow-video defect and the device test circle are
  unchanged** (the feedback loop stays the FIRST workstream):
  1. **The slow-video defect** (video takes 30 s–1 min+ to start; feed
     loads fast). Every host-level suspect is eliminated WITH EVIDENCE
     (UDP/443 gets an instant ICMP port-unreachable — pinned by test;
     the per-flow timing instrumentation ships in debug.5/6). The
     missing input is **Ali's diagnostics report copied at a
     slow-video moment** (Menu → Diagnostics → Copy) + phone model +
     Android version + whether every video or only the first after
     connecting.
  2. **The largely unrun device test circle** (doc 10 §"Device test
     circle"): rows 4–8, rotation/TalkBack/font scaling, the
     cross-device UI verdict on debug.6, the data-usage round, and
     D93's desktop check (Ali's rebuilt exe walked through README
     manual-check steps 4–5).
  3. **The dormant desktop release lane**: v0.7.6 predates D86–D93;
     nine rounds sit under CHANGELOG "Unreleased".
  4. **Public-release engineering not started** (by choice, so far):
     debug-only signing (D84), no CI (no .github/), no frontend unit
     tests, unsigned Windows exe, no update mechanism.
- **The current test-circle build is `v1.6.6-android-debug.6`**
  (versionCode 5, built at 078e3a5, the pinned debug certificate —
  installs over debug.5 in place, settings survive): the cross-platform
  UI unification (D92) + the engine-traced data usage. Play Protect's
  popup at install stays expected; the release keystore stays
  NOT-for-the-test-circle (D88/D90; the ceremony is complete on Ali's
  machine).
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's TWO workstreams

### Workstream 1 — the device-test feedback loop (Task ID M8-n) — STILL FIRST

For each issue Ali reports from `v1.6.6-android-debug.6` (device side)
or his rebuilt desktop exe (the D93 close ruling):

- Root-cause first (the standing discipline: failures are root-caused
  and fixed, never retried away, never hidden). The diagnostics report
  (in-app, redacted) + device model/Android version are the expected
  inputs; for "YouTube still doesn't load" the discriminating evidence
  is the report's DoH/self-test sections plus whether the key icon was
  up (TUN established) and which surface (app vs browser, which
  browser).
- Fix at the correct layer (engine/facade/Kotlin/WebView), keeping
  the D77–D94 honesty rules binding: the UI may only claim what the
  engine actually enforces; ⛔ gates are listed, never dropped.
- Record each closed checklist row with Ali's measured evidence
  (doc 07 §9 / doc 10 §"Device test circle"); rows without
  observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule: updates install over the previous debug build) and cut the
  next debug release — a REGULAR release in the private repo, never
  flagged prerelease (D89) — through the release lane (push the
  code commit BEFORE creating the release — the full-SHA
  target_commitish 422s against an unpushed SHA; upload the APK,
  verify via the API, record asset size + digest) WHEN a code
  change warrants it — do not re-release identical bytes. Full lane
  before release: desktop baseline; the compile matrix in its exact
  form (`go build ./internal/... ./mobile/...` = the 18 non-main
  packages per ABI ×4, plus arm64 linking the full tree `./...` — do
  not batch-build with an empty package list: that builds the MAIN
  package, which legitimately requires cgo on arm/386/amd64);
  `gomobile bind`; **clean** `assembleDebug`; artifact verification
  (sha256, apksigner, badging, manifest dump — doc 06's records);
  commit.
- If feedback has NOT arrived when the session opens, do not block:
  proceed to Workstream 2 and re-check for messages before any
  release-cutting.

### Workstream 2 — the account-gate IMPLEMENTATION lane (Task IDs A1, A2, …) — ONLY after the rulings

A0 (the spec) is done, and A0-R2 recorded D95 (b ruled: admin-only)
and D96 (the two-edition direction, OPEN (g) added). **A1 (the auth
backend) may not start until Ali rules on the REMAINING questions —
(a) and (c)–(f); A2 additionally needs (g)** — if the rulings have
not arrived, this workstream stays closed and the session says so
plainly. If they HAVE arrived: record them in DECISIONS.md first (from
D97), update `docs/auth/00`'s table to mark them ruled, then
implement per the pack, in the ladder it specifies:

- **A1 — the backend**: a small self-hosted HTTPS service
  (login/verify/refresh/logout + admin), the user DB (users/sessions/
  audit per docs/auth/02 §4), the admin CLI (docs/auth/04 §2's
  recommendation), host-tested end to end. Nothing deployed until Ali
  has a location for it.
- **A2 — the client engine gate**: the engine refusal (typed
  AUTH_REQUIRED on the NO_TARGET_APPS road), the `auth: true`
  capability, the auth client package + token file (never the
  password), the signed-out state on the shared screen (D92), Android
  login-before-consent ordering — desktop-first (host-testable),
  then Android wiring.
- **A3 — hardening + the privacy-copy change shipped together with
  the gate** (never silently — D94 ruling 2; docs/auth/05 §6 is the
  drafted copy; doc 07 §9's open row blocks that release until the
  copy changes with it).
- Each step: baseline green after it, scoped commits, evidence in
  doc 06 / PROGRESS.md; the debug build that first carries app code
  bumps versionCode and goes through the regular-release lane (D89).

While implementing, keep the feedback loop alive (Workstream 1 takes
priority whenever Ali's reports arrive).

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-n,
   M8-o, … ; the account-gate lane runs A1, A2, … for implementation
   (A0 the spec round and A0-R2 the ruling round are DONE). **The
   next decision number is D97 — reserved for Ali's rulings on the
   remaining open questions ((a), (c)–(g)); D95 and D96 are spent.**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.6 edition + §"Device test circle"), doc 06 (the M8-c…M8-m +
   A0 records), doc 08 + `DECISIONS.md` (D1–D96; the D94 entry is the
   account-gate ruling, D95/D96 the A0-R2 rulings — read them in
   full), `docs/auth/00-start-here.md`
   (the six questions + where each is specified), and the tail of
   `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the A0-R2 docs
   commit — the auth pack revision per D95/D96 — on top of the A0
   commit on top of cf136aa). Preserve
   any uncommitted work; never reset it just to get a clean diff. If
   push is rejected (credential rotated), the session prompt carries a
   fresh token — update the `origin` remote URL with it.
4. Check whether Ali's device-test feedback from debug.6 has arrived
   (session messages), AND whether his rulings on the remaining open
   questions ((a), (c)–(g)) have arrived. If device feedback is
   present: Workstream 1 first. If the rulings are present: record
   them (DECISIONS.md from D97) before any A1 code. If neither: say so
   plainly; the honest options are the desktop release lane (gap 3)
   or public-release engineering (gap 4) — ask Ali which, do not
   guess.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER (a fresh clone fails the Go lanes until the frontend is
   built): `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); then `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, run twice
   consecutively); `GOOS=windows go build ./...` (exit 0 — it has
   compiled since the A0 fix; a failure there is a regression).
   **~/sdk was wiped before A0 and RESTORED at A0; wipes happen at ANY
   boundary — VERIFY first (`. ~/sdk/env.sh && go version &&
   javac -version`) and only reinstall if actually wiped (the
   cmdline-tools `12.0` alias URL 404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`).**

## Environment (this sandbox resets without warning — verify, don't assume)

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**
  (the env.sh written at A0 also sets JAVA_HOME, ANDROID_HOME and
  puts ~/go/bin, platform-tools and cmdline-tools on PATH).
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk`.
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools
  37.0.1, platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358,
  licenses accepted; gomobile/gobind in `~/go/bin` at the pinned
  x/mobile). Exact pins: doc 06 §1.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore` — after a wipe + re-clone,
  assembleDebug signs identically with NO extra step.
- No /dev/kvm (re-verified every session). Disk is a ~9.9 GB box —
  clean Go/Gradle build caches before big builds.
- The sandbox kills background processes when a Bash command ends —
  run long builds as foreground chunks.
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew`.

If the sandbox was wiped (repo / toolchain / Go missing):

1. Re-clone into `/home/z/my-project/Younblock` from the remote above
   (the token lives in the old clone's `origin` if any copy survives;
   otherwise the session prompt carries it — re-cloning works without
   asking anyone).
2. Reinstall the toolchain exactly per doc 06's recorded pins (Go to
   `~/sdk/go`; JDK, SDK and NDK at the pinned versions; gomobile/
   gobind via `go install` at x/mobile v0.0.0-20261011034528-197770444867;
   module cache via proxy.golang.org; then rely on the committed
   `vendor/`).
3. Re-run the desktop baseline before any new code, and expect the
   first cold-cache run to be slow.

## Standing rules (unchanged)

- Evidence or it didn't happen: no claimed test pass, build, APK or
  push without a verified result. ⛔ with evidence for anything the
  environment cannot run — never fake completion, never silently
  narrow scope.
- Scoped commits; small steps; baseline green after each; no
  force-push; no unrelated changes; **no secrets or RELEASE signing
  keys committed** (D85's deliberate exception: the debug test-circle
  keystore is vendored — the well-known debug identity, not a
  credential; the exception does NOT extend to anything auth-related
  — no auth secret ever lives in the repo); preserve Windows/Linux
  desktop behavior (the full regression gate is part of every change
  set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D94 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped. The
  account gate doubles this discipline: auth is SECURITY — the spec
  and its implementation must name their limits as plainly as their
  features; no recommendation may be recorded as a ruling.
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D95, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-n / A1 / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed, and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.6 has arrived, and if so the exact first issue you will
address (Workstream 1); (5) whether his rulings on the remaining open
questions ((a), (c)–(g)) have arrived — if yes, record them (D97+)
before any A1 code; if no, say so plainly and pick up the honest
alternative (the desktop release lane or public-release engineering —
ask Ali which). Then execute.
