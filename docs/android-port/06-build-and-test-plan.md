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
