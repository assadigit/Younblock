# 06 — Build and Test Plan

**Status:** Updated 2026-10-16 (Phase 1 complete). Verified values are
marked; SDK-dependent values carry their verification status. The
desktop baseline (§A) is recorded and green.

## 1. Reproducibility requirements

| Item | Selected value | Why | Evidence/date |
|---|---|---|---|
| Minimum Android API | **26** — VERIFIED at M3 | Carries forward Ali's prior decision (root 04-android-spec.md §5); FGS rules and per-app routing need ≥21; 26 covers the FGS-era requirements. The tun2socks/gVisor stack libraries impose no higher floor (pure Go). `aapt2 dump badging` reports `minSdkVersion:'26'` | Verified 2026-10-16 (M3, in the shipped APK) |
| Target/compile API | **36** — VERIFIED at M3 | Sideload distribution (ADR-0012) means Play's target-API policy is not binding; Android 16 / API 36 is the newest stable with the FGS `connectedDevice` rules settled. `targetSdkVersion:'36'`, `compileSdkVersion 36` in the APK badging | Verified 2026-10-16 (M3) |
| Android ABIs | arm64-v8a (primary), armeabi-v7a (secondary), x86_64 (debug/emulator only) | Device reach; Go compile matrix green on all four (doc 01 §5) | Verified 2026-10-16 (compile level); **runtime level in the M3 AAR/APK**: `native-code: 'arm64-v8a' 'armeabi-v7a' 'x86_64'` |
| JDK version | **21.0.8 (Temurin, with javac)** | Gradle 9.8.1 launcher + AGP 9.2.0 + gomobile all verified on it | Verified 2026-10-16 (M3) |
| Android Gradle Plugin | **9.2.0** (maven.google.com) | Compatible with Gradle 9.x; Kotlin support is built into AGP 9 (the separate kotlin-android plugin is rejected — D74) | Verified 2026-10-16 (M3) |
| Gradle wrapper | **9.8.1**, committed wrapper (jar + gradlew + properties) | Reproducibility; fresh clones build without a local Gradle | Verified 2026-10-16 (M3) |
| Go version | **1.27.2** (go.mod directive `go 1.26.3` — bumped by the gVisor dependency at M4) | Matches the desktop toolchain; vendored deps build under it | Verified 2026-10-16 (M3; re-verified M4 after the reset) |
| gVisor netstack | **gvisor.dev/gvisor v0.0.0-20260701204157-69c2d17aea96**, vendored (+6 MB, D68/D75) | The M4 userspace stack (internal/tunstack); the desktop lanes never compile it — the regression gate verifies | Verified 2026-10-16 (M4: vendored, 17/17 host tests, 4/4 ABI compile matrix) |
| gomobile/JNI strategy | **`gomobile bind` AAR** (ADR-0004) over `mobile/` facade; x/mobile pinned as a **`go get -tool` directive** (v0.0.0-20261011034528); the bind runs with `GOFLAGS=-mod=mod` because vendor mode cannot answer `go list -m all` (D73) | String JSON API matches gomobile constraints; x/mobile BSD-3, maintained | Verified 2026-10-16 (M3: AAR built + javap'd) |
| Userspace stack | **gVisor netstack direct** — ADR-0008's switch path exercised at M4 (D75); the tun2socks engine package is a CLI singleton whose UDP model demands a UDP-capable proxy, incompatible with the CONNECT-only door | MIT/Apache (Apache-2.0); production pedigree via the same core wiring tun2socks uses; device-measurable criteria ride with M5 | Verified 2026-10-16 (M4: internal/tunstack built and host-tested) |
| Node/package manager | npm, Node v24.21.0, committed lockfile | Frontend unchanged; same lane as desktop | Verified 2026-10-16 |
| wails CLI rule | desktop lanes only; always `-m` (D68) | Android lane never invokes wails | Verified (release v0.7.6 practice) |

Do not hard-code local SDK paths or credentials; the Android lane
script (`build-android.sh`/`.bat`, M8) documents every prerequisite.

## 2. Build validation layers

### A. Existing Go code — BASELINE GREEN (2026-10-16)

```sh
go version                 # go1.27.2 linux/amd64
go vet ./...               # clean
go test -count=1 ./...     # 13/13 packages ok
go build ./...             # clean
# frontend:
cd frontend && npx svelte-check   # 0 errors, 0 warnings
cd frontend && npx vite build     # clean
# Android compile matrix (evidence, not a substitute for runtime):
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -mod=vendor ./...   # exit 0
GOOS=android GOARCH=arm   CGO_ENABLED=0 go build -mod=vendor ./...   # packages ok (link needs NDK cgo)
GOOS=android GOARCH=386   CGO_ENABLED=0 go build -mod=vendor ./...   # packages ok (link needs NDK cgo)
GOOS=android GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor ./...   # packages ok (link needs NDK cgo)
```

Environment limitations (updated 2026-10-16, post-M3): no webkit2gtk →
no GUI `wails dev` (desktop E2E uses the vite preview adapter); **the
Android toolchain IS installed now** (doc 05 M3 item 1) — what remains
⛔ is runtime verification: this sandbox has no /dev/kvm (verified), so
no emulator, and no physical device; the Kotlin-level bridge call and
all M4 on-device gates wait for a KVM-capable host or a real device
(doc 09).

### B. Frontend

Unchanged bundle; the Android lane runs the same `npm ci` →
`vite build` and copies the output to the app assets (M3). No
Wails-generated bindings are needed (the http adapter is used;
`frontend/wailsjs/` is git-ignored and absent on Android).

### C. Android build (validated at M3 2026-10-16; re-validated with the
M4 TUN stack 2026-10-16; §5 verification applies per release)

The real module names (doc 04 §3); run through `build-android.sh`, or
individually:

```sh
gomobile bind -target=android/arm,android/arm64,android/amd64 \
  -androidapi=26 -o android/app/libs/younblock.aar ./mobile   # GOFLAGS=-mod=mod
./gradlew projects        # verify layout
./gradlew assembleDebug   # debug APK
# verification: aapt2 dump badging, apksigner verify --print-certs, sha256sum
```

M4 record (2026-10-16, after the sandbox reset re-cloned the repo at
`1d20460` and the toolchain was reinstalled — JDK 21.0.12.1 with
javac, cmdline-tools 12.0, platforms 26+36, build-tools 36.1.0, NDK
28.2.13676358, gomobile/gobind via `go install` at the pinned x/mobile
version): `gomobile bind` → younblock.aar **24,893,234 B** (3 ABIs;
the gVisor netstack added ~7 MB over M3's AAR); `assembleDebug` →
`app-debug.apk` **34,707,469 B**, SHA-256
`8e9f702a364f1e41a0bf12fa3a89971fced6e993632b56f74b44a55d06c8219b`,
badging: package io.github.assadigit/younblock 1.6.6 (versionCode 1),
minSdk 26, targetSdk 36, `YounblockVpnService` present in the dumped
manifest with `android.permission.BIND_VPN_SERVICE`; `apksigner
verify --print-certs` OK (debug cert). The Kotlin compile validates
the facade bindings (startTun/stopTun/getState) and the M4 shell
(Bridge.kt, YounblockVpnService.kt, MainActivity connect flow).

M5 record (2026-10-17, after yet another full sandbox wipe — the repo
re-cloned at `98cac30` and the toolchain reinstalled per the §1 pins:
Go 1.27.2, JDK 21.0.12.1+1 Temurin, cmdline-tools 12.0, platforms
26+36, build-tools 36.1.0, NDK 28.2.13676358, gomobile/gobind
`go install` at x/mobile v0.0.0-20261011034528-197770444867): `gomobile
bind` → younblock.aar **24,893,759 B** (3 ABIs; `javap` on classes.jar
shows the new M5 bindings `setVpnReady(boolean)` and `vpnState()`
alongside startTun/stopTun/getState); clean `assembleDebug` →
`app-debug.apk` **34,711,941 B**, SHA-256
`c46026a7984d9fda69b45954436c630f625a74fecf451e6e50c2a3d2b6d5bf26`,
badging minSdk 26/target 36, `YounblockVpnService` with
`BIND_VPN_SERVICE` in the dumped manifest, `apksigner verify
--print-certs` OK (debug cert). The Kotlin compile validates the M5
lifecycle shell (consent flow, single-flight, reconciliation, the
vpnState-derived notification). Baseline after: vet clean; `go test
-count=1 ./...` 17/17 packages ok (run twice consecutively — the
concurrent-package PORT_BUSY interaction that once failed the
lifecycle test was root-caused to the test harness's AutoPort=false
and fixed at the source, mobile/tun_test.go); svelte-check 0/0; vite
build clean; compile matrix 4/4 ABIs.

M6 record (2026-10-17, after ANOTHER full sandbox wipe — the third:
the repo re-cloned at `4083e76`, the worklog rebuilt from the session
prompt + repo records, and the toolchain reinstalled per the §1 pins
exactly as at M5; the desktop baseline was re-verified green — vet
clean, 17/17 twice, svelte-check 0/0, vite clean — BEFORE any M6 code):
`gomobile bind` (GOFLAGS=-mod=mod, 3 ABIs, androidapi 26) →
younblock.aar **25,097,538 B** (`javap` on classes.jar shows the M6
binding `routedApps()` alongside setVpnReady/vpnState/startTun/
stopTun/getState/getSettings/saveSettings); clean `assembleDebug`
(after one honest compile fix: the Kotlin read needed
`engine.routedApps()` call syntax + the org.json.JSONArray import —
the first clean attempt failed at compileDebugKotlin with both errors
printed, fixed, re-run CLEAN) → `app-debug.apk` **35,194,757 B**,
SHA-256
`5669efd14a0a523249e55c9f2c7ab0d6b1edb62156c60d76c3ccc38cc59302dc`,
badging: package io.github.assadigit/younblock 1.6.6 (versionCode 1),
minSdk 26, targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64,
`YounblockVpnService` + `BIND_VPN_SERVICE` in the dumped manifest,
`apksigner verify --print-certs` OK (debug cert). The Kotlin compile
validates the settings-driven allow-list consumption
(readRoutedApps); the APK embeds the M6 Svelte bundle (the
apps/privacy/diagnostics sheet). Exit baseline: vet clean; `go test
-count=1 ./...` 17/17 packages ok, run twice consecutively; svelte-check
0/0; vite build clean; Android library compile matrix 4/4 ABIs with
`-mod=vendor` (the honest form: `go build` over the 18 non-main
packages per ABI — on arm/386/amd64 the three main packages refuse
the link without NDK cgo, exactly as §A records; arm64 links the full
tree at CGO_ENABLED=0). Disk note: 4.5 GB free at the SDK/NDK restore,
~2.9 GB free after the clean APK — caches cleaned before the big
builds per the §1 hygiene rule.

M7 record (2026-10-17; the session opened with the environment INTACT
— no wipe this time — repo at 57adf1a, toolchain per the §1 pins
verified with commands, baseline re-run green before any M7 code):
the -race hardening lane added and green on the seven key packages
(engine 10.4 s, android 2.1 s, mobile 38.4 s, tunstack 2.3 s,
proxy 11.4 s, webui 1.1 s, config 1.0 s; `go test -race -count=1`,
gcc present); `gomobile bind` (GOFLAGS=-mod=mod, 3 ABIs, androidapi
26) → younblock.aar **25,072,133 B** (`javap` shows the M7 binding
`failStart(String)` alongside routedApps/setVpnReady/vpnState); clean
`assembleDebug` → `app-debug.apk` **35,298,149 B**, SHA-256
`f27b7eef014f61eff35405f4406bfe9c33d4e34a918f9949eaad417d561c6491`,
badging: package io.github.assadigit/younblock 1.6.6 (versionCode 1),
minSdk 26, targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64;
permission review against the dumped manifest: exactly doc 07 §6's
inventory (INTERNET, FOREGROUND_SERVICE, FOREGROUND_SERVICE_CONNECTED_
DEVICE, POST_NOTIFICATIONS, BIND_VPN_SERVICE service attribute,
<queries> chrome+youtube; ACCESS_NETWORK_STATE absent); apksigner
verify OK (debug cert). One engine regression caught and fixed at the
source while building D81: the immediate double-Start test failed
because D82's honest cancel exposed the spawn-race zombie the old test
codified (see DECISIONS.md D82); the full suite green twice after.
Exit baseline: vet clean; `go test -count=1 ./...` 17/17 packages ok
twice; svelte-check 0/0; vite clean; matrix 4/4 ABIs -mod=vendor
(18 non-main packages per ABI; arm64 links the full tree).

## 3. Test layers

### Unit tests (Go, host-runnable — pattern matches the existing tree)

- SOCKS5 door parsing/validation/fuzz edges; address-type matrix.
- DNS interceptor: A through a fake DoH; AAAA empty; other types
  empty; malformed input.
- `apps` settings: validation, defaults, migration v1→v2.
- Protect hook: every dial calls the counter (fake ControlFunc).
- Platform adapter: capability reporting, ConfigDir, Notify callback.

### Bridge tests (M3)

- Kotlin calls the real AAR: init → health → stop; error strings
  cross verbatim; repeated start/stop leaks nothing (goroutine count
  stable); events stop after Shutdown.

### Integration/network tests (M4 — emulator or device)

- Supported domain over the TCP path through the whole chain.
- Resolution via the configured DoH provider; DNS
  timeout/refusal/malformed behavior.
- Fragmentation behavior across stream chunk boundaries (the pure
  `Plan` tests already cover logic; this covers the wire).
- Reset/close/timeout/partial-read handling; concurrent connections;
  memory ceiling (`MaxConns` cap proven in desktop soak tests — repeat
  under the TUN path).
- IPv6 fail-safe on dual-stack (ADR-0010 invariant: no bypass).
- UDP drop + YouTube-app QUIC→TCP fallback (report actual device
  behavior).
- Loop prevention on-device (protect hook effective).
- Stop while active; partial-start injection at every step.
- Wi-Fi/mobile switch, captive portal, provider unreachable.

### Android lifecycle/device tests (M5/M7)

Fresh install, upgrade, launch; consent grant/deny/revoke; background,
lock screen, resume, rotate, font scale; stop from UI and from system
VPN settings; service death and reconciliation; notification channel
behavior; uninstall cleanliness; reboot (nothing self-starts);
screen-off persistence.

### Desktop regression tests

The §A baseline re-run on every milestone that touches shared code;
new warnings investigated before commit (rule held through v0.7.x).

## 4. Device and protocol support matrix

To be filled with real results during M4/M7 — nothing below is claimed
before it is measured:

| Device/API | ABI | Network | Browser/app | DNS | Traffic family/protocol | Expected behavior | Result |
|---|---|---|---|---|---|---|---|
| Emulator (M4) | x86_64 | emulator | test client | fake DoH | IPv4/TCP/TLS | end-to-end through engine | Not run |
| Emulator (M4) | x86_64 | dual-stack | test client | fake DoH | IPv6 literal | fail-safe, no bypass | Not run |
| Physical device 1 (M7) | arm64-v8a | Wi-Fi | Chrome | configured | IPv4/TCP/TLS | YouTube loads through engine | Not run |
| Physical device 1 (M7) | arm64-v8a | Wi-Fi | YouTube app | configured | TCP + UDP/QUIC drop | playback works (TCP fallback) | Not run |
| Physical device 1 (M7) | arm64-v8a | mobile data | Chrome + app | configured | IPv4/TCP/TLS | same as Wi-Fi | Not run |

## 5. APK verification and artifact handoff

For every produced APK record: exact path + size; debug/release
variant; application ID, versionName/Code; ABIs + min/target SDK
metadata; signing state (cert identity, no private keys published);
SHA-256; install result; launch result; which verification levels from
§5's five-way distinction (compiled / installed / UI smoke / engine
integrated / target traffic path) actually ran. Never claim the APK
exists before checking it.

