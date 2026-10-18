# 01 — Requirements (A0)

> ⛔ **CANCELLED — D97 (2026-10-18, A0-R3): the account gate will
> never be built; nothing below will be implemented. See
> `00-start-here.md` and `DECISIONS.md` D97. Kept as history.**

**Status:** draft for Ali's review; every numbered requirement carries
its verification idea for the A-series implementation. Requirements the
rulings must settle are marked **OPEN (a)–(f)** — see 00's table.
**Revised A0-R2 (2026-10-18): (b) is RULED (D95); OPEN (g) is new
(D96, the two-edition split) — §6 below.**

## 0. The objective (D94, verbatim intent)

Make Younblock members-only: a person must hold a verified
username/password before the app can be used, on every edition —
Windows, Linux, Android (and the web edition inherits it, because the
gate lives in the engine, not in a shell). The objective is
**exclusivity** — controlling who may use the app — not any change to
how traffic is treated once inside.

> **A0-R2 revision (D96, 2026-10-18):** the objective now splits by
> edition. The **members edition** is exclusive as written above. The
> new **free edition** is ungated — no account, no server — and keeps
> today's behavior and claims verbatim. In this pack, "every edition"
> from here on means "every edition that carries the gate" (see §6,
> AUTH-040…044).

## 1. Gate enforcement (the load-bearing requirement)

