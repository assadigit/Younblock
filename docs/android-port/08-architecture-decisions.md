# 08 — Architecture Decision Records

**Status:** Phase 1 complete. ADR-0001/0002/0006/0007 were accepted as
constraints and are now backed by audit evidence; ADR-0003/0004/0005
moved from Proposed to **Accepted** on 2026-10-16 with the evidence
below; ADR-0008…0012 record the new decisions this phase produced.
Update records when evidence changes.

## ADR-0001 — Keep desktop app working; isolate Android integration

- **Status:** Accepted (evidence-backed 2026-10-16).
- **Context:** Younblock ships on Windows and Linux (v0.7.6, 14
  artifacts); Android introduces different platform APIs and lifecycle.
- **Decision:** Android integration adds new packages only
  (`internal/platform/android`, `internal/proxy/socks5.go`, `mobile/`,
  `android/`); desktop files and build lanes stay untouched except
  additive, regression-gated changes (e.g. the `apps` settings field).
- **Evidence:** the isolation already exists — build tags route
  `internal/platform/{windows,linux,stub}`; the engine speaks to the OS
  only through `engine.Platform`; the whole module compiles for
  `GOOS=android` without touching desktop code (audit doc 01 §5).
- **Alternatives rejected:** fork (drift risk — the v0.7.x history
  shows how fast two UIs drift, hence D66's freeze discipline);
  rewrite-all-platforms; one runtime for all (Wails is not Android).
- **Consequences:** shared-code edits carry the desktop-baseline gate
  (doc 06 §A). **Revisit when:** never on current evidence.

## ADR-0002 — Do not assume Wails is the Android application runtime

- **Status:** Accepted (constraint, verified).
- **Evidence:** the desktop runtime lives in the root package +
  `wails.json` (audit §3.1); nothing under `internal/` imports Wails;
  the UI already runs headless via the http adapter (web edition,
  D50) — so the Android runtime never needs Wails.
- **Decision:** Android = Kotlin shell + WebView + gomobile AAR; Wails
  stays desktop-only.

## ADR-0003 — Traffic ingress: local VpnService + per-app allow-list

- **Status:** **Accepted 2026-10-16** (was: proposed / blocked pending
  evidence).
- **Context:** Android exposes no app-settable system proxy; Chrome
  Android has no user proxy setting; the product goal is the user's own
  Chrome + the YouTube app.
- **Decision:** `VpnService` TUN with `addAllowedApplication` per-app
  routing (default `com.android.chrome` +
  `com.google.android.youtube`), userspace TCP/IP stack reassembling
  flows into the engine (see ADR-0008), virtual DNS → DoH (ADR-0009),
  `protect()` via the netx Control hook (ADR-0011), foreground service
  type `connectedDevice` on API 34+.
- **Evidence:** engine seams verified in source (netx hook written for
  protect; Platform interface; string JSON API); whole module compiles
  for all four Android ABIs; the identical architecture ships in
  production (tun2socks-family VPN apps; Jigsaw Intra for
  gomobile+VpnService+DoH); prior product decision by Ali (root
  04-android-spec.md §0-3) reached the same conclusion.
- **Fallback (documented, not primary):** the engine's HTTP proxy is
  directly usable by proxy-capable browsers (Firefox Android via
  about:config) — a zero-permission narrow mode, disclosed as such.
- **Release gate:** no "works with every app" claims; device-gate items
  (QUIC fallback, protect-on-device) live in roadmap M4.

## ADR-0004 — Go integration: gomobile bind AAR over a narrow facade

- **Status:** **Accepted 2026-10-16** (was: proposed, pending
  prototype).
- **Decision:** `golang.org/x/mobile` `gomobile bind` producing
  `younblock.aar`, wrapped by a small `mobile/` package whose methods
  are the engine's existing string-in/string-out JSON API. No internal
  object graphs cross the boundary.
- **Evidence:** x/mobile BSD-3-Clause, actively maintained (pushed
  2026-10-11); the JSON API's shape was chosen years ago exactly for
  gomobile's no-slices-of-structs constraint (root 04-android-spec.md
  §4; jsonapi.go confirms); compile matrix green on all ABIs.
