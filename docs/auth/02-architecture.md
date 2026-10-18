# 02 — Architecture (A0)

**Status:** draft for Ali's review; revised A0-R2 (2026-10-18): §1
qualified per D96, §6 expanded with the platform analysis Ali asked
for (OPEN (a)), §7 added (the two-edition build, OPEN (g)). All
component names below that already exist are the repository's real
names (verified this session); names that would be new are marked
**(new)** and exist only as design.

## 1. Where the gate sits — engine-enforced, every shell inherits

> **A0-R2 note (D96, 2026-10-18):** "every shell" now means every
> shell **of the members edition**. The free edition compiles the gate
> out entirely (build tags — §7). Nothing else in this section
> changes: within the members edition, a shell still cannot bypass the
> gate, because a shell cannot start traffic without `Engine.Start()`.

The pattern follows the one the port already proved for consent
(D77's `StartGater`) and typed refusals (D81's `PlatformRefusalError`):
**the engine is the single brain** (doc 04 §1–3), shells only render
what it reports. The gate adds one more thing the engine refuses to do
without, exactly like capabilities and consent:

```text
Engine.Start() called (any shell)
→ engine checks the auth state (new, in-process):
    no token / expired / revoked → typed refusal AUTH_REQUIRED,
    phase stays "off" (no churn, no listener, no TUN, no consent
    dialog — Start refuses before PhaseConnecting)
→ valid cached token within its offline grace → the connect run
    proceeds unchanged (listener → attach → QuickCheck …)
→ valid token needing server confirmation → the connect run waits
    for the verify answer inside the existing start-gate door
    (the D77 seam), holding "connecting" honestly
```

- The check is in the engine (`internal/engine`), **not** in a shell —
  so the Windows/Linux Wails shell, the Android facade
  (`mobile/` → AAR → Kotlin), and `cmd/core --web` all inherit it by
  construction (AUTH-001). A shell cannot bypass it because a shell
  cannot start traffic without `Engine.Start()`.
