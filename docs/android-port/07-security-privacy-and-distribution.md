# 07 — Security, Privacy, and Distribution

**Status:** Updated 2026-10-16 with the accepted architecture. The
permission inventory (§6) and disclosure answers (§5) are drafted from
verified facts; the release-blocking checklist (§9) stays open until
each item is tested on the artifact.

## 1. Truthful product claims

Younblock is not a conventional VPN: it does not hide the user's IP,
adds no encryption of its own, and sends nothing to a Younblock-operated
server. The Android edition uses the VPN slot **as a local
traffic-routing interface for the allow-listed apps only** (ADR-0003/
0007). Required in-app statements (M6 wires the copy; sources verified
against the audited code):

- "Younblock uses Android's VPN feature only as a local tunnel for the
  apps you choose (Chrome and the YouTube app by default). Nothing is
  sent to any Younblock server."
- "Android shows a key icon because Younblock occupies the VPN slot.
  It is not a VPN." (the old spec's honest line, kept verbatim)
- "Your IP address is not hidden. Traffic to sites is protected only
  by the sites' own HTTPS."
- "DNS requests for the routed apps go to the secure-DNS providers you
  configure in Settings."
- Scope statement: TCP traffic of the allow-listed apps is processed;
  other apps, and non-TCP traffic except DNS, are untouched; IPv6 is
  intentionally disabled (fail-safe, ADR-0010).

## 2. Permission and consent

Flow (M5): the app's own explanation screen appears **before**
`VpnService.prepare()` triggers the system dialog; denial returns to a
usable Disconnected state with how-to-continue text; a competing VPN
replaces ours per platform behavior (onRevoke → honest off); the
notification shows the live state with a Turn-off action; permission
revocation from system settings stops the service and reconciles the
UI. Foreground-service/notification design follows the current API
level's rules — verified this session: API 34+ requires declared FGS
types; VPN apps use `connectedDevice` with
`FOREGROUND_SERVICE_CONNECTED_DEVICE`; POST_NOTIFICATIONS is a runtime
permission on 13+ (re-verify at M3 against the final target SDK).

The browser-proxy fallback mode (doc 03 §2A), if ever surfaced, needs
no VPN permission and must say so.

## 3. Traffic processing and failure safety