- **AUTH-001 (engine-enforced):** the gate lives in the engine, like
  capabilities and consent: `Engine.Start()` refuses to begin a connect
  run when the app is not signed in, with a typed error code
  (`AUTH_REQUIRED`) in the engine's existing UI-error vocabulary — the
  same machinery `NO_TARGET_APPS` uses today (engine.go's typed-code
  path, the facade's whitelist at mobile.go:199). No shell may start
  traffic without the engine, so **every shell inherits the gate by
  construction** — the Windows/Linux Wails shell, the Android
  facade/WebView, and `cmd/core --web`. Nothing is enforced only in a
  UI layer (a UI-only gate would be a claim the engine does not back —
  the D77–D94 rule).
- **AUTH-002 (signed-out is a state, not a dialog):** when signed out,
  the connect control is not offered at all. The shared screen shows a
  signed-out state before the connect controls (D92's one-UI rule: the
  same screen, the same layout, on all three platforms), with a sign-in
  form and an honest line about why (the app is members-only).
- **AUTH-003 (token persistence, never passwords):** the device
  persists a token + its expiry — **never the password** — in its own
  file, separate from settings.json (e.g. an auth file in the
  platform's ConfigDir), 0600, not synced, not backed up to any cloud,
  cleared on sign-out. The password exists in memory only while the
  sign-in request is in flight.
- **AUTH-004 (login precedes consent on Android):** the connect flow
  order becomes: **sign-in (if needed) → first-run explanation (once) →
  `VpnService.prepare()` consent → service/TUN → engine start**. Login
  comes before consent (doc 04 §6's ordering gains one step at the
  front; the consent dialog is never shown to a signed-out user).
- **AUTH-005 (offline startup):** with no network at launch, a
  still-valid cached token lets the app open into its normal
  signed-in state; the engine then applies the offline-grace ruling
  **OPEN (c)** at connect time. A signed-out device with no network
  simply cannot sign in — honest message, no workaround.
- **AUTH-006 (token expiry mid-session):** if a token expires or is
  revoked (admin disabled the user) while connected, the engine does
  **not** cut an active session mid-stream; it requires a valid sign-in
  at the next connect, and the notification states it plainly. (Cutting
  live traffic to enforce expiry was considered and rejected for v1:
  visible breakage of the user's stream for no traffic-safety gain; the
  gate's job is who may start, not who may finish.)
- **AUTH-007 (FGS notification honesty):** the Android foreground
  service notification never claims a signed-out state is connected;
  a signed-out exit stops the service like any other stop.

## 2. The backend (a small self-hosted HTTPS auth service)

- **AUTH-010:** an HTTPS service with: login (verify credentials, issue
  token), verify (is this token valid right now), refresh (roll the
  token), logout (revoke), and the admin operations. Endpoints and
  wire format in 03.
- **AUTH-011 (user database):** a users table (id, username — unique,
  password hash, created, disabled, expires) + a sessions table + an
  audit log; schema in 02 §4. Passwords are stored only as
  argon2id/bcrypt hashes **OPEN (e)** — never plaintext, never
  recoverable; the only reset path is an admin reset.
- **AUTH-012 (no recovery questions):** no secret questions, no
  email-based recovery, no self-service reset. A locked-out member
  contacts the admin. (Matches the no-email-accounts reality of the
  member circle and keeps the server's data minimal.)

## 3. Admin surface

- **AUTH-020:** the admin can: create, disable, enable, reset password,
  set expiry, list users with last-login. Delivered as a minimal
  server CLI and/or a minimal web admin — **OPEN (f)** carries the
  form details; 04 §2 recommends CLI-first.
- **AUTH-021:** disable takes effect at the next verify/refresh — the
  server answers "not valid"; combined with AUTH-006's semantics.

## 4. Privacy statement changes (drafted now, disclosed when shipped)

- **AUTH-030:** when the gate ships, the standing claims change from
  "no account, no data collection" to the drafted copy in 05 §6 —
  published in the release notes, the in-app Privacy sheet, README and
  doc 07 §1 at that release, never silently (D94 ruling 2).
- **AUTH-031 (what the gate does NOT change):** the proxying stays
  local: no VPN server, no IP hiding, no added encryption, usage
  history stays on-device. The gate gates usage, not traffic.

## 5. Non-goals (stated as plainly as the goals)

- **Not DRM:** the gate is not copy protection. A determined member can
  extract and share a token, or patch a client to skip the check; 05 §2
  bounds this honestly and lists what raises the cost. No
  obfuscation/anti-tamper claims will be made (they would be claims the
  implementation cannot back).
- **Not a traffic change:** zero new processing of routed traffic — the
  allow-list, DoH, fragmentation, .ir-direct rules are untouched.
- **Not an identity system:** no profiles, no emails required, no
  per-device registration, no analytics. One username+password per
  member; the server stores the minimum 05 §6 lists.
- **Not a CAPTCHA/anti-bot system in v1** — **OPEN (f)**; brute-force
  protection is rate-limiting, stated in 03 §7.

## 6. The two editions (NEW, A0-R2 — OPEN (g); design draft per D96)

Added 2026-10-18 from Ali's direction (D96): "two apk version, one
with auth, one without (free for all)". Verification ideas included;
nothing below is built, and (g1)–(g4) wait on Ali's rulings.

- **AUTH-040 (build-time only, one source tree):** two APKs come from
  the same tree — the members edition compiles the gate in; the free
  edition compiles it out entirely. Never a runtime toggle: one APK
  with a flag is not "two versions", and it would place the gate's
  code inside the free APK too. Mechanism (design; A2 implements only
  after the rulings): Go build tags on the engine gate and the facade
  (the pattern the A0 `//go:build unix` fix used) + Gradle product
  flavors on Android.
- **AUTH-041 (the free edition is gate-free by construction):** no
  auth client package, no token file, no signed-out state, no
  `auth` capability bit, no auth methods in the facade — verifiable
  by absence (the free APK's engine is exactly today's engine). It
  has no server dependency at all: it keeps working forever, even if
  the auth server dies or is never paid for again. Its behavior and
  its privacy claims are today's, verbatim.
- **AUTH-042 (the members edition is the gate as designed):**
  AUTH-001…007 apply as written — nothing is weakened by the split;
  the gate code is identical to the single-edition design.
- **AUTH-043 (privacy copy splits per edition, disclosed together):**
  the free edition keeps the standing "no account, no data collection"
  claims verbatim (still literally true of it); the members edition
  ships 05 §6's drafted copy. Both disclosed at the first release that
  ships the split — never silently (D94 ruling 2).
- **AUTH-044 (the release lane carries both, clearly labeled):** two
  APKs per release cut, one per edition; separate `versionCode`
  sequences if the applicationIds differ (**OPEN g1**); the signing
  story is unchanged (the debug keystore for the test circle, the
  release keystore for public — D84/D88/D90 discipline).
- **OPEN (g) sub-rulings, none a decision yet:** (g1) package identity
  — side-by-side install (recommended: an `.auth` applicationId
  suffix) vs one replacing package; (g2) distribution channel per
  edition; (g3) scope — Android only first (recommended; the same Go
  tags extend the split to the desktop shells later) vs desktop from
  day one; (g4) the two names for release notes and privacy sheets.