- **Alternatives rejected:** handwritten JNI (gomobile industrializes
  it); separate local process (lifecycle cost; its one advantage —
  loopback transport — is realized in-process by ADR-0005); native
  rewrite (no need — audit §5).
- **Prototype gate remains:** the first `gomobile bind` run needs
  SDK+NDK+JDK (absent in the audit sandbox — doc 01 §7, doc 09); it is
  M3's first task, and its outcome (a building AAR + a Kotlin-side
  init/health/stop call) is M3's exit criterion.

## ADR-0005 — UI: the existing Svelte bundle in a WebView, served by
the in-process loopback webui server

- **Status:** **Accepted 2026-10-16** (was: proposed).
- **Decision:** reuse the production UI unchanged. The app embeds
  `internal/webui` (the same server behind `cmd/core --web`), binds it
  to loopback, and the WebView loads it; the frontend's **existing
  http adapter** (JSON + SSE) talks to it — zero frontend changes.
  Cleartext is permitted for 127.0.0.1 only via the network security
  config. The classic screen remains available via `?ui=classic`.
- **Evidence:** the http adapter + webui pair ships in the Linux
  single-executable edition since v0.7.2 (D50) — production-proven
  transport; the pro screen is responsive to 300×620 with
  reduced-motion support (Phase 20 verification); the UI's Wails-only
  touchpoints are already capability-gated behind the Backend
  interface (audit §6).
- **Alternatives:** JS-bridge adapter (`addJavascriptInterface`) — the
  old spec's plan; kept as the documented fallback if loopback-in-
  WebView misbehaves on target devices (more code, same UI); native
  Compose UI — rejected for v1 (duplicates two production UIs),
  revisit only on evidence that WebView UX is unacceptable.
- **Consequences:** the Android UI inherits every state, notice, and
  self-test surface for free; Ali's "one engine, one UI" holds.
  WebView hardening rules are in doc 07 §6.

## ADR-0006 — Never show a false Connected state

