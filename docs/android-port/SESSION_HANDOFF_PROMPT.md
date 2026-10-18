# Prompt — Continue Younblock: the device-test feedback loop + the account-gate design lane (after the M8-m gap analysis / D94)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M8-m / D94 round close)

- **Phase 2 is host-complete. M0–M8 are all done** (2026-10-18): the
  Go engine ported and hardened behind ADRs, the gomobile facade with
  the full binding set, the Kotlin VPN service (consent gate, honest
  routing report, the NO_TARGET_APPS refusal door, run generations),
  the mobile settings/UX, the audits, and the packaging.
- **M8-m (2026-10-18, D94): the gap-analysis round — analysis-only,
  zero code changes.** Ali asked for a complete gap analysis; the
  full report was delivered in chat (two read-only inventory passes
  + a doc baseline + an independent mobile.zip re-verification:
  100% of every round's instructions applied — the open item is
  data, VERIFIED_SITES, not code). The verdict: host-complete and
  green everywhere the sandbox can measure. The real gaps, in
  priority order:
  1. **The open slow-video defect** (video takes 30 s–1 min+ to
     start after a tap; feed loads fast). Every host-level suspect
     is eliminated WITH EVIDENCE (UDP/443 gets an instant ICMP
     port-unreachable — pinned by test; TCP rcvWnd 1 MiB; fragment
     pauses 1–10 ms; DoH cached/singleflight/prewarmed; the connect
     gate runs a real proxied YouTube check before "on"). The
     per-flow timing instrumentation ships in debug.5/6 (`tunnel up
     dns=Xms dial=Yms` / `first-byte` / `done` in the diagnostics
     ring). **The missing input is Ali's diagnostics report copied
     from a slow-video moment** (Menu → Diagnostics → Copy) + phone
     model + Android version + whether every video or only the
     first after connecting.
  2. **The largely unrun device test circle** (doc 10 §"Device test
     circle"): rows 4–8 (youtube.com per browser, .ir direct, stop
     paths, battery/Doze, network switch), rotation/TalkBack/font
     scaling, the cross-device UI verdict on debug.6, the
     data-usage round, and D93's desktop check (Ali's rebuilt exe
     walked through README manual-check steps 4–5).
  3. **The dormant desktop release lane**: the last desktop
     artifacts (v0.7.6) predate D86–D93 — nine rounds of changes
     sit under CHANGELOG "Unreleased", including the whole unified
     UI (D92), data usage, and the close ruling (D93).
  4. **Public-release engineering not started** (by choice, so
     far): debug-only signing (D84), no CI (no .github/ at all),
     no frontend unit tests, unsigned Windows exe, no update
     mechanism, no LICENSE file.
- **D94 (2026-10-18): the account-gate lane is OPEN — the project's
  next feature.** Ali's ruling: "Planning to add login authorization
  for this, like a normal VPN app, where users must have been
  signed up or input credible and verified user/password. The
  objective is to make the app more exclusive, so only people who
  have username password can use it. so we need login page etc.
  plan and design ahead for it's backend requirements for example
  user database, admin managing users etc." Design-first: the spec
  lane (Task ID A0) comes BEFORE any code, and the privacy-claim
  conflict is named up front — the standing truthful-claims set
  says "no account, no data collection", and that CHANGES when the
  gate ships (disclosed, never silently), while the proxying itself
  stays local (no VPN server, no IP hiding, no added encryption —
  the gate gates USAGE, not traffic).