## 6. CI

Deferred (no CI exists for desktop either — releases are agent-driven
with recorded evidence). If added later: pinned tools, Go tests,
svelte-check, Android unit tests, assembleDebug; no signing material
in CI.

## 7. Test exit criteria

- All P0 tests pass, or a documented no-go/known limitation blocks.
- Traffic coverage matches every user-facing claim (per-app, TCP,
  IPv4, DNS-through-DoH).
- Desktop baseline never silently regressed.
- Build reproducible from a clean machine with the repo's
  instructions.

M8-a record (2026-10-17; the second consecutive no-wipe session —
repo at 707cf04, toolchain verified, baseline green before any change:
vet clean, 17/17 ×2, svelte-check 0/0, vite clean): Ali's two product
rulings (D83) — the empty allow-list default and the .ir
never-processed rule — implemented and fully re-verified: `gomobile
bind` (GOFLAGS=-mod=mod, 3 ABIs, androidapi 26) → younblock.aar
**25,160,550 B** (javap: the nine bindings — failStart/getSettings/
getState/routedApps/saveSettings/setVpnReady/startTun/stopTun/
vpnState); clean `assembleDebug` → `app-debug.apk` **35,567,561 B**,
SHA-256
`ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64`,
badging: package io.github.assadigit/younblock 1.6.6 (versionCode 1),
minSdk 26, targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64;
the dumped manifest carries the D83 launcher-intent `<queries>` (MAIN
+ LAUNCHER) and exactly doc 07 §6's four permissions (ACCESS_NETWORK_
STATE absent); apksigner verify OK (debug cert). Matrix 4/4 ABIs
(arm64 full tree; 18 non-main × arm/386/amd64); -race green on the
seven key packages (engine 10.4 s, proxy 17.9 s, mobile 38.3 s);
17/17 ×2 at the exit; webui dist synced (TestWebUILifecycle green on
the new bundle). Disk: ~1.9 GB free at the exit (Go module cache
cleaned mid-session and re-fetched through the bind — network
reachable); `go clean -modcache` forces the re-download on the next
`-mod=mod` lane, budget ~5 min.

M8 record (2026-10-17; the third consecutive no-wipe session — repo
at 9a05f05, toolchain verified: go1.27.2, javac 21.0.12.1, NDK
28.2.13676358; baseline green at the open BEFORE any work: vet clean,
17/17 ×2, svelte-check 0/0, vite clean, webui dist synced with zero
diff). D84 — the debug-APK-only test circle — reshaped the milestone:
no signed variant, no release keystore; the debug APK is the
distributed artifact. **No application code changed at M8** (docs +
release only), so the shipped artifact is the M8-a clean-build output,
re-verified on disk before distribution instead of pointlessly
rebuilt (a rebuild would only churn build timestamps; the honest form
is the recorded artifact re-verified):

- `sha256sum android/app/build/outputs/apk/debug/app-debug.apk` →
  `ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64`
  (35,567,561 B — the exact M8-a record);
- `apksigner verify --print-certs` → Verifies, v2 scheme true, signer
  CN=Android Debug, cert SHA-256 digest
  `c76bb7c61ec623892eabc37ac90ac1bc0041d39490f2d48073676d6e2ed8551d`;
- `aapt2 dump badging` → package io.github.assadigit.younblock
  versionName 1.6.6 versionCode 1, minSdkVersion 26, targetSdkVersion
  36, native-code arm64-v8a/armeabi-v7a/x86_64;
- the dumped manifest → exactly the four permissions of doc 07 §6 +
  the launcher-intent `<queries>` (MAIN + LAUNCHER);
- `javap` on the AAR's classes.jar → the full binding set
  (newEngine, start/startTun/stopTun, setVpnReady, getState/vpnState,
  getSettings/saveSettings, routedApps, failStart, getAppInfo,
  getDiagnosticReport, runConnectionTest, autoFix, shutdown);
- `go list -deps ./mobile` (vendor mode) → the same audited closure
  as M7 (gVisor pkg subset, golang.org/x/{net,sync,sys,time,exp} +
  x/mobile, google/btree).

**The release lane (new at M8):** tag `v1.6.6-android-debug.1`
(prerelease) on the M8 commit; the APK uploaded as the release asset
through the GitHub API; verified after creation via the API — tag,
prerelease flag, asset name/size, and the downloaded asset's SHA-256
matching the on-disk artifact. **The D84 versionCode rule: every
successive debug build bumps `versionCode` (android/app/
build.gradle.kts) so it installs over the previous one (same debug
certificate); this first artifact keeps versionCode 1 / versionName
1.6.6.** The platform-requirements recheck (doc 07 §9) ran against
developer.android.com primary sources this session. Disk at the exit:
~1.9 GB free. The standing ⛔ (no /dev/kvm → no emulator, no device)
was re-verified: install/launch smoke and the traffic-path halves are
handed to Ali's device through the release's test circle (doc 10
§"Device test circle") — never claimed here.

