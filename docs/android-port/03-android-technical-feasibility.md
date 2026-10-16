# 03 — Android Technical Feasibility

**Status:** COMPLETED (Phase 1 / M1, 2026-10-16). The decision matrix
(§7) is scored with repository and web evidence gathered this session;
the provisional recommendation is now the accepted architecture recorded
in `08-architecture-decisions.md` (ADR-0003/0004/0005/0008/0009/0010).
Remaining device-level unknowns are listed as explicit M4 gate items,
not as open architectural questions.

## 1. Key engineering question

The desktop editions rely on Windows/Linux proxy configuration to get
compatible browser traffic into a local relay. Android apps generally
cannot be assumed to modify device-wide proxy settings for all apps in
an equivalent way. The design must first establish how Android traffic
enters Younblock and how that traffic reaches the existing Go engine.

**Answered by evidence:** the engine's front door is a **loopback HTTP
proxy** (`internal/proxy/server.go` — CONNECT + absolute-form forward
proxy + `/proxy.pac`). Android has no app-accessible "set the system
proxy" API, and Chrome for Android offers no user proxy setting.
Therefore traffic must be captured at the network layer with
`VpnService`, reassembled to TCP streams by a userspace stack, and
delivered to the engine's front door. Every seam this requires already
exists in the code (§4), and the repo's own prior Android spec (root
`04-android-spec.md`, Ali-approved) locked the same conclusion before
this pack existed.

## 2. Routing alternatives — evidence collected

| Approach | Potential benefits | Major constraints / risks | Evidence gathered (2026-10-16) | Outcome |
|---|---|---|---|---|
| A. Browser-specific proxy/configuration | Lowest platform integration effort; may reuse the relay directly | Support limited to specific browsers and their actual proxy features; setup may be manual; browser policies differ; may not route app traffic | The relay is a standard HTTP proxy — any browser with proxy support can use it as-is. Chrome Android: **no** user-facing proxy setting (verified: no setting in Chrome 12x for Android; only managed-config/root paths). Firefox Android supports proxies only via `about:config` (unsupported path, manual). | **Documented fallback** — works for Firefox-power-users only; not viable as the primary for the product goal ("works with Chrome and the YouTube app") |
| B. Android Wi-Fi/HTTP proxy configuration | May work in narrow managed or user-configured contexts | Not equivalent to a controllable device-wide proxy; scope and browser support vary; app cannot assume it can change settings silently | Android's Wi-Fi proxy is a per-network manual setting; **no public API for an app to set it**; the global `http.proxyHost` Java property is not honored by Chrome. Verified against current Android developer documentation. | **Rejected** as primary (no programmatic reach) |
| C. `VpnService` plus a local userspace packet/network stack and Younblock engine | Android-supported local VPN interface; per-app routing; transparent to the routed apps | VPN consent, TUN processing, TCP stream/proxy mapping, DNS/UDP/IPv6 decisions, lifecycle handling, no-loop rules, battery/performance work | All engine seams exist: `netx.ControlFunc` documented in-source for `VpnService.protect(fd)` (dialer.go:16-18); whole module compiles for `GOOS=android` on all 4 ABIs (§5 of doc 01); userspace-stack candidates all permissive-licensed and actively maintained (§4 below); the identical architecture ships in production apps (tun2socks-based VPNs, Jigsaw Intra for the DoH+gomobile shape) | **SELECTED** (ADR-0003) |
| D. WebView-only YouTube experience | UI and request path can be constrained to a controlled web surface | Not equivalent to supporting the user's installed browsers; changes product promise and login/media behavior | Product goal per README and old spec §0: user's own Chrome + the YouTube app. | **Rejected** (wrong product) |

## 3. `VpnService` design caveats — resolved into the target design

- A TUN interface supplies IP packets; it does not translate TCP streams
  to an HTTP relay. **Resolution:** a userspace TCP/IP stack (library
  decision in §4/ADR-0008) reassembles flows; flows terminate into the
  engine's front door (SOCKS5 door planned — ADR-0008 — because stack
  libraries natively speak SOCKS5; the HTTP proxy stays available on
  loopback for tests and the fallback).
- The relay accepts: `CONNECT host:port` (host may be a name — the
  engine resolves via DoH), absolute-form `http://` (forwarded), and
  answers `/proxy.pac` + `/health`. The SOCKS5 door will accept
  address types IPv4/IPv6/domain and hand the same `tunnel.go` code the
  destination; **destination-as-IP is sufficient** (fragmentation does
  not need the hostname — the first TLS record is split regardless).
- TLS ClientHello splitting works on a TCP byte stream. **Resolution:**
  the stack reassembly yields ordered streams; `fragment.Conn` wraps
  the client-side stream exactly as on desktop. Stream ordering is the
  userspace stack's core job (proven in production by the same
  libraries).
