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

## ADR-0015 — The in-app diagnostics surface: a first-class Diagnostics
screen with an honest clipboard ladder, and the facade's evidence in the
engine report (M8-p)

- **Status:** Accepted 2026-10-18 (M8-p, Ali's debug.8 round: the
  stall persisted AND the promised evidence path did not exist —
  "there is no diagnostic option in the apk app setting").
- **Context:** The device feedback loop's required input (doc 10's
  circle: "the diagnostics report is the single most valuable thing
  to send") had a broken delivery path. The report copy button lived
  buried in Help → Still stuck, and its implementation —
  `navigator.clipboard.writeText` with a catch that toasts "Copied"
  regardless — silently fails inside the Android WebView, where the
  async clipboard API is missing or rejects depending on the WebView
  build. Two root-caused fixes (ADR-0013/0014) had already been
  disproven on-device in a row; a third guess without evidence would
  violate the loop's own discipline. The correct engineering move is
  to make the evidence reachable and verifiably delivered: the
  diagnostics surface itself is the round's deliverable (Ali's D100
  directive).
- **Decision (four pieces, one theme — evidence that actually arrives):**
  1. **The Diagnostics screen** (frontend, every edition): a menu
     entry (between Help and About) AND a Settings row — the two
     places a user looks — showing the full report with three ways
     out: Copy, Share (Android only), and the selectable text itself
     (a readonly textarea — the path of last resort that cannot
     fail). Refresh re-pulls the snapshot.
  2. **The honest clipboard ladder** (`lib/clipboard.ts`): the
     Android shell bridge first (window.YounblockShell — see piece 3),
     navigator.clipboard second, an execCommand fallback third. Every
     path resolves a real boolean; the toasts say "Copied" only on
     success and otherwise instruct the manual path. Help's copy
     button joins the same ladder (its silent false-success is fixed).
  3. **The Android shell bridge** (Kotlin PageBridge,
     addJavascriptInterface "YounblockShell"): copyToClipboard via
     the native ClipboardManager (thread-safe binder call; works on
     every WebView build) and shareText via the system share sheet
     (marshalled to the main thread). The exposed surface is
     deliberately write-only with caller-provided text — nothing
     reads data back out, and only the engine's own loopback page can
     load in this WebView at all (the navigation guard, doc 07 §6).
  4. **The facade's evidence in the report** (Go): the engine gains
     an optional embedder hook — `Engine.SetReportExtra(func()
     string)` — which the mobile facade installs with (a) the TUN
     counters (flows/rstUnmapped/overCap/doorFailures/dnsQueries/
     udpDropped — the one surface that names the "flow never reached
     the door" case and the QUIC/UDP row's suspect, invisible to the
     engine by construction) and (b) the shell build identity
     ("android shell: 1.6.6 (versionCode 8)", set by Kotlin from
     PackageManager — a pasted report names its own release, since
     versionName stays 1.6.6 across debug rounds). The report's log
     window doubles from 100 to 200 lines (a stalled video's retry
     loop writes ~3 lines per attempt; the wider window keeps the
     whole stall story). Counters only in the new section — the R21
     redaction (no hostnames, no addresses) extends to it, pinned by
     test.
- **Evidence:** `internal/engine/engine.go` (SetReportExtra) +
  `internal/engine/jsonapi.go` (the hook call, the 200-line window)
  pinned by TestEngineReportExtra; `mobile/mobile.go` (the facade
  hook, SetShellVersion, diagnosticsExtra) + `mobile/report_test.go`
  (NEW — TestReportSectionWithoutTun, TestReportSectionWithLiveTun:
  the section lands in the real report, inactive and active branches,
  no address leak); frontend `lib/clipboard.ts`, `screens/
  Diagnostics.svelte`, the Menu/Settings entries, i18n keys
  (svelte-check 0/0, vite build clean); `MainActivity.kt` (PageBridge
  + the EngineHolder version call) compiled by the clean
  assembleDebug; AAR carries the setShellVersion binding. Desktop
  baseline green (vet clean, 18/18 ×2, GOOS=windows exit 0); Android
  compile matrix 4/4 ABIs + arm64 full tree; debug.9 APK verified
  (sha256, apksigner pinned cert, badging versionCode 8).