- **The current test-circle build is `v1.6.6-android-debug.6`**
  (versionCode 5, built at 078e3a5, the pinned debug certificate —
  installs over debug.5 in place, settings survive, usage history
  started fresh): the cross-platform UI unification (D92) — Ali's
  mobile.zip instruction rounds applied to the ONE shared screen
  every edition embeds (Lucide icons, the "…" menu button, the
  sheet's in-header close, 0.80 em digit boxes, ping verdicts,
  honest -1 speeds, the ChitaCodes credit, the Other-websites
  section) PLUS the round-3 data usage (session bytes counted ONCE
  in the engine, usage.json — times and byte totals only, 500 cap,
  15 s crash autosave — the "X used" line, the Last-session line,
  the menu's Data usage section with a confirmed Clear). Play
  Protect's popup at install stays expected; the release keystore
  stays NOT-for-the-test-circle (D88/D90; the ceremony is complete
  on Ali's machine).
- **M8-l / D93 (in the tree, awaiting Ali's rebuilt-exe check):
  the desktop close ruling.** Closing the window is a REAL shutdown
  by default (the app leaves the taskbar; the shutdown path
  restores the Windows proxy first); tray-keeping is the explicit
  `keepInTrayOnClose` opt-in (settings.json, default false),
  surfaced on both screens behind capabilities.tray. No Android
  release rode this change; Ali rebuilds the Windows exe
  (build.bat / build.sh) and walks README's updated manual-check
  steps 4–5.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's TWO workstreams

### Workstream 1 — the device-test feedback loop (Task ID M8-n)

For each issue Ali reports from `v1.6.6-android-debug.6` (device
side) or his rebuilt desktop exe (the D93 close ruling):

- Root-cause first (the standing discipline: failures are
  root-caused and fixed, never retried away, never hidden). The
  diagnostics report (in-app, redacted) + device model/Android
  version are the expected inputs; for "YouTube still doesn't
  load" the discriminating evidence is the report's DoH/self-test
  sections plus whether the key icon was up (TUN established) and
  which surface (app vs browser, which browser).
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
  packages per ABI ×4, plus arm64 linking the full tree `./...` —
  do not batch-build with an empty package list: that builds the
  MAIN package, which legitimately requires cgo on arm/386/amd64);
  `gomobile bind`; **clean** `assembleDebug`; artifact verification
  (sha256, apksigner, badging, manifest dump — doc 06's records);
  commit.
- If feedback has NOT arrived when the session opens, do not block:
  proceed to Workstream 2 and re-check for messages before any
  release-cutting.

### Workstream 2 — the account-gate design lane (Task ID A0)

Ali wants the app members-only: a verified username/password gate,
a login page, a user database, and an admin surface for managing
users — "plan and design ahead". The deliverable of A0 is the SPEC
PACK (docs first, the M0–M2 pattern the Android port followed),
NOT code. Write it as new spec documents (suggest
`docs/auth/00-start-here.md`, `01-requirements.md`,
`02-architecture.md`, `03-auth-protocol.md`,
`04-admin-and-operations.md`, `05-security-review.md` — or extend
the android-port numbering if that fits better; record the choice),
then the ADRs into doc 08 + DECISIONS.md as the rulings land.

The design must cover, at minimum:

1. **Where the gate lives.** The recommendation to evaluate: gate
   enforcement in the ENGINE (like capabilities/consent — every
   shell inherits it once; the engine refuses to Start while
   unsigned-out, on every platform) with the login surface on the
   ONE shared screen (a signed-out state before the connect
   control — D92's one-UI rule keeps Windows/Linux/Android
   consistent by construction). Consider what the desktop editions
   do at launch, what Android does before consent, and how the
   state persists (token in settings.json? a separate auth.json?
   NEVER the password itself).
2. **The backend.** A small self-hosted authentication service
   (Ali's VPS?) speaking HTTPS: login (verify credentials, issue a
   token), verify/refresh (the app checks the token at startup /
   periodically), and admin operations. Database schema: users
   (id, username, password-hash, salt, created, disabled, expires,
   notes), sessions/tokens (opaque server-side sessions vs signed
   JWTs — with revocation-on-disable argued), audit log. Passwords:
   argon2id or bcrypt, never plaintext, never recoverable; admin
   resets instead of password recovery emails.
3. **The admin surface.** Who creates accounts (the "exclusive"
   objective points at admin-created users; open signup is the
   alternative — Ali's ruling needed): a minimal admin web page, a
   CLI on the server, or both. Operations: create, disable, enable,
  reset password, set expiry, list users, see last-login.
4. **The six open questions from D94 ruling 3 — each needs Ali's
   ruling before implementation:** (a) where the backend lives;
   (b) admin-created-only vs open signup with approval; (c)
   fail-open vs fail-closed when the auth server is unreachable
   (recommendation: fail-closed with a bounded offline grace —
   but the ruling is Ali's); (d) token/session design and expiry;
   (e) password storage parameters (the scheme is decided — the
   parameters are not); (f) brute-force protection (rate limiting,
   lockouts) and the login surface's exact shape.
5. **The truthful-claims change.** Doc 07 §1, doc 10 §2, the
   in-app Privacy sheet and the README all say "no account, no
   data collection". The spec must draft the NEW copy (what data
   the auth server holds, what login traffic leaves the device,
   retention) so the change ships disclosed, per D94 ruling 2. The
   proxying itself stays local — the spec must say what the gate
   does NOT change.
6. **Android specifics.** The VPN-slot consent flow already
   exists — where does login sit relative to it (login before
   consent is the natural order)? The WebView loads the loopback
   webui — the login screen is another webui state. Consider
   offline launch behavior, token expiry mid-session, and what
   the FGS notification says while signed out.
7. **Threat model, briefly.** What the gate protects against
   (casual sharing of the app beyond the member circle) and what
   it does NOT (a determined user extracting the token / patching
   the client — name this honestly; the client is open-source in
   Ali's hands).

While writing the spec, keep the feedback loop alive (Workstream
1 takes priority whenever Ali's reports arrive). Do NOT implement
login code in A0; the implementation rounds run as A1, A2, … after
Ali approves the spec and answers the six questions.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-n,
   M8-o, … ; the account-gate lane runs A0 (this spec round), then
   A1, A2, … for implementation. **The next decision number is
   D95.**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.6 edition + §"Device test circle"), doc 06 (the M8-c…M8-m
   records), doc 08 + `DECISIONS.md` (D1–D94; the D94 entry is the
   account-gate ruling — read it in full before designing), and the
   tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-m
   docs-only commit on top of 9615936). Preserve any uncommitted
   work; never reset it just to get a clean diff. If push is
   rejected (credential rotated), the session prompt carries a
   fresh token — update the `origin` remote URL with it.
4. Check whether Ali's device-test feedback from debug.6 has
   arrived (session messages). The expected first inputs: the
   cross-device UI match verdict (phone vs his rebuilt
   Windows/Linux builds), the data-usage round (counter climbing,
   Last-session line, menu totals, Clear with both buttons), the
   diagnostics report copied from a slow-video moment, and the D93
   desktop check (rebuilt exe, close = full stop; the opt-in keeps
   the tray). If present: Workstream 1 first. If absent: proceed
   to Workstream 2 (the A0 spec) and say so plainly.
5. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **18/18
   packages ok**, run twice consecutively); webui `svelte-check`
   (0/0) and `vite build`. D74 trap: a stale webui dist fails
   `TestWebUILifecycle` (in the mobile package) — sync
   `frontend/dist → internal/webui/dist` before judging.
   **~/sdk was wiped before M8-i and RESTORED at M8-i-a; wipes
   happen at ANY boundary — VERIFY first (`. ~/sdk/env.sh &&
   go version && javac -version`) and only reinstall if actually
   wiped (the system JRE has keytool but no javac, so a missing
   ~/sdk/jdk means builds fail; the cmdline-tools `12.0` alias URL
   404s — use the versioned `commandlinetools-linux-11076708_latest.zip`).**

## Environment (this sandbox resets without warning — verify, don't assume)

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**.
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk-21.0.12.1+1`.
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools,
  platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358, licenses
  accepted). Exact PATH / JAVA_HOME / ANDROID_HOME setup: doc 06.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore` — after a wipe + re-clone,
  assembleDebug signs identically with NO extra step.
- No /dev/kvm (re-verified at M8/M8-b/M8-c/M8-d). Disk is a ~9.9 GB
  box — clean Go/Gradle build caches before big builds.
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
  credential); preserve Windows/Linux desktop behavior (the full
  regression gate is part of every change set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D94 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped. The
  account gate doubles this discipline: auth is SECURITY — the spec
  must name its limits as plainly as its features.
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D95),
  `CHANGELOG.md`; append the worklog entry (Task ID M8-n / A0 / …);
  commit and push; cut the debug release with the bumped
  versionCode when APP code changed, and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.6 has arrived, and if so the exact first issue you will
address (Workstream 1); (5) if no feedback arrived, say so and begin
Workstream 2 (the A0 account-gate spec): state which spec documents
you will create and the first three things the design must decide.
Then execute.