D85 record (2026-10-17; the fourth consecutive no-wipe session — repo
at f61bfae, toolchain verified, baseline intact; the device-test
feedback loop's first session, triggered by Ali's observation "Android
didn't let me install it"):

- **The served-artifact exoneration** (before any fix, per the
  honesty rules): the release asset downloaded through the API →
  sha256 `ccf46a2e…7b64`, byte-identical to the on-disk record;
  apksigner v2 OK, the recorded debug cert; badging minSdk 26 /
  target 36 / versionCode 1 / the four permissions; the binary
  manifest dump → `debuggable=true` (normal for debug, does not
  block sideload) and NO `testOnly`; the zip central directory
  intact (20 entries, testzip clean). The asset's download_count
  was 0 — an interrupted download from the Iranian network is the
  probable cause; the troubleshooting path (size check first, PC +
  USB fallback) was PATCHed onto the live release body (HTTP 200).
- **The D85 keystore pin:** `android/app/debug.keystore` is now
  vendored (the exact key that signed debug.1 — keytool digest
  C76BB7C6…D8551D matches the APK cert) and wired as the debug
  signingConfig in `android/app/build.gradle.kts` (store password
  "android", alias androiddebugkey — the well-known debug identity;
  release keystores stay out of the repo per D84/doc 07 §8). The
  lane: clean `assembleDebug` → BUILD SUCCESSFUL (37/37 tasks) →
  apksigner on the rebuild → the SAME certificate digest
  `c76bb7c6…8551d` → and the rebuild is **byte-identical** to the
  published release (sha256 match — deterministic packaging). Every
  future assembleDebug now signs with the committed key, so the D84
  update promise survives sandbox wipes (the machine-local
  ~/.android/debug.keystore regenerates per wipe — three wipes in
  this project's record).
- No debug.2 cut: the artifact is unchanged and byte-reproducible;
  the next release rides on the next code-changing observation.
  Exit gates: vet clean; go test 17/17 ×2; svelte-check 0/0; vite
  build clean; webui dist zero-diff (the D74 trap checked).

M8-c record (2026-10-17; the fifth consecutive no-wipe session — repo
at d9f48d6 at the open, toolchain verified, worklog carried; the
device-test feedback loop's second round, Ali's ruling D86 + the first
functional observation "the first version didnt work, whether in
youtube app or youtube in chrome"; debug.1's download_count had reached
2 — the install completed):

- The root-cause pass BEFORE any fix: the whole routing chain read
  end-to-end — tunstack.go/dns.go (UDP:53 to any destination answered
  by the virtual DNS; fake-IP reversal; AAAA/HTTPS answered empty —
  correct client-fallback semantics), socks5.go + server.go
  (routedHost: the mode/list rule minus .ir; fragmentation on the
  first record for routed domains), YounblockVpnService.kt (establish,
  allowed-apps, virtual DNS, v6 blackhole). Conclusion: the ENGINE
  chain is sound; the failure is the SHELL — the born-empty list (D83)
  plus an entry path that refused everything but exact package names,
  so "youtube" could not be added and nothing ever routed.
- The D86 implementation: DefaultApps=[com.google.android.youtube];
  settingsVersion 2→3 with the one-time migration (v1/v2 absent-or-
  empty apps → the default; v3 empty = user agency, kept; custom lists
  kept); RoutedApps unchanged (fail-toward-empty guard stays);
  frontend — quick-add chips (+ YouTube / + Chrome), friendly-name
  aliases in the add field, the honest copy updates (intro, empty
  state, note, How-it-works, the NO_TARGET_APPS message); Kotlin
  comments updated to the D86 semantics, behavior unchanged.
- Lanes: vet clean; go test 17/17 ×2 (exit 0 both); -race green on
  config/engine/mobile; svelte-check 0/0; vite build clean + dist
  synced (TestWebUILifecycle re-run green on the new bundle — the D74
  trap checked); compile matrix 4/4 ABIs — the exact historical form
  re-derived (`go build ./internal/... ./mobile/...` = the 18 non-main
  packages per ABI; arm64 additionally links the full tree `./...`;
  a probing subtlety was resolved: `go build` with an EMPTY package
  list builds the main package, which legitimately requires cgo on
  arm/386/amd64 — earlier "failures" were that, not the matrix);
  gomobile bind → younblock.aar 25,162,951 B (javap: the full binding
  set); clean assembleDebug → app-debug.apk 35,716,265 B, sha256
  9cca38c9e904755ea2a4359650b8815c2756cf4f88272f91ca1809190026811b,
  versionCode 2 (the D84 rule), the pinned D85 certificate
  (c76bb7c6…8551d — installs over debug.1 in place), badging minSdk 26
  / target 36 / 3 ABIs, the four-permission manifest, no testOnly; the
  D86 WebView copy verified embedded in the AAR's libgojni.so.
- The release lane: v1.6.6-android-debug.2 (prerelease) on the M8-c
  commit; the APK uploaded; verified via the API after creation. Doc
  10's stale DoH default list corrected to the code truth
  (RethinkDNS, Cloudflare, Google).

M8-d record (2026-10-17 later that day; the sixth consecutive no-wipe
session — repo at 096e8c0 at the open, toolchain verified, worklog
carried; the device-test feedback loop's third round, Ali's second
ruling D87 — "Add chrome to the default addlist as well, also firefox,
opera, brave browser"):

- The risk audited BEFORE the change: a multi-app default meets
  devices that lack the apps — the Kotlin establish loop already
  catches NameNotFoundException per entry and skips it (refusing only
  when zero resolve), so the five-app default cannot break establish
  on a device with only some installed. Recorded in D87.
- The D87 implementation: DefaultApps = [com.google.android.youtube,
  com.android.chrome, org.mozilla.firefox, com.opera.browser,
  com.brave.browser]; settingsVersion 3→4 with the era-precise
  migration (v1/v2 absent-or-empty → the new default; v3 EXACTLY the
  D86 default → the new default — the debug.2 device case; v3 empty =
  agency, kept; every custom list kept, including a v2 hand-typed
  [youtube]); RoutedApps unchanged (fail-toward-empty guard stays);
  frontend — the chip row widened to all five defaults, aliases
  extended (firefox/opera/brave/brave browser), the honest copy
  updates (intro, empty state, note, How-it-works line 1, the
  NO_TARGET_APPS message); Kotlin comments updated to the D87
  semantics, behavior unchanged; versionCode 2→3 (the D84 rule).
- Lanes, all with evidence: gofmt-canonical on every touched Go file
  (the Edit-tool tab-to-space corruption on the three legacy-tabbed
  files caught and reverted with gofmt -w; settings_test.go still the
  known legacy space-indented file, untouched); vet clean; go test
  17/17 ×2 (exit 0 both); -race green on config/engine/mobile;
  svelte-check 0/0; vite build clean + dist synced (the webui suite
  green on the new bundle; com.brave.browser verified in the built
  assets); compile matrix 4/4 ABIs — the exact historical form
  (18 non-main packages each; arm64 links the full tree); gomobile
  bind → younblock.aar 25,174,619 B (javap: the full binding set; the
  D87 WebView copy verified embedded in libgojni.so); clean
  assembleDebug → app-debug.apk 35,816,233 B, sha256
  81515f0dd1b0f1ff2d3fb1a87769c8c6d4df3dc0225efeb975bd9f543257d6a1,
  versionCode 3, the pinned D85 certificate (c76bb7c6…8551d — installs
  over debug.1/debug.2 in place), badging minSdk 26 / target 36 /
  3 ABIs, the four-permission manifest, no testOnly.
- The release lane: v1.6.6-android-debug.3 (prerelease) on the M8-d
  commit; the APK uploaded; verified via the API after creation (served
  bytes re-downloaded and hashed).

M8-e record (2026-10-17, later still; the device-test feedback loop's
fourth round — no device observations arrived; Ali's ask instead:
"Can you fix the serial number and etc issue as well? like giving me
a bat file that auto creates it's password locally on my system and
backs up that password as an md file. if you insist it must be done
locally. at least you can create the script." → D88, the local
release-keystore ceremony script):

- The open found a PARTIAL sandbox wipe: `~/sdk` gone (Go 1.27.2, the
  Temurin JDK, the Android SDK/NDK), while the repo (clean at 2a8b678,
  the M8-d commit, in sync with origin), the worklog and the Next.js
  scaffold survived. The system Java on the box is a JRE 21.0.12.1
  whose `keytool` DOES run (no javac — useless for builds, sufficient
  for this task).
- NO Go/Svelte/app code changed this session (the deliverable is a
  Windows batch script + docs + a release asset), so the desktop
  baseline stands as M8-d recorded it at this exact clean tree (vet
  clean; go test 17/17 ×2; svelte-check 0/0; vite build clean, dist
  synced). Stated plainly rather than re-run: the toolchain to re-run
  them is the thing that was wiped. The next session that touches
  Go/Svelte code must reinstall the toolchain per §1's pins FIRST and
  re-establish the baseline.
- What WAS verified, with evidence, on this host: the real keytool
  flow — `keytool -genkeypair` (RSA 4096, PKCS12, validity 10950 →
  Oct 2056, `-dname` pre-filled → zero prompts, exit 0) and
  `keytool -list -v` (exit 0; a WRONG storepass exits 1 — the script's
  verification step rides on exactly that semantic) using the system
  JRE's keytool 21.0.12.1 (the same version family as the pinned
  Temurin JDK); the parsed line shapes confirmed against that real
  output ("Serial number:", tab-indented "SHA256:", "Valid from:",
  "Owner:"); the password alphabet proven to be exactly 64 unique
  characters (zero modulo-bias mapping, ~192 bits for 32 chars) with
  the algorithm simulated end-to-end; the shipped file checked pure
  ASCII, 253/253 CRLF lines, 11,527 B, sha256
  82e6dded40a8eb415127c6d46b3cb4acb0633ab2cd13ac8b809a8ae8c7c74090,
  and byte-identical through the delivery surfaces (repo, preview
  asset, release upload).
- The ⛔ list, stated plainly: the build host is Linux — the .bat's
  batch/PowerShell wrapper cannot be executed here (no Windows, no
  pwsh). Mitigations: conservative batch constructs only (no delayed
  expansion, an alphabet with no cmd-special characters, every
  password/path handling quoted), and the script verifies its own
  work on Ali's machine before declaring success (the read-back
  round-trip + the explicit failure messages per exit code).
- The delivery lane: committed at the repo root, pushed; attached as
  a second asset to the EXISTING v1.6.6-android-debug.3 release via
  the API (no new tag, no new APK — the D84 rule: identical APK bytes
  are not re-released); the served asset re-downloaded and hashed to
  the same sha256. The sandbox preview (the Next.js / route) serves
  the same bytes with a copy/download page.
- Docs: doc 07 §8 (D88), doc 10 §1 (the helper-asset note), README
  (the signed-lane section — closing the hole where doc 07 §8 pointed
  at "README backup steps" that did not exist), .gitignore (the
  release-keystore + backup patterns), DECISIONS.md D88, PROGRESS.md,
  CHANGELOG.md, doc 09 handoff fields, SESSION_HANDOFF_PROMPT.md
  (post-D88 edition incl. the ~/sdk wipe state).

M8-f record (2026-10-17, later; the delivery-verification round —
zero repo changes): Ali's instruction was "Commit and deploy and push
to their main branch, the last release is for yesterday." The audit
found everything already landed, so the round produced evidence, not
bytes: the worktree clean at cd53540 and `git ls-remote origin main`
byte-identical to HEAD; the debug.3 release live through the API with
both assets (APK sha256 81515f0d…d6a1; make-release-keystore.bat
sha256 82e6dded…4090, the served asset re-downloaded and hashed
byte-exact); the Next.js preview page browser-verified (render, the
byte-exact download link, the copy-to-clipboard flow with its toast,
zero console/page errors, the footer math at 390 and 1280, dev.log
clean). The timeline question resolved: "yesterday's" release is the
desktop v0.7.6 (Oct 16); the android circle is all Oct 17. No lanes
ran — nothing changed in the tree.

M8-g record (2026-10-17, later still; the fifth feedback round —
D89, the release-flag ruling): Ali: "Do not prelease , just release,
the repo is private. Please finish all of your specs first." The spec
audit ran first: doc 05's Phase 2 is host-complete end to end (M0–M8
all ✅; every remaining ⛔ is an on-device gate needing Ali's physical
device — doc 09's standing blocker), the docs were current through
D88, and the M8-f round had re-verified every delivery surface —
nothing host-side was left unfinished. Then the release-lane change,
with evidence: the three android releases switched to regular
releases through the GitHub API (prerelease true→false on
debug.1/.2/.3; tags, assets and sha256 digests verified unchanged
before and after), debug.3 explicitly marked Latest
(/releases/latest now returns it — a plain flag flip does not move
the marker by itself), and its body gaining exactly one D89 note
paragraph (the pre-edit body reconstructed byte-identical apart from
that note — 8,579 → 9,017 chars). The honest framing recorded in the
note: the repo is private, so the flag carried no public meaning; the
build is still the debug-signed device-test build, and nothing about
the artifacts changed (D84's no-identical-re-release rule respected —
a flag flip is metadata, not a build). Docs updated: DECISIONS.md
D89, docs 06/07/09/10, README, PROGRESS, CHANGELOG,
SESSION_HANDOFF_PROMPT. No lanes ran — ZERO Go/Svelte/app code
touched; versionCode stays 3; ~/sdk remains wiped (the reinstall
caution stands for the next code-touching session).

M8-h record (2026-10-17, later again; the sixth feedback round —
D90, PowerShell-less PATH): Ali's first helper field report: the
ceremony stopped with "'powershell' is not recognized" — his PATH
does not include PowerShell (the desktop setup scripts' v0.7.5
environment class). The fix, desk-checked to the D88 standard: the
script resolves powershell.exe by full standard path first
(setup-offline.bat's shipped pattern), then where.exe, then a
pure-cmd fallback (six RANDOM draws, digits and dashes, ~90 bits);
the .md backup and DONE summary record the generator; structural
checks on the new file clean (even quotes per line, block parens
balanced, pure ASCII CRLF, 13,683 B, sha256 ee35e456…9558). No
lanes ran — no Go/Svelte/app code touched (~/sdk still wiped; not
needed: a batch-file fix). Delivery: the fix committed at cb28507
and tagged v1.6.6-android-debug.4 — a REGULAR release (D89),
Latest-marked, carrying only the fixed .bat; the APK is NOT
re-attached (D84: the app is unchanged, debug.3's APK is the
current build); the asset's served bytes re-downloaded through the
API and hashed byte-exact; debug.3's body gained the pointer note
superseding its older helper asset (9,017 → 9,650 chars, prefix
byte-identical); docs synced in the follow-up commit (this record
among them). The Windows execution itself remains ⛔ on this host
(as at D88); the script self-verifies on Ali's machine. Next Task
ID M8-i; next decision D91.

M8-i record, part a (2026-10-17, later again; the recovery round —
the M8-i session's opening, recorded as M8-i-a; NO device feedback
arrived: checked the session messages and the GitHub API — zero
issues on the repo, so the ingestion itself stays open as M8-i-b):
the environment was found wiped AGAIN (~/sdk absent — the fourth
wipe; the repo itself INTACT at f8b2edc, clean and in sync, with
the untracked build state surviving: frontend/node_modules,
frontend/dist, internal/webui/dist, android/app/libs, and the Go
module cache). The toolchain was reinstalled per §1's pins with
one operational discovery: **the cmdline-tools alias URL
`commandlinetools-linux-12.0_latest.zip` now 404s at dl.google.com
— use the versioned URL
`commandlinetools-linux-11076708_latest.zip` (153,607,504 B),
which is the same cmdline-tools 12.0** (`sdkmanager --version` →
12.0 verified after install). Restored and verified: Go 1.27.2
(~/sdk/go), JDK 21.0.12.1+1 Temurin (javac 21.0.12.1 — the
Adoptium API's current 21 GA, same build as M5/M6), platform-tools
37.0.1 (adb OK), platforms 26+36, build-tools 36.1.0, NDK
28.2.13676358, gomobile + gobind via `go install` at x/mobile
v0.0.0-20261011034528-197770444867 (they land in ~/go/bin, on PATH
via env.sh; `gomobile help bind` works); ~/sdk/env.sh and the
git-ignored android/local.properties (sdk.dir) rewritten. The
desktop baseline was re-verified GREEN before anything else, on
the unchanged tree: `go vet ./...` clean; `go test -count=1
./...` **17/17 packages ok, run twice consecutively** (engine
9.6 s, proxy 16.2 s, mobile 36.8 s on the second pass — matching
the recorded baseline); svelte-check 0 errors 0 warnings; vite
build clean (✓ 1.05 s) with `frontend/dist → internal/webui/dist`
re-synced — and the worktree stayed CLEAN through the rebuild
(content-hashed output byte-identical to the committed dist).
Android library compile matrix 4/4 ABIs, the honest §A form:
arm64 links the full tree at CGO_ENABLED=0 (exit 0); the 18
non-main packages (the tree minus the root wails main and
cmd/core + cmd/pacgen) build clean × arm/386/amd64 (exit 0 each).
Gradle wiring re-proven through the committed wrapper:
`./gradlew projects` → BUILD SUCCESSFUL (Gradle 9.8.1 dist
re-downloaded; JDK/SDK detection + local.properties verified).
The standing runtime blocker re-confirmed: `ls /dev/kvm` → No
such file or directory — no emulator, no device, as ever. Release
lane re-verified live through the GitHub API: v1.6.6-android-debug.4
regular (draft=false, prerelease=false) and the repo's Latest;
debug.3/.2/.1 regular; zero open issues. ZERO app code touched;
no release cut (nothing changed — D84 discipline); versionCode
stays 3; the APK bytes are unchanged. Disk: 4.3 GB free after the
restore. Next: M8-i-b, the device-results ingestion (and the
fixed helper's first DONE output) — the environment is READY for
an immediate fix-→-bump-→-debug.5 cycle when that data lands.

M8-i record, part b (2026-10-17, the second M8-i exchange — Task
ID M8-i-b, another PC-side round; the DEVICE results still have
not arrived): Ali reported "I created a key chain now what should
i have to do? also the last release only has keychain, no apk to
download." Read: (1) the keystore ceremony SUCCEEDED on his
machine — the D88/D90 loop's first completed run (which generator
ran / the DONE output were not pasted; the shareable identifiers,
if ever needed, are the certificate serial + SHA-256 fingerprint —
never the password). (2) A real delivery-UX report: the Latest
release (debug.4) shows only the helper .bat in its asset list,
and its title said "(no new APK)" — accurate but not actionable;
Ali had to ask where the app is. Fix (delivery-lane only, D84
untouched — no bytes duplicated, the pointer strengthened): the
debug.4 release TITLE retitled through the API from "…the keystore
helper fixed (no new APK)" to "…the keystore helper fixed · no APK
here — the app is on debug.3" (release id 415870871; make_latest=
"true" on the PATCH — debug.4 stays the repo's Latest, verified
through /releases/latest after; tag, body, and assets unchanged).
The body's first line already carried the direct APK link
(unchanged); the README's download line already explains the split
(unchanged). What Ali was told: the APK to install is debug.3's
app-debug.apk (35,816,233 B, installs over debug.1/.2 in place —
the pinned debug certificate); the keystore needs NOTHING now —
it is for the future signed public release (README §signed lane:
keep younblock-release.keystore + younblock-keystore-backup_<
date>.md in two different safe places, private, never committed,
never paste the .md in chat — it holds the password); and the
actual next input remains the device test circle (doc 10) with the
diagnostics report for failures. Offered, not decided: attaching
the APK to debug.4 as well (every release page carrying the app)
is Ali's call against D84's no-duplicate discipline — he has not
ruled. Zero app code; no release cut; versionCode stays 3; the
device-results ingestion moves to the next Task ID, M8-j.

M8-j record (2026-10-17, later again; the EIGHTH feedback round — the
first DEVICE feedback, D91): the session opened with the FIFTH
sandbox wipe (~/sdk gone AGAIN, mid-session this time — between the
M8-i-b docs commit and the code work; the repo and worklog intact,
the untracked build state surviving; the toolchain reinstalled per
§1's pins exactly as recorded, ~15 minutes). Ali's report: "it worked
on android. Play store message popped up… I generated key ring, but I
didn't attach to APK if needed… After connection, the youtube app is
routed through this system. But actually when clicking to play a
video, it took like 30 sec - 1 min or more to actually show that
video… feed quickly refreshed but actual video playing delayed…
The numbers in the app are too near each other, they need regular
spacings." The round's work:

- **The video-start delay, root-caused at host level to ELIMINATION:
  a temporary probe (a hand-built QUIC-shaped UDP/443 datagram
  injected through the crossed-wire stack) proved the netstack
  answers non-DNS UDP with ICMP 3/3 port-unreachable** — the QUIC
  fast-fail contract; the probe was then hardened into the permanent
  TestUDP443FastFailICMP (asserts the ICMP type/code AND that the
  quote carries the refused dport 443). Every other code-level
  suspect was checked and ruled out with the code as evidence: the
  TCP forwarder's receive window (rcvWnd 0 → gVisor's
  DefaultReceiveBufferSize = 1 MiB, and the phone↔netstack RTT is
  sub-ms so window/RTT is not the mobile bottleneck), the fragment
  presets (Pause 1–10 ms), DoH (keep-alive + HTTP/2 transport,
  singleflight, 120 s min TTL, 6 h serve-stale), the conn caps
  (MaxConns 1024, MaxFlows 1024), the TUN (MTU 1500, v4 default
  route + v6 blackhole route, virtual DNS 10.111.0.53), and the
  connect gate (connectSequence runs the QuickCheck — listener, dns,
  youtube — BEFORE PhaseOn; Ali's "extend the connection time"
  suggestion is already the built behavior). What remains is
  device/network-side — so the round INSTRUMENTED: every tunnel now
  logs its phases into the log ring the redacted diagnostics report
  carries (tunnel up dns=…ms dial=…ms; tunnel first-byte …ms — the
  TLS/origin-response start; tunnel done dur=…ms up=… down=… — with
  per-flow byte totals; hostnames only behind LogHostnames; the
  first-record write now counts on the flow meter too — the flow
  timing tests caught that gap). Pinned by TestFlowTimingLines /
  TestFlowTimingHostnamesGated / TestFlowTimingNoLoggerNilSafe.
- **The numbers spacing, root-caused and FIXED**: the pro screen's
  .stage was width: var(--stage-w) = 22.5 rem fixed — 360 px at 1×,
  but 495 px in a 320 px viewport at 1.4× font scale (measured live
  through the vite preview + the browser): the stage outgrew the
  screen and pushed the timer/speeds against the edges (the two
  speed values sat 24 px apart with 29 px screen margins). Fix:
  .stage width min(var(--stage-w), 100vw); the speeds row gap
  1.1 rem → 1.75 rem + icon gap 0.5 rem + white-space nowrap; the
  timer digit boxes 0.62 em → 0.66 em (colon 0.36 → 0.4) +
  font-variant-numeric tabular-nums. Verified before/after through
  the rendered app at the worst case (stage 495 → 320 px, value gap
  24 → 39 px, all in view) and the canonical case (360 px,
  unchanged), VLM-checked both.
- **The embed hygiene discovery**: internal/webui/dist had
  accumulated every build generation's stale hashed assets (the
  build.sh cp is additive), and go:embed all:dist carries them all
  into the AAR/APK. The dist is now CLEAN-synced (rm + placeholder
  restored + cp) — file sets identical to frontend/dist; the AAR
  shrank accordingly (25,162,951 → 24,914,100 B, partly this).
- **Baseline after the changes, all green**: go vet ./... clean; go
  build ./... clean; go test -count=1 ./... 17/17 packages ok run
  TWICE consecutively; svelte-check 0 errors 0 warnings; vite build
  clean; the webui dist clean-synced and the webui+mobile packages
  re-run green after the sync.
- **The build lane**: versionCode bumped 3 → 4 (the D84 rule);
  gomobile bind (GOFLAGS=-mod=mod, 3 ABIs, androidapi 26) →
  younblock.aar 24,914,100 B; clean assembleDebug → app-debug.apk
  **34,831,081 B**, SHA-256
  **5df2acc39d569336a4663e879cfdc59ea2fcb4a22b75cd69b18f56e038cd02b4**;
  badging: io.github.assadigit/younblock 1.6.6 (versionCode 4),
  minSdk 26, targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64;
  apksigner verify --print-certs OK — the pinned debug certificate
  c76bb7c6…8551d (installs over debug.3 in place, settings survive).
- **The release lane**: v1.6.6-android-debug.5 created at ad02980 —
  a REGULAR release (D89), Latest-marked (verified through
  /releases/latest), carrying the new APK (release id 416019596;
  asset id 651411314, 34,831,081 B); the served bytes re-downloaded
  through the API asset endpoint (Accept: octet-stream — the plain
  browser-URL fetch cannot see a private repo's asset unauthenticated)
  and hashed byte-exact. One honest API wrinkle: creating a release
  with a SHORT target_commitish 422s with a misleading
  "tag_name is not a valid tag" — the full SHA works. The release
  body carries the four answers Ali needs: the spacing fix, the
  instrumentation ask (diagnostics copy when a video is slow), the
  Play Protect explanation, and the keystore's do-nothing-now (plus
  the warning NOT to sign test builds with it — that would break the
  pinned-cert update chain).
- Play Protect observed at install (expected for sideloaded
  debug-signed APKs — documented, no code change). The DEVICE test
  circle rows answered so far: row 1 pass-with-note (install +
  first launch worked; the Play Protect popup is the note; rotation/
  TalkBack still unreported), row 2 pass (the default list routes out
  of the box — the feed refreshes through the tunnel), row 3 PARTIAL
  (browse fast; video-start delayed — the round's subject), row 9's
  host half pinned by test (the QUIC refusal contract). The
  font-scaling observation arrived via the spacing complaint — the
  M6 gate's first live data. Next Task ID M8-k: ingest the debug.5
  device round (the diagnostics report with the timing lines is the
  expected input). Next decision D92.

M8-k record (2026-10-18; the TENTH feedback-loop exchange — Ali's
cross-platform UI unification round, Task ID M8-k, decision D92).
Ali uploaded mobile.zip to the repo root carrying the UI instruction
documents his Windows/Linux builds already follow (roast-report.md
+ fix-instructions.md = round-2; feature-spec.md +
agent-instructions.md + usage-screens.png = round-3;
site-test-plan.md) and reported the mobile UI is not consistent
with the desktops. Applied to the ONE shared screen (D92's ruling:
the pro screen every edition embeds — Wails desktops through
frontend/dist, the Linux web edition and the Android WebView
through internal/webui — is the compliance point, so all devices
are consistent by construction):

- Round-2 fixes: lucide-svelte@1.0.1 installed and every icon now
  comes through src/lib/icons.ts (Power, Ellipsis, ChevronsUp/
  Down, ArrowDown/Up, X — only the logo and the faint play mark
  stay drawn); the menu button is Lucide "…" top-RIGHT at 44 px
  (--menu-btn 2.75 rem); the sheet has a fixed in-header close
  (title left, neutral 44 px X right) with the body scrolling
  underneath; timer digit boxes 0.80 em (widest digit 0.76 em);
  the connected ring family capped (arcs 4.0–6.0 rem around the
  resting thumb) so no dot reaches the hint, which moved to
  37.25 rem and grew to 12 px / 82 % white; ping carries a verdict
  (Good ≤ 120 ms, OK ≤ 400 ms, Slow); unmeasured speeds stay
  hidden (-1, never a fake 0); "Created by ChitaCodes" rides the
  About version line; the honest Other-websites section exists
  (VERIFIED_SITES empty until a site passes the two-day plan);
  the Wails window is 360×640, resizable, floored at 320×560.
- Round-3 data usage, tracked ONCE in the engine (both UI
  transports sit on it): internal/usage (the store:
  <ConfigDir>/usage.json, records of times + byte totals only,
  500 cap, 15 s autosave, crash recovery at Open); the engine
  tracks the session (goOn starts it at the relay-counter
  baseline — on↔degraded flaps keep the original start; anything
  leaving on/degraded ends it; a cancelled or failed connect
  never creates one); State gains downBytes/upBytes/lastSession
  (explicit null after a clear); GetUsage/ClearUsageJSON on the
  engine, mirrored by app.go's bindings and the webui's
  /api/usage + /api/usage/clear; the Wails bridge folds the
  engine's Usage() into every snapshot (UsageSource; -1/-1 for an
  engine that cannot count) and RefreshUsage covers the
  clear-while-disconnected case; the screen shows "X used" + the
  ↓/↑ split under the timer (SHOW_SPEEDS=false by default) and
  "Last session: size · length" under Not Connected; the menu
  gains the Data usage section (Today/This month/All time cards,
  the last 10 sessions, the provider disclaimer, a confirmed
  Clear with Delete-vs-Keep).
- The reference code folder (younblock-ui/round-3/) was NOT in
  the upload — implementation follows the documents; every rule
  is traceable to a spec line (D92 ruling 3).
- Host verification in place of the missing Python harness:
  svelte-check 0/0, vite build clean; agent-browser over vite
  preview at 320×560, 356×625 and 360×760 across Not Connected
  (with Last session), Connected (usage ticking + Ping verdict),
  the menu (cards, rows, the confirm flow — Keep keeps, Delete
  empties and drops the Last-session line) and Error — no
  overlap, no scrollbars at any size; and the byte path proven
  on the REAL binary (the webui edition): a 949,386-byte
  YouTube page pulled through the engine's own tunnel recorded
  as session downBytes 980,303 / upBytes 1,933; after the stop,
  the 80-second record sat in usage.json and the summary read
  todayBytes = allTimeBytes = 982,236.
- Full baseline: go vet clean; go test 18/18 packages ×2 (the
  new internal/usage among them; -race on the touched packages);
  webui dist CLEAN-synced (identical to frontend/dist).
- Build lane: gomobile bind → younblock.aar 25,003,698 B;
  assembleDebug → app-debug.apk 34,951,881 B (SHA-256
  bc4b239e…d1fe8), versionCode 5, badging minSdk 26/target 36,
  3 ABIs, apksigner cert c76bb7c6…8551d (the pinned D85 debug
  certificate — installs over debug.5 in place).
- Release lane: v1.6.6-android-debug.6 created regular at the
  code commit 078e3a5 (release id 416096215, asset id
  651728792), Latest-marked, the APK re-downloaded through the
  API asset endpoint and hashed byte-exact. Second API wrinkle
  for the record: the full-SHA target_commitish 422s exactly
  like the short one UNTIL the commit is pushed — create
  releases only after the push.

Next Task ID M8-l: ingest the debug.6 device round (the UI
consistency check across Ali's devices + the still-expected
diagnostics report for the video-start delay). Next decision
D93.

M8-l record (2026-10-18; the ELEVENTH feedback-loop exchange — a
desktop-side ruling between device rounds, Task ID M8-l, decision
D93; no Android code changed). Ali's instruction: "By default, when
exe file is closed and app is shut down, the app must not remain on
the taskbar — only keep being in taskbar if user selected this
option on the settings."

- Go: internal/config gains `keepInTrayOnClose` (default false,
  settingsVersion stays 4 — a pre-D93 file never recorded the old
  unconditional tray-keeping as a choice, so no migration is owed);
  tray_windows.go's closeHidesToTray now takes the setting (the
  Windows answer), tray_other.go always answers false (no tray,
  even opted-in); app.go's beforeClose composes them — allow the
  real close by default, hide only on the opt-in; the shutdown
  path it hands control to (browser session → tray → bridge
  disconnect → eng.Shutdown, Stop restoring the proxy first) is
  the one the Linux edition already ran for years. --minimized
  and the autostart launch flag are untouched (launch choices,
  not close semantics).
