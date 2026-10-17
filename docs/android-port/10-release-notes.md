# 10 — Release Notes (v1 DRAFT + the shipped debug test circle)

**Status:** updated at M8 (2026-10-17, D84). This document now holds
two things: §1 the **shipped** debug test-circle release
(`v1.6.6-android-debug.1`, a prerelease on the private repo — the same
text lives on the GitHub release itself), and §2 the **public v1
DRAFT**, unpublished until the ⛔ device gates close or Ali explicitly
accepts shipping with them recorded as known limitations. Every claim
traces to implementation evidence (docs 01–09, DECISIONS.md D1–D84).

## 1. v1.6.6-android-debug.1 — the device test circle (SHIPPED, prerelease)

A debug-signed build for **Ali's own device testing** — "we're not
releasing yet" (D84): this is not a public release. Expect Android's
"built for testing" and Play Protect warnings for unknown developers;
both are expected and honest for this circle.

**The honest headline.** Younblock for Android helps YouTube load
when the network gets in the way. It routes **only the apps you add
to its list** — none are routed by default — through an on-device
engine that uses secure DNS and TLS record splitting, exactly like
the desktop edition. It is **not a VPN in the privacy sense**: it
does not hide your IP address, adds no encryption, and sends nothing
to any Younblock-operated server (none exists). It uses Android's
VPN slot as a **local traffic-routing interface** — that is the key
icon you will see while it runs.

**The artifact.** `app-debug.apk`, 35,567,561 B, SHA-256
`ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64`;
applicationId `io.github.assadigit.younblock`, versionName 1.6.6,
versionCode 1; Android 8.0+ (minSdk 26), targets API 36; native code
for arm64-v8a, armeabi-v7a and x86_64; debug certificate (SHA-256
digest `c76bb7c61ec623892eabc37ac90ac1bc0041d39490f2d48073676d6e2ed
8551d`), apksigner v2-scheme verified. Successive debug builds bump
versionCode and install over this one (same certificate).

**Install (private repo, doc 07 §8).** Download the APK from this
release with your GitHub account, allow installs from your browser
app, accept the Play Protect warning, install. Nothing auto-starts;
connecting is always a user action.

### Device test circle — what to test and report