- **What this ADR does NOT decide:** the still-open per-video stall
  itself (row 3 stays open — the next root-cause waits for a real
  pasted report; no third guess); QUIC/UDP forwarding policy
  (ADR-0009 unchanged — the tun: line's udpDropped counter is the
  OBSERVATION surface, not a policy change); whether Share grows into
  a full export/save path; log-file export (the report's ring
  snapshot is the surface; the rotating app.log stays a developer
  path).

## ADR-0016 — The diagnostics surface ships in the variant the edition
serves: the pro screen's first-class Diagnostics row and report view
(M8-q, the ADR-0015 port)

- **Status:** Accepted 2026-10-18 (M8-q, Ali's debug.9 round: "I still
  dont see diagnostic button" — after a release that claimed to ship
  exactly that button).
- **Context:** ADR-0015 shipped the diagnostics surface into the
  classic UI tree (`src/classic/*`). But the variant dispatcher
  (`src/main.ts`, D66) bakes the UI at build time from
  `VITE_UI_VARIANT` — set ONLY by the desktop wails lanes; the
  Android lane (`build-android.sh`) leaves it unset, so the APK
  serves the pro screen (`src/App.svelte` + `src/lib/MenuSheet.svelte`).
  Ali's device therefore never rendered the M8-p surface at all. The
  pro screen's only diagnostics surface was the M6-era sheet section
  buried at the very bottom (below About, How it works, Privacy,
  Other websites and Help) behind an `android` platform gate, whose
  Copy used the raw `navigator.clipboard` call — the exact
  silent-failure ADR-0015 documents. The web single-file edition
  (also pro by default) had the same hole. A shipped-in-the-wrong-
  tree feature is indistinguishable from a not-shipped feature on the
  device; D101 records the rule, this ADR records the port.
- **Decision:** The ADR-0015 surface exists in BOTH UI trees, and the
  pro screen's entry is placed where the evidence path leads:
  1. **The lead row of the menu sheet** (`MenuSheet.svelte`): a
     full-width card row — activity icon, "Diagnostics", "Copy a
     support report to paste into a chat", chevron — the FIRST thing
     in the sheet on every platform (no android gate, no scrolling;
     56px target). It dispatches `diagnostics`; the sheet closes and
     the view opens.
  2. **The full-screen report view** (`src/lib/DiagnosticsView.svelte`,
     new): auto-loads the report on open; Copy through the SAME
     ladder (`classic/lib/clipboard.ts` imported cross-tree — pure
     DOM/window code, no classic-UI dependencies); Share only where
     the shell bridge offers it (`canShare()`); Refresh; honest
     inline status ("Copied — paste it into the chat." / the manual
     instruction on failure); the report in a readonly selectable
     textarea (the path of last resort). Escape and Back close it;
     Tab wraps inside; focus returns to the menu button.
  3. **The old buried section is REMOVED** — one surface, not two,
     and the broken-copy path goes with it. The classic tree keeps
     its M8-p screen unchanged (verified: `?ui=classic` still shows
     the Menu → Diagnostics entry).
- **Evidence:** svelte-check 0/0; vite build clean; the served
  surface verified END-TO-END through the real webui server
  (`cmd/core --web`, the same embedded dist + dispatcher the APK
  serves): menu open → Diagnostics is the first row → the view loads
  the real report → Copy resolves honestly → Back returns; classic
  variant re-verified through the same server. The AAR's embedded
  Go library carries the new UI strings (`diag-entry`, the row hint);
  debug.10 APK verified (sha256, apksigner pinned cert, badging
  versionCode 9). Desktop baseline green (vet clean, 18/18 ×2,
  GOOS=windows exit 0).
