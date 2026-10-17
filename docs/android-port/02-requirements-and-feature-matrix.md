# 02 — Android Requirements and Feature Matrix

**Status:** Reconciled with the completed audit and accepted
architecture (2026-10-16). Priorities below are final for v1; the
traceability table (§7) maps every requirement to its design section
and current implementation status. Device-verified columns fill during
Phase 2 (never before).

## 1. Product goal

Deliver an installable Android app that makes the existing Younblock
functionality available on compatible Android devices, with honest
explanations about which traffic is processed. Preserve the
recognizable interaction model: one main connection control, real
service state, actionable errors, settings, diagnostics.

## 2. Scope principles

- Correct, reliable core connection over feature count or parity.
- Preserve existing Go logic (verified: reused unchanged — doc 01 §4).
- Traffic interception and permissions are first-class product
  behaviors, disclosed in-app (doc 07).
- No support claims beyond the per-app allow-list and tested matrix.
- No silent privacy-claim changes (ADR-0007).
- Desktop features with no Android meaning are deferred and
  documented (§4: AND-026…028).

## 3. Priority definitions

- **P0** — release blocker. **P1** — first release when the
  architecture supports it (else documented limitation). **P2** —
  later. **N/A** — desktop-specific; documented reason.

## 4. Feature matrix (reconciled 2026-10-16)