- Fail-safe defaults: hard startup failure tears the TUN down
  (traffic reverts to direct — the platform's safe state); degraded
  keeps serving with the watchdog running (verified engine behavior).
- Loop prevention: every engine socket is protect()-ed (ADR-0011);
  M3/M4 tests enforce it.
- IPv6 leak prevention: AAAA answered empty + fail-safe v6 routing
  (ADR-0010); dual-stack device test in M4.
- Never "Connected" while bypassing: the self-test gates the state
  (ADR-0006) and the checks run **through the live path**.
- Logs: `internal/logx` keeps hostnames off by default (R21);
  diagnostics redact; no URLs/query strings/TLS payloads logged
  (verified in `privacy_test.go` — engine package).
- TLS certificate validation is never disabled (verified: DoH client
  and tunnel paths use standard verification; only tests inject trust
  pools).
- Timeouts/bounded retries/cancellation on every network op (verified
  engine config: per-IP, overall, request, first-record, idle, start
  total 35 s, heal cooldown, notification rate limit).
- Settings and diagnostics live in app-private storage.

## 4. Threat model (limits stated plainly)

Younblock's goal is not a general-purpose security tunnel. Assessed
for v1: malicious/compromised DoH provider (mitigated by the
user-configurable list + private-range blocking; not eliminated —
stated); network attacker observing metadata (HTTPS is the user's
protection; we add none); hostile packet/stream inputs (userspace
stack hardening + the engine's existing malformed-input tests +
panic recovery per connection); memory exhaustion (MaxConns cap +
idle timeouts); traffic leakage at startup/shutdown (transactional
start, atomic TUN teardown — M4/M5 tests); dependency vulnerabilities
(license+maintenance audit done 2026-10-16; per-release re-check);
untrusted provider URLs (Normalize validates); debug logs (hostnames
off by default; release builds minimal); the local application
receiving routing privileges (this app — the permission is the
product's core consent).

## 5. Data handling and privacy disclosure (answers)

- Analytics/crash reports: **none** (no such code exists in the tree —
  verified; the Android shell adds none in v1).
- Data leaving the device: only (a) DNS queries to the user-configured
  DoH providers, for the routed apps' lookups; (b) the routed apps'
  own traffic to its destinations (as without Younblock, minus
  hostname-filtered DNS and with fragmented first TLS records).
  Nothing to any Younblock server (no such endpoint exists in code).
- Default DNS providers: `config.Defaults()` (ElectroDNS/Ruby/AdGuard
  per defaults.go — M6 lists them in-app with the same names the
  desktop About shows).
- Logs: local only, rotating, hostnames off by default; user-clearable
  via app data clear; no sync.
- Provider selections/diagnostics synced anywhere: no.
- Other apps' traffic: only the allow-listed apps', TCP + DNS as
  above; the list is user-editable (M6) and shown in the footer.

## 6. Android permissions and platform behavior — inventory

| Permission / attribute | Runtime consent | Feature requiring it | Denial handling | Necessary? |
|---|---|---|---|---|
| `INTERNET` | install-time | engine sockets (DoH, upstream) | n/a (normal perm) | Yes |
| `FOREGROUND_SERVICE` | install-time | keeping the tunnel alive (AND-018) | n/a | Yes |
| `FOREGROUND_SERVICE_CONNECTED_DEVICE` (API 34+) | install-time | FGS type for the VPN service | n/a | Yes (per current policy) |
| `POST_NOTIFICATIONS` (13+) | runtime request | state notification | app remains fully functional; state visible in-app; re-request politely on next connect | Yes for AND-018 UX; degrades gracefully |
| `BIND_VPN_SERVICE` (service attribute, not app perm) | system-bound | only the system may bind YounblockVpnService | n/a | Yes |
| VPN consent (`VpnService.prepare`) | system dialog | establishing the TUN | AwaitingConsent → Disconnected with explanation (AND-003) | Yes |
| `<queries>` for target packages (11+) | n/a | checking Chrome/YouTube installed (`NO_TARGET_APPS`) | n/a | Yes |
| `ACCESS_NETWORK_STATE` | install-time | netmon network-change detection (optional — Go polls interfaces; decide at M3 if needed) | n/a | To verify at M3 |

No location, no storage beyond app-private, no background location,
no SYSTEM_ALERT_WINDOW, no request-install-packages. WebView
hardening: local assets only (WebViewAssetLoader or loopback webui
per ADR-0005 — either way no remote pages), navigation blocked,
file/content access disabled, no remote debugging in release.

## 7. Dependencies and licenses (audited 2026-10-16)

| Dependency | Version pin | License | Status |
|---|---|---|---|
| `golang.org/x/mobile` (gomobile) | latest stable at M3, pinned then | BSD-3-Clause | maintained (pushed 2026-10-11) |
| `xjasonlyu/tun2socks` (engine pkg) | latest stable at M4, pinned then | MIT | maintained (pushed 2026-10-11) |
| alternative: `google/gvisor` netstack | pinned subset | Apache-2.0 | very active |
| fallback: `heiher/hev-socks5-tunnel` | — | MIT | active (C/NDK; only if forced) |
| existing tree deps (wails excluded from Android) | go.mod pins | permissive | vendored |

Re-run the audit at M8; record transitive natives (gVisor pulls its
own module set) and note obligations (all permissive → attribution
notices in About suffice).

## 8. Distribution and signing

- Debug APK for the test circle; release APK signed with a local
  keystore, never committed (`.gitignore`), README backup steps
  (losing it = uninstall-before-update for friends).
- Sideload only (ADR-0012): README documents the
  allow-installs-from-this-source step and the Play Protect warning.
- Play Store policy review happens only if Ali changes distribution
  scope; nothing in v1 assumes Play.
- Release notes list supported Android versions (minSdk set at M3),
  traffic scope, IPv4-only DNS posture, UDP/QUIC behavior, and known
  limitations (doc 02 §4).

## 9. Release-blocking checklist

- [ ] Permission rationale and service state understandable (M5/M6 test)
- [ ] No overstated VPN/privacy/coverage language (copy review vs §1)
- [ ] DNS and traffic scope documented accurately (§5 verified vs code)
- [ ] Startup and stop paths tested for leakage/cleanup (M4/M5)
- [ ] IPv4/IPv6 and TCP/UDP claims match test results (M4 matrix)
- [ ] Sensitive logs/configuration minimized and redacted (inherited + spot-check)
- [ ] Dependency/license review re-run at M8
- [ ] Android platform requirements rechecked against primary sources at M3/M8
- [ ] APK signing, version, checksums verified (M8)
- [ ] Known limitations disclosed in-app and in release notes (M6/M8)
