# 04 — Admin and Operations (A0)

**Status:** draft for Ali's review. The admin model rides **OPEN (b)**
(who creates accounts) and the surface choice is **OPEN (f)**'s
sibling; recommendations below.

## 1. Deployment and day-to-day (rides OPEN (a))

- One small self-hosted box (02 §6's sizing). The service runs under
  systemd, as a dedicated unprivileged user; the DB file and the
  service's config live under that user's home, 0600.
- TLS: a reverse proxy with a real certificate (Let's Encrypt or an
  equivalent) is the recommended shape; the service itself speaks
  plain HTTP on loopback to the proxy. The cert renewal is an
  operational item with a calendar note — an expired cert is the same
  user-visible failure as the server being down (03 §4's honest code),
  so it belongs in monitoring, not in hope.
- Monitoring: `GET /v1/health` (03 §2) polled externally (any free
  uptime pinger); the poller alerts Ali. No metrics stack in v1 — the
  audit log is the operational record.

## 2. The admin surface (recommendation: CLI first)

- **A1 ships a server CLI** (subcommands mirroring the endpoints:
  `users add|list|disable|enable|reset|expire`, `audit tail`) — the
  smallest surface that covers every operation, no new web attack
  surface, usable over SSH. The admin authenticates to the CLI via the
  admin credential (the first user created at bootstrap is the admin —
  recorded at A1; the admin row is what the admin endpoints check).
- **A minimal web admin is an A2+ candidate**, added when the member
  count or the admin's patience warrants it — it reuses the same
  endpoint set behind the same admin credential. Deferred, not
  promised (the D77–D94 rule: no claims ahead of implementation).

### User lifecycle flows

| Operation | Effect | Audit entry |
|---|---|---|
| create | admin sets username (+ optional password; else server generates and shows once) | `created` |
| disable | next verify/refresh answers not-valid; takes `locked`/`disabled` semantics apart (03 §1) | `disabled` |
| enable | clears the disabled bit; existing sessions still revoked-by-disable are gone (new login needed) | `enabled` |
| reset | new password (admin-set or generated); **revokes every session** of that user | `reset` |
| expire | sets `expires_at` (future date); when it passes, verify answers `expired` | `expired` |
| list | usernames, created, disabled, expires, **last login** (from `sessions.last_seen_at`) | — (reads are not audited) |

- Last-login is the only presence data kept (05 §6); it serves the
  admin's actual question — "is this account still being used?" —
  without any per-session history.

## 3. Backups

- The DB is one file (SQLite recommended, 02 §4): nightly offline copy
  pulled off the box (the same calendar discipline as the TLS renewal).
  A restore is: stop service, replace file, start — the sessions
  survive; if the backup is older, members re-login (honest, bounded
  impact; the audit log is in the same file, so it restores with it).
- The backup destination and retention are Ali's operational choice;
  the A1 ops note will carry the exact commands.

## 4. Updates

- The service is a single static binary; updates are
  stop → replace → start, with `/v1/health` confirming. No in-place
  migrations in v1 (the schema is born complete at A1); any future
  migration ships as a versioned, tested `migrate` subcommand before
  the A-series release that needs it — recorded then, not now.

## 5. What operations must never do

- Never commit the DB, the service config, or TLS keys to any
  repository (the D85 debug-keystore exception does not extend here:
  there is no reason for any auth secret to live in the repo).
- Never paste a generated password into chat in plaintext — hand it
  over privately; the audit log records that a `create` happened, not
  the password (03 §2).
- Never reuse the admin credential as a member account.