- Frontend, both screens behind capabilities.tray (Android, the
  Linux web edition and previews never render it): the pro
  MenuSheet gains a SETTINGS section with the toggle + teaching
  hint (the sheet's settings load now runs for perAppRouting OR
  tray); the classic Settings view gains the row beside
  connect-on-launch; the classic first-run tray variant now
  promises "Closing the window stops Younblock."; both Help decks
  reword the "window disappeared" entry around the opt-in. The
  one-time close notice stays unwired exactly as it always was
  (Go never emitted the event; the hint + Help carry the
  disclosure — D93 ruling 2).
- Tests: close_test.go (root package, real engine on the stub
  platform) — the keepInTrayOnClose parse table, the no-tray
  platform answer, beforeClose quitting on defaults AND on
  opt-in-without-tray, quitting always allowed mid-Quit; config —
  default false, a pre-D93 file loads false, the opt-in
  round-trips through Save/Load and lands in settings.json under
  its API name.
- Verification: gofmt drift on touched files normalized (the
  14-file frozen set untouched); go vet clean; go test 18/18 ×2,
  -race on the root package and internal/config; svelte-check
  0/0; vite build + webui dist CLEAN-synced; `GOOS=windows go
  build` compiles the whole Windows shell (the tray_windows
  branch); and the UI exercised end-to-end on a REAL `cmd/core
  --web` engine through a DISCLOSED scratch build whose Linux
  platform reported tray:true (one line, reverted before commit):
  both screens' toggles saved keepInTrayOnClose through
  /api/settings into the real settings.json and back
  (VLM-checked screenshots of both), and the CLEAN rebuild showed
  both sections hidden — the negative check for the capability
  gate. README's close/uninstall/manual-check copy updated to the
  new contract.