| ID | Feature / behavior | Priority | Acceptance criteria | Verification method | v1 disposition |
|---|---|---:|---|---|---|
| AND-001 | Install and launch | P0 | APK installs on the declared API range and opens without crash | Install/launch on emulator or device | M8 (needs SDK — blocked in sandbox) |
| AND-002 | Connect/disconnect | P0 | UI reflects actual engine phase; rapid taps create no duplicate workers | Engine tests (existing phase guards) + device smoke | Engine side verified; UI side M5 |
| AND-003 | Permission and consent | P0 | VPN consent explained before the system prompt; denial handled | Manual permission tests | M5 (design: doc 04 §6) |
| AND-004 | Core traffic processing | P0 | Allow-listed apps' TCP traverses the engine end-to-end | M4 network integration test on device/emulator | Architecture settled (ADR-0003/0008); runtime pending |
| AND-005 | Accurate connection check | P0 | "on" only after the self-test steps pass (existing behavior) | Existing engine tests + failure injection | **Inherited from the engine** (verified: QuickCheck gates `goOn`) |
| AND-006 | Safe stop / cleanup | P0 | Stop removes the TUN and restores normal routing | Device/lifecycle tests (M5) | Engine stop path verified; service side M5 |
| AND-007 | DNS behavior | P0 | DNS through configured DoH providers; failures surfaced, no silent fallback contrary to settings | DNS integration tests (M4) | Resolver reused unchanged; policy ADR-0009 |
| AND-008 | TLS ClientHello handling | P0 | First-record fragmentation applies to routed domains' TCP streams in the supported path | Protocol tests (M4) | `internal/fragment` reused; stream-level (audit §3.6) |
| AND-009 | Retry and recovery | P1 | Bounded heal/watchdog behavior (existing) | Existing engine tests | **Inherited** (verified: cooldowns, 35 s cap, watchdog) |
| AND-010 | Persistent settings | P1 | settings.json survives restart; invalid input rejected by `Normalize` | Storage/restart tests (M6) | Schema reused; `apps` field added (M6) |
| AND-011 | Editable DNS providers | P1 | Provider list viewable/changeable/validated; live swap while on (D44) | Existing behavior + UI tests | **Inherited** (engine + both UIs already do this) |
| AND-012 | YouTube traffic scope | P0 | The supported path (Chrome + YouTube app, TCP) works; no universal claims | Browser/app matrix (M4/M7) | Per-app allow-list (ADR-0003); device gate |
| AND-013 | Other websites / all-traffic mode | P1 | `all` mode explicit and tested | Domain/traffic classification tests | Mode exists in engine+UI (verified); scope disclosure in doc 07 |
| AND-014 | Response-time display | P2 | Label matches what is measured (application round trip, D41) | Unit/UI tests | **Inherited** (LatencyMs = youtube step ms) |
| AND-015 | Status/error diagnostics | P0 | Errors distinguish listener/DNS/youtube/video/engine/permission failures | Error-mapping tests | **Inherited** codes + `NO_TARGET_APPS` (new, M3) |
| AND-016 | Version/About | P2 | Version from the backend (`GetAppInfo`), never hard-coded | Manual check | **Inherited** (fixed in 1.6.6; backend-driven) |
| AND-017 | Help/privacy disclosure | P0 | Scope, DNS providers, no-remote-server statement in-app | Content review (M6) | Copy drafted in doc 07 §5; M6 wires it |
| AND-018 | Background operation | P0 | Connection persists minimized/screen-off via compliant FGS | Device tests (M5) | `connectedDevice` FGS design (doc 07 §6) |
| AND-019 | Network changes | P1 | Wi-Fi/mobile transitions recover or fail visibly, never stale "on" | Netmon behavior (existing) + device switch tests | **Inherited** engine netmon (verified); re-check on device |
| AND-020 | IPv4 and IPv6 | P0 for claimed support | IPv4 supported; IPv6 explicitly disabled with fail-safe routing, disclosed | Dual-stack tests (M4) | Policy ADR-0010 (AAAA empty + v6 route fail-safe) |
| AND-021 | TCP and UDP/QUIC | P0/P1 per scope | TCP processed; DNS UDP answered; other UDP dropped and disclosed | Protocol tests (M4) | Policy ADR-0009; QUIC-fallback device gate |
| AND-022 | Local network and bypass rules | P1 | No recursive interception (protect hook); allow-list bounds scope | Loop tests (M3/M4) | ADR-0011; M3 call-counting test |
| AND-023 | Responsive mobile UI | P1 | Usable on narrow phones, rotation, font scaling | Manual/device UI tests | Responsive ladder verified 300×620 (Phase 20); WebView re-check M6 |
| AND-024 | Desktop regression | P0 | §7 baseline table of doc 01 stays green on every shared-code change | Baseline comparison per milestone | Baseline recorded (doc 01 §7) |
| AND-025 | Build reproducibility | P0 | Pinned toolchain, documented commands, APK path + checksum | Clean build verification (M8) | Toolchain table in doc 06 (SDK-dependent items pending) |
| AND-026 | Two desktop UI editions | P2 | Both screens available, pro default (mirrors the web edition, D50/D66) | `?ui=classic` on device | Zero-cost inheritance; v1 keeps it |
| AND-027 | Desktop tray integration | N/A | Replaced by the FGS notification ("on/off" + Turn-off action) | Design review | Doc 07 §6 |
| AND-028 | Start on OS login | N/A for v1 | Android policy + product honesty: nothing auto-starts after reboot (old spec §8: "reboot and confirm nothing starts by itself") | Device test (M7) | Documented limitation |

## 5. User-visible states

Kotlin adds `AwaitingConsent` before the engine's existing machine
(off/connecting/on/degraded/error/stopping — doc 04 §5). "Connected"
maps to engine `on` only (ADR-0006). Transitions, ownership, and UI
meaning are the engine's, unchanged.

## 6. Acceptance scenarios

1. First launch: purpose + VPN-slot explanation → only then the system
   consent prompt (M5).
2. Permission denied: usable Disconnected state with an explanation
   (M5).
3. Connect success: consent → service → TUN → engine start → self-test
   pass → Connected (M4/M5).
4. DNS failure: engine surfaces `DNS_*` codes; no fake success
   (inherited, verified in engine tests).
5. Network blocked: bounded heal, honest degraded/error (inherited).
6. User stops: TUN down, notification gone, normal routing (M5).
7. Process/service interruption: onRevoke → off; UI reconciles from
   GetState on resume (M5).
8. Unsupported traffic (UDP beyond DNS, IPv6): documented fail-safe
   behavior (ADR-0009/0010), no silent loss (M4).
