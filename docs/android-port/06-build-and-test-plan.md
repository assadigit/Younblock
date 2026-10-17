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
