# 06 — Build and Test Plan

**Status:** Updated 2026-10-16 (Phase 1 complete). Verified values are
marked; SDK-dependent values carry their verification status. The
desktop baseline (§A) is recorded and green.

## 1. Reproducibility requirements

| Item | Selected value | Why | Evidence/date |
|---|---|---|---|
| Minimum Android API | **26 (provisional)** | Carries forward Ali's prior decision (root 04-android-spec.md §5); FGS rules and per-app routing need ≥21; 26 covers the FGS-era requirements. Re-confirm at M3 against the chosen stack library's minSdk | Verified: prior decision; stack-library minSdk **to verify at M3** |
| Target/compile API | Decide at M3 | Sideload distribution (ADR-0012) means Play's target-API policy is not binding; choose the newest stable that the FGS `connectedDevice` rules are settled for | Open — M3 |
| Android ABIs | arm64-v8a (primary), armeabi-v7a (secondary), x86_64 (debug/emulator only) | Device reach; Go compile matrix green on all four (doc 01 §5) | Verified 2026-10-16 (compile level) |
| JDK version | 17+ (provisional per prior spec; sandbox has JRE 21, no javac) | Gradle/AGP compatibility | To verify at M3 |
| Android Gradle Plugin | To verify at M3 | Compatibility with the chosen Gradle | Open — M3 |
| Gradle wrapper | Committed wrapper, version chosen at M3 | Reproducibility | Open — M3 |
| Go version | **1.27.2** (go.mod directive `go 1.26.0`) | Matches the desktop toolchain; vendored deps build under it | Verified 2026-10-16 |
| gomobile/JNI strategy | **`gomobile bind` AAR** (ADR-0004) over `mobile/` facade | String JSON API matches gomobile constraints; x/mobile BSD-3, maintained | Verified 2026-10-16 |
| Userspace stack | tun2socks engine package primary; gVisor-direct alternative (ADR-0008) | MIT/Apache; production pedigree; measured switch criteria at M4 | Verified licenses/maintenance 2026-10-16 |
| Node/package manager | npm, Node v24, committed lockfile | Frontend unchanged; same lane as desktop | Verified 2026-10-16 |
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

Environment limitations (unchanged, standing): no webkit2gtk → no GUI
`wails dev` (desktop E2E uses the vite preview adapter); **no Android
SDK / NDK / javac in the audit sandbox** → `gomobile bind`, Gradle,
emulator all ⛔ until the toolchain is installed (doc 09).

### B. Frontend

Unchanged bundle; the Android lane runs the same `npm ci` →
`vite build` and copies the output to the app assets (M3). No
Wails-generated bindings are needed (the http adapter is used;
`frontend/wailsjs/` is git-ignored and absent on Android).

### C. Android build (defined at M3, validated at M8)

Projected tasks (to be replaced with the real module names after the
Gradle project exists — do not run these before then):

```sh
./gradlew projects        # verify layout
./gradlew test            # Kotlin unit tests
./gradlew assembleDebug   # debug APK
```

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
