# 08 — Architecture Decision Records

**Status:** Phase 1 complete. ADR-0001/0002/0006/0007 were accepted as
constraints and are now backed by audit evidence; ADR-0003/0004/0005
moved from Proposed to **Accepted** on 2026-10-16 with the evidence
below; ADR-0008…0012 record the new decisions this phase produced;
ADR-AUTH-0001 (2026-10-18) opened the auth lane from D94 ruling 1 and
was **RETIRED the same day by D97** (the gate cancelled before
implementation — kept below as history).
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

## ADR-0013 — First-flow latency: fast address failover in the one
dialer, and a budget-true TUN handshake timeout (M8-n)

- **Status:** Accepted 2026-10-18 (M8-n, the first-video device defect:
  feed fast, first video 30 s–3 min, Windows instant — root-caused to a
  code-level asymmetry between the two doors, not to the network).
- **Context:** The Android TUN path accepted the phone's TCP handshake
  locally (netstack), then asked the SOCKS5 door, bounded by
  `HandshakeTimeout` — a 10 s default the facade never overrode. The
  door's own honest setup (DoH resolve, up to 3 servers × 3 s, plus the
  upstream dial, up to 12 s overall) could legitimately exceed 10 s:
  googlevideo answers on filtered networks carry silently-blackholed
  addresses, and the dialer walked them SEQUENTIALLY at 5 s each. The
  desktop doors have no such abort — the browser waits out the same
  budget and gets its tunnel. The retry trap: DoH answers are cached
  with a 120 s minimum TTL, so every app retry repeated the same
  address walk into the same 10 s wall until the cache rotated — the
  reported minutes-long first-video wait. (M8-j had eliminated the
  host-level suspects; this pair was the Android-only structural
  difference left standing.)
- **Decision (two halves, one fix set):**
  1. `netx.DialHost` dials the address list with Happy-Eyeballs-style
     staggered parallelism (RFC 8305's reasoning): attempt 0 starts
     immediately, attempt k at k×`FallbackDelay` (default 750 ms), each
     attempt keeps its full `PerIPTimeout`, first success wins and the
     losers are canceled. A blackholed front address costs ~0.75 s +
     RTT instead of 5 s — on BOTH doors (the browser path benefits
     equally). File 03 R4's "try every address" semantics are preserved
     (every address is still tried; error surface unchanged).
  2. The TUN handshake budget must cover the door's full budget:
     `config.TunHandshakeTimeout` (30 s = 3×3 s DoH + 12 s dial +
     slack), passed explicitly by the mobile facade, with the tunstack
     package default raised to the same value as the backstop.
     `TestHandshakeTimeoutDefaultCoversDoorBudget` pins the lockstep
     and the covers-the-door-budget invariant.
- **Evidence:** `internal/netx/dialer.go` (the race + cancel-on-winner),
  `dialer_test.go` (TestDialHostRacesPastBlackholedFrontAddress,
  TestDialHostWinnerCancelsLosers), `internal/config/defaults.go`
  (TunHandshakeTimeout + its derivation comment),
  `internal/tunstack/tunstack.go` (defaultHandshakeTimeout),
  `mobile/tun.go` (the explicit pass); desktop baseline green
  (vet clean, 18/18 ×2, GOOS=windows exit 0); compile matrix 4/4 ABIs +
  arm64 full tree; debug.7 carries the fix (versionCode 6).
- **Alternatives rejected:** replying SOCKS5 success before the
  upstream dial (hides failures from the app — dishonest per D77's
  rule); shrinking the door's budgets instead (the desktop path relies
  on them; per-IP walking exists precisely for filtered networks);
  prewarming rr*.googlevideo.com (video CDN names are per-video random
  — nothing stable to warm).
