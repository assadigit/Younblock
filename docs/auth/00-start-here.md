# Younblock Account Gate — Specification Pack (A0)

**Repository:** <https://github.com/assadigit/Younblock>
**Documentation location:** `docs/auth/`
**Status:** A0 DRAFT — design-first per D94 (2026-10-18); delivered
2026-10-18; **revised in place 2026-10-18 (second exchange, A0-R2)**
after Ali's next message: question (b) is now RULED (D95), the
two-edition direction is recorded (D96) and adds OPEN (g), and (a)
carries the platform analysis he asked for (02 §6). **Nothing is
built.** No login code, no server, no database, no keys exist anywhere
in the repository or in production. This pack exists so the
implementation (A1, A2, …) starts from an approved design, not from
code. It opens with Ali's own words (D94) as its charter:

> "Planning to add login authorization for this, like a normal VPN app,
> where users must have been signed up or input credible and verified
> user/password... The objective is to make the app more exclusive, so
> only people who have username password can use it. so we need login
> page etc. plan and design ahead for it's backend requirements for
> example user database, admin managing users etc."

## What this pack is, and is not

- **Is:** the complete design for the account gate — requirements,
  architecture, wire protocol, admin/operations, and a security review
  that names the gate's limitations as frankly as its features (the
  D77–D94 honesty rules bind this lane exactly as they bind every other:
  auth is security-related, so no claim below may exceed what the
  implementation will actually enforce, and every limitation is stated).
- **Is not:** code. A0 writes specifications and ADRs only. A1+
  implements, in small milestones with the baseline-green discipline
  after each, and only after Ali rules on the six open questions.

## Reading order

1. [`01-requirements.md`](01-requirements.md) — what the gate must do,
   per edition; the non-goals; the open questions in requirement form.
2. [`02-architecture.md`](02-architecture.md) — where the gate sits
   (engine-enforced, every shell inherits), the components, the data
   model, and the untouched traffic path.
3. [`03-auth-protocol.md`](03-auth-protocol.md) — the wire protocol,
   token/session design, password storage, brute-force protection.
4. [`04-admin-and-operations.md`](04-admin-and-operations.md) — the admin
   surface and how the service is operated day to day.
5. [`05-security-review.md`](05-security-review.md) — the threat model,
   honestly bounded, and the privacy-statement changes drafted for the
   day the gate ships.

## The open questions (D94 ruling 3) — consolidated, revised A0-R2

Each needs Ali's ruling before any implementation starts. The
recommendations are the design team's; **they are not decisions** —
recording them as if ruled would fake Ali's choices. D95+ in
DECISIONS.md is reserved for the rulings. **(b) is now ruled (D95);
(a) has the platform analysis Ali asked for; (g) is new (D96).**

| # | Question | Recommendation / status | Where specified |
|---|---|---|---|
| a | Where does the backend live? (Ali asked: Cloudflare Workers / GitHub Pages / Vercel / Netlify?) | Recommendation unchanged: a small always-on host (a few US$/month; the design ships as specced — Go + SQLite + full-strength argon2). The strictly-$0 alternative is Cloudflare Workers + D1, with named trade-offs (a TS/Go-WASM service, weaker hashing under CPU budgets, platform lock-in). GitHub Pages can never host a backend (static only). Vercel/Netlify split state across two vendors. The 2026-10-18 platform matrix + the Iran-reachability requirement: **02 §6**. | 02 §2, 02 §6, 04 §1 |
| b | Who creates accounts? | **RULED (D95, 2026-10-18): admin-only creation** — Ali creates every account; no open signup, no approval queue. Matches the A0 recommendation. | 01 §3, 04 §2 |
| c | Fail-open or fail-closed when the auth server is unreachable? | **Fail-closed with a bounded offline grace** (the device may keep using a cached token for a limited window, then requires a reachable server; never an indefinite open). Lower-stakes under D96: only the members edition is affected — the free edition has no server at all. | 03 §6 |
| d | Token/session design and expiry? | Opaque server-side session tokens (256-bit random, stored hashed server-side), revocable on disable; ~30-day expiry with refresh. JWT is the documented alternative. Open sub-item: single active session per account (a second login revokes the first) — the cheap anti-sharing lever. | 03 §3–4 |
| e | Password storage parameters? | argon2id (m=64 MiB, t=3, p=4 as the starting point, tuned on the server); bcrypt cost ≥ 12 is the fallback. Never plaintext, never recoverable; reset only by admin. Now also rides (a): full strength presumes an always-on host; serverless CPU budgets force a re-tune down, honestly re-worded (03 §5 note). | 03 §5 |
| f | Brute-force protection + the login form? | Per-username and per-IP rate limits with exponential backoff and admin-clearable lockout; the login form is the shared screen's signed-out state (D92's one-UI rule), username + password, no CAPTCHA in v1. The admin surface form rides with it: CLI first, web admin only if later warranted. | 03 §7, 01 §R3 |
| g | The two-edition split — packaging, install identity, distribution, naming (NEW, D96) | Build-time editions from one source tree (Go build tags + Gradle product flavors): the members edition carries the full gate; the free edition contains zero auth code and no server dependency. Open sub-rulings: (g1) side-by-side install (recommended: an `.auth` applicationId suffix) vs one replacing package; (g2) distribution channel per edition; (g3) Android-only first (recommended) vs desktop split too; (g4) the two names for release notes and privacy sheets. | 01 §6, 02 §7 |

## The implementation ladder (nothing below runs before the rulings)

- **A1** — the auth backend (server + database + admin CLI), built and
  tested host-side; deployed only when Ali has a location for it.
  Gated on (a) + (c)–(f); (b) is ruled (D95) and (g) shapes only the
  client side (A2).
- **A2** — the client engine gate (`internal/` + facade + the three
  shells' signed-out state), desktop-first (host-testable), then
  Android wiring.
- **A3** — hardening + the privacy-copy change shipped together with the
  gate (never silently — D94 ruling 2), release notes updated, the
  debug-circle build carries it for Ali's device round.
- Each step: baseline green after it, scoped commits, evidence recorded
  in doc 06 / PROGRESS.md; each debug build that changes app code bumps
  `versionCode` and goes through the regular-release lane (D89).

## Relation to the existing spec pack

The Android port pack (`docs/android-port/`) remains the ground truth
for everything it covers. This pack cross-references it (doc 04 §6's
connect ordering; doc 07 §1's truthful claims; doc 10 §2's
What-it-does-NOT set) and changes exactly one thing in it when the gate
ships: the "no account, no data collection" claim (doc 07 §1, doc 10
§2, the in-app Privacy sheet, README) — disclosed at that release,
never silently (D94 ruling 2). The proxying itself is untouched by this
feature: no VPN server, no IP hiding, no added encryption — the gate
gates **usage**, not traffic (D94 ruling 2, verbatim).
