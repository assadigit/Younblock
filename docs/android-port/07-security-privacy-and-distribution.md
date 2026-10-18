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
  apps on its list — the YouTube app and the Chrome, Firefox, Opera and
  Brave browsers are added by default and you add or remove apps in the
  menu. Nothing is sent to any Younblock server."
  (D87: the five-app default — D86 put the YouTube app on the list
  after Ali's first device observation, and his second ruling extended
  it to the four browsers)
- "Android shows a key icon because Younblock occupies the VPN slot.
  It is not a VPN." (the old spec's honest line, kept verbatim)
- "Your IP address is not hidden. Traffic to sites is protected only
  by the sites' own HTTPS."
- "DNS requests for the routed apps go to the secure-DNS providers you
  configure in Settings."
- "Websites ending in .ir are never processed — no unblocking, no
  fragmentation: they always connect directly, exactly as if Younblock
  were not running." (D83)
- Scope statement: TCP traffic of the allow-listed apps is processed
  (the YouTube app and the four mainstream browsers are the default
  entries — D87; entries whose apps are not installed are skipped at
  establish; the list is editable
  in-app); other apps, and non-TCP traffic
  except DNS, are untouched; every host under .ir is excluded from the
  app's machinery in every mode (D83); IPv6 is intentionally disabled
  (fail-safe, ADR-0010).

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
  above; the list is user-editable (M6), starts EMPTY (D83 — no
  default), and is shown in the footer.
- .ir domains (D83): never routed, never fragmented, never
  redirected — connected direct, byte-transparent. One honest limit:
  on Android their name lookups still resolve through the configured
  DoH providers (the process has no other resolver; ACCESS_NETWORK_
  STATE is deliberately not requested — D74). The in-app Privacy
  section and the About sheet state both halves.

## 6. Android permissions and platform behavior — inventory

| Permission / attribute | Runtime consent | Feature requiring it | Denial handling | Necessary? |
|---|---|---|---|---|
| `INTERNET` | install-time | engine sockets (DoH, upstream) | n/a (normal perm) | Yes |
| `FOREGROUND_SERVICE` | install-time | keeping the tunnel alive (AND-018) | n/a | Yes |
| `FOREGROUND_SERVICE_CONNECTED_DEVICE` (API 34+) | install-time | FGS type for the VPN service | n/a | Yes (per current policy) |
| `POST_NOTIFICATIONS` (13+) | runtime request | state notification | app remains fully functional; state visible in-app; re-request politely on next connect | Yes for AND-018 UX; degrades gracefully |
| `BIND_VPN_SERVICE` (service attribute, not app perm) | system-bound | only the system may bind YounblockVpnService | n/a | Yes |
| VPN consent (`VpnService.prepare`) | system dialog | establishing the TUN | AwaitingConsent → Disconnected with explanation (AND-003) | Yes |
| `<queries>` launcher-intent (11+) | n/a | seeing launchable apps so a user-added package is checked — and routed — honestly (`NO_TARGET_APPS`) | n/a | Yes (D83: the list is user-driven; the old chrome+youtube-only block would have made every user-added package invisible → refused as "not installed"; non-launchable packages cannot be verified — the stated limit) |
| `ACCESS_NETWORK_STATE` | — | netmon network-change detection | n/a | **Not requested — verified at M3**: the Go netmon reads interfaces in-process (`net.InterfaceAddrs`) and never touches ConnectivityManager, so the permission would be a claim the code does not make (D74) |

No location, no storage beyond app-private, no background location,
no SYSTEM_ALERT_WINDOW, no request-install-packages. WebView
hardening: local assets only (WebViewAssetLoader or loopback webui
per ADR-0005 — either way no remote pages), navigation blocked,
file/content access disabled, no remote debugging in release.

## 7. Dependencies and licenses (audited 2026-10-16; re-run 2026-10-17 at M7)

| Dependency | Version pin | License | Status |
|---|---|---|---|
| `golang.org/x/mobile` (gomobile) | v0.0.0-20261011034528-197770444867 | BSD-3-Clause | maintained (pushed 2026-10-11) |
| `gvisor.dev/gvisor` netstack subset | v0.0.0-20260701204157-69c2d17aea96 | Apache-2.0 (LICENSE vendored) | very active |
| `golang.org/x/{net,sync,sys,time,exp}` | go.mod pins | BSD-3-Clause | maintained |
| `github.com/google/btree` | v1.1.2 (gVisor transitive) | Apache-2.0 | stable |
| `xjasonlyu/tun2socks` (engine pkg — NOT shipped; D75) | — | MIT | rejected at M4, out of the closure |
| fallback: `heiher/hev-socks5-tunnel` | — | MIT | not used (C/NDK) |
| existing tree deps (wails excluded from Android) | go.mod pins | permissive | vendored |