The build environment has no emulator and no device (verified — doc
09's standing blocker), so every device-side claim is **unclaimed
until observed on your device**. This circle is the checklist's
closure path; report pass/fail per row plus the in-app diagnostics
report (Menu → Diagnostics → copy) and your device model + Android
version:

1. **Install + first launch** — the sideload path above works; the
   app opens on the connect screen; rotation, font scaling and
   TalkBack behave; the sheet's Apps-to-unblock row renders.
2. **First connect, empty list** — the first connect with an empty
   allow-list refuses with the honest NO_TARGET_APPS message (none
   of the apps on your list is installed — because the list is
   empty; nothing is routed by default).
3. **Add the YouTube app** (`com.google.android.youtube`) to the
   list (Menu → Apps to unblock), connect again: the consent
   explanation → the system VPN dialog → Allow; the key icon
   appears; the notification honestly tracks connecting / active /
   degraded.
4. **The product's purpose** — YouTube actually loads in the
   YouTube app (browse + video playback) while connected; the
   self-test and recovery behave when YouTube stops responding.
5. **.ir stays direct** — with a routed app (e.g. a browser you
   added), any `.ir` site loads as if Younblock were not there: no
   unblocking, no fragmentation, always direct (D83/D84's ruling;
   their name lookups still use your configured secure-DNS
   providers — the stated limit).
6. **Stop paths** — disconnect from the app; disconnect from the
   system's VPN settings (onRevoke); reboot (nothing self-starts);
   reconnect works.
7. **Persistence + battery** — screen-off/Doze behavior while
   connected; how the notification channel behaves; the battery
   feel vs. the notification-disclosed budget.
8. **Network switch** — Wi-Fi ↔ mobile data while connected.
9. **QUIC fallback** — whether the YouTube app falls back from
   QUIC/UDP to TCP as expected (UDP beyond DNS is dropped by
   design; routed apps must fall back, never silently leak).

## 2. The public v1 DRAFT (UNPUBLISHED)

Nothing here may be published until the ⛔ device gates (§"Open
before release" below) close or Ali explicitly accepts shipping with
them recorded as known limitations. M8 performed the final copy
review against the shipped debug artifact.

### What it does

- Routes the TCP traffic of the apps you add to the allow-list (the
  list starts empty — D83; it is editable in-app, changes apply on the
  next connect; an empty list or all-uninstalled entries refuse the
  connect with the honest reason).
- Resolves the routed apps' DNS through the secure-DNS (DoH) providers
  you configure, tried in order (defaults: RethinkDNS, ElectroDNS,
  Ruby).
- Splits the first TLS record for the routed YouTube domains into
  smaller pieces — the same fragmentation the desktop edition uses.
- Leaves every website ending in .ir completely untouched — no
  unblocking, no fragmentation: always a direct connection, exactly as
  if Younblock were not running (D83; their name lookups still use
  your configured secure-DNS providers).
- Keeps a persistent, honest status notification while routing: active
  only when traffic actually routes, degraded when partially working,
  connecting otherwise.
- Carries the desktop edition's self-test, automatic recovery, and
  redacted diagnostics report.

### What it does NOT (the truthful-claims review, doc 07 §1)

- No IP hiding, no added encryption — YouTube's own HTTPS protects the
  content.
- No remote VPN server; nothing leaves the device except the routed
  apps' own traffic and their DNS queries to your configured DoH
  providers.
- No analytics, no crash reports, no account, no data collection.
- No claim to work with "every app": only the allow-listed apps' TCP
  and DNS are processed; other apps and protocols are untouched.

### Scope and limitations (doc 02 §4 — the release-notes obligations)

- **IPv4 only for DNS resolution**: AAAA records are answered empty so
  routed apps stay on IPv4 (the desktop default). IPv6 traffic from
  routed apps is routed into the tunnel and dropped (fail-safe —
  visible breakage for that flow, never a silent leak past the engine).
- **UDP beyond DNS is dropped**: routed apps fall back to TCP. The
  YouTube app's QUIC-to-TCP fallback behavior is a device gate (§Open)
  — reported when measured, not promised here.
- **Supported path**: launchable Android apps over TCP (added by you;
  the tested circle is Chrome and the YouTube app), with the
  configured DoH providers. No support claims beyond the per-app
  allow-list and the tested matrix.
- **.ir websites are always direct** (D83) — never routed, fragmented
  or redirected; non-launchable packages cannot be verified and are
  refused as not-installed (the launcher-visibility limit, doc 07 §6).
- **Per-app list changes apply on the next connect** (the list is
  baked into the TUN at establish).
- **Nothing auto-starts after reboot** (AND-028): connecting is always
  a user action.
- Requires Android 8.0+ (minSdk 26); targets Android 16 (API 36).
- Distributed as a sideloaded APK — debug-signed for the current test
  circle (D84); release signing per doc 07 §8 when a public release
  happens.

### Open before release (⛔ — the device gates)

The build environment cannot run them (no /dev/kvm, no device — doc
09's standing blocker); **never claimed, never faked**. Since D84
they have their closure path — Ali's own device, through the debug
test circle above; his observations become the recorded evidence:

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

### Install (doc 07 §8)

Sideload only: allow installs from the chosen source, accept the
Play Protect warning for unknown developers, install the APK. When
the signed lane returns: losing the release keystore means
uninstall-before-update for everyone who installed the signed build
(README documents backup steps).

### Open-source components

gVisor netstack (Apache-2.0), gomobile (BSD-3), and other permissive
Go libraries — the full audited set in doc 07 §7 (re-verified against
the final artifact at M8). Attribution in-app: the About sheet.
