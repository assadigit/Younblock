# 10 — v1 Release Notes (DRAFT — M7)

**Status:** DRAFT for the first Android release, prepared at M7
(2026-10-17). Nothing here may be published until the ⛔ device gates
(§"Open before release") close or Ali explicitly accepts shipping with
them recorded as known limitations. Every claim below traces to
implementation evidence (docs 01–09, DECISIONS.md D1–D82); M8 performs
the final copy review against the built artifact.

## The honest headline

Younblock for Android helps YouTube load when the network gets in the
way. It routes **only the apps you choose** — Chrome and the YouTube
app by default — through an on-device engine that uses secure DNS and
TLS record splitting, exactly like the desktop edition. It is **not a
VPN in the privacy sense**: it does not hide your IP address, adds no
encryption, and sends nothing to any Younblock-operated server (none
exists). It uses Android's VPN slot as a **local traffic-routing
interface** — that is the key icon you will see while it runs.

## What it does

- Routes the TCP traffic of the allow-listed apps (Chrome + YouTube by
  default; the list is editable in-app, changes apply on the next
  connect).
- Resolves the routed apps' DNS through the secure-DNS (DoH) providers
  you configure, tried in order (defaults: RethinkDNS, ElectroDNS,
  Ruby).
- Splits the first TLS record for the routed YouTube domains into
  smaller pieces — the same fragmentation the desktop edition uses.
- Keeps a persistent, honest status notification while routing: active
  only when traffic actually routes, degraded when partially working,
  connecting otherwise.
- Carries the desktop edition's self-test, automatic recovery, and
  redacted diagnostics report.

## What it does NOT (the truthful-claims review, doc 07 §1)

- No IP hiding, no added encryption — YouTube's own HTTPS protects the
  content.
- No remote VPN server; nothing leaves the device except the routed
  apps' own traffic and their DNS queries to your configured DoH
  providers.
- No analytics, no crash reports, no account, no data collection.
- No claim to work with "every app": only the allow-listed apps' TCP
  and DNS are processed; other apps and protocols are untouched.

## Scope and limitations (doc 02 §4 — the release-notes obligations)

- **IPv4 only for DNS resolution**: AAAA records are answered empty so
  routed apps stay on IPv4 (the desktop default). IPv6 traffic from
  routed apps is routed into the tunnel and dropped (fail-safe —
  visible breakage for that flow, never a silent leak past the engine).
- **UDP beyond DNS is dropped**: routed apps fall back to TCP. The
  YouTube app's QUIC-to-TCP fallback behavior is a device gate (§Open)
  — reported when measured, not promised here.
- **Supported path**: Chrome and the YouTube app over TCP, with the
  configured DoH providers. No support claims beyond the per-app
  allow-list and the tested matrix.
- **Per-app list changes apply on the next connect** (the list is
  baked into the TUN at establish).
- **Nothing auto-starts after reboot** (AND-028): connecting is always
  a user action.
- Requires Android 8.0+ (minSdk 26); targets Android 16 (API 36).
- Distributed as a sideloaded APK (debug-signed for the test circle;
  release signing per doc 07 §8).

## Open before release (⛔ — the standing device gates)

Blocked by the build environment (no /dev/kvm, no device — doc 09's
standing blocker), riding together until a KVM-capable host or a real
device appears; **never claimed, never faked**:

- The live consent dialog flow, notification channel behavior, and
  screen-off/Doze persistence.
- Real DoH resolution through the full stack; the YouTube app's actual
  QUIC→TCP fallback; protect() on real sockets; loop prevention
  on-device.
- Network switching on a real radio; stop from system VPN settings
  (onRevoke); reboot (nothing self-starts).
- Live WebView observation: rotation, font scaling, TalkBack, the
  Apps-to-unblock row on a real screen.
- Battery/behavior measurement (the notification-disclosed budget).

## Install (doc 07 §8)

Sideload only: allow installs from the chosen source, accept the
Play Protect warning for unknown developers, install the APK. Losing
the release keystore means uninstall-before-update for everyone who
installed the signed build (README documents backup steps).

## Open-source components

gVisor netstack (Apache-2.0), gomobile (BSD-3), and other permissive
Go libraries — the full audited set in doc 07 §7. Attribution in-app:
the About sheet.
