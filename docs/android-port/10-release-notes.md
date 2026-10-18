# 10 — Release Notes (v1 DRAFT + the shipped debug test circle)

**Status:** updated at the A0 session (2026-10-18 — the account-gate
spec round: the six-document design pack now exists under `docs/auth/`
per D94's design-before-code rule, plus one host-side fix — the
missing `//go:build unix` on `mobile/nonblock_unix.go`, which had kept
`GOOS=windows go build ./...` from ever compiling since M4; NO
Android change, so `v1.6.6-android-debug.6` remains the CURRENT build,
and the D93 close ruling still rides Ali's next rebuilt desktop exes /
a future debug.7). Updated again at A0-R3 (2026-10-18, the third
exchange of the A0 day): **the account gate is CANCELLED (D97)** —
see the closing note of this status block. This document now holds two things: §1 the
**shipped** debug test-circle releases (`v1.6.6-android-debug.6` is
the CURRENT build — a regular release in the private repo per D89,
the same text lives on the GitHub release itself; beneath it:
debug.5 the first-device-feedback build, debug.4 the helper-fix
maintenance release, debug.3 the device-circle build whose APK ran
on Ali's phone, and debug.2/debug.1 superseded and kept as history),
and §2 the **public v1 DRAFT**, unpublished until the ⛔ device gates
close or Ali explicitly accepts shipping with them recorded as known
limitations. Every claim traces to implementation evidence (docs
01–09, DECISIONS.md D1–D97, of which D97 cancels the gate D94 had
announced). The D94 announcement is recorded here
for the release notes' future: when the account gate ships, the
"no account, no data collection" line in §2's What-it-does-NOT
section changes with it — disclosed at that release, never silently
(today nothing of it exists: no login, no server, no database; the
drafted replacement copy lives in docs/auth/05 §6, and doc 07 §9's
open row blocks whichever release first carries the gate).
**A0-R3 closing note (D97, 2026-10-18): the gate is CANCELLED — it
will never ship, so none of that future ever happens. The "no
account, no data collection" line in §2 stays verbatim PERMANENTLY;
no release ever changes it for a gate; `docs/auth/` carries dated
cancellation banners and stays as history; doc 07 §9's row is closed
as cancelled; ADR-AUTH-0001 is retired (doc 08). Younblock stays one
free-for-all app.**

## 1a. v1.6.6-android-debug.6 — one UI on every device + data usage (SHIPPED — a regular release, the CURRENT build)

> Note added at D93 (2026-10-18): the shared tree has since gained the
> desktop close ruling (closing the window is a real shutdown by
> default; tray-keeping is the opt-in setting) — nothing an Android
> build renders differently (no window close, no tray, the row is
> capability-gated off), so debug.6 stays the current APK; the ruling
> rides the next debug build whenever an app change next warrants one.

**What this build is:** versionCode 5, built at 078e3a5, APK
34,951,881 B (SHA-256 bc4b239e…d1fe8), the pinned debug certificate
— **installs directly over debug.5** in place; settings survive
(but not usage history — it starts fresh with this build's new
file). It carries ONE theme: **Ali's uploaded instruction rounds
(mobile.zip) applied to the one shared screen, so the phone, the
Windows desktops, the Linux editions and the web edition render the
same UI by construction** (D92):

1. **The round-2 audit fixes, everywhere:** real Lucide icons
   through one file (the power mark, the “…” menu button now at
   the top-right at 44 px, the pill's chevrons, the usage arrows,
   the sheet's X); the menu sheet has a proper header — title
   left, neutral 44 px close right — with the body scrolling
   underneath; the countdown digits sit in 0.80 em boxes so they
   can never touch; the connected ring family stops clear above
   the hint, which is bigger and brighter; the ping line gets a
   word (Good / OK / Slow); the About section carries “Created by
   ChitaCodes”; the honest Other-websites section exists; and the
   desktop window became 360×640, resizable, floored at 320×560.
2. **The round-3 data-usage feature, new on every platform:**
   while connected the screen shows **“X used”** with the ↓/↑
   split under the timer, counting up live; after a disconnect,
   **“Last session: size · length”** sits under Not Connected; the
   menu's new **Data usage** section shows Today / This month /
   All time, the last sessions, and a confirmed Clear history
   (Delete vs Keep — only Delete clears). The history is one
   local file — times and byte totals only, no website names,
   never sent anywhere — capped at 500 sessions, autosaved every
   15 s, and recovered after a crash or power loss. The live
   Mb/s row is off by default now (the total is what matters).

**Verified before shipping** (the reference code folder was not in
the upload, so the acceptance items ran the honest way): the full
test baseline green — 18 packages, twice, race detector on the
touched ones; the screen rendered and exercised at 320×560,
356×625 and 360×760 across every state with no overlap and no
scrollbars; the byte path proven on a real engine (a 949 KB
YouTube page through the tunnel = a recorded 980,303-byte session
in the history file after the stop).

**→ What to check on your devices, Ali:** install debug.6 on the
phone AND rebuild the desktops from this tree (build.bat / build.sh)
— the UIs should now match yours exactly; watch the data counter
climb while browsing, look at the Last-session line and the menu's
Data usage after a disconnect, and try Clear history (both
buttons). The video-start diagnostics ask from debug.5 still
stands when a video is slow.

## 1a. v1.6.6-android-debug.5 — the first device feedback answered (SHIPPED — a regular release, superseded by debug.6)

**What this build is:** versionCode 4, built at ad02980, APK
34,831,081 B (SHA-256 5df2acc3…02b4), the pinned debug certificate —
**installs directly over debug.3** in place; settings survive. It
carries TWO changes from the first real device round:

1. **The numbers-spacing fix (Ali's report: "the numbers in the app
   are too near each other").** Root cause: the connect screen's
   stage was fixed at 22.5 rem — right for a 360 px viewport at 1×
   font scale, but on narrower screens or with Android's font-scale
   setting it outgrew the screen and pushed the timer and the two
   speed values against the edges (measured: 495 px of stage in a
   320 px viewport at 1.4×). The stage now never exceeds the
   viewport, the two speed numbers carry wider spacing that never
   wraps, and the timer digits got more air. Verified at the worst
   case AND the canonical case before shipping (D91).
2. **The video-start instrumentation (Ali's report: feed refreshes
   fast, but playing a video takes 30 s–1 min+ to show).** The
   host-side suspects were each eliminated with evidence (D91 —
   including a new test that PINS the QUIC refusal contract: non-DNS
   UDP gets an immediate ICMP port-unreachable, so routed apps fall
   back to TCP fast instead of blackholing). What remains is
   device/network-side, so this build logs every tunnel's phases —
   DNS time, dial time, first-byte time, transfer totals — into the
   log ring the redacted diagnostics report carries.
   **→ When a video is slow on your phone: Menu → Diagnostics →
   Copy, and send it** (plus phone model, Android version, and
   whether it's every video or just the first after connecting).
   The next report names the phase that eats the time.

Also answered in the release body, no code change: the **Play
Protect popup** at install is expected for a sideloaded
debug-signed APK (Android being cautious about unknown
signatures — not a defect), and the **release keystore needs
nothing now** (keep it private; it is NOT used for the test circle —
signing a test build with it would break the installs-over-debug.3
update chain; it waits for the signed public release).

## 1. v1.6.6-android-debug.3 — the device test circle (SHIPPED — a regular release in the private repo, D89)

A debug-signed build for **Ali's own device testing** — "we're not
releasing yet" (D84): this is not a public release. Expect Android's
"built for testing" and Play Protect warnings for unknown developers;
both are expected and honest for this circle.

**The release flag (D89, 2026-10-17).** This build and the two
earlier debug builds are now published as regular releases, not
prereleases — Ali's ruling: "Do not prerelease, just release, the
repo is private." The repo is private, so the flag carried no public
meaning and the releases remain visible to the repo's collaborators
only. Nothing about the build changed: it is still the debug-signed
device-test build this page describes.

**The honest headline.** Younblock for Android helps YouTube load
when the network gets in the way. It routes **only the apps on its
list** — the YouTube app **and the Chrome, Firefox, Opera and Brave
browsers are added by default** (D87), and you add or remove apps in
the menu — through an on-device engine that uses secure DNS and TLS
record splitting, exactly like the desktop edition. It is **not a VPN
in the privacy sense**: it does not hide your IP address, adds no
encryption, and sends nothing to any Younblock-operated server (none
exists). It uses Android's VPN slot as a **local traffic-routing
interface** — that is the key icon you will see while it runs.

**What changed since debug.2** (Ali's ruling: "Add chrome to the
default addlist as well, also firefox, opera, brave browser"):

- **The four browsers join the YouTube app on the default list (D87)**
  — a fresh install routes the YouTube app and every installed one of
  Chrome, Firefox, Opera and Brave out of the box. A browser you
  don't have installed is simply skipped (it can never break the
  connect); install it later and it routes on the next connect.
- **A debug.2 install updates to the new default automatically**: its
  saved [YouTube]-only list was the shipped default of that build,
  never a hand-made choice, so the one-time v4 settings migration
  upgrades it to the five apps. A debug.1 install (born-empty list)
  migrates straight to the five apps. A list YOU edited (added or
  removed apps) is kept exactly as you left it — and the quick-add
  buttons now cover all five defaults, so regaining any of them is
  one tap.
- **Adding apps is still the D86 path, widened**: one-tap quick-add
  buttons for the known apps (YouTube, Chrome, Firefox, Opera,
  Brave) and friendly names in the type-in field — "youtube",
  "chrome", "firefox", "opera", "brave" all resolve; other apps
  still need the package name.
- Same debug certificate as debug.1/debug.2 (pinned in the repo, D85):
  this build **installs over them** as an update — no uninstall
  needed, settings survive.

**The artifact.** `app-debug.apk`, 35,816,233 B, SHA-256
`81515f0dd1b0f1ff2d3fb1a87769c8c6d4df3dc0225efeb975bd9f543257d6a1`;
applicationId `io.github.assadigit.younblock`, versionName 1.6.6,
versionCode 3; Android 8.0+ (minSdk 26), targets API 36; native code
for arm64-v8a, armeabi-v7a and x86_64; the same pinned debug
certificate as debug.1/debug.2 (SHA-256 digest `c76bb7c61ec623892eabc
37ac90ac1bc0041d39490f2d48073676d6e2ed8551d`, at
`android/app/debug.keystore` since D85), apksigner v2-scheme
verified; the four-permission manifest, no testOnly.

**Release-signing helper (added after the APK upload — M8-e / D88;
the APK itself is unchanged).** This release carries a second asset,
`make-release-keystore.bat` (11,527 B, SHA-256
`82e6dded40a8eb415127c6d46b3cb4acb0633ab2cd13ac8b809a8ae8c7c74090`,
pure ASCII, CRLF): the local one-double-click ceremony for the future
signed lane. Run it in a private folder on Windows and it generates
the keystore password **on your own machine** with Windows' crypto
RNG, creates `younblock-release.keystore` with every keytool question
pre-filled (RSA 4096, 30-year validity), verifies the result by
reading it back, and writes a `younblock-keystore-backup_<date>.md`
next to it — the password, the alias, the certificate serial number,
the SHA-256 fingerprint and the full keytool record. Not needed for
this debug build; it exists so the signed lane is ready the day you
decide to publish (doc 07 §8 / D88). The keystore and that backup
never go in the repo.

**The helper-fix maintenance release: v1.6.6-android-debug.4 (D90,
2026-10-17 — no new APK).** Ali's first run of the helper on his
machine stopped at password generation — "'powershell' is not
recognized as an internal or external command": his PATH variable
does not include PowerShell (Windows itself ships it; the desktop
setup scripts took the same fix at v0.7.5). The fixed
`make-release-keystore.bat` (13,683 B, SHA-256
`ee35e4565dadf2a4a1c6e150208b8749aa6e5c3b8aec6303cc57025c06939558`,
pure ASCII, CRLF) resolves PowerShell by its full standard install
path instead of through PATH, and if PowerShell cannot be found at
all the script's own fallback generator builds the password — the
ceremony always completes, and the .md backup records which
generator ran. debug.4 is a regular release (D89) carrying ONLY the
fixed helper: **the app itself is unchanged — keep using debug.3's
`app-debug.apk` above** (D84: identical bytes are never re-released;
debug.3's page carries the pointer note superseding its older helper
asset). Ali's "can't I use an online keyring generator instead?" is
answered NO (D90): the keystore is the app's signing identity —
whoever holds it and its password can sign updates as you — so it
is made once, on your own machine, and never passes through any
server, website or chat. The release body documents a one-line
manual keytool alternative for machines without PowerShell.

**Superseded: v1.6.6-android-debug.2** (versionCode 2, SHA-256
`9cca38c9…811b`) — the D86 build: it put the YouTube app on the
default list and fixed the entry path (quick-add chips + friendly
names); its [YouTube]-only default is what debug.3's v4 migration
upgrades. **Superseded: v1.6.6-android-debug.1** (versionCode 1,
SHA-256 `ccf46a2e…7b64`) — the first test-circle build; shipped with
the born-empty list (D83) whose entry-path gap is what debug.2 fixed.
Its release page carries the install-blocked troubleshooting guide
(D85), which still applies unchanged to downloads from Iranian
networks. Their release flags were switched to regular releases
together with debug.3's (D89, 2026-10-17); their bodies stay exactly
as published — history.

**Install (private repo, doc 07 §8).** Download the APK from this
release with your GitHub account, allow installs from your browser
app, accept the Play Protect warning, install. If debug.1 or debug.2
is already installed, this build updates it in place. Nothing
auto-starts; connecting is always a user action.

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
   (35,816,233 bytes)** — a few-KB file is an error page, not the
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

**The debug.6 round (D92) adds two checks of its own, above the
rows below: (a) the cross-device UI match — install debug.6 on the
phone AND rebuild the Windows/Linux desktops from the same tree;
the screens should now be identical to Ali's instruction-following
builds (menu top-right "…", the sheet's header close, Lucide
icons, the usage lines); and (b) the data-usage feature — watch
"X used" climb while browsing, the Last-session line after a
disconnect, the menu's Today / This month / All time, and Clear
history with both buttons (Keep must keep, Delete must empty and
remove the Last-session line).**

1. **Install + first launch** — the sideload path works (the D85
   troubleshooting guide covers blocked installs); the app opens on
   the connect screen; rotation, font scaling and TalkBack behave;
   the sheet's Apps-to-unblock row renders **with the five default
   apps already on the list** (YouTube, Chrome, Firefox, Opera,
   Brave — whichever are installed).
   **Observed on Ali's device (debug.3, 2026-10-17): PASS with a
   note** — "it worked on android"; the install completed; a Play
   Protect popup appeared (the expected unknown-developer warning
   for a sideloaded debug APK — D91 documents it; no action). The
   font-scaling half arrived via the spacing complaint (the M6
   gate's first live data — fixed in debug.5); rotation and
   TalkBack remain unreported.
2. **First connect (the default list)** — the YouTube app and the
   installed default browsers route out of the box: the consent
   explanation → the system VPN dialog → Allow; the key icon
   appears; the notification honestly tracks connecting / active /
   degraded; a browser that is NOT installed is skipped without
   breaking anything. (If you emptied the list on purpose, the
   connect refuses with the honest NO_TARGET_APPS message instead.)
   **Observed on Ali's device (debug.3, 2026-10-17): PASS** — "the
   youtube app is routed through this system" and the feed
   refreshes fast while connected.
3. **The product's purpose** — YouTube actually loads in the
   YouTube app (browse + video playback) while connected; the
   self-test and recovery behave when YouTube stops responding.
   **Observed on Ali's device (debug.3, 2026-10-17): PARTIAL** —
   browse/feed fast; **video playback took 30 s–1 min+ to show**
   ("feed quickly refreshed but actual video playing delayed").
   The host-side suspects were eliminated with evidence and the
   per-flow timing instrumentation shipped in debug.5 (D91); the
   diagnostics report from a slow-video moment is the expected
   next input.
4. **youtube.com in a browser** — Chrome, Firefox, Opera and Brave
   are on the default list now (D87): youtube.com loads inside each
   installed one of them after connecting. For a browser you removed
   from the list, tap its **+** chip in the menu, reconnect — this
   row also re-tests the one-tap add path.
5. **.ir stays direct** — with a routed app (e.g. one of the default
   browsers), any `.ir` site loads as if Younblock were not there: no
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
   **Host half PINNED by test (D91)**: the stack answers non-DNS
   UDP (443) with an immediate ICMP port-unreachable
   (TestUDP443FastFailICMP) — the phone kernel turns that into an
   instant refusal on the app's socket, the fast-fail path. The
   on-device observation (does HIS YouTube app actually fall back
   fast) rides with the video-start timing data.

## 2. The public v1 DRAFT (UNPUBLISHED)

Nothing here may be published until the ⛔ device gates (§"Open
before release" below) close or Ali explicitly accepts shipping with
them recorded as known limitations. M8 performed the final copy
review against the shipped debug artifact.

### What it does

- Routes the TCP traffic of the apps on the allow-list (the YouTube
  app and the Chrome, Firefox, Opera and Brave browsers are the
  default entries — D87; entries whose apps are not installed are
  skipped at establish; the list is editable in-app,
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