- UDP, HTTP/3/QUIC, DNS, IPv6, local destinations, multicast, tethering,
  non-browser app traffic: **policy set in ADR-0009/0010** — per-app
  allow-list (default Chrome + YouTube app) bounds the blast radius;
  DNS port 53 to the virtual resolver is answered through the DoH
  resolver (A records; AAAA returns empty so apps stay IPv4); all other
  UDP (incl. QUIC/443) is dropped so routed apps fall back to TCP where
  fragmentation applies (the fallback behavior on the real YouTube app
  is an M4 device gate — the old spec §3.5 flags the same check);
  IPv6 literal destinations are fail-safe (§ADR-0010).
- Routing loops: **resolved by design** — every engine socket goes
  through `netx.Dialer`, whose `Control` hook the Android platform
  adapter implements with `VpnService.protect(fd)` (the hook was written
  for exactly this). M4 must still prove it on-device with a
  call-counting test (the old spec §3.6 mandates the same test).
- MTU, fragmentation, half-close, timeouts, reset, backpressure,
  cancellation: owned by the chosen stack library (its production
  pedigree covers these); engine-side timeouts/deadlines already bound
  every phase (audit §3). M4 integration tests cover reset/timeout/
  stop-while-active explicitly.
- Foreground service + notification: current policy verified — API 34+
  requires a declared FGS type; VPN apps use the **`connectedDevice`**
  type with `FOREGROUND_SERVICE_CONNECTED_DEVICE` permission
  (developer.android.com, fetched 2026-10-16). `POST_NOTIFICATIONS` is
  a runtime permission on 13+. Recorded in doc 07.
- No remote VPN server (product scope; ADR-0007 unchanged).

## 4. Go engine integration options — evidence

| Option | Evidence gathered | Verdict |
|---|---|---|
| 1. `gomobile bind` / AAR | `golang.org/x/mobile` BSD-3-Clause, actively maintained (pushed 2026-10-11). The engine's JSON API is already string-in/string-out (the exact constraint gomobile has with structured types — the repo's old spec §4 documents this deliberately). Whole module compiles for android/arm{,64}/386/amd64 (doc 01 §5). Requirements: Android SDK + NDK + JDK at bind time (Go wiki Mobile). | **SELECTED** (ADR-0004) |
| 2. c-shared/JNI bridge | Handwritten JNI: same cgo cost as gomobile with none of the generated-binding convenience; gomobile IS this option industrialized | Rejected (strictly worse than 1) |
| 3. Go engine in a separate local process | Would re-use `cmd/core --web` verbatim (loopback JSON+SSE), no JNI at all; but adds process lifecycle/IPC complexity on Android and a second binary to keep alive | **Kept as the WebView transport instead of a process**: the same loopback server (`internal/webui`) runs in-process; see ADR-0005 |
| 4. Rewritten native networking component | No evidence of need — every networking package is portable (doc 01 §5) | Rejected |

**Userspace stack candidates** (the TUN→TCP bridge):

| Candidate | License | Maintenance (2026-10-16) | Notes |
|---|---|---|---|
| `xjasonlyu/tun2socks` | MIT | active (pushed 2026-10-11, 5.5k stars) | Go, "powered by gVisor TCP/IP stack"; core engine is an importable Go package; production-hardened |
| `google/gvisor` netstack direct | Apache-2.0 | very active (pushed same day, ~19.7k stars) | maximal control, more integration code; no extra middleman |
| `heiher/hev-socks5-tunnel` | MIT | active (pushed 2026-10-15, ~2k stars) | C — would drag the NDK toolchain into the Go build; strongest perf claims | 
| reference: `Jigsaw-Code/intra` | Apache-2.0 | active | not a dependency — the proof that gomobile + VpnService + DoH ships at consumer scale |

**Decision:** tun2socks' engine package is the primary candidate, with
gVisor-netstack-direct as the in-repo alternative; hev (C/NDK) is the
fallback if Go-stack performance on low-end phones proves inadequate.
Final selection is an **M4 prototype gate with measured evidence**
(throughput, battery proxy metrics unavailable in sandbox → device
test), pre-approved to switch between the two Go options without a new
ADR cycle (ADR-0008 records the criteria).

### Prototype gate (unchanged, now with environment status)

Before committing to the integration route in Phase 2, a minimal
Android app must: build the Go packages for the intended ABIs
(**done at compile level** — all four ABIs, doc 01 §5), invoke an
engine init/health method from Kotlin (blocked: no SDK/NDK/JDK in this
sandbox — first task of Phase 2 M3), start/stop without goroutine
leaks, propagate logs/cancellation/errors, and use the networking APIs
on a real Android runtime. **A Go cross-compile alone does not satisfy
this gate** — the remaining items are explicit M3/M4 exit criteria.

