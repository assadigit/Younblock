# 05 — Security Review (A0)

> ⛔ **CANCELLED — D97 (2026-10-18, A0-R3): the account gate will
> never be built; the §6 privacy-copy change it drafted will never
> ship — today's "no account, no data collection" claims stay
> literally true, permanently. See `00-start-here.md` and
> `DECISIONS.md` D97. Kept as history.**

**Status:** draft for Ali's review; revised A0-R2 (2026-10-18): §2
and §6 carry the two-edition addenda (D96). Per D94's own standard —
auth is security-related, so this review names the gate's limitations
as frankly as its features. Nothing here is a claim about shipped code;
A0 builds nothing, and every statement below is a design statement the
A-series must re-verify against the implementation before any of it
appears in user-facing copy (the D77–D94 rule).

## 1. What the gate is for

The gate answers one question: **who may start the app** — on every
edition, enforced in the engine (02 §1), so no shell offers a connect
path around it. That is the exclusivity objective (D94), and it is the
only security objective this feature claims.

## 2. What the gate blocks — and what it does not (the honest bounds)

**Blocks:** casual use and casual sharing. Someone without a member
account cannot use the app on any edition; a shared APK outside the
member circle is a door with no key. A disabled or expired member
stops at the next verify (AUTH-021).

**Does not block — stated plainly:**

> **A0-R2 addendum (D96, the two-edition split):** the bounds below
> apply to the **members edition** and are unchanged by the split —
> identical gate code, so identical bounds. The **free edition** has
> no gate to bypass at all: nothing in it is exclusive, by design —
> that is the honest meaning of "free for all", and it is also its
> resilience: the free edition keeps working even if the auth server
> dies or is never paid for again. The split neither weakens nor
> strengthens the members edition; it only makes each edition say
> exactly what it is.

- **Token extraction.** A determined member can read their own token
  file (it is their device) and hand it to a friend. The friend then
  holds one working session until it expires/revokes. Mitigations that
  raise the cost (all optional, none decisive): short expiry
  (OPEN (d)), single-active-session-per-user (a possible ruling under
  (d) — a second login revokes the first), disable-on-suspicion (the
  admin's real lever — revocation at the next verify, 03 §2).
- **Client patching.** The check runs on the member's device; a skilled
  attacker with the APK can patch the check out. **The gate is not
  DRM** and will not be described as protection against a determined
  technical adversary. What the engine-level design buys: the attacker
  must patch the engine (the strongest seam), not just click past a
  UI — and the project will never ship obfuscation or anti-tamper
  claims it cannot back (01 §5).
- **A member sharing the account credentials themselves** — one
  username+password knowingly given to a friend is indistinguishable
  from the member. Single-active-session (above) is the partial answer;
  the real answer is admin trust, and that is an operational fact, not
  a technical one. It is said here, not hidden.

## 3. Server and transport

- The auth server is a new, standing attack surface. Design keeps it
  minimal: one service, one DB file, loopback-to-proxy TLS (04 §1),
  no shell beyond SSH. Its compromise = control over who enters (not
  over anyone's traffic — the server never sees routed-app traffic,
  02 §5). Mitigations: the small surface, updates via binary replace
  (04 §4), offline backups (04 §3), and audit logging (below).
- **DB leak:** password hashes resist offline cracking under the
  argon2id parameters (03 §5) — the recommendation is tuned so a
  leaked hash costs an attacker days per member at 64 MiB × 3, and the
  parameters are encoded with the hash so they can be raised. The
  audit log in the same file reveals member usernames and login times
  — that sensitivity is why backups are pulled offline (04 §3), and it
  is named in the privacy copy (§6).
- **TLS:** system CA validation; no pinning in v1 (03 §1) — so a
  member device with a maliciously-installed root CA could in principle
  intercept login traffic (a limit of v1, stated here; pinning is an
  A3+ candidate to be weighed against its brittleness then).
- **Brute force:** server-side only (03 §7) — the client enforcing
  anything against itself would be theater.

## 4. Client side

- The token file is 0600 on the device (02 §3). On a rooted device /
- an attacker with the member's unlocked phone, the token is readable —
  that is the token-extraction case in §2, same honesty.
- **No password is ever written to disk** (AUTH-003); it exists in
  memory only for the in-flight login. The signed-out state offers no
  "remember password" — by design; no v1 claim of secure storage will
  be made beyond these facts.

## 5. Availability

- Fail-closed with bounded offline grace (OPEN (c) recommendation,
  03 §6): the app's usefulness during an auth-server outage is bounded
  by the grace window; the honest `AUTH_SERVER_UNREACHABLE` refusal
  follows. The availability lever is operational (monitoring + backups,
  04 §1/§3), not cryptographic.

## 6. The privacy statement changes — drafted now, disclosed when shipped

When the gate ships, the standing set (doc 07 §1, doc 10 §2, the
in-app Privacy sheet, README) changes as follows — **draft copy:**

> **Account.** Younblock is members-only: you sign in with a
> Younblock account (username and password) before you can connect.
> At sign-in, the account server receives your username, your
> password (once, to verify it), the app's version, and — like every
> HTTPS connection — your IP address and connection time. It stores
> your username, a salted password hash (never the password, never
> recoverable), the account's created/disabled/expires state, your
> most recent login, and an audit record of account changes. It never
> sees your browsing, your usage numbers, or your diagnostics; the
> traffic you route through Younblock never passes through it.
>
> **What this does not change.** The proxying itself stays local and
> exactly as it was: there is still no Younblock VPN server, your IP
> is still not hidden, no encryption is added to your traffic, and
> your usage history still stays on your device.

The old "no account, no data collection" line is **replaced by this**
at the release that ships the gate — in the changelog, the release
notes, the Privacy sheet and README together (D94 ruling 2: disclosed,
never silent). Until that release, nothing of the gate exists in any
build, and the old copy stays true of every shipped build.

> **A0-R2 addendum (D96, 2026-10-18) — the copy splits per edition:**
> the **free edition** keeps the standing "no account, no data
> collection" claims VERBATIM — they stay literally true of it (it
> talks to nothing; it has no server). The **members edition** ships
> the drafted account copy above. Both are disclosed together at the
> first release that ships the split — never silently.

## 7. Review obligations for the implementing milestones

- A1 (backend): host tests for every §2/§3 mitigation actually claimed
  above; the parameters recorded with evidence (03 §5's tuning note).
- A2 (client gate): the engine refusal pinned by tests (the
  `NO_TARGET_APPS` pattern — a signed-out engine refuses `Start()`
  with the typed code; no shell-side-only gate).
- A3 (ship): this review re-run against the shipped code, and the
  privacy copy above verified line-by-line against what the server
  actually stores — the same standard the debug.3 release notes were
  held to (doc 10 §1a's honest headline).
