# 04 - Android Spec (separate, later run)

**Audience:** a coding agent. **Start only after** desktop phases 1-5 (file 01) are accepted and Ali says go. Locked decision (Ali chose "one engine, one UI"): the **same Go engine** packaged as an Android library, plus the **same Svelte UI** shown inside a thin Kotlin shell. Read `00-start-here.md`, `01-go-backend-rewrite-spec.md` (sections 3, 7 and 10 especially) and `02-ui-design-spec.md` first.

## 0. Plain-language summary (for Ali)

On Android there is no "set a proxy for Chrome" switch the way Windows has. The only way for an app to handle other apps' traffic is Android's VPN feature. Younblock uses it as a **local tunnel only**: no server of ours, no account. It sends only Chrome's and the YouTube app's traffic through the same engine you already have on Windows, which looks up addresses with secure DNS and sends the first "hello" message in small pieces. Everything else on the phone is untouched. Friends install an APK file directly, since this is not for the Play Store. The first time, Android shows a standard "set up a VPN connection?" box; the app explains it beforehand.

## 1. Goal and non-goals

**Goal:** an installable APK with the same one-button experience, the same design, the same self-healing, working for Chrome and the YouTube app on Android.

**Non-goals:** not a real VPN (no remote server, no IP hiding); not for the Play Store; no accounts or analytics; no routing of other apps unless chosen in Advanced; no iOS.

## 2. Architecture

```
 Svelte UI (same bundle as desktop, adapter: android.ts)
        |  JS bridge (JSON in, JSON out, events out)
 Kotlin shell: MainActivity + WebView, YounblockVpnService, Quick Settings tile, notifications
        |  gomobile AAR (JSON API + callbacks)
 Go engine (same packages as desktop; platform adapter: mobile/)
        |
 TUN device from VpnService -> userspace TCP/IP stack -> SOCKS5 front door -> shared tunnel code
```

Rules: reuse `engine`, `fragment`, `doh`, `health`, `stats`, `logx`, `netx` and `proxy/tunnel.go` **unchanged**. Android adds only: a SOCKS5 front door, a DNS interceptor, a platform adapter, and the Kotlin shell. If the shared packages must change, say why in `DECISIONS.md`.

## 3. Traffic path (the part to get right)

1. **Per-app routing.** `VpnService.Builder.addAllowedApplication(pkg)` for each installed app in the target list, default `com.android.chrome` and `com.google.android.youtube`. Only those apps enter the tunnel. If none is installed, return error `NO_TARGET_APPS`. (Android does not allow mixing allowed and disallowed lists; use allowed only.)
2. **Tunnel interface.** Virtual address (for example `10.111.222.1/24`), route `0.0.0.0/0`, one virtual DNS server address, MTU 1500. Read packets from the TUN file descriptor into a userspace TCP/IP stack.
3. **TCP.** Each TCP flow from the stack is handed to the engine's **SOCKS5 front door** (new `internal/proxy/socks5.go`: CONNECT only, IPv4, IPv6 and domain address types, loopback only, no auth) which calls the same `tunnel.go` as the desktop HTTP front door: dial the destination IP through `netx.Dialer`, fragment the first TLS record, copy both ways. Destination arrives as an IP address, which is enough; no hostname is needed for fragmenting.
4. **DNS.** The stack answers UDP port 53 to the virtual DNS address through the engine's DoH resolver. **A** queries get answers from DoH. **AAAA** queries get an empty successful answer, so apps use IPv4 only and no IPv6 traffic bypasses the tunnel. Other record types: forward through DoH or return empty; record the choice.
5. **Other UDP, including QUIC (UDP 443): drop.** The YouTube app and Chrome then fall back to TCP, where the fragmenting applies. Verify on a real phone; if the YouTube app does not fall back cleanly, report it instead of guessing a fix.
6. **Protect our own sockets.** Every outgoing socket from the engine (upstream connections and DoH) must be exempted from the tunnel with `VpnService.protect(fd)`, otherwise traffic loops back into the tunnel. Implement `netx.Dialer`'s `Control` hook with a Kotlin-implemented `SocketProtector { Protect(fd int32) bool }` passed through gomobile. **Test that every dial calls it** (a fake counts calls).
7. **If the service dies or is revoked,** Android removes the tunnel and traffic goes direct. This is the safe failure mode; make `onRevoke` and `onDestroy` set the state to `off`.