- The typed code (`AUTH_REQUIRED`) travels the existing vocabulary path:
  the engine's error codes (engine.go's typed switch), the JSON API
  (`errResult`/`mapErrorCode`, jsonapi.go), and the facade whitelist
  (mobile.go's code list) — the same road `NO_TARGET_APPS` travels.
- **Capabilities grow one field (new):** `auth: true` on every platform
  once the gate ships, so the shared screen renders the signed-out
  state uniformly behind it (the D92 one-UI rule; same mechanism
  `keepInTrayOnClose` uses behind `capabilities.tray`).

## 2. Components

```text
┌─ device (each member) ─────────────────────────────┐
│ shells: Wails (Win/Linux) · Android WebView · web  │
│   ↓ same JSON API                                 │
│ engine: auth check (new) → connect run (unchanged) │
│ auth client (new internal package): token cache,   │
│   login/verify/refresh calls, offline-grace clock  │
└───────────────┬────────────────────────────────────┘
                │ HTTPS (TLS, system CA) — only auth
                │ traffic; never the routed apps' traffic
┌───────────────▼────────────────────────────────────┐
│ auth server (new, self-hosted — OPEN (a)):         │
│  /v1/login /verify /refresh /logout + /v1/admin/*  │
│  user DB (users · sessions · audit)                │
│  admin CLI (A1) / minimal web admin (A2+, 04 §2)   │
└────────────────────────────────────────────────────┘
```

- **auth client (new):** a small `internal/authclient`-style package —
  no UI, no state machine of its own; it answers questions
  ("valid token? expires when?") and performs requests. It owns the
  token file (AUTH-003: 0600, separate from settings.json, in the
  platform's `ConfigDir()`), and never holds the password beyond the
  in-flight login request.
- **auth server (new):** one small service; language/runtime is an A1
  decision recorded then (the repo is Go; a Go service keeps one
  toolchain, one vendoring story, one test discipline — that is the
  recommendation, not a ruling). It talks to nothing else; it sees
  only auth traffic (05 §6 states exactly what it stores).
- **The traffic path is untouched.** The tunnel, allow-list, DoH,
  fragmentation, .ir-direct, usage history — none of them learn the
  gate exists. Login traffic itself does not go through the tunnel
  (chicken-and-egg: sign-in happens before connect; it is ordinary
  HTTPS from the device to the auth server).

## 3. Token persistence on device

- File: `ConfigDir()/auth.json` (new) — `{ "token": "…", "expiresAt":
  "…", "username": "…" }`. 0600, single user per device profile, no
  cloud backup, cleared on sign-out. **No password is ever written to
  disk** (AUTH-003). Settings.json learns nothing about auth (the
  settings schema stays at v4 — no migration owed).
- The engine reads it only through the auth client; the facade gains
  sign-in/sign-out/get-auth-state methods in A2 (string JSON, the
  established bridge shape — no internal objects cross).

## 4. Data model (server)

```text
users:     id · username (unique) · password_hash · created_at
           · disabled (bool) · expires_at (nullable)
sessions:  id · user_id → users.id · token_hash (sha-256 of the
           opaque token) · created_at · last_seen_at · expires_at
           · revoked (bool)
audit_log: id · actor (admin|user:<id>|system) · action
           (created|disabled|enabled|reset|expired|login_ok|
            login_denied|logout|…) · target · at · detail (never a
            password, never a token value)
```

- Opaque tokens (recommendation, **OPEN (d)**): the server stores only
  `sha256(token)`; the device holds the token itself. Disable/expire
  checks are answered from `users` at verify/refresh time.
- SQLite is the recommended store for a friends-circle scale (one
  file, WAL mode, trivially backed up — 04 §5); the choice rides
  **OPEN (a)**'s ruling.

## 5. What the gate does not touch (explicit)

- No VPN server appears (the proxy stays local — the standing
  truthful-claims set, doc 07 §1).
- The engine's connect sequence, self-test, recovery, diagnostics
  report, usage history: unchanged. The diagnostics report gains no
  auth fields (the report stays redacted exactly as today; a login
  event is not network evidence).
- The debug/release signing lanes, the keystore ceremony, D84/D89
  release discipline: unchanged.

## 6. Deployment shape (OPEN (a) — recommendation + the 2026-10-18
platform analysis Ali asked for)

The A0 recommendation stands: **one small always-on host** — the auth
service behind system TLS, the DB file on the same box, offline
backups pulled on a schedule (04 §5). No container required; a systemd
unit is enough. Sizing for a friends-circle (~tens of members): ~1
vCPU / 1 GB RAM / 20 GB disk — login/verify is rare traffic (once per
member per launch at most, plus refresh windows).

Ali asked (2026-10-18): *can it be hosted on Cloudflare Workers /
GitHub Pages / Vercel / Netlify?* The honest matrix — all figures are
the vendors' public free-tier documentation, re-checked at A1; none
of this is a ruling:

| Host | Can it run the auth backend? | The trade-offs, stated plainly |
|---|---|---|
| GitHub Pages | **No — never.** Static hosting only: no server code, no database, no execution. | Still useful for the project website and the APK downloads (GitHub Releases is the better binary host). |
| Cloudflare Workers + D1 | **Yes — the only single-vendor $0 path.** API + a SQLite-compatible DB (D1) in one place; free tier ~100k requests/day, D1 ~5 GB / 5M reads / 100k writes per day — far beyond this scale. | (1) The service is written in TypeScript (or Go compiled to WASM within the low-MB script-size limits) — the one-Go-toolchain story is lost for the server; the client engine stays Go regardless. (2) **Per-request CPU budgets (~10 ms on the free tier) cannot run the A0 argon2id parameters** (03 §5) — hashing must be re-tuned down and 05 §3's cracking-resistance claim re-worded to match, honestly. (3) Platform lock-in (D1's API). (4) `*.workers.dev` has been intermittently filtered from Iranian ISPs — a custom domain in front of Cloudflare is the robust answer; the free tier needs no payment method, paid plans do. |
| Vercel | Partly. Go serverless functions are supported, but there is **no persistent disk** — the DB must live at a second vendor (Neon / Turso / Supabase). | Two dashboards, two failure domains; state externalized. The hobby tier's terms restrict use from sanctioned countries including Iran — a real account risk to verify before relying on it. `*.vercel.app` has been intermittently filtered from Iranian ISPs. |
| Netlify | Partly — same shape as Vercel: functions + an external DB vendor. | Same trade-offs; no advantage over the alternatives for this use case. |
| Small always-on host (the recommendation) | **Yes, as designed** — the whole A0 spec runs unmodified: Go + SQLite + full-strength argon2id, one binary, systemd, offline backups (04 §1/§3). | A few US$/month at 1 vCPU / 1 GB — or genuinely-free always-on tiers (e.g. Oracle's always-free ARM) if a card at signup can be arranged. **The practical constraint from Iran is the payment method, not the price** (a non-Iranian card or crypto via a provider that accepts it are the usual answers). |

**The cross-cutting hard requirement, whichever host wins (a):** the
auth endpoint must be **reachable from Iranian ISPs as ordinary
HTTPS**. Login happens *before* connect (AUTH-004) and never rides the
tunnel — the tunnel only ever carries the routed apps' traffic (§2
above), so the login must cross the Iranian filter on its own. If the
host's domain is blocked or throttled from Iran, members cannot sign
in (fail-closed answers honestly — but the members edition is useless
while it lasts). **Reachability from inside Iran is therefore the
primary hosting criterion.** What the design can verify from the
sandbox: nothing — the design team cannot test Iranian-ISP paths from
here, and says so; the one-minute check is Ali's (a phone, no VPN,
open the candidate domain). A custom domain (any non-`.ir` — `.ir`
cannot sit on Cloudflare under the sanctions rules) proxied through
Cloudflare's edge is the recommended shape on every row of the table
above; bare vendor subdomains (`*.workers.dev`, `*.vercel.app`,
`*.netlify.app`, `*.github.io`) have all been reported filtered from
Iranian ISPs at times.

## 7. The two editions (NEW, A0-R2 — OPEN (g); the build mechanism per D96)

Design only; A2 implements after the rulings. One source tree, two
editions, split at build time — never a runtime toggle (01 §6
AUTH-040):

- **Go side — build tags** (the pattern the A0 `//go:build unix` fix
  just proved in this tree): the engine's gate files come in a pair
  (`gate on` / `gate off`) behind one tag; the facade's auth methods
  and the `auth: true` capability bit compile only into the members
  build; the auth client package is linked only by the members build.
  In the free build there is no gate to call, no token file to read,
  and no auth method in the AAR — verifiable by absence.
- **Android side — Gradle product flavors**: one `edition` dimension,
  `free` and `auth`; the flavor selects the Go tag set for `gomobile
  bind` and (per the OPEN g1 ruling) the applicationId — the
  recommendation is a suffix (e.g. `io.github.assadigit.younblock.auth`)
  so both editions install side-by-side on one phone. Separate
  `versionCode` sequences when the ids differ; the signing story is
  unchanged (D84/D88/D90).
- **The honest split:** the free edition is server-independent forever
  (it works even if the auth server dies or is never paid for again —
  that is what "free for all" means, stated, not implied); the members
  edition is exactly the gate as designed in §1–§4 above — identical
  code, identical bounds (05 §2). The desktop shells can join the same
  split later via the same Go tags if Ali rules so (OPEN g3).