- **What this ADR does NOT decide:** which variant the APK should
  serve (D66 unchanged — pro stays the Android screen; switching the
  whole UI on Ali's device mid-loop would be its own disruption);
  the per-video stall (doc 10 row 3 stays open — the port makes the
  evidence path reachable; the report itself is still Ali's to
  send); the classic tree's own entry placement (its Menu shape is
  unchanged).

## ADR-0017 — The heal sweep probes from zero: reset the resolver when a
sweep starts, restore the method when it finds nothing, and bake the DNS
failure reasons into the report's log lines (M8-r, the debug.10 report
round)

- **Status:** Accepted 2026-10-18 (M8-r — the FIRST pasted diagnostics
  report, Ali's "the problem persists" round; the evidence D100 built
  the loop around).
- **Context:** Ali's debug.10 report (versionCode 9) finally carried the
  engine's own telemetry, and it read back against the code exactly:
  `state: degraded`, `last failed check: dns (DNS_UNREACHABLE)`,
  `heal sweep found no working method` 12.4 s into the run,
  `method: legacy`, `dns ok 58, fail 16` — while the same log showed
  the sweep's own video handshakes COMPLETING (three of four methods'
  `tunnel first-byte` lines) and, minutes later, a 2.5 MB download.
  The reconstruction: the QuickCheck's dns step failed against all
  three DoH servers in ~170 ms each (a startup blip), which
  negative-caches `www.youtube.com` for the 15 s negative TTL; the
  automatic heal sweep triggered by that very failure then probed
  `www.youtube.com` for EVERY method, and every probe failed
  INSTANTLY on the negative entry (`ErrRecentlyFailed` — no server is
  asked again inside the window); the sweep declared "no working
  method", the engine went degraded, and Ali received "Couldn't fix
  it. Open Younblock." 21 s after pressing Start on a network that had
  already recovered. Two collateral defects rode the same report:
  (a) `applyMethod` mutates the live setting as the sweep probes, so
  the FAILED sweep silently left `method: legacy` — the last-tried
  preset, dragged off the user's saved choice by the very sweep that
  found nothing; (b) the resolver's `dns server failed` line logged
  its server and reason as slog ATTRS — the log ring that feeds the
  diagnostic report keeps only the MESSAGE (by design, R21-adjacent),
  so the pasted report showed three bare `dns server failed` lines
  that could not say timeout vs reset vs bad answer. The sweep's
  structural incapacity — poisoned by its own trigger — was the
  round's root cause; the per-video stall itself (five ~51 s flows
  with ~7 KB down; `udpDropped=76`) is DPI-side and stays open as
  row 3 evidence, not guessed at.