- **Status:** Accepted (verified in the engine's design).
- **Evidence:** "on" requires QuickCheck (listener+dns+youtube) to
  pass; degraded is an honest partial state with the watchdog running
  (R14/D44); `LatencyMs` is a real application round trip (D41).
- **Android mapping:** the WebView UI projects engine phases verbatim;
  service death forces `off`; consent denial never fabricates states.

## ADR-0007 — Privacy claims follow implementation evidence

- **Status:** Accepted; disclosure copy drafted in doc 07.
- **Android-specific statement:** Younblock uses Android's VPN slot as
  a **local traffic-routing interface for the allow-listed apps
  only**; no remote server of ours receives traffic; the DoH providers
  (user-configurable) see the DNS queries those apps make; the user's
  IP is not hidden; no encryption is added beyond normal TLS.

## ADR-0008 — Userspace stack: tun2socks engine package (primary),
gVisor netstack direct (alternative), hev-socks5-tunnel (fallback)

- **Status:** Accepted 2026-10-16 as a **gated selection**; **resolved
  at M4 (2026-10-16) to gVisor netstack direct** — the switch this ADR
  pre-authorized, exercised on design-level evidence (D75): the
  tun2socks `engine` package is a CLI global singleton whose UDP model
  demands a UDP-capable proxy, and cannot integrate with the
  CONNECT-only door (ADR-0009) or the engine's per-cycle server. The
  device-measurable criteria below still apply to the chosen stack and
  ride with the M5 device checklist (the wire is a channel endpoint;
  fdbased is the documented same-seam upgrade).
- **Criteria (measurable at M4):** TCP throughput and latency on a
  reference device/emulator within 2× of direct; CPU/battery within
  the app's notification-disclosed budget; clean integration with the
  protect() hook and the SOCKS5 door; license stays MIT/Apache; no NDK
  requirement (rules out hev unless performance forces it — hev is C
  and drags the NDK in).
- **Evidence:** licenses + maintenance fetched 2026-10-16 (doc 03 §4);
  both Go options embed the same netstack lineage (gVisor).
- **Consequence:** the SOCKS5 front door (doc 04 §2.6) keeps the stack
  swappable and independently testable.

## ADR-0009 — Protocol policy inside the tunnel (scoped, fail-safe)

- **Status:** Accepted 2026-10-16.
- **Policy:** TCP: all flows of allow-listed apps enter the SOCKS5 door
  (routed-domain fragmentation per the existing mode rules). DNS:
  UDP:53 to the virtual resolver is answered through the engine's DoH
  resolver — A records answered; **AAAA answered empty** so apps stay
  on IPv4 (matching the desktop default, `QueryAAAA` off); other
  record types: empty non-error answers (recorded choice). UDP other
  than DNS (incl. QUIC/443): **dropped** — routed apps fall back to
  TCP where the fragmentation applies; the YouTube app's actual
  fallback behavior is an M4 device gate (old spec §3.5 flags the same
  check — no guessing, report what the device does).
- **Rationale:** matches the desktop engine's IPv4-only DNS posture,
  keeps the first-record fragmentation meaningful (TCP only), bounds
  the UDP surface we must reason about.

## ADR-0010 — IPv6: explicitly disabled with fail-safe routing

- **Status:** Accepted 2026-10-16.
- **Policy:** the TUN advertises IPv4 routes only; AAAA answers are
  empty (ADR-0009) so DNS-discovered destinations stay IPv4. If a
  routed app still sends to a literal IPv6 destination on a network
  with IPv6, that traffic must not silently bypass the tunnel: the
  service adds an IPv6 route to the TUN as well and the stack
  blackholes v6 flows (fail-safe: visible breakage for that flow, not
  a leak) — M4 verifies the chosen stack's behavior and adjusts the
  route set so the invariant "no routed-app traffic bypasses the
  engine" holds on dual-stack devices.
- **Rationale:** doc 07 §3's no-leak rule; honest "IPv4-only" scope
  statement in all user-facing text (AND-020).

## ADR-0011 — Loop prevention is the dialer's job

- **Status:** Accepted 2026-10-16.
- **Policy:** every engine upstream socket (tunnel dials, DoH
  bootstrap connections) goes through `netx.Dialer`; the Android
  platform adapter's `DialControl()` returns a ControlFunc that calls
  `VpnService.protect(fd)`. M3 unit-tests with a call-counting fake
  (the old spec §3.6 mandates "test that every dial calls it");
  M4 re-verifies on-device (no loop = youtube still loads while the
  tunnel is up).
- **Evidence:** the hook, its doc comment, and its plumbing through
  the DoH config exist in source today (netx/dialer.go:16-18,
  doh/resolver.go Config.Control, engine platform.DialControl).

## ADR-0012 — Distribution: sideloaded APK, debug-signed for the
circle of friends; Play is out of scope

- **Status:** Accepted 2026-10-16 (carries forward Ali's prior
  decision, root 04-android-spec.md §0/§7).
- **Policy:** debug APK for testing; release APK signed with a
  locally-generated keystore that is never committed; README gets
  plain-language backup instructions; Play Protect's sideload warning
  is documented. Play Store policy review happens only if Ali changes
  distribution scope (doc 07 §8).
- **Consequence:** Play-specific compliance (FGS declaration review,
  Data Safety forms) is explicitly not blocking for v1.

## Decision-change procedure

When changing an accepted decision: new evidence, alternatives
reconsidered, affected files/requirements/tests, user-visible impact;
update this file and the pack README before implementing dependent
work. Gated selections (ADR-0008) switch on their listed criteria with
a measurement note, without a full re-litigation.
