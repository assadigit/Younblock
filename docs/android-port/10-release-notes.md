# 10 — Release Notes (v1 DRAFT + the shipped debug test circle)

**Status:** updated at the D86 session (2026-10-17, the device-test
feedback loop's second round). This document now holds two things: §1
the **shipped** debug test-circle release (`v1.6.6-android-debug.2`, a
prerelease on the private repo — the same text lives on the GitHub
release itself; `v1.6.6-android-debug.1` is superseded by it and kept
as history below), and §2 the **public v1 DRAFT**, unpublished until
the ⛔ device gates close or Ali explicitly accepts shipping with them
recorded as known limitations. Every claim traces to implementation
evidence (docs 01–09, DECISIONS.md D1–D86).

## 1. v1.6.6-android-debug.2 — the device test circle (SHIPPED, prerelease)

A debug-signed build for **Ali's own device testing** — "we're not
releasing yet" (D84): this is not a public release. Expect Android's
"built for testing" and Play Protect warnings for unknown developers;
both are expected and honest for this circle.

**The honest headline.** Younblock for Android helps YouTube load
when the network gets in the way. It routes **only the apps on its
list** — the YouTube app is added by default, and you add or remove
apps in the menu (D86) — through an on-device engine that uses secure
DNS and TLS record splitting, exactly like the desktop edition. It is
**not a VPN in the privacy sense**: it does not hide your IP address,
adds no encryption, and sends nothing to any Younblock-operated server
(none exists). It uses Android's VPN slot as a **local
traffic-routing interface** — that is the key icon you will see while
it runs.

**What changed since debug.1** (Ali's observation: "the first version
didnt work, whether in youtube app or youtube in chrome" — root
cause: debug.1's list was born empty (D83) and its entry path demanded
exact package names, so typing "youtube" was refused and nothing was
ever routed):

- **The YouTube app is the default list entry (D86)** — a fresh
  install routes the YouTube app out of the box, and a debug.1 install
  updating to this build gets the default through the one-time v3
  settings migration (its saved empty list was the shipped default,
  never a choice).
- **Adding apps is now usable**: the menu's Apps-to-unblock row gains
  one-tap quick-add buttons for the known apps (YouTube, Chrome) and
  the type-in field accepts friendly names — "youtube" and "chrome"
  resolve to their packages; other apps still need the package name.
- Same debug certificate as debug.1 (pinned in the repo, D85): this
  build **installs over** debug.1 as an update — no uninstall needed,
  settings survive.
- YouTube **in Chrome** still needs Chrome on the list (it is not part
  of the default — D86 implements the literal ruling): open the menu,
  tap the **+ Chrome** chip, reconnect. youtube.com inside other
  browsers is the same one-tap add.

**The artifact.** `app-debug.apk`, 35,716,265 B, SHA-256
`9cca38c9e904755ea2a4359650b8815c2756cf4f88272f91ca1809190026811b`;
applicationId `io.github.assadigit.younblock`, versionName 1.6.6,
versionCode 2; Android 8.0+ (minSdk 26), targets API 36; native code
for arm64-v8a, armeabi-v7a and x86_64; the same pinned debug
certificate as debug.1 (SHA-256 digest `c76bb7c61ec623892eabc37ac90ac
1bc0041d39490f2d48073676d6e2ed8551d`, at `android/app/debug.keystore`
since D85), apksigner v2-scheme verified; the four-permission manifest,
no testOnly.

**Superseded: v1.6.6-android-debug.1** (versionCode 1, SHA-256
`ccf46a2e…7b64`) — the first test-circle build; shipped with the
born-empty list (D83) whose entry-path gap is what debug.2 fixes. Its
release page carries the install-blocked troubleshooting guide (D85),
which still applies unchanged to downloads from Iranian networks.

**Install (private repo, doc 07 §8).** Download the APK from this
release with your GitHub account, allow installs from your browser
app, accept the Play Protect warning, install. If debug.1 is already
installed, this build updates it in place. Nothing auto-starts;
connecting is always a user action.

### If the install is blocked — read this first (D85)

Ali's first observation: "Android didn't let me install it." The
artifact was exonerated end-to-end first (the served bytes hash to
the recorded sha256; apksigner/badging/manifest clean; no testOnly;
zip intact) — and the asset's download_count was **0**, pointing at
an interrupted download as the probable cause. The ordered path now
lives on the release body itself:

1. **The download didn't complete** (most likely from Iranian
   networks — release assets are served from
   release-assets.githubusercontent.com, frequently slow/unreliable
   there). Check the file size: it must be **about 34 MB
   (35,567,561 bytes)** — a few-KB file is an error page, not the
   APK. Retry on Wi-Fi, or download on a PC and copy via USB (the
   most reliable route).
2. **"For your security, your phone is not allowed to install
   unknown apps from this source."** → Settings on the dialog →
   allow **Install unknown apps** for the opening app (Chrome /
   Files) → back → Install.
3. **Play Protect "Unsafe app blocked"** → More details →
   **Install anyway (unsafe)** — expected for a debug-signed unknown
   developer.
4. **"There was a problem parsing the package"** → usually also a
   truncated download; if the size is right, check the Android
   version — **8.0+ required**.
5. **"App not installed"** at the end → free storage; uninstall any
   older Younblock first.

If none match, the exact on-screen wording + Android version is the
feedback this circle needs.

### Device test circle — what to test and report

The build environment has no emulator and no device (verified — doc
09's standing blocker), so every device-side claim is **unclaimed
until observed on your device**. This circle is the checklist's
closure path; report pass/fail per row plus the in-app diagnostics
report (Menu → Diagnostics → copy) and your device model + Android
version. **If anything "doesn't work", the diagnostics report is the
single most valuable thing to send** — it carries the engine's own
view of what happened, redacted for privacy:

1. **Install + first launch** — the sideload path works (the D85
   troubleshooting guide covers blocked installs); the app opens on
   the connect screen; rotation, font scaling and TalkBack behave;
   the sheet's Apps-to-unblock row renders **with the YouTube app
   already on the list and the + Chrome quick-add chip visible**.
2. **First connect (the default list)** — the YouTube app is routed
   out of the box: the consent explanation → the system VPN dialog →
   Allow; the key icon appears; the notification honestly tracks
   connecting / active / degraded. (If you emptied the list on
   purpose, the connect refuses with the honest NO_TARGET_APPS
   message instead.)
3. **The product's purpose** — YouTube actually loads in the
   YouTube app (browse + video playback) while connected; the
   self-test and recovery behave when YouTube stops responding.
4. **youtube.com in Chrome** — tap **+ Chrome** in the menu,
   reconnect, and YouTube loads inside Chrome as well (Chrome is NOT
   part of the default — D86 — so this row tests the one-tap add
   path too).
5. **.ir stays direct** — with a routed app (e.g. the browser you
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

- Routes the TCP traffic of the apps on the allow-list (the YouTube
  app is the default entry — D86; the list is editable in-app,
  changes apply on the next connect; an emptied list or
  all-uninstalled entries refuse the connect with the honest
  reason).
- Resolves the routed apps' DNS through the secure-DNS (DoH) providers
  you configure, tried in order (defaults: RethinkDNS, Cloudflare,
  Google — matching internal/config's defaultDoHServers; this line
  previously named a stale set and was corrected at D86).
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