- **Decision:** Three engine changes, each pinned by a regression test
  that FAILS on the pre-fix code:
  1. **A heal sweep resets the DNS resolver when it starts**
     (`healSweep` → `resolver.Reset()`): a sweep is a
     suspected-network-change event and probes from zero — the exact
     semantics the network monitor already applies on a real change
     (R19). The sweep's youtube step re-queries the servers instead of
     tripping over the check that triggered it; the sticky server and
     cached answers are re-earned, not inherited from the broken
     moment. This does NOT make a dead network look alive: if the DoH
     is still down, the sweep's fresh attempts fail honestly and the
     engine degrades with the dns failure recorded (D44's test
     unchanged and green).
  2. **A failed sweep restores the previous method**
     (`applyMethod(prev)` on the no-working-method path): nothing was
     found, so the live fragmentation preset returns to what ran
     before the sweep. A successful sweep still saves its winner
     (`METHOD_SWITCHED` / `RECOVERED` notices unchanged).
  3. **The DNS failure lines carry their evidence in the message**:
     `dns server failed server=#N reason=<kind>` (and the fallback
     line `…failed=#N now=#M`, plus `reason=` on the serve-stale and
     the proxy's `dns lookup failed for tunnel` lines) — the report's
     ring keeps the message, so the next pasted report names the
     failure KIND (timeout vs network vs DNS_UNREACHABLE-class). The
     configured server NAME stays a file-log attr; the message
     carries only the 1-based index (no user-chosen strings enter the
     report — the R21 posture holds).
- **Evidence:** The host reproduction is the round's proof: with the
  fixes reverted, `TestHealSweepSurvivesFreshNegativeCache` fails into
  EXACTLY the pasted report's signature — `Phase:degraded
  Method:legacy` on a recovered network (a DoH that blips once at
  start); with the fixes, the engine reaches ON through the sweep
  (method balanced), and the report carries the baked failure line.
  `TestFailedSweepRestoresPreviousMethod` fails without fix 2
  (method=legacy persists); `TestServerFailureLinesCarryEvidence`
  fails without fix 3 (the bare message). Full baseline green: go vet
  clean; 18/18 packages ×2; GOOS=windows exit 0; the Android compile
  matrix 4/4 ABIs; the full build-android.sh lane with the embedded
  engine verified in the AAR (the baked log strings present in
  libgojni.so); debug.11 APK artifact-verified (badging versionCode
  10, apksigner pinned cert, sha256 recorded in doc 06).
- **What this ADR does NOT decide:** the CAUSE of the startup DoH
  blip itself (three ~170 ms fast-failures — DPI reset vs a routing
  transient at VPN establish; fix 3 exists so the NEXT report answers
  it with `reason=` evidence — no third guess); the per-video stall
  (row 3: the five 51 s / ~7 KB flows and the QUIC drops are
  ISP-side interference on the direct path — the app cannot un-throttle
  the network, and no steering change is made without a ruling); the
  QUIC drop policy (ADR-0009 stands; `udpDropped` keeps measuring);
  the manual Test-connection button's own negative-cache cascade (the
  same poison in miniature — left as documented scope, the sweep is
  the automatic path that produced the report).

## ADR-0018 — The first-byte budget: a processed host's silent origin
fails the flow fast, and the suspect answer set is demoted (M8-s, the
debug.11 report round)

- **Status:** Accepted 2026-10-18/19 (M8-s — the SECOND pasted
  diagnostics report; Ali's "the problem persists, but when after a
  while, video plays" round).
- **Context:** Ali's debug.11 report (versionCode 10) first PROVED the
  M8-r fixes on-device: state on (not degraded), a heal sweep that
  found a working method (strong) 3 s in, no false "Couldn't fix it",
  the startup DoH blip still logging its three
  `dns server failed server=#N reason=DNS_UNREACHABLE` lines (the
  baked evidence working; ~170 ms each, self-healed — the M8-r
  sharpened question answered: a transient at Start, not a
  persistent bootstrap filter). The stall (row 3) re-observed with a
  SHARPER signature: at each video start the app opens waves of
  parallel tunnels; within one wave some flows complete (first-byte
  56–619 ms, KBs down) while others never receive one byte at all —
  some close in ~0.5 s with `down=0 B` (the origin reset right after
  the fragmented hello), and the rest hang until EXACTLY ~60.0–60.1 s
  with `down=0 B`. Read back against the code: NOTHING in the engine
  ends a flow at 60 s (IdleTimeout is 300 s; the TUN keepalive idle is
  60 s but it only probes and the phone kernel answers; every other
  candidate is 5/6/8/10/12/30/35/120 s) — the ~60 s closer is the
  app's own patience, and the engine AMPLIFIES it by holding the
  silent flow open the whole time. Worse: because the dial SUCCEEDED,
  the M8-o Invalidate loop (dial-failure feedback) never fires — the
  suspect answer set stays fresh for its 120 s+ TTL and every retry
  wave can keep landing on the same dead node. Ali's discriminators
  (smooth after start, instant rewind/next, PC instant on the same
  network) localize the defect to the first-byte phase of fresh
  routed flows to specific cache nodes — the same per-node
  interference the M8-r telemetry named, now with its exact shape.