**Library choice (agent decides, records in `DECISIONS.md`):** options for the userspace stack are a Go tun-to-SOCKS library built on gVisor netstack (for example `xjasonlyu/tun2socks`), gVisor netstack used directly, or a small C tunnel library (for example `hev-socks5-tunnel`) through the NDK. Criteria: APK size, maintenance, license compatibility, works with `gomobile bind`, passes the tests below. **Study as references only** (as far as I know, both exist as open source: Intra from Jigsaw, and ByeDPI-based Android apps): check their licenses before reusing any code; do not copy code without checking.

## 4. Go side (`mobile/` package, `internal/platform/android`)

- `mobile` exposes a gomobile-friendly API: `NewEngine(configDir string, protector SocketProtector, sink EventSink) *Engine`, plus the JSON methods from file 01 section 10 (`GetState`, `Start`, `Stop`, `Toggle`, `GetSettings`, `SaveSettings`, `RunConnectionTest`, `AutoFix`, `GetDiagnosticReport`, `GetAppInfo`) as string-in, string-out. gomobile cannot pass slices of structs or maps, which is why everything is JSON.
- Platform adapter: `Name() = "android"`, capabilities `{tray:false, startWithOS:false, systemProxy:false, perAppRouting:true}`, `Attach`/`Detach` no-ops (the VPN service owns routing), `DialControl` = the protector hook, `ConfigDir` = the app's private files directory, `Notify` = callback into Kotlin.
- Settings gains `apps` (list of package names; default Chrome and YouTube) used only on Android.
- Build: `gomobile bind -target=android -androidapi 26 -o android/app/libs/younblock.aar ./mobile` for arm64-v8a and armeabi-v7a (add x86_64 for the emulator in debug builds). Prerequisites: Android SDK, NDK, JDK 17, `golang.org/x/mobile/cmd/gomobile` and `gomobile init`. Verify current requirements before use.

## 5. Kotlin shell (`android/`)

Plain Kotlin and Gradle; no Compose, no Capacitor. Minimum SDK 26; target the latest stable SDK (verify). Permissions: `INTERNET`, `FOREGROUND_SERVICE` with the correct foreground service type for a VPN on the target SDK (**verify the current Android documentation**), `POST_NOTIFICATIONS` (Android 13+ runtime request), service declared with `BIND_VPN_SERVICE`. Add a `<queries>` entry for the target packages so installed-app checks work on Android 11+.

- **MainActivity + WebView:** load the built UI from `assets/ui` through `WebViewAssetLoader` (host `appassets.androidplatform.net`); no network access to remote pages; block navigation to anything else; disable file and content access; no remote debugging in release builds; edge-to-edge with safe-area insets.
- **JS bridge:** `addJavascriptInterface(Bridge, "YounblockBridge")` with `call(method: String, argsJson: String): String` for quick calls and `callAsync(callId: String, method: String, argsJson: String)` for long ones (results delivered with `evaluateJavascript("window.__younblockResolve(callId, json)")`). Events: `window.__younblockEvent(name, json)`. Only our own local page can reach it.
- **`YounblockVpnService : VpnService`:** builds the tunnel (section 3), runs as a foreground service with a notification ("Younblock is on", action "Turn off"), starts the engine, forwards events. Handles the consent flow: `VpnService.prepare()` then the system dialog, with the app's own explanation shown first.
- **Quick Settings tile (`TileService`):** toggles on or off and mirrors the state. If consent was never given, it opens the app.
- **Back button:** inside a view goes back; on Connect it sends the app to the background (never stops the VPN).
- **Battery:** no wake locks; timers run only while On; zero work when Off. Add a Help entry "Keep Younblock running" explaining battery-optimization exemptions for phones that kill background apps.