M7 audit evidence: the AAR's actual dependency closure (`go list
-deps ./mobile`, the honest set — not the whole 224-module go.mod
graph) is the gVisor `pkg/` netstack subset, golang.org/x/{net,sync,
sys,time,exp}, x/mobile, and google/btree. All permissive; the
attribution line lives in the app's About sheet ("gVisor network stack
(Apache-2.0) and gomobile (BSD-3)"). Re-run the closure check at M8
against the final artifact.

## 8. Distribution and signing

- **D84 (2026-10-17): the test circle is debug-APK-only.** Ali's
  ruling: "yes keep it debug version. we're not releasing yet. but
  release in github for me to download and debug" — the debug APK is
  the distributed artifact, published via GitHub Releases on the
  private repo (prerelease tag `v1.6.6-android-debug.1`), downloadable
  with his own GitHub credentials. No signed release APK exists at
  this stage; the local-keystore alternative stayed documented until
  D88 turned it into a shipped script.
- **D88 (2026-10-17, M8-e): the local keystore ceremony is a shipped
  script.** Ali's ask: "Can you fix the serial number and etc issue
  as well? like giving me a bat file that auto creates it's password
  locally on my system and backs up that password as an md file. if
  you insist it must be done locally. at least you can create the
  script." The security boundary stands — a signing-key password is
  generated where it is used and never transits the chat, the agent,
  or the repo — and what ships is the automation:
  `make-release-keystore.bat` (repo root + an asset on the debug.3
  release; offline, pure ASCII, CRLF). One double-click on his
  Windows machine: keytool is located (PATH / JAVA_HOME / Android
  Studio JBR / common JDK dirs), a 32-character password is generated
  with Windows' CSPRNG (an exactly-64-character alphabet, zero
  modulo bias), the keystore is created with every keytool prompt
  pre-filled (RSA 4096, PKCS12, 10950 days — the "serial number and
  etc" interactive ceremony eliminated), the result is verified by
  reading it back with the same password (a wrong password exits 1 —
  proven against the real keytool), and a
  `younblock-keystore-backup_<date>.md` records the password, the
  alias, the certificate serial number, the SHA-256 fingerprint and
  the full keytool record. The script refuses to overwrite an
  existing keystore, warns when run inside a git repo, and the
  `.gitignore` guards both generated patterns. Honest limit: the
  build host is Linux — the batch file itself is ⛔ there; the
  keytool flow, the parsed output shapes and the failure semantics
  were verified against the real JDK 21 keytool on the host, and the
  script self-verifies on its first run on Ali's machine.
- **D89 (2026-10-17, M8-g): the android circle ships as regular
  releases in the private repo — no prerelease flag.** Ali's ruling:
  "Do not prerelease, just release, the repo is private." The flag
  carried no public meaning there (every release in a private repo is
  collaborator-only), and it kept the repo's Latest pointer off the
  current test-circle build. The three android releases were switched
  via the API and verified (tags, assets, digests unchanged;
  /releases/latest → debug.3, set explicitly); their honest
  "debug-signed test build" copy lives in the release bodies, which
  stay as published apart from debug.3's one-paragraph D89 note. The
  prerelease flag's real semantics (public update channels, the
  public Latest badge) return only if the repo ever goes public.
- **D90 (2026-10-17, M8-h): the keystore ceremony survives
  PowerShell-less PATH machines.** Ali's first field report: the
  script stopped at password generation — "'powershell' is not
  recognized" — his PATH leaves PowerShell out (the same
  environment class the desktop setup scripts fixed at v0.7.5). The
  script now locates powershell.exe by its full standard path first
  (setup-offline.bat's exact pattern), then where.exe, and if
  PowerShell cannot be found at all a pure-cmd fallback builds the
  password from six RANDOM draws (digits and dashes, ~90 bits) —
  the step can never dead-end again, and the .md backup records
  which generator ran. His online-generator question ("can't I use
  an online keyring generator instead?") is answered NO in the
  ruling: the signing identity never transits a third party (D88's
  boundary); the one-line manual keytool alternative is documented
  in the release notes instead. Ships as v1.6.6-android-debug.4 —
  a regular release (D89) carrying only the fixed .bat (no APK;
  the app is unchanged — D84), with debug.3's page carrying the
  pointer note.
- When the signed lane returns: release APK signed with that local
  keystore, never committed (`.gitignore`), created and backed up
  with `make-release-keystore.bat` (README documents the steps;
  losing it = uninstall-before-update for everyone who installed the
  signed build). Successive debug builds bump `versionCode` so they
  update over each other (same debug certificate).
- Sideload only (ADR-0012): README documents the
  allow-installs-from-this-source step and the Play Protect warning
  (the debug build additionally shows Android's "built for testing"
  warning — expected, honest).
- Play Store policy review happens only if Ali changes distribution
  scope; nothing in v1 assumes Play.
- Release notes list supported Android versions (minSdk set at M3),
  traffic scope, IPv4-only DNS posture, UDP/QUIC behavior, and known
  limitations (doc 02 §4).

## 9. Release-blocking checklist

- [x] Permission rationale and service state understandable (M5/M6 test) — closed at M7: the consent copy + honest notification shipped in M5/M6; the permission inventory verified against the built APK's manifest (doc 05 M7 item 5)
- [x] No overstated VPN/privacy/coverage language (copy review vs §1) — closed at M7: the D80 sheet copy (About/Privacy/How-it-works/Help) re-reviewed against §1/§5 and doc 10's headline; the VPN-slot statement carries the not-a-VPN qualifier everywhere it appears
- [x] DNS and traffic scope documented accurately (§5 verified vs code) — closed at M7: §5's claims re-checked against the implementation (no analytics code exists; no Younblock-run endpoint; DoH providers user-configurable; logs local) and rendered as the in-app Privacy section (M6)
- [x] Startup and stop paths tested for leakage/cleanup (M4/M5) — closed at M5/M7: the lifecycle suite (M5) + the -race lane and D82 generation fix (M7)
- [ ] IPv4/IPv6 and TCP/UDP claims match test results (M4 matrix) — host-level policy tests green; the device matrix rows stay open (doc 06 §4) — now with a concrete closure path: Ali's device testing through the D84 debug release (doc 10 §"Device test circle")
- [x] The account gate ships only with its privacy-copy change (opened at A0, 2026-10-18) — CLOSED as CANCELLED at A0-R3 (D97, 2026-10-18): the gate will never ship, so the condition is permanently satisfied — today's "no account, no data collection" line in §1, doc 10 §2, the in-app Privacy sheet and README stays verbatim forever; the drafted replacement copy (docs/auth/05 §6, AUTH-030) never ships. No future release changes anything for the gate — there is none
- [x] Sensitive logs/configuration minimized and redacted (inherited + spot-check) — closed at M7: the diagnostic report redaction tests inherit green; the report is the only diagnostics surface and ships hostnames-off by default (R21)
- [x] Dependency/license review re-run at M8 — closed at M8 (D84 session): `go list -deps ./mobile` re-run against the shipped artifact — the same audited set as M7 (gVisor `pkg` netstack subset, golang.org/x/{net,sync,sys,time,exp} + x/mobile, google/btree), all permissive, LICENSEs vendored, the in-app attribution line unchanged
- [x] Android platform requirements rechecked against primary sources at M3/M8 — closed at M8 (D84 session): the five load-bearing claims re-verified via developer.android.com (VpnService/Builder per-app routing; BIND_VPN_SERVICE "only the system can bind"; package-visibility filtering + the `<queries>` element on API 30+; foreground-service types required targeting API 34+ — the app declares connectedDevice with its permission, verified in the dumped manifest; POST_NOTIFICATIONS as an API 33+ runtime permission). The device-behavior halves (which FGS type behaves best for a VPN in practice, notification behavior on real OEM builds) ride with Ali's device checklist
- [x] APK signing, version, checksums verified (M8) — closed at M8 for the shipped debug artifact (D84: no signed release exists): apksigner v2-scheme verify OK, debug certificate SHA-256 digest c76bb7c6…8551d recorded; versionName 1.6.6 / versionCode 1, minSdk 26 / target 36, 3 ABIs; APK SHA-256 ccf46a2e…7b64 re-verified on disk and against the uploaded release asset. The release-keystore half is superseded by D84 (deferred until a public release)

The remaining open item is the device-gate row (IPv4/IPv6 and
TCP/UDP measured on a real device), which now has its closure path:
Ali's own device testing through the debug releases — as of M8-n
(2026-10-18) the test-circle build is `v1.6.6-android-debug.7`
(versionCode 6, the first-video fix — ADR-0013), and D98 puts the
debug APK on every release page. (The
account-gate row closed as CANCELLED at A0-R3 — D97, 2026-10-18: the
gate will never ship.) The known-
limitations disclosure obligation is discharged host-side (M6's
in-app copy + doc 10's limitations section); the device-matrix rows
that would let the remaining claims carry measured results are the
Ali-device gate. M8 closes Phase 2 host-side (doc 05 §M8).