- No Android release: nothing changes on the phone (no window
  close, no tray, the row is gated off, the field round-trips
  untouched — the `apps` contract on desktops). debug.6
  (versionCode 5) stays the repo's Latest; Ali rebuilds the
  Windows exe from this tree (build.bat / build.sh) and walks
  README manual-check steps 4–5.

Next Task ID M8-m: ingest the debug.6 device round (the UI
consistency check across Ali's devices + the still-expected
diagnostics report for the video-start delay + the D93 desktop
check). Next decision D94.

### M8-m — the complete gap analysis + the account-gate direction — analysis-only, 2026-10-18 (DECISIONS.md D94)

No build, no code change: the round was a full read-only audit (the
gap report delivered in chat + the worklog's consolidated records).
The baseline stands unchanged and green at 9615936 as verified at the
M8-l close (vet clean, 18/18 packages ×2, svelte-check 0/0, the webui
dist clean-synced) — not re-run on unchanged bytes. Findings that
touch this doc's lanes: (1) NO CI exists — no .github/ directory at
all; the entire verification circle is manual per-session, so a
GitHub Actions lane (vet / test / svelte-check / dist-sync /
cross-compile checks) is the top QA gap; (2) the desktop release lane
is dormant — the last desktop artifacts (v0.7.6) predate D86–D93, nine
rounds of fixes sit under CHANGELOG "Unreleased"; (3) no frontend unit
tests exist (svelte-check + the session's manual browser exercises are
the whole UI defense); (4) mobile.zip compliance re-verified
independently: 100% of every round's instructions applied, the open
item is data (VERIFIED_SITES), not code. D94 opens the account-gate
feature lane (login authorization, a user database, admin user
management) under the design-before-code rule — the A0 spec round is
the next session's second workstream, and the feedback loop continues
in parallel as M8-n. Next decision D95.

### A0 — the account-gate spec round + a baseline defect fixed — 2026-10-18 (docs-only plus a two-line Go fix; no release)

The session opened on the FIFTH sandbox wipe (~/sdk and the repo both
gone; the worklog missing — recreated with a loss note; the repo
re-cloned at cf136aa, `core.fileMode false` + `gradlew +x` re-applied).
No device feedback had arrived (the repo's issues list: zero, all
states; none in the session messages), so per the no-block rule the
session ran the A0 design lane. The toolchain was reinstalled per §1's
pins with the recorded wrinkle (cmdline-tools via the versioned
`11076708` URL, 153,607,504 B exact): Go 1.27.2, JDK 21.0.12.1+1
Temurin, sdkmanager 12.0, platform-tools 37.0.1 (adb), platforms
26+36, build-tools 36.1.0, NDK 28.2.13676358, gomobile/gobind via
`go install` at x/mobile v0.0.0-20261011034528-197770444867; env.sh +
android/local.properties rewritten. 3.6 GB free after.

**Baseline sequence on a fresh clone (the D74 order, learned again):**
the frontend must be built BEFORE judging the Go lanes — `npm ci` →
svelte-check 0 errors 0 warnings → vite build clean (✓ 3.99 s) → the
webui dist CLEAN-synced (rm + placeholder restored + cp; the fresh
clone otherwise fails `go vet ./...` on main.go's `all:frontend/dist`
embed pattern, and the mobile package's TestWebUILifecycle fails on
the placeholder index — both the documented source-only-checkout
state, not regressions).

**A defect the full sweep caught, root-caused and fixed at the source:**
`GOOS=windows go build ./...` failed in `mobile` —
`nonblock_unix.go` (born at M4, 9431477) carries no build constraint,
so on Windows it collides with `nonblock_other.go` (whose `//go:build
!unix` exists precisely so "go build ./... compiles everywhere" — its
own comment). The scoped shell builds (root / cmd / internal) always
compiled — that is what M8-l/M8-m's "the whole shell" records measured;
the full `./...` sweep had never compiled on Windows since M4. Fix:
one line, `//go:build unix` added to nonblock_unix.go (+ the blank
line). Verified on the fixed tree: gofmt clean (touched file only);
vet clean; go test 18/18 ×2 (three consecutive green passes, zero
FAIL lines); `GOOS=windows go build ./...` exit 0 for the first time
since M4. No Android behavior change by construction: every android
ABI is unix, so the same file compiles as before; nonblock_other.go
was already excluded on unix. No release (D84: nothing app-facing;
versionCode stays 5, debug.6 stays Latest).

**A0 delivered (the session's main output):** `docs/auth/` — the
six-document spec pack (00-start-here with the consolidated six
questions + recommendations, 01-requirements AUTH-001…031, 02
-architecture, 03-auth-protocol, 04-admin-and-operations, 05-security
-review with the drafted privacy copy). Design-first per D94: nothing
built, no ADR beyond the one grounded in D94 ruling 1 (engine-enforced
gate — doc 08's ADR-AUTH-0001); the six questions stay OPEN — D95+ in
DECISIONS.md is reserved for Ali's rulings, and no recommendation was
recorded as if ruled. Next Task IDs: M8-n (the device round, still
awaiting Ali's feedback) and A1 (the auth backend) — A1 runs only
after the rulings. Next decision D95.

### A0-R3 — the account gate CANCELLED before any code — 2026-10-18 (docs-only; no build, no release)

The third exchange of the A0 day. Ali's message: "You know I've
decided to defer and cancel this auth version, only work on non-auth
version. it doesnt worth this headache. cancel it altogether, no auth
needed anymore." — recorded verbatim as **D97**: the account gate is
cancelled outright (the closing words govern: cancellation, not
deferral). Younblock stays ONE free-for-all app: no login, no
accounts, no auth server, no admin surface, no editions. D95 and D96
are superseded (kept as history); every open auth question — (a),
(c)–(f), (g1)–(g4) — closed as MOOT, never ruled on its merits;
ADR-AUTH-0001 (doc 08) is RETIRED; the A1/A2/A3 implementation ladder
is cancelled and those Task IDs retire.

**Nothing needed removing from the code because nothing was ever
added** — the design-first rule (D94) held all the way to the
cancellation: no login code, no server, no database, no keys exist
anywhere in the repository or in production (A0 and A0-R2 were both
docs-only commits; this round is too). Today's privacy claims — "no
account, no data collection" (doc 07 §1, doc 10 §2, the in-app
Privacy sheet, README) — were always literally true and stay so
permanently; the copy change D94 anticipated never ships.

Session actions (all docs): D97 written in DECISIONS.md (verbatim
quote, supersession, what survives, what it does NOT decide — the
cancellation governs the gate only; website/distribution/release-lane
questions stay untouched); `docs/auth/` bannered CANCELLED on all six
files (full notice in 00, one-line banners in 01–05, body text kept
as history; 02 §6's Iran-reachability finding noted as staying on
record — it is a network constraint, not a feature property); doc 07
§9's account-gate row closed as CANCELLED; doc 08's ADR-AUTH-0001
retired; doc 10's status block carries the closing note; doc 09's
handoff fields rewritten for the single remaining workstream;
README's docs map updated; CHANGELOG's Planned section rewritten as
cancelled; PROGRESS.md entry added; SESSION_HANDOFF_PROMPT.md
rewritten for M8-n-only.

Verification this round: docs-only, so no build/test lane applies;
`go vet ./...` run on the untouched tree as a health check — clean
(exit 0). No release cut (D84/D89: nothing app-facing changed);
versionCode stays 5, `v1.6.6-android-debug.6` stays Latest. Next
Task ID: M8-n (the device round — still awaiting Ali's feedback).
Next decision: D98.

M8-n record (2026-10-18, evening; the ELEVENTH feedback-loop exchange —
Ali's first DEVICE feedback round on debug.6, Task ID M8-n, decision
D98). His report: "The problem of video waiting to load stays. the
windows app doesnt have this problem. youtube instantly refreshes and
video instantly open. but the APP android needs 2-3 minutes, less or
more, for first video to actually play." Plus the standing ruling:
"also from now on, add the APK version (debug version) in the
releases." The round's work:

- **The slow first video, ROOT-CAUSED to a code-level asymmetry between
  the two doors — the first Windows-vs-Android difference the M8-j
  elimination had left standing.** The new discriminator (Windows
  instant, Android minutes) localized the defect to the Android-only
  layers; reading the flow-setup path end to end found it:
  1. The TUN stack accepts the phone's TCP handshake locally
     (netstack), then asks the SOCKS5 door bounded by
     HandshakeTimeout — a 10 s DEFAULT the facade never overrode
     (mobile/tun.go built tunstack.Config without it; tunstack.go
     defaulted 10 s).
  2. The door's own honest setup inside that budget: the DoH resolve
     (up to DoHTimeout 3 s × 3 servers) plus the upstream dial
     (DialTimeoutOverall 12 s) — it replies only after the upstream
     connects. The desktop HTTP-CONNECT door runs the SAME resolve+dial
     with NO client-side abort: the browser waits ≤~21 s and gets its
     tunnel. THAT is why Windows never shows the problem.
  3. On filtered networks googlevideo answers carry silently-blackholed
     addresses (SYN dies, no RST), and netx.DialHost walked the list
     SEQUENTIALLY at DialTimeoutPerIP 5 s each — one blackholed front
     address put the dial at 5–6 s, two past 10 s. The first video is
     the first rr*.googlevideo.com flow (the feed's domains are
     pre-warmed — PrewarmNames + the QuickCheck resolves
     www.youtube.com and handshakes redirector.googlevideo.com BEFORE
     PhaseOn — which is why the feed is fast and only video start
     waits).
  4. The retry trap that turns 10 s into minutes: the DoH answer is
     cached with a 120 s MINIMUM TTL, so every app retry re-dials the
     SAME addresses in the SAME order into the SAME 10 s wall; the
     escapes are the cache rotating (~120 s — matching "2-3 minutes,
     less or more") or DoH landing a working front address — then the
     player's own retry/backoff ladder stacks the visible wait.
- **The fix, two halves at the two correct layers (ADR-0013):**
  `netx.DialHost` now races the address list Happy-Eyeballs-style
  (attempt k starts at k×FallbackDelay, default 750 ms; every attempt
  keeps its full per-IP timeout; first success wins, losers canceled;
  error surface unchanged) — a blackholed front address costs ~0.75 s +
  RTT instead of 5 s, on BOTH doors; and the TUN handshake budget is
  now budget-true: config.TunHandshakeTimeout 30 s (= 3×3 s DoH + 12 s
  dial + slack), passed explicitly by the facade, with the tunstack
  default raised to the same value as the backstop. Pinned by
  TestDialHostRacesPastBlackholedFrontAddress,
  TestDialHostWinnerCancelsLosers and
  TestHandshakeTimeoutDefaultCoversDoorBudget (the lockstep with the
  config constant + the covers-the-door-budget invariant).
- **D98 recorded** (DECISIONS.md): every release page carries the debug
  APK asset from now on — the standing M8-i-era offer, answered by
  Ali's message; debug.4's missing APK was the observable trigger.
- **Baseline after the changes, all green (D74 order):** svelte-check
  0/0; vite build clean; webui dist CLEAN-synced; go vet clean; go
  test -count=1 ./... 18/18 packages ok run TWICE consecutively;
  GOOS=windows go build ./... exit 0; the compile matrix in its exact
  form — CGO_ENABLED=0 go build ./internal/... ./mobile/... = 19
  non-main packages per ABI, 4/4 (arm64, arm, 386, amd64), plus arm64
  linking the full tree ./... (CGO_ENABLED=0) — the count grew from
  M8-i's 18 with internal/usage (M8-k), not with this round.
- **The build lane:** versionCode bumped 5 → 6 (the D84 rule); gomobile
  bind (GOFLAGS=-mod=mod, 3 ABIs, androidapi 26) → younblock.aar
  25,002,994 B; clean assembleDebug → app-debug.apk **34,960,649 B**,
  SHA-256
  **dc01c0419a211ca007235ffbeaec6ab5acecb2e5930a4efd4642097209cdd83d**;
  badging: io.github.assadigit/younblock 1.6.6 (versionCode 6), minSdk
  26, targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64; apksigner
  verify --print-certs OK — the pinned debug certificate c76bb7c6…8551d
  (installs over debug.6 in place, settings survive).
- **The release lane (D98's first application):** v1.6.6-android-debug.7
  created at the code commit — a REGULAR release (D89), Latest-marked,
  carrying app-debug.apk (verified through the API: asset bytes
  re-downloaded and hashed; size + digest recorded in doc 10).
- One sandbox note: the session's first compile-matrix attempt ran the
  arm64 full-tree link with the HOST gcc (aarch64 assembly errors);
  the recorded matrix form (CGO_ENABLED=0, doc 06 §3's early evidence)
  is what the lane actually uses — rerun in that form, 4/4 + full tree
  green. No app-facing consequence; recorded here so the next session
  does not repeat the 90-second detour.
- The DEVICE circle rows answered so far: row 3 (video start) now has
  a code fix + this evidence — but the row stays OPEN for Ali's
  on-device confirmation on debug.7 (the honest rule: no row closes
  without his measured evidence). Row 9's device half (QUIC fast-fail
  observed on a real app flow) still open. Next Task ID: M8-o (ingest
  the debug.7 device round — first-video timing first). Next decision:
  D99.

M8-o record (2026-10-18, later evening; the TWELFTH feedback-loop
exchange — the debug.7 device round, Task ID M8-o, decision D99).
Ali's report: "The problem persists, youtube feed refreshes, but it
takes a while until first video can be actually played. but the pc
version is instant… even after 1-2 mins of first video to be played,
every other video has same buffer problem… apk is laggy and buffers a
lot, but in UI-wise it connects as quickly. so it's NOT routing or
method or even internet speed… same ISP and net connection works
instantly using windows version." Plus the standing naming rule:
"mention the version name in the apk. like v16.6-debug.apk". The
round's work:

- **The discriminator sharpened**: EVERY video stalls (not just the
  first), feed and UI stay instant, Windows instant on the same ISP —
  so the hunt narrowed to mechanisms that poison PER-HOSTNAME
  connection setup repeatedly (each video = fresh rr*.googlevideo.com
  hostnames; the feed's hosts are prewarmed and warm-cached). Re-read
  the whole flow path end-to-end (tunstack wire/dns/tunstack/socks5,
  the door's socks5 server half + tunnel relay + resolve, the DoH
  resolver + cache, netx, the Kotlin service): the door, the tunnel
  relay and the upstream dial are byte-identical on both platforms
  (one engine, one resolver, one dialer — the M8-m gap audit's
  "desktop release lane dormant" note means Ali's Windows exe predates
  M8-n, but the chronic per-video stall predates M8-n too (debug.5's
  30 s-1 min report), so the M8-n changes are not the regression).
- **Root-caused the SURVIVING amplifier (ADR-0014)**: ADR-0013 made
  the walk fast but cannot help when the WHOLE answer set is
  blackholed — rr hostnames carry the few addresses of ONE cache node
  each, so one filtered node poisons the set. The app retries every
  few seconds; every retry re-dials the same cached set and fails; the
  120 s-minimum-TTL cache keeps serving the suspect set until it
  rotates — the observed 1-2 min per-video wait, and (videoplayback
  URLs rotate rr hostnames mid-play) the mid-play re-buffering. The
  fix closes the loop at the only honest point: a TOTAL dial failure
  now demotes the host's cached answer to stale (doh.Resolver.
  Invalidate; the next retry re-queries the servers; serve-stale
  preserved so the failure path is never worse). Wired at all three
  dial sites (HTTP-CONNECT, plain-http forward, SOCKS5 door) through
  an optional-capability probe. Consistency fix: the resolver's own
  bootstrap dial now RACES its IPs (the same DialHost) instead of
  walking them at a full Timeout each — ADR-0013's reasoning applied
  to the resolver's own path.
- **Pinned by seven new tests**: doh — TestInvalidateForcesRequery
  (the re-query lands a different answer), TestInvalidateKeepsServeStale
  (the stale window survives invalidation), TestInvalidateNoops
  (literals/negatives/misses), TestDialBootstrapFailsOverToSecondBootstrapIP;
  proxy (new dialfeedback_test.go) — TestSocks5DialFailureInvalidatesDNS,
  TestConnectDialFailureInvalidatesDNS,
  TestDialFailureWithoutInvalidatorIsGraceful (the capability probe).
  The M8-n gofmt lesson applied again: the Edit tool space-converted
  the two tab-indented proxy files; gofmt -w restored them (real diff
  +16/-0 in proxy).
- **Baseline, all green on the fixed tree**: svelte-check 0/0; vite
  build clean; webui dist CLEAN-synced; go vet clean; go test 18/18
  ×2 consecutive; GOOS=windows go build ./... exit 0; Android compile
  matrix 4/4 ABIs (CGO_ENABLED=0) + arm64 full tree ./... exit 0.
- **The build lane**: versionCode bumped 6 → 7 (the D84 rule); one
  gradle-daemon wrinkle (a stale daemon held the system JRE — `./gradlew
  --stop` + explicit JAVA_HOME=~/sdk/jdk cleared it; recorded so the
  next session skips the detour); gomobile bind (GOFLAGS=-mod=mod,
  3 ABIs via the recorded -target=android/arm,android/arm64,android/amd64
  form, androidapi 26) → younblock.aar 24,993,512 B; clean assembleDebug
  → app-debug.apk **34,978,761 B**, SHA-256
  **c0c4019ffd3fb9b458b67a2cbffdc62d8f1f6632a3007347a78022ab5b9cd5f6**;
  badging: io.github.assadigit/younblock 1.6.6 (versionCode 7), minSdk
  26, targetSdk 36; apksigner verify --print-certs OK — the pinned
  debug certificate c76bb7c6…8551d (installs over debug.7 in place,
  settings survive).
- **D99 recorded** (the APK naming rule, extending D98): every APK
  asset is named v<versionName>-debug.apk — first application
  v1.6.6-debug.apk on this release.
- **The release lane**: v1.6.6-android-debug.8 created at the pushed
  full SHA (regular release, D89), Latest-marked, APK asset uploaded
  AS v1.6.6-debug.apk (D99's first application), served bytes
  re-downloaded through the API and hashed byte-exact.
- **The honest ask for the debug.8 round** (put in the release notes
  and the wrap-up page): if ANY video still stalls, open the app's
  Diagnostics after it happens and copy the "tunnel" lines — they
  carry per-flow dns/dial/first-byte/dur numbers that split the
  remaining suspects in one read (slow dns/dial = the network answer
  path; fast setup + slow dur = a throughput leg; nothing logged at
  all = the flow never reached the door). Row 3 stays OPEN until Ali's
  measured evidence; row 9's device half unchanged.

M8-p record (2026-10-18, night; the THIRTEENTH feedback-loop exchange
— the debug.8 device round, Task ID M8-p, decision D100, ADR-0015).

- **Ali's inputs**: the per-video stall PERSISTED on debug.8, and the
  round's own evidence ask was impossible to fulfill — "there is no
  diagnostic option in the apk app setting. add an option to copy
  paste logs and diagnostics so i can paste it here." (D100.)
- **What the round therefore did (ADR-0015)**: NO third root-cause
  guess — two disproven fixes in a row make guessing the one thing the
  loop must not do; the deliverable is the evidence path itself:
  1. Engine: `Engine.SetReportExtra` (the embedder hook; runs outside
     the engine lock, pinned by TestEngineReportExtra) + the report's
     log window doubled 100 → 200 lines.
  2. Facade: the hook installed with the TUN counters line
     (flows/rstUnmapped/overCap/doorFailures/dnsQueries/udpDropped —
     counters only, R21 redaction extends to it) + `SetShellVersion`
     (Kotlin passes "1.6.6 (versionCode 8)" from PackageManager at
     EngineHolder.ensure — best-effort, never blocks). Pinned by the
     NEW mobile/report_test.go (WithoutTun + WithLiveTun through the
     full serving harness + socketpair TUN).
  3. Frontend (every edition): the Diagnostics screen (screens/
     Diagnostics.svelte) — menu entry + Settings row; Copy/Share/
     Refresh + the selectable readonly textarea; the honest clipboard
     ladder (lib/clipboard.ts: shell bridge → navigator.clipboard →
     execCommand; "Copied" only on real success, else the manual-path
     instruction); Help's copy button joins the ladder (its silent
     false-success fixed); i18n keys; `backend` became $state in
     App.svelte (the prop crossing fixed the non_reactive_update
     warning at the source).
  4. Kotlin: MainActivity.PageBridge (addJavascriptInterface
     "YounblockShell") — copyToClipboard (native ClipboardManager,
     binder-thread-safe) + shareText (system share sheet, marshalled
     to main); write-only surface, only the loopback page can load.
     strings.xml: share_diagnostics.
- **Baseline, all green**: svelte-check 0/0 (one round on the
  $state/app-referenced-locally pair — fixed with the repo's
  established svelte-ignore pattern); vite build clean; webui dist
  CLEAN-synced (placeholder.txt restored, built files ignored); go vet
  clean; go test 18/18 ×2 consecutive; GOOS=windows exit 0; Android
  compile matrix 4/4 ABIs (CGO_ENABLED=0) + arm64 full tree exit 0.
- **The build lane**: versionCode 7 → 8 (the D84 rule; the gradle
  wrinkle did not recur this session — plain `./gradlew --stop` +
  JAVA_HOME form); gomobile bind (GOFLAGS=-mod=mod, the recorded
  3-ABI target form, androidapi 26) → younblock.aar 25,035,860 B (the
  binding verified to carry setShellVersion before gradle ran); clean
  assembleDebug → app-debug.apk **35,026,961 B**, SHA-256
  **1a2b2bf75553ea2b4d9d2eddc6ba8e2f694e40e2e1e47b25954cbb04c73d7833**;
  badging: versionCode 8 / versionName 1.6.6 / minSdk 26 / targetSdk
  36; apksigner OK — the pinned debug certificate c76bb7c6…8551d
  (installs over debug.8 in place, settings + usage history survive).
- **D100 recorded** (the diagnostics surface ruling): the APK carries
  the first-class diagnostics path; after this round a "still broken"
  report without the pasted diagnostics is incomplete.
- **The release lane**: v1.6.6-android-debug.9 at the pushed full SHA
  (regular release, D89, Latest), the APK asset uploaded AS
  v1.6.6-debug.apk (D99), served bytes re-verified through the API.
- **The honest ask for the debug.9 round** (release notes + wrap-up
  page): play videos until one stalls, then Menu → Diagnostics →
  Copy (or Share) and paste the report here. The tunnel lines
  (dns/dial/first-byte/dur), the tun: line (flows/udpDropped) and the
  speed number at the slow moment split the remaining suspects in one
  read. Row 3 stays OPEN; no guess ships before that report arrives.

### M8-q — the ADR-0015 port: the surface ships in the variant the APK serves (2026-10-18)

- **The round's discriminator** (Ali on debug.9: "I still dont see
  diagnostic button"): the M8-p surface shipped ONLY in the classic
  tree, but the APK bakes the pro screen — D66's dispatcher defaults
  to pro when `VITE_UI_VARIANT` is unset, and `build-android.sh`
  never sets it (only the desktop wails lanes bake classic). The
  phone rendered the pro screen; its only diagnostics surface was the
  M6-era sheet section buried below Help behind an `android` gate,
  with the raw `navigator.clipboard` Copy that silently fails in the
  WebView. The fix is not a third stall guess — it is making the
  promised button exist in the variant the device renders (D101,
  ADR-0016).
- **The port** (frontend only — the engine, the facade and the Kotlin
  bridge from M8-p are variant-independent and unchanged):
  1. `src/lib/DiagnosticsView.svelte` (NEW, the pro screen's report
     view): auto-load on open, Copy through the SAME ladder
     (cross-tree import of `classic/lib/clipboard.ts` — pure
     DOM/window code), Share behind `canShare()`, Refresh, honest
     inline status, the readonly selectable textarea; Escape/Back
     close, Tab wraps, focus returns to the menu button.
  2. `MenuSheet.svelte`: Diagnostics is now the LEAD row of the sheet
     (full-width card, activity icon, 56px target, every platform, no
     gate) dispatching `diagnostics`; the old bottom section and its
     broken copy path are REMOVED (one surface, not two).
  3. `App.svelte`: the `diagOpen` state, the sheet→view handoff
     (sheet closes, view opens, focus returns on close);
     `icons.ts` grows the six Lucide imports the view needs.
- **The verification is the point of the round**: the served surface
  verified END-TO-END through the real webui server (`cmd/core --web
  --webport 8097`, the same embedded dist + dispatcher the APK
  serves; mobile viewport 360×760): menu open → Diagnostics is the
  FIRST row → the view loads the real report ("Younblock 1.6.6
  diagnostic report…") → Copy resolves to the honest "Copied" state →
  Back returns to the app; no console errors. The classic variant
  re-verified through the same server (`?ui=classic`: Menu →
  Diagnostics entry intact — the M8-p screen is unchanged).
- **Baseline, all green**: svelte-check 0/0 (two quick rounds: a
  component-invalid class: directive, then an unused-selector warn —
  both fixed at the source); vite build clean; webui dist
  CLEAN-synced (placeholder.txt restored); go vet clean; go test
  18/18 ×2 (plus an uncached engine+mobile round); GOOS=windows
  exit 0.
- **The build lane**: versionCode 8 → 9 (the D84 rule); the full
  `./build-android.sh` lane (frontend rebuild → dist sync → gomobile
  bind → assembleDebug) exit 0; the AAR's embedded Go library VERIFIED
  to carry the new UI (`diag-entry` ×2, the row's hint text — the
  go:embed content is inside libgojni.so); app-debug.apk
  **35,053,265 B**, SHA-256
  **4264996ae39bb3e268bcf28913d3884d038c0833bb35ffd2233d60edd51ef9e7**;
  badging: versionCode 9 / versionName 1.6.6 / minSdk 26 / targetSdk
  36; apksigner OK — the pinned debug certificate c76bb7c6…8551d
  (installs over debug.9 in place).
- **D101 recorded** (the variant rule): a surface ships in the
  variant(s) the edition serves; the served-variant check is part of
  the ship checklist from this round on.
- **The release lane**: v1.6.6-android-debug.10 at the pushed full
  SHA (regular release, D89, Latest), the APK asset uploaded AS
  v1.6.6-debug.apk (D99), served bytes re-verified through the API.
- **The honest ask for the debug.10 round** (release notes + wrap-up
  page): open the menu — Diagnostics is the first row now — open it,
  Copy, paste the report here (even with nothing stalled: the build
  identity line proves the round). Then, when a video stalls mid-
  session, Diagnostics → Copy again and paste that report too. Row 3
  stays OPEN; no guess ships before a real pasted report arrives.

### M8-r — the first pasted report acted on: the heal sweep probes from zero (2026-10-18)

M8-r record (2026-10-18, late night; the FOURTEENTH feedback-loop
exchange — the debug.10 device round, Task ID M8-r, ADR-0017; no new
D-number — no ruling landed, D102 stays reserved for Ali).

- **The round's input:** Ali's FIRST pasted diagnostics report
  (D100's standing evidence path, finally used): "the problem
  persists" + the full report — build identity versionCode 9 (the
  debug.10 APK — the M8-q surface WORKED), state degraded, last
  failed check dns (DNS_UNREACHABLE), "heal sweep found no working
  method" 12.4 s into the run, method legacy, dns ok 58 / fail 16,
  tun active flows=37 udpDropped=76, and 133 log lines that read
  back against the code EXACTLY (the reconstruction is in
  ADR-0017's Context): a startup DoH blip (three ~170 ms
  fast-failures, once) negative-cached www.youtube.com for the 15 s
  TTL; the automatic sweep probed that same name for all four
  methods and failed instantly on the memory every time
  (ErrRecentlyFailed — the servers are never re-asked inside the
  window); the sweep's OWN video handshakes were completing (3 of 4
  methods) while it declared no working method → degraded + the
  "Couldn't fix it" notification, 21 s after Start, on a recovered
  network. Collateral: the failed sweep left method=legacy (the
  last-tried preset) live; the report's DNS lines were attr-stripped
  by the ring (bare "dns server failed" — no reason).
- **The host reproduction IS the proof:**
  `TestHealSweepSurvivesFreshNegativeCache` (new; a DoH that fails
  exactly the first request, the dns step and the youtube step
  coupled on the same name — production's shape) fails on the
  pre-fix code into the report's exact signature —
  `Phase:degraded Method:legacy`, never ON — and passes with the
  fix (ON, method balanced, the report carrying the baked failure
  line). `TestFailedSweepRestoresPreviousMethod` (new) fails
  pre-fix (method=legacy persists); `TestServerFailureLinesCarryEvidence`
  (new, internal/doh) fails pre-fix (the bare fallback message).
  All three verified BOTH ways (fix reverted → FAIL; fix applied →
  PASS).
- **The changes (ADR-0017):** the heal sweep resets the resolver at
  start (the netmon's network-change semantics — a sweep is a
  suspected-network-change event and probes from zero); a failed
  sweep restores the previous method; the DNS failure/fallback
  lines bake server=#N + reason= into the message (the ring keeps
  only the message; the configured server NAME stays a file-log
  attr; the report gains the failure KIND — timeout vs network vs
  DNS_UNREACHABLE-class).
- **Baseline, all green:** frontend untouched this round (the
  M8-q dist carries over — verified intact: placeholder.txt +
  hashed assets; the frontend tree clean at HEAD); go vet clean;
  go test 18/18 ×2; GOOS=windows exit 0; the Android compile
  matrix in its exact form (arm64 full tree + the non-main set
  4/4 ABIs, CGO_ENABLED=0, -mod=vendor).
- **The build lane:** versionCode 9 → 10 (the D84 rule); the full
  `./build-android.sh` lane exit 0 (disk managed: the Go build
  cache cleared before the bind); the AAR's embedded Go library
  VERIFIED to carry the new engine strings (`dns server failed
  server=#` inside libgojni.so); app-debug.apk badging versionCode
  10 / versionName 1.6.6 / minSdk 26 / targetSdk 36; apksigner OK
  — the pinned debug certificate c76bb7c6…8551d (installs over
  debug.10 in place); size + SHA-256 in the release record below.
- **The release lane:** v1.6.6-android-debug.11 at the pushed full
  SHA (regular release, D89, Latest), the APK asset uploaded AS
  v1.6.6-debug.apk (D99), served bytes re-verified through the API.
- **The honest ask for the debug.11 round:** Start on the normal
  network → Connected without the false "Couldn't fix it"; the
  method stays what was set. On the next stall: menu → Diagnostics
  (first row) → Copy → paste — the report now carries the DNS
  failure reasons (the missing stall-row evidence) and row 3's
  sharpened question: reason=timeout vs reason=network on the
  startup blip.


### M8-s — the second pasted report acted on: the first-byte budget (2026-10-18/19)

M8-s record (2026-10-18/19; the FIFTEENTH feedback-loop exchange — the
debug.11 device round, Task ID M8-s, ADR-0018; no new D-number — no
ruling landed, D102 stays reserved for Ali, and the round SURFACED the
method-policy question for him: see the end of this record).

- **The round's input:** Ali's SECOND pasted diagnostics report (the
  M8-r evidence loop, working): "the problem persists, but when after
  a while, video plays, it can be easily rewinded and go next, which
  means the speed is okay, the routing is okay... only the problem is
  each video starting to play which lags considerably. IN pc, it's
  instant." + the full report — build identity versionCode 10 (the
  debug.11 APK), state on, method strong, response time 726 ms,
  dns ok 60 / fail 4, tun active flows=27 doorFailures=2
  udpDropped=82, and 124 log lines that read back against the code
  exactly (the reconstruction is in ADR-0018's Context): the M8-r
  fixes PROVEN on-device (the sweep found strong 3 s in; the startup
  blip still logging its three baked
  `dns server failed server=#N reason=DNS_UNREACHABLE` lines and
  self-healing — the sharpened question answered: a transient, not a
  filter); the stall re-observed with its exact shape — video-start
  waves where some flows complete (first-byte 56-619 ms) while others
  dial fine, send the ~1.7 KB hello, and never receive one byte:
  ~0.5 s closes with `down=0 B` (origin reset) and EXACTLY ~60.0 s
  hangs with `down=0 B` (silent blackhole; nothing in the engine
  closes at 60 s — IdleTimeout is 300 s, the TUN keepalive only
  probes — the ~60 s closer is the app's own patience, and the engine
  amplified it by holding the flow open; and because the dial
  SUCCEEDED, the M8-o Invalidate never fired, so the suspect answer
  set stayed fresh and every retry wave could land on the same dead
  node until TTL expiry).
- **The host reproduction IS the proof:** the two new tests in
  internal/proxy/firstbyte_test.go, verified BOTH WAYS — with the
  tunnel.go restructure stashed (the pre-M8-s relay),
  `TestFirstByteTimeoutFailsSilentRoutedTunnelFast` FAILS at ~5.0 s
  (the silent processed flow hangs to the idle timeout; the device's
  60 s signature compressed) and
  `TestFirstByteFailureInvalidatesSuspectSet` FAILS (no demotion on a
  connect-but-never-speak origin); with the fix both PASS (~250 ms
  close, the `tunnel first-byte failed reason=timeout` line, the done
  line `down=0 B`, the demotion recorded). The two guards
  (`TestFirstByteBudgetSparesUnroutedHosts`,
  `TestFirstByteWithinBudgetFlowsNormally`) pass both ways, as guards
  must.
- **The changes (ADR-0018):** `FirstByteTimeout = 8 s`
  (internal/config, the CONFIGURATION block); the proxy Config gains
  the field (zero = the default, negative = disabled); the engine
  wires it; tunnel.go's relay splits into two shapes — the budgeted
  processed-host shape (the origin's first byte under the deadline;
  silent = fail fast, both ends closed, the error counted, the reason
  baked into the message, and the host's cached answer set demoted
  exactly like a total dial failure — the M8-o loop extended to
  silent origins) and the plain two-goroutine shape every other
  tunnel keeps byte-for-byte; `firstByteReason` maps timeout /
  closed / reset (the OS reset string carries addresses before its
  first colon, so it is matched directly).
- **Baseline, all green:** frontend untouched this round (the
  committed dist verified intact; the frontend tree clean); go vet
  clean; go test 18/18 x2; GOOS=windows exit 0; the Android compile
  matrix in its exact form (arm64 full tree + the non-main set 4/4
  ABIs, CGO_ENABLED=0, -mod=vendor); disk managed (the Go build cache
  cleared before the bind).
- **The build lane:** versionCode 10 -> 11 (the D84 rule); the full
  `./build-android.sh` lane (run detached, the double-fork setsid
  pattern); the AAR's embedded engine VERIFIED to carry the new
  strings ("tunnel first-byte failed" inside libgojni.so); the
  app-debug.apk **35,076,497 B**, SHA-256 **ce25f9cdd9a103f26a1286
  1c21c989e0ecb451a2dfe9dd2251bc421beeab261c**; badging versionCode
  11 / versionName 1.6.6 / minSdk 26 / targetSdk 36; apksigner OK —
  the pinned debug certificate c76bb7c6...8551d (installs over
  debug.11 in place).
- **The release lane:** v1.6.6-android-debug.12 at the pushed full
  SHA (regular, D89, Latest), the APK asset uploaded AS
  v1.6.6-debug.apk (D99), served bytes re-verified through the API.
- **The question surfaced for Ali (D102 material, NOT decided):**
  the sweep picks the method from its redirector.googlevideo.com
  probe, but the media flows hit rr*.googlevideo.com cache nodes
  whose tolerance differs per node — this report ran method strong
  while his PC is instant on the same network. Whether the engine
  should retry a dead routed flow once with the NEXT preset in
  HealSweepOrder (per-flow method variation) is a METHOD POLICY
  change: his ruling, not this round's code. The next report is
  expected to carry `tunnel first-byte failed reason=...` lines and
  `dur=~8000ms down=0 B` dones — the tuning evidence for the budget
  and the material for that ruling.

M8-t record (2026-10-18/19, late — the THIRD pasted report round,
debug.12 → debug.13; the environment was found wiped a SIXTH time
before this round: ~/sdk and ~/go erased, the repo itself INTACT and
clean at f4e2091 with the untracked build state surviving —
frontend/node_modules, frontend/dist, internal/webui/dist, the old
AAR. Toolchain reinstalled per §1's pins and verified: Go 1.27.2,
javac 21.0.12.1 (Temurin, the Adoptium API's 21 GA), cmdline-tools
12.0 (the versioned commandlinetools-linux-11076708_latest.zip,
153,607,504 B — the alias URL stays dead), platform-tools 37.0.1,
platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358,
gomobile/gobind via `go install` at the pinned
x/mobile v0.0.0-20261011034528-197770444867; env.sh and the
git-ignored android/local.properties rewritten. TWO operational
discoveries this round: (1) the detached sdkmanager died mid-install
leaving 2.5 GB of stale .temp — cleaned and re-run in the FOREGROUND
to completion; (2) the detached build-android.sh lane died at the
vite step the same way — this sandbox's background killer is more
aggressive than M8-s's, so the lane ran as the documented FOREGROUND
CHUNKS with the caches carrying the progress: npm ci + vite build +
dist sync / gomobile bind / gradle assembleDebug, each exit 0):

- **The round's root cause (read from the report):** debug.12's
  budget fired exactly as designed — 22 `reason=timeout` dones at
  ~8.3 s + 18 `reason=reset` dones at ~0.5 s, counters 38 errors /
  61 conns — roughly half the video-start waves die at first byte
  and the app re-opens wave after wave (~35 s from the first wave to
  the first video bytes, down=4.4 MB once a flow survived). Every
  budget-killed flow was 100 % silent (dead nodes, not slow ones);
  healthy routed origins answered in 420-508 ms (one 2399 ms
  outlier carrying the fragment-pause write).
- **The changes (ADR-0019):** FirstByteTimeout retuned 8 s → 2.5 s;
  the salvage re-dial — a dead first-byte origin is re-dialed INSIDE
  the same flow (same host, SAME preset captured once per flow, the
  client's buffered bytes replayed byte-exact through a fresh
  fragment conn, dead-IP exclusion on the re-resolve), up to
  FirstByteSalvageDials re-dials (default 2 → 3 total attempts);
  new report lines `tunnel first-byte retry reason=... attempt=N`
  and `tunnel first-byte salvaged dials=N`. tunnelCloser.setB keeps
  Stop's sweep on the live origin; the client pump + swap-able
  origin link keep the chatty-client live-write semantics of the old
  up-copy; unrouted relays never enter the shape (D46).
- **The proof (both ways):** the three new firstbyte_test.go tests
  (TestFirstByteSalvageRecoversDeadOrigin, ...ExhaustsToFailFast,
  TestSalvageReplayIsByteExact) FAIL with the tunnel.go salvage
  stashed (the M8-s shape at HEAD, call sites reverted) and PASS
  with it; the two guards (...SparesUnroutedHosts,
  ...DisabledByConfig) pass both ways; the two M8-s tests now pin
  the terminal shape with FirstByteSalvageDials = -1.
- **Baseline, all green:** frontend untouched this round (vite
  rebuild byte-identical, the dist re-sync left the tree clean);
  go vet clean; go test 18/18 x2; GOOS=windows exit 0; the Android
  compile matrix in its exact form (arm64 full tree + the non-main
  set 4/4 ABIs, CGO_ENABLED=0, -mod=vendor); disk managed (the Go
  build cache cleared before the SDK install; 4.0 GB free after).
- **The build lane:** versionCode 11 -> 12 (the D84 rule); the full
  lane in foreground chunks, each exit 0; the AAR
  **25,101,194 B**, its embedded engine VERIFIED to carry the new
  strings ("tunnel first-byte salvaged" and "tunnel first-byte
  retry" inside libgojni.so in all three ABIs); the app-debug.apk
  **35,130,385 B**, SHA-256 **18b4ea648b0aa02d7eebf84f2dc596d13894
  093a1469dfc4f111ddc5b286b518**; badging versionCode 12 /
  versionName 1.6.6 / minSdk 26 / targetSdk 36 / native-code
  arm64-v8a+armeabi-v7a+x86_64; apksigner OK — the pinned debug
  certificate c76bb7c6...8551d (installs over debug.12 in place).
- **The release lane:** v1.6.6-android-debug.13 at the pushed full
  SHA (regular, D89, Latest), the APK asset uploaded AS
  v1.6.6-debug.apk (D99), served bytes re-verified through the API.
- **The D102 question SHARPENS but stays Ali's:** the salvage
  re-flips the same coin (same preset) up to twice more per flow; if
  the next report still shows terminal first-byte failures
  clustering under one method, the NEXT-preset retry (per-flow
  method variation) remains his ruling, never unilateral. The next
  report's expected evidence: the retry/salvaged line ratios against
  the terminal failures — the tuning material for the budget and the
  D102 material for the method policy.


M8-u record (2026-10-19, early — the FOURTH pasted report round, a
NO-BUILD round: the report ran the previous build):

- **The round's finding:** Ali pasted "The problem persists" + a
  fourth diagnostics report generated at 23:41:37 UTC — but the
  report names itself `android shell: 1.6.6 (versionCode 11)`: the
  PREVIOUS build (debug.12), not the salvage build. The identity is
  authoritative (MainActivity reads it from PackageManager at
  runtime), and the log confirms it three independent ways: 21
  `reason=timeout` dones at 8054-8070 ms (the old 8 s budget — the
  2.5 s build cannot produce an 8 s death), zero `tunnel
  first-byte retry` / `salvaged` lines in the whole 200-line
  window, and the release clock — debug.13 published 23:16:28 UTC,
  the report generated 23:41:37 UTC. Zero bytes of the report
  describe the salvage build's behavior.
- **What the report does confirm (all old-build evidence):** the
  M8-s budget kept firing exactly as designed; DNS fully healthy
  (ok 118 / fail 0); ~24 healthy first-bytes at 58-548 ms — every
  one inside the new 2.5 s budget with 4.5x headroom; the storm
  shape unchanged (21 timeouts + 17 resets at 64-527 ms, 67 errors
  / 114 conns, first video-data responses ~80 s into the window,
  udpDropped=107) — exactly what the salvage re-dial collapses.
- **The round's action:** NO new build (D84: no app change, no
  versionCode bump — nothing to tune yet). debug.13 re-verified
  end-to-end this session: the release asset re-downloaded through
  the API and hash-matched (SHA-256
  18b4ea648b0aa02d7eebf84f2dc596d13894093a1469dfc4f111ddc5b286b518,
  35,130,385 B); the embedded engine checked to carry both salvage
  lines in libgojni.so in ALL THREE ABIs; Latest confirmed via
  /releases/latest; the HEAD knobs re-read (FirstByteTimeout 2.5 s,
  FirstByteSalvageDials 2). The reply and the preview page walk Ali
  through installing v1.6.6-android-debug.13 — every release names
  its APK v1.6.6-debug.apk (D99), so the debug.13 PAGE is what
  disambiguates — and name the next evidence: the retry/salvaged
  line ratios against terminal failures, from a report whose shell
  line reads versionCode 12.

D103 follow-up (same session, Ali's ruling — "please add debug13 in
the apks name, so such accidents does not happen"): all six D99-era
APK assets renamed IN PLACE through the API (v1.6.6-debug8.apk …
v1.6.6-debug13.apk — D99's own stated test, "the asset name must
identify the build without opening the release page," had failed:
six releases shared the identical filename); sizes and download
counts preserved, bytes unchanged (the debug.13 asset re-served and
hash-matched after the rename: 18b4ea648b0aa02d7eebf84f2dc596d1389
4093a1469dfc4f111ddc5b286b518); every affected release body updated
so its download instructions name the new file and carry the rename
note. The naming rule is amended from debug.14 on: the asset is
v<versionName>-debug<N>.apk, N = the release tag's build sequence.
The pre-D99 releases (debug.1-7, the app-debug.apk era) keep their
historical names — they predate the rule, and Android refuses
same-signature versionCode downgrades, so the silent wrong-build
install cannot happen through them. No app change: versionCode
stays 12, debug.13 stays the CURRENT build, no release cut.
M8-v (2026-10-19, the FIFTH report round — the first from the salvage
build itself, shell line versionCode 12): the report proved the
salvage on-device (retry lines absorbing dead attempts, salvaged
dials=1/2 saving flows at 0.7-1.4 s, errors 37/81 ≈ 46 % from 59 %)
and named the two remaining start-lag sources: the SERIAL salvage
chain (21 terminal reason=timeout dones at 8014-8227 ms = three
2.5 s budgets walked one after another, + 11 terminal resets at
0.6-1.9 s) and the mid-stream stall (five video-start flows done in
the same instant at 00:18:17.2, dur=57290-64620 ms, up=2.4-5.0 KB
down=6.6-7.7 KB one 121.1 KB — first byte plus ~7 KB, then total
silence until the APP's own ~60 s patience killed them; the retry
wave right after answered in 60-261 ms). The round ships ADR-0020:
the salvage RACE (the remaining re-dials launch in parallel on the
rotated live set, first speaker wins, terminal ~2x the budget never
3x) + the stall watch (StallTimeout 15 s default, both-directions
silence closes the flow engine-side, new report line
"tunnel stall killed idle=Nms"). Both scoped to processed hosts
(D46); D102 (the next-preset retry) stays Ali's ruling with this
report's evidence: terminal first-byte failures still cluster under
method=strong.

Lane evidence: desktop baseline in the D74 order all green (npm ci,
svelte-check 0/0, vite build, CLEAN-sync dist, go vet clean, 18/18
packages ×2, GOOS=windows exit 0). Both-ways proof: with tunnel.go
stashed (the M8-t serial shape), TestStallKillClosesSilentRelayFlow,
TestSalvageRaceRedialsInParallel, TestSalvageRaceTerminalIsBounded
and the updated TestFirstByteSalvageRecoversDeadOrigin all FAIL
exactly as designed; with the fix all 16 first-byte/salvage/stall
tests PASS; guards pass both ways (the streaming-download guard
caught an inverted-stamp bug mid-round — the watch must measure from
the NEWEST activity clock). Full suite 18/18 ×2 again after the fix.
Compile matrix 4/4 ABIs (CGO_ENABLED=0 ./internal/... ./mobile/...
per android/arm,arm64,amd64,386) + arm64 full tree ./... green.
Disk managed per the M8-t lesson (go clean -cache before the bind;
2.9 GB free at the bind, gradle --stop first). gomobile bind
(-target=android/arm,android/arm64,android/amd64 -androidapi=26,
GOFLAGS=-mod=mod) exit 0; younblock.aar 25,157,947 B with the stall
string verified in libgojni.so in all three ABIs; clean assembleDebug
BUILD SUCCESSFUL (44 s). Artifact: app-debug.apk 35,182,673 B,
SHA-256 154d40355bee702fcc751ead595688b5732abeb44cf186913dddbc5a99c
ad5e7; apksigner OK (the pinned c76bb7c6…8551d); badging versionCode
13 / versionName 1.6.6 / minSdk 26 / targetSdk 36; the stall string
also verified in the APK's libgojni.so in all three ABIs.
versionCode 13 (the D84 rule); release lane: v1.6.6-android-debug.14
with the APK asset named v1.6.6-debug14.apk (D103).
