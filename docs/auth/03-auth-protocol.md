# 03 — Auth Protocol (A0)

**Status:** draft for Ali's review. Everything here is design; the
parameters marked **OPEN** wait on the six rulings (00's table).

## 1. Transport

- HTTPS only, validated against the system CA store. No custom pinning
  in v1 (pinning was considered; rejected for v1 because it adds a
  brittle failure mode for near-zero gain at this threat level — 05 §3
  revisits it honestly).
- The client sends: `Content-Type: application/json`, a `User-Agent`
  naming the app and version (`Younblock/<version> (<platform>)`).
- All responses are JSON; errors carry a machine-readable `code` and a
  human string. Codes: `invalid_credentials`, `locked`, `disabled`,
  `expired`, `rate_limited`, `server_error`.

## 2. Endpoints

```text
POST /v1/login     {username, password}
                    → 200 {token, expiresAt, username}
                    → 401 invalid_credentials · 423 locked · 429 rate_limited
POST /v1/verify    {token}
                    → 200 {valid: true, username, expiresAt}
                    → 200 {valid: false, reason: disabled|expired|revoked}
                      (a "not valid" answer is 200, not an error —
                       it is an answer, not a failure)
POST /v1/refresh   {token}
                    → 200 {token', expiresAt}   (old token revoked)
                    → 200 {valid: false, …}      (same semantics as verify)
POST /v1/logout    {token}                      → 204
GET  /v1/health                                   → 200 {ok}
--- admin (authenticated by the admin credential, 04 §2) ---
POST   /v1/admin/users            {username, password?} → creates
PATCH  /v1/admin/users/{id}       {disabled?|expiresAt?|newPassword?}
GET    /v1/admin/users            → list incl. last-login
GET    /v1/admin/audit?since=…    → audit events
```

- `POST /v1/admin/users` accepts an optional password; without one the
  server generates a strong random password and returns it **once**,
  for the admin to hand over privately (04 §2's flows).
- Login requests never carry a device identifier — no install ID, no
  hardware fingerprint, no analytics (05 §6). The server necessarily
  sees the client's IP and TLS metadata at request time; that fact is
  in the privacy copy, not hidden.

## 3. Token design (recommendation; OPEN (d))

- **Opaque session tokens**: 256-bit random from the server's CSPRNG,
  Base64url. The server stores `sha256(token)` in `sessions` (02 §4);
  the raw token exists only on the member's device and in the single
  response that carried it.
- Expiry ~30 days sliding: any verify/refresh within the window rolls
  `expiresAt` forward. Refresh revokes the old token at issue time.
- Revocation on disable is answered at the next verify/refresh
  (AUTH-021); combined with AUTH-006 (no mid-stream cut).

**JWT alternative (documented, not recommended):** stateless signed
tokens would remove the verify round-trip, but disable/revocation then
needs a server-side deny-list anyway — the statelessness saving
evaporates exactly when it matters (a disabled member), and the token
becomes readable by the member (it is a claim bag, not a secret).
Opaque + hashed-at-rest keeps the server the sole authority on
validity; that is the recommendation.

## 4. Session semantics on device

- The engine consults the token at `Start()` (02 §1). Validity windows:
  fresh token → connect proceeds; token older than the
  verify-interval → the connect run waits on `/v1/verify` inside the
  D77 start-gate door (phase honestly "connecting" while it waits).
- **Verify interval (recommendation):** once per launch, and at latest
  24 h. **Offline grace (OPEN (c)):** fail-closed with a bounded grace
  — recommendation 72 h from the last successful verify, after which
  `Start()` refuses with `AUTH_SERVER_UNREACHABLE` (a distinct, honest
  code — the UI says the auth server could not be reached, not
  "wrong password"). Never an indefinite open.

## 5. Password storage (OPEN (e))

- **argon2id, recommended parameters:** memory 64 MiB, iterations 3,
  parallelism 4, salt 16 bytes random per user, encoded with the
  parameters in the hash string (so they can be raised later without
  invalidating old hashes). Tuned on the actual server at A1 (the
  login latency budget: < 1 s at the friends-circle scale).
- **bcrypt fallback** (cost ≥ 12) if the chosen server runtime's
  argon2 story is weak — recorded at A1 with the runtime ruling.
- Never plaintext; never recoverable; no hints, no questions (AUTH-012).
- Password policy: minimum length 10, no composition rules, no forced
  rotation (current guidance; the admin reset path makes policy simple
  by design).

## 6. Fail-open vs fail-closed (OPEN (c))

The recommendation is **fail-closed with the bounded offline grace**
(§4). What that means operationally: a member on a plane / during an
auth-server outage keeps using the app up to the grace window; after
it, connects refuse with the honest `AUTH_SERVER_UNREACHABLE` message
until the server is reachable again. **Fail-open was considered and is
documented, not recommended**: it would make the exclusivity guarantee
hinge on the auth server's uptime in the opposite direction — every
outage would open the app to stale tokens indefinitely.

## 7. Brute-force protection (OPEN (f))

- Server-side only (the client cannot be trusted to enforce anything
  against itself — 05 §2).
- Per-username: 5 consecutive failures → lockout 15 min, doubling each
  repeat (15 → 30 → 60 …), capped at 24 h; admin can clear via
  `PATCH /v1/admin/users/{id}` (`locked` state is a users-row bit, so
  it survives restarts and lists in the admin surface).
- Per-source-IP: a global requests/minute ceiling on `/v1/login`
  (protects username-sweeping); returns `429 rate_limited`.
- No CAPTCHA in v1 (01 §5 non-goals); the lockout path is the honest
  answer. The login form itself: the shared screen's signed-out state
  (D92) — username + password, a sign-in button, an error line that
  shows the server's code verbatim, and nothing else.

## 8. What the protocol does not do

- No channel binding, no mTLS, no per-device certificates in v1 —
  stated plainly; 05 §3 says what that omits.
- No password change endpoint in v1 (admin reset covers it; a
  self-service change endpoint is an A3+ candidate, recorded then).
- The protocol never carries routed-app traffic, usage numbers, or
  diagnostics — only the fields in §2. That limit is in the privacy
  copy verbatim (05 §6).