- **Decision:** A first-byte budget on processed (routed) tunnels
  only — the same species as M8-n's handshake-budget fix and M8-o's
  demotion (failure detection and answer-set hygiene, NOT steering):
  after the first client record is written upstream, the origin's
  first byte must arrive within `FirstByteTimeout` (8 s, >12x the
  worst healthy first byte observed in either report; the constant
  lives in the CONFIGURATION block). On expiry — or a reset/EOF with
  zero bytes — the flow fails FAST: both ends close immediately (the
  client sees a reset and retries, exactly as it already retries the
  ~0.5 s reset waves ~1 s later), the error is counted, a
  `tunnel first-byte failed reason=timeout|reset|closed` line lands
  in the log ring with the reason baked into the MESSAGE (the M8-r
  lesson — the ring drops attrs), and the host's cached answer set is
  demoted exactly like a total dial failure: a set that connects but
  never speaks is suspect on this network, so the next lookup
  re-queries the servers instead of serving the same set until TTL
  expiry. Unrouted relays (non-list hosts, .ir, plain-http forwards)
  keep the plain two-goroutine copy shape byte-for-byte — the budget
  is part of the app's processing of the hosts it processes (the D46
  scoping), not of the relay. The value 8 s is a measurement-derived
  default: it turns a silent flow's cost from the app's ~60 s into
  8 s while never approaching a healthy origin's round trip.
- **Evidence:** `TestFirstByteTimeoutFailsSilentRoutedTunnelFast` and
  `TestFirstByteFailureInvalidatesSuspectSet` (new,
  internal/proxy/firstbyte_test.go) both verified BOTH WAYS: with the
  tunnel.go restructure stashed (the pre-M8-s relay), the silent
  processed flow hangs to the idle timeout and FAILS the tests at
  ~5.0 s (the report's 60 s signature compressed); with the fix, the
  flow closes in ~250 ms, the reason line and the done line
  (`down=0 B`) land, and the demotion is recorded.
  `TestFirstByteBudgetSparesUnroutedHosts` and
  `TestFirstByteWithinBudgetFlowsNormally` pin the no-regression
  guards (they pass both ways, as guards must). Full baseline green:
  go vet clean; 18/18 packages ×2; GOOS=windows exit 0; the Android
  compile matrix 4/4 ABIs + arm64 full tree; the full
  build-android.sh lane with the embedded engine verified in the AAR
  ("tunnel first-byte failed" present in libgojni.so); the debug.12
  APK artifact-verified (badging versionCode 11, apksigner pinned
  cert, sha256 in doc 06).
- **What this ADR does NOT decide:** the ORIGIN of the per-node
  silence (why specific rr*.googlevideo.com cache nodes reset or
  blackhole the fragmented hello while others answer — ISP-side, not
  observable from the app; no third guess); any METHOD steering —
  in particular whether a dead routed flow should be retried once
  with the NEXT preset in HealSweepOrder (per-flow method variation):
  the sweep picks the preset from its redirector.googlevideo.com
  probe, but the media flows hit rr*.googlevideo.com nodes whose
  tolerance differs per node — the report shows method strong while
  Ali's PC is instant (possibly a different preset on the same
  network). That is a POLICY change and stays Ali's ruling (D102
  material, surfaced in doc 10 row 3); the QUIC drop policy
  (ADR-0009 stands; udpDropped 82 and climbing keeps measuring); the
  budget's value beyond this round (the next report's
  `dur≈8000ms down=0 B` + reason lines are the tuning evidence).

## Decision-change procedure

When changing an accepted decision: new evidence, alternatives
reconsidered, affected files/requirements/tests, user-visible impact;
update this file and the pack README before implementing dependent
work. Gated selections (ADR-0008) switch on their listed criteria with
a measurement note, without a full re-litigation.