## 6. UI changes (same Svelte code, driven by capabilities)

- `lib/adapters/android.ts` implements the `Backend` interface over the bridge (file 02, section 9). Add a mock for the bridge so the UI can be tested in a browser.
- Settings: hide "Open Younblock when Windows starts" (no `startWithOS`); in Advanced show **Apps to unblock** (checkboxes for installed supported apps; default Chrome and YouTube).
- Footer note: "Works with Chrome and YouTube" (built from the `apps` list).
- First-run text on Android: "Android will ask to set up a VPN connection. Younblock uses it only as a local tunnel for Chrome and YouTube on this phone. Nothing is sent to any Younblock server." Button "Continue" opens the system dialog.
- Error `NO_TARGET_APPS`: "Chrome and YouTube aren't installed on this phone."
- About: add one honest line: "Android shows a key icon because Younblock uses its VPN slot. It is not a VPN."
- Everything else (button, states, timer, quiet line, menu, Help test, copy) is unchanged. Layout is already fluid (file 02, section 3).

## 7. Build, signing, delivery

- Scripts `build-android.bat` and `build-android.sh`: (1) build the UI for Android into `android/app/src/main/assets/ui`; (2) `gomobile bind`; (3) `gradlew assembleDebug` and `assembleRelease`; output `dist/younblock-<version>.apk` and its SHA-256.
- Release signing: generate a keystore locally; **never commit it or its passwords** (`.gitignore`). `README.md` tells Ali, in plain steps, to back it up, because losing it means friends must uninstall before updating.
- Target APK size 25 MB or less (use ABI splits if needed).
- Friends install by opening the APK and allowing installs from this source; Google Play Protect may show a warning. Document both in `README.md`.

## 8. Phases and acceptance criteria

**A1 - Go additions and AAR.** SOCKS5 front door, DNS interceptor, protector hook, `mobile` package, `gomobile bind` works. Tests: SOCKS5 parsing and fuzzing; DNS (A answered through a fake DoH, AAAA empty, other UDP dropped); the protector is called for every dial; `go test -race ./...` and the platform-neutral checks still pass.
**A2 - Kotlin shell and VPN service.** The service starts and stops, the foreground notification shows, state events reach the WebView, `onRevoke` returns to `off`. Test on the emulator.
**A3 - UI adapter and Android screens.** The same screenshots as file 02, section 10, at 360 x 800, plus the Android first-run and Apps-to-unblock row.
**A4 - Tile, notifications, polish.**
**A5 - Build scripts, signing, README, report.**

**Manual check for Ali on a real phone** (numbered in `README.md`): install, open, allow the VPN box, press the button, open YouTube in Chrome, then the YouTube app, press off and confirm both still work normally, turn on and swipe the app away and confirm the notification or tile still controls it, reboot and confirm nothing starts by itself.

## 9. Honest limits

- The agent can build the APK and test most logic on an emulator, but cannot prove unblocking on Ali's real network or every phone brand. Some manufacturers kill background apps aggressively; the Help entry mentions it.
- Only one VPN can run at a time on Android, so Younblock and a real VPN app cannot be on together.
- The YouTube app's behavior when UDP/QUIC is dropped must be confirmed on a real phone (section 3, step 5).
- Android versions differ in foreground-service and notification rules; follow the current documentation for the target SDK and report any deviation.

## 10. Rules for you, the agent

Complete files, no placeholders. Verify every API against installed versions and current Android documentation. Never invent gomobile, Gradle or Android calls. Run what you can (Go tests, emulator checks, `assembleDebug`) and report real output. Say plainly what only a real phone can confirm.