- **What this ADR does NOT decide:** the on-device verification — the
  fix is host-proven (timing tests pin both halves), but "first video
  starts like Windows" on Ali's phone is his device round to confirm
  (doc 10 §"Device test circle" row 3 stays open until then). QUIC
  fast-fail on device (row 9's device half) is likewise still his.

## ADR-AUTH-0001 — The account gate is engine-enforced (from D94 ruling 1) — RETIRED

- **Status:** **RETIRED (D97, 2026-10-18, A0-R3)** — Ali cancelled the
  account gate before a line of it was built ("cancel it altogether,
  no auth needed anymore"); the A1/A2 implementation ladder that
  would have re-verified this ADR's seams is cancelled with it. The
  ADR never governed shipped code (A0 built nothing — it was a design
  decision for a future feature), so retirement changes nothing in
  the product; it is kept below as the history of the cancelled
  design, alongside the bannered `docs/auth/` pack. D95 and D96 are
  superseded by the same ruling; all auth open questions closed as
  moot. Originally: accepted as the gate's architecture (D94,
  2026-10-18); elaborated in `docs/auth/02-architecture.md` (A0,
  2026-10-18). The implementing A-series was to re-verify every seam
  claim against code before any user-facing copy claimed it (D77–D94
  rule) — that re-verification never ran and will never run.
- **Context:** D94 opens the members-only lane; the gate must hold on
  every edition and cannot be a UI-layer promise.
- **Decision:** the gate lives in the engine — `Engine.Start()` refuses
  a signed-out app with a typed code (`AUTH_REQUIRED`, the
  `NO_TARGET_APPS` road: the engine's typed-code switch, jsonapi's
  `errResult`, the facade whitelist) — so the Wails shells, the Android
  facade and `cmd/core --web` inherit it by construction. Capabilities
  gain `auth: true` so the shared screen renders one signed-out state
  on all platforms (the D92 mechanism `keepInTrayOnClose` uses).
- **Evidence (the seams, verified in source this session):**
  `internal/engine/platform.go` (Platform, Capabilities, StartGater
  D77, PlatformRefusalError D81); `engine.go` Start()/connectSequence;
  `jsonapi.go` typed results; `mobile/mobile.go` code whitelist;
  doc 04 §6's connect ordering (login precedes consent).
- **Alternatives rejected:** per-shell gates (drift + a UI-only promise
  the engine does not back); a VPN-server-style gate (would change the
  traffic story — D94 ruling 2 forbids it: the gate gates usage, not
  traffic).
- **What this ADR does NOT decide:** the six open questions (backend
  location, account creation, fail-open/closed, token design, password
  parameters, brute-force form) — they stay OPEN for Ali (docs/auth/00
  §"The six open questions"); D95+ records the rulings when they land.

## ADR-0014 — Dial-failure feedback into the DNS cache: a total dial
failure demotes the cached answer to stale (M8-o)

- **Status:** Accepted 2026-10-18 (M8-o, the per-video stall loop:
  feed instant, EVERY video 1-2 min to start and re-buffering mid-play,
  Windows instant on the same ISP — the surviving amplifier behind the
  symptom M8-n's dialer fix could not cure).
- **Context:** ADR-0013 made the address walk fast (750 ms failover
  past a blackholed front address) and the handshake budget honest.
  But the device round on top of it (debug.7) reported the same
  per-video stall — because racing cannot help when the WHOLE answer
  set is blackholed: rr*.googlevideo.com hostnames map to the few
  addresses of one cache node each, and on a filtered network a
  filtered node poisons the entire set. The phone app retries every
  few seconds; every retry re-dials the same cached set, fails, RSTs —
  and the DoH cache (120 s minimum TTL) keeps serving that suspect set
  until it rotates. The app's only escape is a DIFFERENT answer, which
  the cache blocks for up to two minutes — exactly the observed
  per-video wait, and (because videoplayback URLs rotate rr hostnames
  during playback) exactly the mid-play re-buffering. The feed never
  suffers: its hosts are prewarmed, warm-cached and long-lived.
  The desktop door shares this path but the browser player's retry
  profile hides the loop better — the feedback loop is a defect on
  BOTH doors regardless; fixing it cannot regress the desktop.
- **Decision (one mechanism, three call sites + one consistency fix):**
  1. `doh.Resolver.Invalidate(host)`: demote a still-fresh success
     entry to STALE (expiresAt = now; staleUntil unchanged). The next
     Lookup re-queries the servers; if they fail, serve-stale still
     answers (R5 preserved — invalidation never makes a failure path
     worse). IP literals, negatives and misses are no-ops (negatives
     already re-query at their own 15 s expiry).
  2. Both doors report total dial failures: the HTTP-CONNECT tunnel,
     the plain-http forward and the SOCKS5 door call
     `Server.invalidateDNS(host)` after `DialHost` fails on every
     address. Optional capability (interface assertion) so the tests'
     minimal fakes keep working unchanged.
  3. The resolver's OWN bootstrap dial joins ADR-0013: dialBootstrap
     now races the bootstrap IPs through the same `DialHost` instead
     of walking them sequentially at one full Timeout each — a
     blackholed front bootstrap costs ~0.75 s, not 3 s, per query.
- **Evidence:** `internal/doh/cache.go` (invalidate),
  `internal/doh/resolver.go` (Invalidate + the racing bootstrap),
  `internal/proxy/server.go` (invalidateDNS + the two HTTP-door call
  sites), `internal/proxy/socks5.go` (the SOCKS5 call site); pinned by
  TestInvalidateForcesRequery, TestInvalidateKeepsServeStale,
  TestInvalidateNoops, TestDialBootstrapFailsOverToSecondBootstrapIP
  (doh) and TestSocks5DialFailureInvalidatesDNS,
  TestConnectDialFailureInvalidatesDNS,
  TestDialFailureWithoutInvalidatorIsGraceful (proxy, the new
  dialfeedback_test.go). Desktop baseline green on the fixed tree
  (vet clean, 18/18 ×2, GOOS=windows exit 0); Android compile matrix
  4/4 ABIs + arm64 full tree.
- **What this ADR does NOT decide:** whether the rr-hostname answer
  sets are actually all-blackholed on Ali's network (that is the
  device question the debug.8 round answers — if the stall persists
  with this fix, the diagnostics report's tunnel timing lines carry
  the decisive dns/dial/first-byte/dur numbers, and the QUIC row 9
  device half stays open); QUIC/UDP forwarding policy (ADR-0009
  unchanged); the DNS cache TTL constants themselves.

## Decision-change procedure

When changing an accepted decision: new evidence, alternatives
reconsidered, affected files/requirements/tests, user-visible impact;
update this file and the pack README before implementing dependent
work. Gated selections (ADR-0008) switch on their listed criteria with
a measurement note, without a full re-litigation.