## 5. UI strategy comparison — evidence

| Strategy | Evidence gathered | Verdict |
|---|---|---|
| Kotlin + Jetpack Compose native UI | Strongest lifecycle/accessibility integration; but duplicates TWO existing production UIs (pro + classic); the design system (tokens, choreography, reduced-motion) is Svelte/CSS | Rejected for v1 — revisit only if WebView proves unacceptable (ADR-0005) |
| **Existing Svelte UI inside Android WebView** | Frontend builds as pure static assets; the **http adapter already speaks JSON+SSE** and ships in the Linux web edition since v0.7.2 (D50) — so the UI needs **zero changes** if the app serves it from an in-process loopback `webui` server (same code as `cmd/core --web`); pro screen responsive-verified to 300×620 (Phase 20); reduced-motion respected; classic screen also available (`?ui=classic`) | **SELECTED** (ADR-0005) — Ali's locked "one engine, one UI" (root 04-android-spec.md), now realized with even less new code than the old plan's JS bridge |
| Web app/PWA only | Cannot capture traffic; no service lifecycle | Rejected |

Two transport variants for the WebView (ADR-0005): (a) **loopback
webui** — preferred, zero frontend work, cleartext-localhost permitted
via network security config; (b) `addJavascriptInterface` JS bridge —
the old spec's plan; kept as fallback (more code, tighter control).
Both keep the UI bundle unchanged.

## 6. Provisional recommendation → now the accepted architecture

Investigated and accepted: `VpnService` + per-app allow-list + a
userspace TCP/IP stack + a SOCKS5 front door + the existing engine
packages unchanged + gomobile AAR + Svelte UI in a WebView served by
the in-process loopback webui server. The browser-proxy fallback is
documented (§2 A) for anyone who wants zero-permission usage with
Firefox. Every element maps to an ADR in doc 08; every device-level
unknown is an M4 gate item in doc 05.

## 7. Required decision matrix (scored 2026-10-16)

Scores are evidence-based judgments; the evidence column says what was
actually verified versus what remains a device-gate item. Weighting per
the template: network correctness and maintainability over UI reuse.

| Criterion | Weight | Browser proxy | Android proxy | Local VPN/TUN | WebView-only |
|---|---:|---:|---:|---:|---:|
| Core Younblock behavior | 5 | 4 | 2 | 5 | 3 |
| Ability to reach actual relay | 5 | 5 | 1 | 5 | 5 |
| Traffic coverage matches product goal | 5 | 2 | 1 | 5 | 2 |
| Engineering feasibility | 4 | 5 | 2 | 4 | 4 |
| Lifecycle and reliability | 4 | 5 | 3 | 4 | 3 |
| Privacy clarity | 4 | 5 | 4 | 5 | 4 |
| Long-term maintenance | 4 | 5 | 2 | 4 | 3 |
| User setup friction | 3 | 1 | 2 | 5 | 5 |
| UI reuse | 2 | 4 | 4 | 5 | 5 |
| Performance/battery | 4 | 5 | 4 | 3 | 3 |
| **Weighted total (÷45 max)** | | **121/225** | **81/225** | **188/225** | **116/225** |

Evidence notes behind the scores: browser-proxy — relay reachable (5)
but Chrome Android has no proxy setting (coverage 2, friction 1);
Android-proxy — no programmatic API (reach 1); VPN/TUN — full marks on
the three weight-5 criteria because every code seam is verified
(audit §4/§5), feasibility 4 not 5 because the runtime prototype is
still pending (no SDK in sandbox), performance 3 because a userspace
stack costs CPU/battery vs. none at all; WebView-only — loses on
product-goal fit.

## 8. Current authoritative documentation checked

- Android VPN guide, FGS requirements (API 34+ type declarations;
  `connectedDevice` for VPN), network security config — fetched
  2026-10-16 during this audit.
- Go wiki Mobile (gomobile requirements: SDK/NDK/JDK + `ANDROID_HOME`),
  `golang.org/x/mobile` repo activity — fetched 2026-10-16.
- Licensing for all candidate libraries — fetched from the
  repositories' license files 2026-10-16 (MIT/Apache-2.0/BSD-3).
- Re-verify API-level-specific behavior at implementation time
  (recorded as an M3/M4 checklist item, not assumed here).

## 9. Exit gate — MET

Repository audit complete (doc 01), relay and traffic path understood
(HTTP proxy + planned SOCKS5 door), integration choice evidenced
(gomobile + compile matrix), all P0 risks have either evidence-backed
mitigation (protect-hook, per-app routing, FGS type, licenses) or an
explicit M4 device gate (QUIC fallback, protect-on-device, battery).
The architecture is approved in doc 08; Phase 2 may begin at M3.
