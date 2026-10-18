# 02 — Architecture (A0)

**Status:** draft for Ali's review. All component names below that
already exist are the repository's real names (verified this session);
names that would be new are marked **(new)** and exist only as design.

## 1. Where the gate sits — engine-enforced, every shell inherits

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

## 6. Deployment shape (recommendation; rides OPEN (a))

One tiny VPS: the auth service behind system TLS (a reverse proxy or
the service's own TLS), the DB file on the same box, offline backups
pulled on a schedule (04 §5). No container required; a systemd unit is
enough. Sizing for a friends-circle (~tens of members): ~1 vCPU /
1 GB RAM / 20 GB disk — login/verify is rare traffic (once per member
per launch at most, plus refresh windows).