9. Device restart: nothing intercepts traffic (AND-028; M7 device
   check).
10. Desktop regression: doc 01 §7 baseline green on every milestone.

## 7. Requirement traceability

| Requirement ID | Design section | Implementation files | Test name/result | Release note |
|---|---|---|---|---|
| AND-001 | doc 04 §3 | `android/` (M3) | not run (no SDK) | M8 |
| AND-002 | doc 04 §5 | `internal/engine` (existing) + `android/` | engine phase-guard tests PASS (2026-10-16) | — |
| AND-003 | doc 04 §6, doc 07 §2 | `android/app` (M5) | not run | M5 |
| AND-004 | doc 04 §7, ADR-0003/0008 | `internal/proxy/socks5.go`, stack integration (M4) | not run | M4 |
| AND-005 | ADR-0006 | `internal/engine/health.go` (existing) | QuickCheck gates `goOn` — engine tests PASS | — |
| AND-006 | doc 04 §8 | VpnService teardown (M5) | engine Stop 3 s budget — proxy tests PASS | M5 |
| AND-007 | ADR-0009 | `internal/doh` (existing) | resolver tests PASS (2026-10-16) | — |
| AND-008 | doc 04 §7 | `internal/fragment` (existing) | plan/conn tests PASS (2026-10-16) | — |
| AND-009 | doc 01 §3.3 | `internal/engine/health.go` | engine tests PASS (heal cooldown, 35 s cap) | — |
| AND-010 | doc 04 §9 | `internal/config` + `apps` field (M6) | schema v2 + Normalize + v1→v2 migration tests PASS (apps_test.go, 2026-10-17 M6) | M6 |
| AND-011 | doc 01 §3.5 | existing engine+UI | resolver+UI behavior verified | — |
| AND-012 | ADR-0003 | allow-list (M4) | device matrix pending | M4/M7 |
| AND-013 | doc 01 §3.7 | existing mode logic | proxy behavior tests PASS | disclosure note |
| AND-014 | doc 01 §3.8 | existing | latency tests PASS | honest label note |
| AND-015 | doc 04 §4 | existing codes + `NO_TARGET_APPS` | engine error tests PASS; NO_TARGET_APPS implemented M7 (D81: gate refusal → error phase; engine + adapter + facade suites) | — |
| AND-016 | doc 01 §3.14 | `GetAppInfo` (existing, 1.6.6) | backend-driven verified | — |
| AND-017 | doc 07 §5 | UI copy (M6) | wired M6 (D80: the sheet's Privacy section); copy review vs §1 re-run M7 (doc 07 §9) | — |
| AND-018 | doc 07 §6 | VpnService FGS (M5) | device test pending | M5 |
| AND-019 | doc 01 §3.3 | `startNetMon` (existing) | engine tests PASS; device re-check | M7 |
| AND-020 | ADR-0010 | route set + AAAA policy (M4) | dual-stack test pending | disclosure note |
| AND-021 | ADR-0009 | stack policy (M4) | protocol test pending | disclosure note |
| AND-022 | ADR-0011 | netx Control hook (existing seam) | M3 call-counting test | — |
| AND-023 | doc 01 §6 | existing Svelte bundle | responsive verified (Phase 20); host-half a11y checks PASS M6 (D80); WebView live re-check ⛔ device checklist | — |
| AND-024 | doc 01 §7 | — | baseline green 2026-10-16 (go 13/13, svelte 0/0, vite OK) | per-milestone |
| AND-025 | doc 06 | `build-android.*` (M8) | not run | M8 |
| AND-026 | doc 04 §2.1 | existing bundle | `?ui=classic` loads (web edition, verified) | — |
| AND-027 | doc 07 §6 | FGS notification (M5) | design only | — |
| AND-028 | §4 above | — | M7 device check | disclosure note |

## 8. Out of scope by default

Unchanged from the baseline: no remote VPN server or claims; no IP
hiding or invented encryption; no circumvention of Android security
controls; no guaranteed-access claims; no desktop rewrites; no fake
signing credentials.
