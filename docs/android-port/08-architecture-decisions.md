# 08 — Architecture Decision Records

**Status:** Phase 1 complete. ADR-0001/0002/0006/0007 were accepted as
constraints and are now backed by audit evidence; ADR-0003/0004/0005
moved from Proposed to **Accepted** on 2026-10-16 with the evidence
below; ADR-0008…0012 record the new decisions this phase produced;
ADR-AUTH-0001 (2026-10-18) opened the auth lane from D94 ruling 1 and
was **RETIRED the same day by D97** (the gate cancelled before
implementation — kept below as history). ADR-0013…0027 record the
debug-circle rounds M8-n…M8-z (the reliability ladder through Ali's
pasted reports); ADR-0028…0030 record the debug.19 round (M8-aa, the
dead-origin-pool fixes the first debug.18 report measured);
ADR-0031…0033 record the debug.20 round (M8-ab, the escape-the-pool
fixes the first debug.19 report measured); ADR-0034…0036 record the
debug.21 round (M8-ac, the hostile-network fixes the first debug.20
report measured); ADR-0037…0039 record the debug.22 round (M8-ad, the
long-lived-connection fixes the post-debug.21 report named — the
mid-video wedge and the legacy hello the tester pointed back at);
ADR-0040…0042 record the debug.23 round (M8-ae, the wire-discipline
round); ADR-0043…0045 record the debug.24 round (M8-af, the
prove-before-change round: per-flow kernel-TCP and TLS-record
evidence, the experiment switches and the one-button network test,
the breaker sensor fix and the media-evidence-only substitution
pool — the external engineer's debug.24 spec executed end to end).
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

## ADR-0019 — The salvage re-dial: a processed host's dead first-byte
origin is re-dialed INSIDE the same flow (same host, same preset), the
client's buffered bytes replayed (M8-t, the debug.12 report round);
FirstByteTimeout retuned 8 s → 2.5 s from that report's tuning evidence

- **Status:** Accepted 2026-10-18/19 (M8-t — the THIRD pasted
  diagnostics report; Ali's "The problem persists" round, versionCode
  11).
- **Context:** The debug.12 report PROVED ADR-0018 on-device exactly as
  designed — `tunnel first-byte failed reason=timeout` dones at
  ~8.3 s (22 in the window: the budget firing on silent origins) plus
  `reason=reset` at ~0.5 s (18: the origin/path resetting right after
  the fragmented hello lands, at the same latency a healthy answer
  arrives) — and it carried the tuning evidence the ADR asked for:
  every budget-killed flow was 100 % silent (`down=0 B`; the same
  flows sat silent through the app's full 60 s patience in the previous
  report — dead nodes, not slow ones), while every healthy routed
  origin answered in 420–508 ms (one 2399 ms outlier that also carries
  the ~0.1–0.2 s fragment-pause write before the budget clock starts).
  What REMAINED is the user-visible per-video start lag: roughly half
  the video-start waves die at first byte (counters: 38 errors / 61
  conns), and the app re-opens wave after wave until one lands —
  observed ~35 s from wave one to the first video bytes (down=4.4 MB
  once a flow survived). The serial chain of 8.3 s budget deaths IS
  the lag; the budget made each link cheaper (8 s vs 60 s) but the
  chain itself is the app's, not the engine's.
- **Decision:** Two changes, both the M8-n/o/s species (failure
  detection, answer-set hygiene, bounded recovery — NOT steering):
  1. **Retune** `FirstByteTimeout` 8 s → 2.5 s (CONFIGURATION block):
     ~5x the worst healthy routed wait, still covering the 2399 ms
     outlier, cutting each silent attempt's cost 3.3x.
  2. **Salvage re-dial:** when a processed host's origin dies at first
     byte (reset/EOF/budget timeout, zero bytes down), the engine
     closes ONLY the origin — never the client side — demotes the
     answer set (the M8-o loop, per dead attempt), re-resolves
     (excluding the IPs whose origins already died this flow; a
     single-answer host falls back to the full set), re-dials the SAME
     host with the SAME preset (captured once per flow, so even a
     mid-flight method change cannot vary a flow's wire shape), and
     replays the buffered client bytes through a fresh fragment conn —
     the first record re-fragmented identically (the fragment conn
     splits its first Write; the replay writes record-then-extras),
     everything after byte-exact. Up to `FirstByteSalvageDials` re-dials
     (default 2 → 3 total attempts, bounded), the client never sees a
     failure: its serial retry chain collapses into in-engine attempts
     of ~0.5–2.5 s each. When the attempts run out, or a re-dial cannot
     connect, the flow fails FAST exactly as at M8-s (both ends closed,
     error counted, `tunnel first-byte failed reason=…`, done line,
     demotion). Mechanically: the budgeted shape owns the first
     record's write (it must replay it); a single client pump reads the
     client for the whole first-byte phase, teeing into a 64 KB replay
     buffer (a truncated buffer never replays — a partial replay would
     corrupt the fresh origin's stream) and writing LIVE to the current
     origin through a mutex-guarded swap-able link (a chatty client —
     TLS early data — is never blocked behind the budget, exactly like
     the old up-copy; the swap closes the dead origin under the pump so
     a racing write fails fast with its bytes safe in the buffer). Once
     the origin speaks, the pump is stopped (its reads all delivered)
     and the plain two-goroutine relay takes over byte-for-byte.
     tunnelCloser learns setB so Stop's force-close sweep hits the
     live origin. New report lines, all reason-in-message (the M8-r
     lesson): `tunnel first-byte retry reason=… attempt=N` (Warn,
     absorbed attempts) and `tunnel first-byte salvaged dials=N`
     (Info, a flow the salvage saved); the terminal line is unchanged.
     Unrouted relays never enter the shape (D46).
- **Evidence:** internal/proxy/firstbyte_test.go —
  `TestFirstByteSalvageRecoversDeadOrigin` (the first origin silent,
  the flow delivers the echo and stays up; the client NEVER sees a
  close; retry + salvaged lines logged; exactly 2 dials),
  `TestFirstByteSalvageExhaustsToFailFast` (all origins silent: exactly
  1+2 dials, then the M8-s terminal signature — reason line, done
  `down=0 B`, one demotion per dead attempt), and
  `TestSalvageReplayIsByteExact` (the fresh origin receives the client
  record plus its early data byte-exact) all verified BOTH WAYS: with
  the tunnel.go salvage stashed (the M8-s shape at HEAD), each FAILS —
  the client sees the budget close, no retry line, no echo — and with
  the fix all PASS. The guards
  (`TestFirstByteSalvageSparesUnroutedHosts`,
  `TestFirstByteSalvageDisabledByConfig` — the negative knob pins
  M8-s exactly) pass both ways, as guards must. The two M8-s tests now
  set `FirstByteSalvageDials = -1` (they pin the TERMINAL shape).
  Full baseline green: go vet clean; 18/18 packages ×2; GOOS=windows
  exit 0; the Android compile matrix 4/4 ABIs + arm64 full tree; the
  full build-android.sh lane with the embedded engine verified in the
  AAR; the debug.13 APK artifact-verified (badging versionCode 12,
  apksigner pinned cert, sha256 in doc 06).
- **What this ADR does NOT decide:** per-flow METHOD variation — the
  salvage never changes the preset (it is captured once per flow);
  whether a dead routed flow should be retried once with the NEXT
  preset in HealSweepOrder remains D102, Ali's ruling (surfaced again
  in doc 10 row 3 with this round's sharper evidence: the salvage
  makes flows survive per-node intolerance, but a systematically
  intolerant network for one preset is still a method-policy
  question). The ORIGIN of the per-node silence stays unobservable
  from the app (no third guess). The QUIC drop policy stands
  (ADR-0009; udpDropped=88 this round keeps measuring). The budget's
  value beyond 2.5 s of evidence: the next report's retry/salvaged
  line ratios are the tuning material.

## ADR-0020 — The salvage race + the stall watch: the remaining salvage
re-dials run in PARALLEL (first origin to speak wins), and a processed
host's relay that goes silent in BOTH directions for StallTimeout is
closed engine-side (M8-v, the debug.13 report round)

- **Status:** Accepted 2026-10-19 (M8-v — the FIFTH pasted diagnostics
  report; Ali's "The problem persists" round, versionCode 12 — the
  first report from the salvage build itself).
- **Context:** The debug.13 report (shell line `versionCode 12`) proved
  the salvage on-device exactly as designed — `tunnel first-byte
  retry reason=… attempt=N` lines absorbing dead attempts the app never
  saw, `tunnel first-byte salvaged dials=N` lines saving flows at
  0.7-1.4 s (errors down to 37/81 ≈ 46 % from 67/114 ≈ 59 %) — and it
  named the two remaining start-lag sources with new precision:
  (1) the SERIAL salvage chain's cost — 21 terminal
  `reason=timeout` dones at 8014-8227 ms = three 2.5 s budgets plus
  dials walked one after another, plus 11 terminal `reason=reset`
  dones at 0.6-1.9 s; every terminal flow is one app retry wave away
  from a fresh origin; (2) the mid-stream stall — five video-start
  flows (done in the same instant at 00:18:17.2) lived 57290-64620 ms
  with up=2.4-5.0 KB and down=6.6-7.7 KB (one 121.1 KB): first byte
  plus ~7 KB down, then TOTAL silence until the APP's own ~60 s
  patience killed them all — and the retry wave right after answered
  in 60-261 ms first-byte. Nothing before M8-v looked past the first
  byte: the budget owns the handshake, the salvage stops at the first
  byte, and the idle timeout is 300 s, so the engine held the dead
  flows open through the whole minute. The video start this report
  measured: ~65 s from the first wave to playing.
- **Decision (two engine changes, both scoped to processed hosts —
  the D46 scoping, unrouted relays keep the plain shape
  byte-for-byte):**
  1. **The salvage race.** When attempt 0 dies at first byte, the
     engine no longer walks the remaining `FirstByteSalvageDials`
     re-dials one budget at a time — it RACES them: it re-resolves,
     excludes the flow's dead IPs, dedupes, and dials every remaining
     candidate at once (candidate k starts at the k-th rotated entry
     of the live set, and each candidate's DialHost still walks its
     whole rotated list with the M8-n stagger, so a blackholed address
     fails over inside the candidate), replays the buffered client
     bytes byte-exact through each, and waits one budget per candidate
     for the first byte. The FIRST origin to speak wins: its first
     bytes are delivered, `tunnel first-byte salvaged dials=N` logs
     the save (N = the re-dials launched), the losers are closed and
     reaped in the background (a late winner's conn closed, a late
     death still demoted — one demotion per dead attempt, the M8-o
     loop preserved). All dead: the M8-s terminal exactly, with the
     last death's reason. The retry lines keep their format and their
     count semantics (one `attempt=N` line per launched candidate).
     Wall time: a silent-host terminal costs ~2x the budget (attempt
     0 + the race), never the serial chain's 3x; a save lands at the
     fastest live origin's answer, not after every slower predecessor
     died. The total attempt count is UNCHANGED (1 + 2 default), so
     the same-IP transient-reset recovery the serial chain had on
     single-answer hosts is preserved (round-robin repeats the list);
     the dial budget per candidate stays DialTimeoutOverall. The
     client's bytes during the race wait in the socket buffer (the
     pump is stopped) and flow to the winner in order, right after
     the replay it already holds.
  2. **The stall watch.** New knob `StallTimeout` (default 15 s; 0 in
     a proxy Config → the default; negative → disabled, the M8-u
     behavior exactly). The budgeted flow's relay phase runs under a
     watchdog: each direction's read side stamps an activity clock
     (the copy loops' only activity signal; the idle ends keep the
     plain IdleTimeout), and when BOTH clocks are older than
     StallTimeout the flow is closed engine-side — both ends (the
     app's read errors NOW, so its retry lands on a fresh origin with
     its own budget + salvage in seconds, not a minute), the answer
     set demoted (the M8-o loop extends to mid-stream stalls), the
     error counted, and a new report line
     `tunnel stall killed idle=Nms` (Warn, reason-in-message). One
     direction flowing always refreshes the watch: a pure download
     (client silent, origin streaming) and a pure upload (origin
     silent, client streaming) are healthy relay shapes and are never
     killed — only a whole-flow stall is dead. The false-positive
     case is a keepalive idle > StallTimeout: the kill costs the app
     one reconnect (~hundreds of ms, invisible), the same death it
     already accepts from NATs; the true-positive case buys back
     ~45 s of the ~60 s the report measured. The watch is a
     per-budgeted-flow goroutine with a ticker (min
     StallTimeout/4, 50-500 ms) over atomic stamps — no lock shared
     with the copy loops, stopped when the relay ends.
- **Evidence:** internal/proxy/stallrace_test.go (new) +
  firstbyte_test.go updated to the raced shape, all verified BOTH
  WAYS: with tunnel.go stashed (the M8-t serial shape at HEAD),
  `TestStallKillClosesSilentRelayFlow` FAILS (the flow hangs to the
  idle timeout, no stall line),
  `TestSalvageRaceRedialsInParallel` FAILS (the two re-dials land a
  full budget apart — three loopback origins, accept-timestamped),
  `TestSalvageRaceTerminalIsBounded` FAILS (the close arrives at
  3x the budget, not ~2x), and the updated
  `TestFirstByteSalvageRecoversDeadOrigin` FAILS (one retry line,
  dials=1, 2 accepts — the serial shape); with the fix all PASS. The
  guards pass both ways, as guards must:
  `TestStallKillSparesStreamingDownload` (a pure download streams
  across many stall budgets and is never killed — this guard caught
  an inverted-stamp bug during the round: the watch must measure
  silence from the NEWEST activity clock, not the oldest, or a
  download whose client never writes dies mid-stream),
  `TestStallKillDisabledByConfig` (negative → the M8-u behavior),
  `TestStallKillSparesUnroutedHosts` (D46),
  `TestSalvageRaceWinnerDeliversByteExact` (the raced winner delivers
  record + early data byte-exact and the flow stays a working
  relay), and the untouched M8-s/M8-t pins
  (`TestFirstByteSalvageExhaustsToFailFast` still 3 dials / 3
  demotions / the terminal signature; `TestSalvageReplayIsByteExact`
  unchanged). Full baseline green: go vet clean; 18/18 packages ×2;
  GOOS=windows exit 0; the compile matrix 4/4 ABIs + arm64 full
  tree; gomobile bind with the new strings verified in libgojni.so
  in all three ABIs; the debug.14 APK artifact-verified (badging
  versionCode 13, apksigner pinned cert, sha256 in doc 06).
- **What this ADR does NOT decide:** per-flow METHOD variation — the
  race re-flips the SAME coin on the SAME preset (captured once per
  flow); whether a flow that exhausts its raced salvages should be
  retried once with the NEXT preset in HealSweepOrder stays D102,
  Ali's ruling, and this report's evidence makes it LIVE: terminal
  first-byte failures still cluster under method=strong (21 timeouts
  + 11 resets, ~40 % of conns). The ORIGIN of the per-node
  silence/reset stays unobservable from the app (no third guess).
  The stall budget's value beyond 15 s of reasoning: the next
  report's `tunnel stall killed idle=…` lines are the tuning material
  (too many kills on flows that were fine → raise; none while starts
  still stall → the stall is elsewhere). The QUIC drop policy stands
  (ADR-0009; udpDropped=78 this round keeps measuring). The idle
  timeout is untouched (300 s for every relay; the stall watch only
  shortens the BOTH-silent case for processed hosts).

## ADR-0021 — The first-dial race: the whole dial budget is spent in
PARALLEL from the flow's start (the primary plus FirstByteRaceDials
extra candidates, first origin to speak wins), and the health probe
asks the servers NOW (ClearNegative before the dns step) (M8-w, the
debug.14 report round)

- **Status:** Accepted 2026-10-19 (M8-w — the SIXTH pasted diagnostics
  report; Ali's "The problem persisted" round, versionCode 13 — the
  first report from the salvage-race + stall-watch build itself).
- **Context:** The debug.14 report (shell line `versionCode 13`) proved
  ADR-0020 on-device: the `tunnel first-byte retry` lines batch at the
  same millisecond (the re-dials race), the stall watch kills at 15 s
  (`tunnel stall killed idle=15xxx ms` lines, exactly the budget),
  saves land at 738-848 ms, and 60-second mid-stream hangs are gone.
  It also named the two remaining costs with new precision:
  (1) the SERIAL FRONT — every routed flow still walked attempt 0
  ALONE through its full 2.5 s budget before the salvage race even
  launched: six terminal `reason=timeout` dones at 5320-5576 ms (the
  two-budget chain), 1131-1292 ms reset terminals, and the mixed
  windows recovering only at salvage time — while CONCURRENT flows to
  the SAME hosts answered at 72-162 ms in the same seconds (five
  sub-250 ms first-bytes at 01:09:49-50 right between two timeout
  terminals). The dirt is per-origin, not per-moment: a wave of
  sibling flows covers the live set while each individual flow still
  serializes its own first attempt. (2) The DNS degraded state — dns
  fail 16 with `last failed check: dns (DNS_UNREACHABLE)` while the
  network was demonstrably flowing: a dirty-window DoH blip
  negative-cached www.youtube.com (the very name the app's own flows
  resolve), and the periodic check read the 15 s negative memory
  instead of asking any server — the M8-r lesson (the sweep's probe
  poisoned by its own negative cache) recurring on the watchdog path,
  where nothing clears it.
- **Decision (two changes; the first scoped to processed hosts — the
  D46 scoping, unrouted relays keep the plain shape byte-for-byte; the
  second in the health package):**
  1. **The first-dial race.** New knob `FirstByteRaceDials` (default
     2; 0 in a proxy Config → the default; negative → disabled, the
     exact M8-v shape: serial attempt 0 + the raced salvage
     re-dials). A processed host's budgeted flow — when racing is
     enabled and the replay fits — runs ONE raced round INSTEAD of
     the serial loop: the primary origin (already dialed by
     handleTunnel; its dial time is the head start) plus
     FirstByteRaceDials extra candidates dial at once (candidate k
     starts at the k-th rotated entry of the fresh answer set, and
     each candidate's DialHost still walks its whole rotated list
     with the M8-n stagger, so a blackholed address fails over inside
     the candidate), each replays the client's first record
     byte-exact through a fresh fragment conn (the same preset,
     captured once per flow — D102), and each waits one budget for
     the first byte. The FIRST origin to speak wins: its first bytes
     are delivered, the losers are closed and reaped, and a flow the
     race SAVED (the winner is not the primary) logs the new report
     line `tunnel first-byte raced dials=N` (Info; a clean primary
     stays quiet — nothing happened the pre-race shape would
     report). All dead: the M8-s terminal exactly, at ~1 budget
     (the round runs in parallel) instead of the M8-v serial-front
     2x. The client stays FROZEN for the round — the M8-v salvage
     race's own proven semantics: the pump never starts on this
     path, bytes beyond the first record wait in the socket buffer
     and flow to the winner in order through the plain relay. A
     candidate's death is classified before it demotes (the M8-o
     loop): a teardown-caused death — the dial ctx aborted by the
     round's end, or a conn closed under the candidate (the
     swap-closed primary) — never demotes; a death at the
     candidate's OWN budget or an origin reset demotes whenever it
     happens (the origin was silent the whole budget — exactly the
     M8-o evidence, even after a sibling won). The first record is
     counted once (the winner's copy is the flow's — the client's
     own send, not per-candidate), matching the salvage's
     accounting. With racing enabled the salvage rounds never run
     (the budget is spent at the start); `FirstByteSalvageDials`
     keeps its exact meaning when racing is disabled.
  2. **The honest dns probe.** `Resolver.ClearNegative(host)` (new,
     on the doh resolver): drops exactly host's negative-cache entry
     — success entries are untouched (a good cached answer is still
     good; demoting it would only tax the tunnel path with a
     re-query). The health package's dns step calls it on the probe
     name before looking up (optional capability; fakes without it
     are skipped): a health probe's question is "what do the servers
     say NOW", not "what failed moments ago". The watchdog and the
     quick check both run testDNS, so both probe past the memory;
     the heal sweep keeps its full Reset (M8-r — a sweep is a
     suspected-network-change event).
- **Evidence:** internal/proxy/firstrace_test.go (new, six tests) +
  internal/doh/resolver_test.go + internal/health/selftest_test.go
  (one new test each), all verified BOTH WAYS: with the raced round
  neutered (the M8-v shape at HEAD), `TestFirstDialRaceSavesDirtyPrimary`
  FAILS (no raced line — the salvage's retry+salvaged lines instead,
  a full budget later) and `TestFirstDialRaceTerminalIsBounded` FAILS
  (the flow still up at 550 ms — the serial-then-race 2x shape); with
  the ClearNegative call neutered,
  `TestSelfTestDNSProbesPastNegativeCache` FAILS (the dns step reads
  the memory: DNS_UNREACHABLE in 0 ms with the server healthy — the
  report's exact signature); with the fixes all PASS. The guards pass
  both ways, as guards must:
  `TestFirstDialRaceWinnerDeliversByteExact` (the raced winner
  delivers record + early data byte-exact and the flow stays a
  working relay), `TestFirstDialRacePrimaryWinsQuietly` (a clean
  primary wins against slower-but-live extras and NOTHING logs —
  the race costs a healthy flow nothing observable),
  `TestFirstDialRaceSparesUnroutedHosts` (D46),
  `TestFirstDialRaceDisabledByConfig` (negative → the M8-v shape:
  serial attempt 0, the raced salvage re-dials, their retry/salvaged
  lines, never the raced line). The M8-v/M8-t/M8-s pins keep their
  shapes with racing disabled in their configs
  (stallrace_test.go + firstbyte_test.go — the raced round is
  firstrace_test.go's). Full baseline green: go vet clean; 18/18
  packages; the compile matrix 4/4 ABIs + arm64 full tree; gomobile
  bind with the raced string verified in libgojni.so in all three
  ABIs; the debug.15 APK artifact-verified (badging versionCode 14,
  apksigner pinned cert, sha256 in doc 06).
- **What this ADR does NOT decide:** per-flow METHOD variation — the
  race re-flips the SAME coin on the SAME preset on every candidate
  (captured once per flow); whether a flow that exhausts its raced
  round should be retried once with the NEXT preset in
  HealSweepOrder stays D102, Ali's ruling, and this report's evidence
  keeps it LIVE (terminal first-byte failures still cluster under
  method=strong — six timeouts + a reset batch inside four seconds at
  01:09:47-51). The raced round's candidate count (2) and the
  budget (2.5 s) are the next report's tuning material: the raced
  line's dials=N plus first-byte times say whether the count should
  grow (an all-dead set still terminaling) or the extras cost more
  than they save (every raced line a save the primary would have
  made anyway). The ORIGIN of the per-node silence/reset stays
  unobservable from the app. The QUIC drop policy stands (ADR-0009;
  udpDropped=104 this round keeps measuring). The stall watch is
  untouched (ADR-0020; this report's kills at 15.0-15.3 s idle are
  the budget firing as designed).

## ADR-0022 — The tiered stall watch: a flow that proved itself (payload
delivered, or one idle gap survived and closed by fresh activity) is
closed only at StallRichTimeout=90 s and QUIETLY (no demotion, no error
count); the lean tier keeps the exact M8-v semantics for never-proved
flows (M8-x, the debug.14 build's second report round)

- **Status:** Accepted 2026-10-19 (M8-x — Ali's SEVENTH pasted
  diagnostics report; the shell line still reads versionCode 13 — the
  debug.14 build: the device never ran debug.15).
- **Context:** Ali's own words named the M8-v watch's regression:
  "videos used to have lag for starting to play, but after that you
  could easily go next (watch next few minutes) instantly without
  buffer, but now even going next and back in the same video lags a
  lot." The report's log is the smoking gun: thirteen
  `tunnel stall killed idle=15xxx ms` lines in ~70 s of log, every one
  a connection that had ALREADY carried real data — dur=17264 ms
  down=2.1 MB, dur=16756 ms down=1.6 MB, dur=39241 ms down=723.6 KB,
  dur=32268 ms down=633.7 KB, dur=21739 ms down=162.6 KB, plus
  20.9/23.4/9.2/7.4/5.3/4.4/3.6/2.8 KB flows — murdered at exactly the
  lean 15 s budget mid-pacing. YouTube's player POOLS its video
  connections: buffer a stretch, go quiet (often past 15 s), then ask
  for the next range ON THE SAME CONNECTION — the exact reuse that
  made "next" instant in the pre-watch builds. Killing the pooled
  connections forced every next/seek through a fresh dial, and the
  same report shows what a fresh dial costs on that network: ~50 %
  first-byte failure (reason=reset terminals at ~1.2 s, reason=timeout
  at ~5.4 s; errors 70/122 ≈ 57 %). Worse, every kill DEMOTED a
  proven-good answer set (the M8-o loop extended to stalls by M8-v)
  and counted an error — the watch was poisoning the very origins
  that had just delivered megabytes. The M8-v dead cohort (the shape
  the watch was built for: first byte plus ~7 KB, then silence to the
  app's 60 s patience) and the murdered cohort OVERLAP in bytes
  (a 7.4 KB flow killed here; 6.6-7.7 KB flows dead there) — no byte
  threshold alone separates them; the two shapes differ in RHYTHM:
  the dead cohort never woke, the pooled cohort paces.
- **Decision (the stall watch grows a second tier; scoped exactly as
  M8-v — processed hosts only, the D46 scoping; unrouted relays
  untouched):**
  1. **The escalation.** A flow PROVES itself in either of two ways:
     payload — `flow.down >= StallRichBytes` (default 32 KB: past a
     TLS handshake plus response headers and into real payload; the
     murdered cohort started at 162.6 KB, the dead cohort topped out
     at ~7 KB with one 121 KB outlier) — or pacing — the watch
     observes an activity gap ≥ StallTimeout/5 (3 s at the default)
     that CLOSED (fresh bytes flowed after it): the flow has woken
     once, the pooled-connection rhythm. A proved flow's stall budget
     escalates to `StallRichTimeout` (default 90 s: well past every
     real pacing gap at playback, well under the 300 s idle timeout —
     the zombie stays bounded).
  2. **The quiet close.** When the rich budget finally fires, the
     close is HYGIENE, not a dead-origin signal: both ends closed (as
     before) but NO answer-set demotion (an origin that just carried
     megabytes is the opposite of suspect) and NO error count (the
     report's error ratio stops counting pool evictions). The lean
     tier keeps the exact M8-v semantics: demotion + error count +
     the fast 15 s close for the never-proved shape.
  3. **The line.** `tunnel stall killed idle=Nms down=N tier=rich|lean`
     — the prefix stable, the tier and the flow's down-bytes appended:
     the next report's primary tuning evidence (kill counts by tier
     say whether 32 KB / 90 s / the wake gap are set right).
  4. **Knobs.** `StallRichTimeout` (0 in a proxy Config → the
     default; negative or ≤ StallTimeout → no escalation, every flow
     keeps the exact M8-v semantics) and `StallRichBytes` (0 → the
     default; negative → never escalate on bytes — only a survived
     gap escalates). Both wired through proxy Config normalization
     and the engine, like every M8 knob.
- **Evidence:** internal/proxy/stalltier_test.go (new, four tests)
  verified BOTH WAYS: with the tiering stashed (the M8-v watch at
  HEAD), `TestStallRichFlowSurvivesLeanBudget` FAILS (the payload
  flow closed at the 400 ms lean budget — the report's regression
  reproduced in test) and `TestStallWakeEscalatesBudget` FAILS (the
  pacing flow closed at the lean budget right after its second
  flight); `TestStallLeanKillStaysLoud` and
  `TestStallRichEscalationDisabledByConfig` pin the unchanged lean
  behavior (their pre-fix failures are the new tier= line format
  only — the close/demote/count behavior is identical both ways);
  with the fix all four PASS alongside the four untouched M8-v pins
  (stallrace_test.go). Full baseline green: vet clean; 18/18 packages
  ×2; Windows exit 0; the compile matrix 4/4 ABIs + arm64 full tree;
  gomobile bind with the tier= string verified in libgojni.so in all
  three ABIs; the debug.16 APK artifact-verified (badging versionCode
  15, apksigner pinned cert, sha256 in doc 06).
- **What this ADR does NOT decide:** the stall watch stays scoped to
  processed hosts (D46); QUIC stays dropped (ADR-0009; udpDropped=73
  this round keeps measuring). The report never ran debug.15, so the
  first-dial race (ADR-0021) and the honest dns probe remain
  UNVERIFIED on-device — this build carries them unchanged and the
  next report must carry BOTH signatures (raced lines + tier lines).
  The method flip in the report (method: balanced where the sixth
  report said auto→strong) is the heal sweep settling on the first
  passing method after DNS false-failures fired it — ADR-0021's
  ClearNegative attacks the cause; the method policy itself stays
  D102, Ali's. The rich tier's numbers (32 KB / 90 s / the wake gap
  at StallTimeout/5) are the next report's tuning material.

## ADR-0023 — The DNS negative re-ask and the origin-IP penalty box: a
DNS_UNREACHABLE negative re-asks the servers (rate-limited, singleflight-
collapsed) instead of locking a host out for the TTL; a classified
first-byte death marks its origin IP for DeadIPPenalty=60 s and every dial
list skips the marked IPs (a preference, never a verdict) (M8-y, the
debug.16 build's report round — the FIRST report from the right build)

- **Status:** Accepted 2026-10-19 (M8-y — Ali's EIGHTH pasted
  diagnostics report; the shell line finally reads versionCode 15, the
  debug.16 build, and it carries BOTH prior signatures: the raced line
  and the tier= stall line).
- **Context:** Ali's own words closed ADR-0022 and named what remains:
  "The original problem (videos taking time to play) persist, but the
  problem of easily going next and back in same video instantly is
  fixed." The report measures the start lag's two remaining sources:
  **(1) The DNS negative-cache lockout** — `dns ok 142, fail 50` with
  `doorFailures=50`: every resolver failure is a dead door dial (the
  flow dies at `socks5 dns lookup failed` → `tunstack: door dial
  failed`, and the app retries into the same wall). The failure lines
  PAIR at the same millisecond (four pairs inside 22 ms at
  12:52:18.680–.702, then .090/.094, .624/.630, .007/.011, .115/.118,
  .220/.223, .056/.061) — the signature of instant negative-cache
  hits, not server walks (a walk is 3 servers × up to 3 s; these land
  in 3–4 ms). One real DoH blip negative-caches a batch of fresh
  per-video hostnames (the `rr*.googlevideo.com` names are per-video:
  never resolved before, so no success entry exists to serve stale),
  and for the negative's whole 15 s TTL every retry wave dies at the
  door in milliseconds — the blip's tail became the 50 dead flows,
  with the interleaved `tunnel up dns=1413ms` successes being the
  negatives expiring and the re-asks landing. Also: the socks5 door's
  DNS failure line was BARE (the reason an attr the log ring drops —
  the M8-r lesson the HTTP door learned but the SOCKS door never did).
  **(2) The dead-origin re-gamble** — ~17 `reason=timeout` terminals
  at dur=2892–4304 ms plus ~8 `reason=reset` at ~0.6 s, all down=0 B
  through the raced round (both candidates dead). Each terminal
  demotes the answer set; the next flow's resolve re-queries DoH
  (0.5–1.8 s, itself failure-prone); the AUTHORITATIVE answer returns
  the same blackholed IPs; the race gambles again. The dead-IP
  knowledge was per-flow only (the `deadIPs` map in
  copyFirstByteBudgeted died with its flow, and the raced round
  recorded none), and each video's rr hostname is NEW — the second
  video's start (12:53:11–24) paid the full discovery cost again, five
  terminals deep before its first video bytes (dur=4450 ms,
  down=2.8 MB). The blackhole is per ORIGIN IP (cache nodes shared
  across rr hostnames), so the knowledge should outlive the flow and
  cross hostnames. (udpDropped=99 is the M8-j QUIC fast-fail doing its
  job — each drop answered with ICMP port-unreachable; no action.)
- **Decision (two fixes, both scoped to the measured defects):**
  1. **The negative re-ask (doh).** A negative-cache hit whose code is
     `DNS_UNREACHABLE` no longer fails instantly: that code is
     evidence about the RESOLVER'S OWN PATH (all servers unreachable /
     network error / bad status), not a verdict about the name. The
     hit re-asks the servers through the existing singleflight
     (concurrent flows of a retry wave collapse into one walk),
     rate-limited per host to one WALK per `DNSCacheNegativeReask`
     (default 1 s) — and the walk that WROTE the negative counts as
     the window's first probe, so every negative hit is either instant
     (the window is warm — the flow fails fast, the app retries) or a
     genuinely fresh walk, never a redundant one seconds after a
     failure. On failure the negative re-caches (fresh 15 s) and the
     flow gets the real error; on success the answer overwrites the
     negative and the flow proceeds. Server-verdict negatives
     (`DNS_BLOCKED_ANSWER`, `no usable answer` — the servers SPOKE:
     hijack/NXDOMAIN shapes) keep the instant fail: re-asking a
     verdict is pointless. One new log line, `dns negative re-ask
     (path failure, not a name verdict)` — its count in the next
     report IS the evidence of lockouts absorbed. Knob
     `DNSCacheNegativeReask` (0 in a doh Config → the default;
     negative → disabled, the exact M8-w shape).
  2. **The origin-IP penalty box (proxy).** A server-level dead-origin
     memory: every classified first-byte death — the origin connected
     (or reset at the hello write) and never spoke; a writeReplay
     failure or a read death, natural only (teardown-canceled deaths
     and dial failures carry no single-IP evidence) — marks its origin
     IP for `DeadIPPenalty` (default 60 s). Every origin dial list
     (the HTTP door, the SOCKS5 door, the raced round's extras, the
     salvage candidates, the plain-http forward) is filtered through
     the box before DialHost. The box is a preference, never a
     verdict: a list the filter would EMPTY dials in full (an
     all-penalized set gambles exactly as before, never an instant
     fail). NOT marked: stall kills (mid-stream deaths are not
     first-byte evidence — ADR-0022's tiering owns those),
     client-side deaths, teardown-canceled deaths, dial failures.
     Bounded (512 IPs, FIFO). Knob `DeadIPPenalty` (0 in a proxy
     Config → the default; negative → disabled, the exact M8-x shape).
  3. **The baked reason (socks5).** The door's DNS failure line
     carries `reason=` in the message — parity with the HTTP door's
     M8-r fix. The next report says WHY the door lookups fail instead
     of 50 bare lines.
- **Evidence:** internal/doh/reask_test.go (new, four tests) and
  internal/proxy/deadip_test.go (new, four tests), verified BOTH WAYS.
  With the re-ask neutered (the M8-w shape),
  `TestNegativeReaskRecoversAfterBlip` FAILS with the report's exact
  signature ("lookup failed recently: DNS_UNREACHABLE" — the lockout
  reproduced in test), `TestNegativeReaskRateLimited` FAILS (no second
  walk), and the rewritten `TestCacheNegativeTTL` FAILS at its
  recovery assertion (the old test pinned the harmful behavior itself:
  a healthy server still failing at +5 s); the guards
  (`TestNegativeReaskDisabledByConfig`,
  `TestNegativeServerVerdictsDontReask`) pass on both sides. With
  tunnel.go's marking stashed (the M8-x shape),
  `TestDeadIPPreferenceSparesTheNextFlow` FAILS (flow 2 dies on the
  corpse — EOF, the re-gamble reproduced) and
  `TestRacedDeathsMarkTheIP` FAILS (corpse accepts 5 ≠ 3 — flow 2's
  primary and an extra hit the corpse again); the guards
  (`TestDeadIPAllPenalizedStillDials`, `TestDeadIPPenaltyDisabledByConfig`)
  pass on both sides. Full baseline green: vet clean; 18/18 packages
  ×2; Windows exit 0; the compile matrix 4/4 ABIs (CGO+NDK) + arm64
  full tree; gomobile bind with the re-ask and reason strings verified
  in libgojni.so in all three ABIs; the debug.17 APK artifact-verified
  (badging versionCode 16, apksigner pinned cert, sha256 in doc 06).
- **What this ADR does NOT decide:** the DoH server walk stays
  serial-with-sticky (the report's slow-but-successful lookups —
  128–1806 ms — are tolerable once failures stop cascading; racing
  the servers triples DoH traffic for marginal gain — revisit only if
  the next report's re-ask lines show walks still dominating). The
  negative TTL stays 15 s (the re-ask makes it a floor between probes,
  not a lockout). QUIC stays dropped (ADR-0009; the M8-j ICMP
  fast-fail is pinned by test and measuring, udpDropped=99). The
  penalty box's 60 s and the re-ask's 1 s are the next report's tuning
  material.

## ADR-0024 — Fragment every routed app's TLS on Android
(FragmentAllRouted, platform-defaulted through settings v5) and the
plain-relay first-byte safety net: a not-budgeted TLS flow whose origin
never answers the ClientHello closes at 8 s and is counted, never
hanging to the app's patience (M8-z, the debug.18 build — work items A
and B of docs/specs/debug18-cold-start.md)

- **Status:** Accepted 2026-10-19 (M8-z — the debug.18 round; the spec
  is the debug.16 report's H1: the first video's 1–2 minute start,
  with the plain cohort — every host the YouTube app needs that is not
  on the 7-domain routed list — un-fragmented and un-budgeted, hanging
  silently until the app gives up at ~60 s).
- **Context:** Only `defaultYouTubeDomains`' 7 domains were fragmented
  and budgeted; every other host crossed `runBidirectionalCopy` →
  `copyPlain` with no fragmentation, no first-byte budget, no stall
  kill — only the 300 s idle timeout. A blocked ad/attestation host
  therefore hung exactly as long as the app's own patience, and the
  debug.16 report could not even SEE the cohort (no class tag, no
  counter, no host row).
- **Decision (two changes, one scope):**
  1. **FragmentAllRouted (work item A).** A new settings field
     (schema v5) fragments the first TLS record of EVERY non-direct-TLD
     host when on. The single `routedHost` decision splits in two:
     `fragmentHost` (not `.ir` AND (FragmentAllRouted OR the list
     match)) and `budgetedHost` (exactly yesterday's `routedHost`).
     The budgeted shape — first-byte budget, salvage, race, dead-IP
     penalty box, tiered stall kill — stays scoped to `budgetedHost`
     and to it ONLY: fragmentation for everything (cheap, safe), the
     expensive machinery where it pays. Platform default through
     `config.LoadFor(dir, platform)`: true on Android (every flow the
     per-app TUN hands the door belongs to a routed app — the allow
     list is the scope), false on every desktop (hard constraint 5:
     desktop behavior byte-for-byte). A pre-v5 file migrates once on
     Android (the absent field was the shipped default, never a
     choice); a v5 file is the owner's word either way.
  2. **The plain-relay safety net (work item B).** A not-budgeted TLS
     flow arms ONE deadline: the origin's first byte must arrive
     within `PlainFirstByteTimeout` (8 s) of the ClientHello going
     upstream, or both sides close and `plainFirstByteTimeouts`
     counts it — a TLS server always answers a ClientHello quickly,
     silence means dead. No dead-IP mark (the plain cohort never feeds
     the penalty box — the spec's rule), no connError (its own
     counter), non-TLS flows and post-first-byte silence keep the
     plain idle semantics. The guard is a read-side wrapper
     (`plainGuardConn`) that disarms on the first byte — and it
     FORWARDS CloseWrite: an embedded-interface method set hides the
     *net.TCPConn half-close the relay's closeWrite depends on, a
     defect the soak test caught the same day (every guarded flow hung
     to the idle timeout with the half-close lost; the fix is
     explicit in the type).
- **Evidence:** internal/proxy/fragmentall_test.go (new, five tests)
  and internal/proxy/plainnet_test.go (new, four tests), verified BOTH
  WAYS: with `fragmentHost` neutered to the M8-y shape,
  `TestFragmentAllRoutedFragmentsNonListedHost` FAILS (the record
  arrives in one read — the blocker's exact shape); with the guard's
  arming stashed, `TestPlainFirstByteTimeoutClosesSilentTLSOrigin`
  FAILS (the read hangs past the budget on the CLIENT deadline — the
  on-device hang reproduced in test) while the guards
  (answered-within-budget untouched, non-TLS keeps idle semantics,
  disabled knob pins the pre-M8-z shape) pass on both sides. Config:
  internal/config/settings_v5_test.go (new, five tests) with the
  migration stashed BOTH WAYS. Full suite green.
- **What this ADR does NOT decide:** the budgeted machinery's tuning
  (budgets, tiers, the penalty box) is untouched — ADR-0018 through
  ADR-0023 own it and their pins stand. `.ir` stays never-processed
  (D83) in BOTH fragmentHost branches, pinned by test. The desktop
  default stays off, pinned by test. FragmentAllRouted does not widen
  http→https redirects or PAC routing — those still follow
  `routedHost` (the redirect decision is unchanged).
- **How the next report verifies it:** the Top-10 hosts section's
  class=plain rows flip from silent hangs to either fast successes
  (fragmentation fixed them) or counted `plainFirstByteTimeouts` kills
  with the `tunnel plain first-byte failed class=plain` line — the
  cohort becomes visible, and the MARK timeline's elapsed time is the
  number that says whether the minute went away.

## ADR-0025 — DoH over one warm shared HTTP/2 connection with a hedged
server walk: http2.Transport + DialTLSContext over the fragmented
bootstrap dial, ReadIdleTimeout/PingTimeout health checks, WarmProbe at
engine start (re-warm within 1 s of a connection-shaped failure), the
sticky server asked at t=0 and the next at +300 ms (fast failures
trigger the next immediately), the two IP-literal no-SNI endpoints
appended to the default list, and the learned googlevideo sn-<code>
pre-resolution every 5 minutes (M8-z, work item C)

- **Status:** Accepted 2026-10-19 (M8-z; the spec's H2 — the DoH blips:
  cold lookups at 1055–1806 ms, one fresh fragmented TLS handshake per
  concurrent first query, serial 3 s-per-server walks).
- **Context:** `http.Transport` with `MaxIdleConnsPerHost: 2` raced a
  burst of fresh per-video names into separate handshakes, had no
  keep-alive pings, and `askServers` walked servers SERIALLY — a
  silent first server cost its whole 3 s timeout before the next was
  even asked, at the worst moment (the first video's start).
- **Decision:**
  1. **One shared connection.** `golang.org/x/net/http2.Transport`
     with a custom `DialTLSContext`: the TCP leg is the old
     `dialBootstrap` verbatim (bootstrap IPs, netx, the protect hook —
     ADR-0011 — and the fragment.Conn wrapper, so the DoH ClientHello
     stays fragmented), then this function owns the TLS handshake
     with ALPN h2 and ServerName from the server URL's host (Go omits
     SNI for IP literals — which the two new endpoints RELY on).
     http2 multiplexes every query over one connection per server
     address: a burst costs ONE handshake, by construction.
     ReadIdleTimeout 15 s / PingTimeout 5 s keep the connection
     health-checked while it lives.
  2. **Warm and re-warm.** `WarmProbe` opens the connection at engine
     start (goOn and goDegraded — degraded still serves traffic) and
     on the 5-minute prewarm cadence; a connection-phase failure
     schedules a background re-warm within 1 s, debounced to one per
     5 s (the hedge and the next query's own dial also cover the gap —
     the re-warm keeps the connection warm for the NEXT flow).
  3. **The hedged walk.** The sticky server at t=0, the next at
     +DoHHedgeDelay (300 ms), the third at +600 ms, and so on; a
     server that FAILS FAST triggers the next launch immediately; the
     first VALID answer wins and cancels the rest. The per-server 3 s
     timeout, the sticky update, the singleflight, the cache shapes,
     serve-stale, the negative cache, the M8-y negative re-ask and the
     two log lines stand unchanged (the failure line gains `phase=`).
     A hedge WIN is counted only when the winner was launched while an
     earlier server was still pending — the serial fallback after a
     settled failure is not one (pinned by test).
  4. **The IP-literal endpoints.** Cloudflare `https://1.1.1.1/…` and
     Google `https://8.8.8.8/…` appended at the END of the default
     list: dialed by IP, ClientHello with no SNI (nothing for SNI
     inspection to match), certificates carrying IP SANs. They are the
     walk's last resorts.
  5. **The googlevideo pre-resolution.** A resolved
     `rr<N>---sn-<code>.googlevideo.com` (or `r<N>---…`) remembers the
     `sn-<code>` (cap 64, FIFO); at start and every 5 minutes the
     engine resolves `rr1..rr5` of each remembered code in the
     background (≤3 in flight), so the next fresh per-video hostname
     often lands on a warm cache entry. The codes persist to
     `googlevideo-codes.txt` in the app's private config directory:
     checked against DECISIONS.md's privacy rules — R21 governs LOGS
     and the report (hostnames off by default), and the D-entries
     govern disclosures; the app's own private state already carries
     settings.json and the usage store, and an sn-<code> is an
     infrastructure node identifier (one code serves every rr variant
     of a location), not a browsing-history entry. The file is
     0600, capped, and never leaves the device; the report renders
     only its COUNT, never the codes.
  6. **Failure phases.** Dial failures carry `phase=tcp`, handshake
     failures `phase=tls` (path interference), status/content/parse
     failures `phase=status|http|parse` (server problem) — the failure
     line and the report's DNS detail section both carry it.
- **Evidence:** internal/doh/http2_test.go (new, seven tests),
  verified BOTH WAYS: with the transport stashed to the per-request
  `http.Transport` (the M8-y shape), `TestBurstSharesOneConnection`
  FAILS (10 dials for 10 concurrent names — the measured defect
  reproduced); with the hedge clock stashed,
  `TestHedgeSecondServerWinsWhenFirstStalls` FAILS (the answer costs
  the first server's full timeout, the serial shape).
  `TestSerialWalkWhenHedgingDisabled` pins the negative knob's exact
  serial shape on both sides. The existing doh suite (reask, cache,
  fallback, singleflight, bootstrap failover) passes unchanged — the
  fake servers now negotiate h2 (httptest EnableHTTP2), the one
  necessary harness change. Race-detector run on internal/doh: clean.
- **What this ADR does NOT decide:** the servers' ORDER beyond the two
  appended endpoints (the sticky logic owns runtime order); no DoH
  query is ever sent over plaintext (D74 stands); the prewarm loop's
  cadence (5 min) and width (rr1..rr5) are the next report's tuning
  material.
- **How the next report verifies it:** `doh: dials N, reuse M,
  hedgeWins K` — a cold start with a burst of fresh names shows
  dials≈1; remaining failures carry `phase=` (tls = path
  interference, status/parse = server); the prewarm's effect shows as
  the first video's `tunnel up dns=` dropping to the warm ~130 ms.

## ADR-0026 — UDP/443: the ICMP port-unreachable stays gVisor's (the
M8-j guard), and the QUIC fallback becomes measurable: the drop is
logged, the next TCP flow to the same host logs the elapsed time, and
the v6 discards are counted (M8-z, work item D)

- **Status:** Accepted 2026-10-19 (M8-z; the spec's H3 — "do not rely
  on gVisor" was a QUESTION, and the answer is: verify, then rely only
  on what is pinned).
- **Context:** `handleUDP` returns false for non-DNS ports and relied
  on gVisor to send the ICMP port-unreachable that breaks QUIC fast;
  whether it actually did for these spoofed/promiscuous flows was
  never proven, and the fallback's COST (drop → next TCP flow) was
  never measured.
- **Decision:**
  1. **The ICMP reply stays gVisor's.** The spec's own §7.1 step —
     "first write a host test that sends a UDP datagram to a non-DNS
     port through the stack and records whether an ICMP
     port-unreachable comes back" — already exists as
     `TestUDP443FastFailICMP` (the M8-j guard, green since debug.8):
     gVisor DOES send type 3/code 3 quoting the refused datagram for
     these flows. Per the spec's own rule ("if gVisor already sends
     it, keep the code and keep the test as a guard") no explicit ICMP
     writer was added; the guard is the proof.
  2. **The timeline.** A UDP/443 drop logs `udp443 dropped` (no
     names) and remembers the destination; the NEXT TCP flow to the
     same host logs `udp443 fallback elapsed=<ms>` (host only under
     LogHostnames — R21) — the QUIC fallback's cost, measured on the
     report. The memory is one-shot per destination, bounded at 128.
  3. **v6Dropped.** The wire pump's IPv6 discards (ADR-0010's
     fail-safe) are counted and ride the facade's tun line — the
     policy made visible instead of silent.
- **Evidence:** internal/tunstack/udp443_test.go (new, four tests),
  verified BOTH WAYS (the drop log, the fallback gate and the v6
  counter stashed → the timeline and v6 tests FAIL; restored → green).
  The untouched `TestUDP443FastFailICMP` and `TestUDPNon53Dropped`
  pins stand. `TestV6DropsDoNotDisturbTheIPv4Path` guards that the
  counting does not perturb the v4 pump.
- **What this ADR does NOT decide:** no QUIC relay (ADR-0009 stands —
  UDP is dropped, TCP is the path); IPv6 stays disabled (ADR-0010).
- **How the next report verifies it:** the `udp443 dropped` /
  `udp443 fallback elapsed=` pair brackets every QUIC attempt, and
  `v6Dropped` names any v6 leakage the phone's apps attempt.

## ADR-0027 — The debug.18 diagnostics surface: class= on every tunnel
line, the debug-tester hostname default (visible notice, release
hostnames stay off), the Top-10 slowest-or-failed hosts, the
plain-relay summary, the DNS detail with phases, the dial RTT classes,
the one-shot dead-origin SNI probe, and the timeline marks ("I pressed
play" / "First frame") (M8-z, work item E)

- **Status:** Accepted 2026-10-19 (M8-z; the spec's own words: "make
  the next diagnostics report able to prove why").
- **Context:** the debug.16 report could name the routed cohort's
  timings but not the plain cohort's existence, not the DoH failure
  phase, not the dial-vs-first-byte split's distribution, not the
  QUIC fallback's cost, and not the wall-clock time from pressing
  play to the first frame.
- **Decision:**
  1. **class= on the message text** (the ring drops attrs — the
     M8-r lesson, applied to the class): every `tunnel up`,
     `first-byte`, `first-byte failed`, `retry`, `salvaged`, `raced`,
     `done`, `stall killed` and the new plain kill line carries
     `class=routed|plain`; `tunnel up` also carries
     `rtt-class=near|far` (dial under/over 200 ms) and the near/far
     counters.
  2. **The debug-tester hostname default.** ONLY a DebugTesterBuild
     (the `-ldflags -X` lane in build-android.sh; the default here is
     the RELEASE value, false) turns LogHostnames on for Android's
     shipped state, and the report's very first line says so:
     `DEBUG BUILD: hostnames included for diagnosis`. A release build
     never prints it and never prints a host name (R21) — pinned by
     test both ways; an owner's saved logDomains survives verbatim in
     every build.
  3. **The new report sections:** Top 10 slowest or failed hosts (per
     host: flows, failures, plainKills, worstFirstByte, class — from a
     bounded in-memory table, host names redacted to `<redacted>`
     unless hostnames are logged); the plain-relay summary; the DNS
     detail (per recent failure: server number, phase, reason,
     latency; plus the DoH dials/reuse/hedgeWins-by-server counters);
     the one-shot dead-origin probe (up to 2 penalized IPs × a
     ClientHello with SNI www.google.com and one with NO SNI, only
     while On, never on a timer — `probe dead-origin sni=alt|none
     result=answered|silent|reset`, no IPs in the report); and the
     timeline marks (the facade's Mark(label) — the Android
     diagnostics screen's two buttons — with the last pair's elapsed
     time and the window's failed/plainKills/slowFirstByte deltas).
  4. **The counters line:** plainFirstByteTimeouts, dialNear,
     dialFar, slowFirstBytes ride the header; v6Dropped rides the
     facade's tun line (the engine cannot see the stack's own
     discards); doh: dials/reuse/queries/hedgeWins.
  5. **The buttons.** The Svelte Diagnostics screen gains "I pressed
     play" and "First frame" — POST /api/mark on the webui (the
     Android WebView's transport), the Wails binding on desktop, the
     gomobile `Mark(label)` for the shell. Marks reset with the ring
     on Stop (R21 hygiene).
- **Evidence:** internal/engine/report_sections_test.go (new, four
  tests — every section rendered, the debug notice's both-ways
  presence/absence, the window's exact counts, the Stop reset),
  verified both ways (the Top-10 and timeline renders stashed → the
  section assertions FAIL). The privacy pins
  (`TestEngineDiagnosticReportRedacted`) stand — the new sections
  carry no host names or IPs in a release-shaped report.
  flowtiming_test.go's line-parse assertions updated for the class tag
  (the format change is the point).
- **What this ADR does NOT decide:** the report stays plain text, one
  surface, no remote upload (R20/R21); hostnames never persist (the
  host table is memory-only, cleared with the process).
- **How the next report verifies it:** the tester checklist's five
  steps produce exactly the evidence the spec's §12 asks to read:
  the MARK pair's elapsed time (the single number that says whether
  the work succeeded), the class=plain rows in the Top-10, the DNS
  phase column, and the probe's SNI-vs-IP verdict.

## ADR-0028 — The verified prewarm and the raced-path salvage: the
origin-probe pass (a fragmented ClientHello carrying the prewarmed
name, answered→verified-live / silent→dead) feeding the good table
and the strike-escalated penalty box, plus ONE raced salvage round
over the untried origins before the terminal (M8-aa, the debug.19
dead-origin-pool round)

- **Status:** Accepted 2026-10-19 (M8-aa; the debug.18 report's own
  numbers named the round: 38 errors on 70 conns, every one of them
  class=routed, the top host 13 flows for 8 failures, the dead-origin
  probe silent on BOTH sni=alt and sni=none — per the spec's §12 grid,
  "the IP is blackholed and the penalty box is right").
- **Context:** the debug.18 report closed every hypothesis the
  cold-start spec opened: the plain lane is healthy (plain-relay
  flows=6, firstByteTimeouts=0, worst 295 ms — H1 disproven, A+B did
  fix it), DoH is healthy (dials 4 / reuse 48, dns 137/0 — H2 fixed),
  the QUIC fallback instant. What remained is the ROUTED lane's
  dead-origin lottery: Google's authoritative answers mix live and
  IP-blackholed cache nodes, every raced flow gambles three of them,
  an all-dead round terminals at ~1 budget (the paste's ~20
  reason=timeout terminals at dur≈2.6 s), and the app retries in
  waves — 13 flows on one host — faster than the 60 s penalty lapses,
  re-rolling the same toxic mix. The first video pays the whole
  discovery cost; the next ride the warm box (Ali's original
  "first video 1–2 minutes, the rest instant", now measured). The
  prewarm loop fills the cache with rr1..rr5 answers but never
  VERIFIES them — the cache is pre-populated with the same gamble.
- **Decision:**
  1. **The origin-probe pass.** On the existing 5-minute googlevideo
     prewarm cadence (and once at engine start, right after the
     cache-fill), the engine runs one pass through the running proxy:
     for each name in priority order — the stable OriginProbeNames
     (youtubei.googleapis.com, www.googleapis.com, www.youtube.com,
     youtube.com, i.ytimg.com, yt3.ggpht.com — the player request's
     own fronts) first, then the FRESHEST learned sn-codes' rr1..rr5 —
     resolve through the same resolver the flows use, and verify every
     answer IP that is neither verified-live nor penalized right now,
     at most OriginProbePerPass=32 per pass, at most
     OriginProbeInflight=3 in flight, each inside
     OriginProbeBudget=2.5 s. The probe is the EXACT wire shape a real
     flow sends: one fragmented ClientHello (the live preset, captured
     once per pass — D102's discipline) with the prewarmed name as
     SNI, through the engine's one netx dialer (ADR-0011 loop safety),
     tls.Client with InsecureSkipVerify — only whether bytes come
     back matters. This is not the report's one-shot SNI-vs-IP probe
     (ADR-0027 §8.3.5 stays unfragmented and SNI-swapped by design):
     the tuple measured here is the one a real flow will use.
  2. **The good table.** An "answered" verdict marks the IP
     verified-live for OriginProbeGoodTTL=15 min: liveIPs puts the
     verified-live IPs at the FRONT of every dial list it builds (the
     doors' primaries, the raced extras' rotations, the salvage
     candidates) — a preference with the same never-a-verdict
     discipline as the box: an IP without the mark dials exactly as
     before, first place is its only prize. Bounded 512, FIFO.
  3. **The strike-escalated box.** A probe "silent"/"reset" verdict
     marks the IP dead for OriginProbeDeadTTL=10 min (direct
     evidence — the exact hello a flow would send, sent and never
     answered); a flow's classified first-byte death keeps its 60 s.
     Every mark on an already-known IP escalates its TTL to the
     DeadIPStrikeEscalation floor (60 s → 5 min → 30 min): the
     report's re-gamble — waves re-rolling a set the 60 s penalty
     kept re-serving — ends. The box stays a PREFERENCE, never a
     verdict (ADR-0023 unchanged): an all-penalized list dials in
     full, a recovered IP is simply re-learned clean once its TTL
     lapses.
  4. **The raced-path salvage round.** When a raced round dies on
     every candidate it dialed, the flow now computes the answer
     set's UNTRIED origins (a fresh resolve minus every IP the round
     dialed minus everything the box penalizes — verified-live first)
     and, when any exist, races ONE more round over exactly those
     before the terminal: the replay stays byte-exact, the client
     stays frozen exactly as the round kept it (the M8-v proven
     semantics), the app never sees the failure. A set with nothing
     untried keeps the fast M8-w terminal — a hopeless host must not
     burn a second budget (the debug.18 paste's 100%-failure hosts
     keep their ~2.6 s terminal). The candidate core lives in
     raceIPList, shared with the serial path's own salvage (the
     M8-v machinery, extracted, byte-for-byte).
  5. **Android-only.** The probe pass runs on the Android platform
     only (the engine's gvideo loop gates on the platform name): the
     desktop lanes keep the M8-z prewarm-resolution behavior
     byte-for-byte (hard constraint 5's letter), and the stub
     platform that every host test builds on never dials a probe —
     the test herd stays hermetic.
- **Evidence:** internal/proxy/originprobe_test.go (new, six tests,
  four shapes plus two guards): the probe-cleaned pool (the corpse
  never dialed, the flow delivered byte-exact, no terminal), the
  neutered control (DeadIPPenalty=-1, the exact M8-x box-off shape:
  the same fixture gambles the corpse and terminals — the report's
  own pre-fix signature, proving the MARKING changed the flow's
  fate), the raced salvage over the untried origin (retry + salvaged
  lines, no terminal, the live origin dialed once), the fast-terminal
  guard, the strike escalation (live=2 escalated=1, the
  all-penalized list dials in full), and the probe line's R21 pin
  (counts only — no IP, no name). Verified BOTH WAYS: with tunnel.go
  stashed (the debug.18 shape) `TestRacedSalvageCoversTheUntriedOrigin`
  FAILS with exactly the report's signature ("read echo … EOF" — the
  terminal, the live origin never dialed) while every guard and the
  probe tests pass on both sides; the marking's both-ways is the
  in-test neuter (shapes 1 vs 2 on the same fixture). Full suite
  green; -race on doh/engine/stats/tunstack and the touched proxy
  surface green, ZERO data races; the full-package -race stall-timing
  flakes are the documented pre-existing sandbox shape (identical
  failures at HEAD, stashed and re-run this round).
- **What this ADR does NOT decide:** the report's one-shot
  dead-origin probe (ADR-0027 §8.3.5) keeps its own unfragmented,
  SNI-swapped shape — it answers SNI-vs-IP, a different question;
  the race width stays FirstByteRaceDials=2 (Q5's ruling stands —
  the fix is pool hygiene, not width); no SNI is ever rewritten (the
  engine stays a transparent relay); no QUIC relay (Q4 stands).
- **How the next report verifies it:** the `origin probe:` line
  (passes/ips/answered/silent/reset plus good/penalized/strikes) says
  the pass ran and what it found; the first-play wave's terminals
  collapse (fewer `first-byte failed` lines, `salvagedSaves` >
  0 when mixed sets still appear); the Top-10's failure counts drop
  below the debug.18 baseline; and the auto-timeline pair (ADR-0030)
  carries the wall-clock proof.

## ADR-0029 — The Android lean stall budget: 8 s, platform-gated in
the engine's listener build, the proxy's tiered watch untouched, the
desktop's 15 s byte-for-byte (M8-aa)

- **Status:** Accepted 2026-10-19 (M8-aa; the debug.18 report's three
  lean-tier kills at exactly idle=15325–15362 ms, each carrying
  5.0–7.4 KB of a response that never finished — the mid-response
  corpse shape, 45 s of the minute the tester still waited).
- **Context:** the tiered stall watch (ADR-0022) tuned the LEAN tier
  at 15 s for the cohort it then knew: never-proved flows. The
  debug.18 paste shows what that budget costs on a cold start: a
  player-API flow speaks once (first byte plus part of a response),
  goes totally silent — the blackholed cache node died mid-stream —
  and the watch holds the app's retry hostage for the whole 15 s
  before handing the failure back. Three of them is 45 s: nearly half
  the measured minute. A lean flow that has already spoken and then
  sits silent for 8 s is dead on this network — the same number the
  plain-relay safety net already trusts for a never-spoken origin
  (PlainFirstByteTimeout, ADR-0024 work item B).
- **Decision:** Android's engine listener builds the proxy with
  StallTimeout = StallLeanTimeoutAndroid = 8 s; every desktop build
  keeps config.StallTimeout = 15 s byte-for-byte. The gate lives in
  the ENGINE's startListener (the platform name it already reads for
  the settings loader) — the proxy's watch semantics, tiers, and
  knobs are untouched, and the report prints the budget that fired
  (`stall kills: lean=N rich=N leanBudget=8s|15s`) so the next paste
  says which one ran. The rich tier (90 s, StallRichBytes, the
  survived-gap pacing proof) is unchanged — the M8-x protection for
  pooled video connections stands exactly as built.
- **Evidence:** internal/engine/m8aa_test.go
  `TestStallLeanBudgetAndroid`: an android-named platform's report
  carries leanBudget=8s, a desktop-shaped platform's 15s. Verified
  BOTH WAYS: with the engine's M8-aa additions stashed the section
  assertion FAILS (the line is absent — the section-render proof
  shape of ADR-0027's own report test). The stall-tier pins
  (stalltier_test.go) pass unchanged — the proxy semantics moved
  nowhere.
- **What this ADR does NOT decide:** the rich tier, StallRichBytes,
  and the survived-gap escalation stay ADR-0022's exactly; no
  user-facing setting is added (the budget is engine timing, not
  policy — the platform default IS the knob, like the M8-s budgets
  before it).
- **How the next report verifies it:** the lean kills land at
  idle≈8.3 s (three of them ≈ 25 s, not 45), the leanBudget= line
  says 8s, and the auto-timeline pair shrinks accordingly.

## ADR-0030 — The auto-timeline heuristic and the M8-aa counters: a
play-suspected→first-data pair measured WITHOUT the buttons, the
buttons staying authoritative; the tier split, the saves, the routed
flow starts, and the probe's verdicts on the report (M8-aa)

- **Status:** Accepted 2026-10-19 (M8-aa; the debug.18 report's own
  confession: "timeline: no marks (the \"I pressed play\" / \"First
  frame\" buttons stamp it)" — the tester never pressed them, and
  the one number that says whether the round succeeded was missing).
- **Context:** ADR-0027's timeline depends on the tester tapping two
  buttons at the right moments. The debug.18 circle proved that is
  not a dependable measurement path — the report arrived with no
  marks and no play→frame number, exactly the evidence the spec
  called "the single number that says whether the work succeeded".
- **Decision:**
  1. **The auto-timeline.** While the engine is On (degraded
     included), a 500 ms poller over the counters the flows already
     feed: a routed-flow burst after ≥5 s of routed quiet stamps
     "auto: play-suspected" (the player wave opening connections);
     a ≥100 KB down-burst while a stamp is open pairs it with "auto:
     first-data" (video bytes flowing). Both stamps ride the log
     ring as AUTO-MARK lines (timestamps included in the paste); the
     report renders the LAST pair's elapsed time and the pair count
     beside the manual marks, labeled "heuristic; the buttons stay
     authoritative" — the heuristic cannot tell a feed load from a
     play, only the buttons can. Pairs bounded at 8; everything
     resets with the ring on Stop (R21 hygiene).
  2. **The M8-aa counters and sections.** The report header gains
     `counters: stallKillsLean N, stallKillsRich N, racedSaves N,
     salvagedSaves N, routedFlows N`; the plain-relay summary is
     followed by `stall kills: lean=N rich=N leanBudget=…` and
     `origin probe: passes=… ips=… answered=… silent=… reset=…
     good=… penalized=… strikes=…` (counts only — no IP, no name,
     R21; the good/penalized/strikes numbers come from the running
     proxy's box and good table, zero-safe when off).
- **Evidence:** internal/engine/m8aa_test.go (three tests): the
  pair-from-counters flow (burst → stamp → burst → pair → report →
  Stop reset), the Android/desktop lean budget split, and the
  full-section render with the R21 no-leak guards. Verified BOTH
  WAYS: with the engine's M8-aa additions stashed, all three FAIL on
  the missing sections (the section-render proof shape); the no-leak
  guards pass on both sides.
- **What this ADR does NOT decide:** the buttons stay the
  authoritative measurement (ADR-0027 §8.3.6 unchanged); the
  heuristic adds no UI, no persistence, and no per-host data — two
  timestamps and a byte threshold over counters that already exist.
- **How the next report verifies it:** the `auto-timeline:` line
  carries the play→data elapsed time even when the buttons go
  unpressed; `racedSaves`/`salvagedSaves` say the machinery saved
  flows; `origin probe:` says the pool was verified before the wave.

## ADR-0031 — The cross-origin substitution: a *.googlevideo.com
flow whose own answer set keeps dying dials a VERIFIED-LIVE pool
origin instead (SNI unchanged — Google's video edge serves any
googlevideo.com name), the pool fed by probe verdicts AND real flow
first-bytes, and a flow death demoting the stale mark (M8-ab, the
debug.20 escape-the-pool round)

- **Status:** Accepted 2026-10-20 (M8-ab; the debug.19 report's own
  numbers named the round: auto-timeline play-suspected ->
  first-data elapsed=44999ms pairs=5 — five app retry waves gambling
  fresh rotated qxau5 unknowns while the good table held 24 verified
  origins no dial list ever saw, racedSaves=9 but salvagedSaves=0,
  and the video starting only when the app ITSELF fell back to a
  foreign cache hostname — r5---sn-4g5e6nsy and r2---sn-nv47lnsr,
  the only googlevideo hosts with clean first-bytes in the whole
  window).
- **Context:** ADR-0028 built the verified pool, but the debug.19
  report proved its SCOPE is the failure: liveIPs, the raced extras,
  and the untried salvage all draw from the SAME hostname's rotating
  answer set, so the pool's marks help only when a fresh answer
  happens to overlap them — Google's rotating per-video answers
  mostly do not. Five waves re-gambled unknown qxau5 IPs (every
  raced death invalidating the DNS set, the next resolve rotating)
  until the player's own fallback landed a foreign cluster. The
  knowledge existed the whole 45 s (24 verified origins) and no dial
  list could reach it. Two latent leaks rode along: a first-byte
  death on a good-marked IP left the stale mark live (the ADR-0028
  comment claimed the drop; the code never ran it) — a dead IP
  resurfacing at the FRONT of every dial list once its box entry
  lapsed — and flow successes never wrote good marks at all (the
  session's one foreign-cache win armed nothing).
- **Decision:**
  1. **The substitute candidate.** A *.googlevideo.com flow's raced
     round replaces ONE own-set rotation with a substitute candidate
     that dials the good pool's gvideo-flagged verified-live origins
     (not in the current answer set, not penalized, freshest verdict
     first, up to SubstitutionPoolDials=3 walked with DialHost's
     stagger). The substitute replays the identical fragmented
     ClientHello — SNI unchanged, the client's own hello — onto the
     pool origin: Google's video edge is one wildcard service behind
     the per-cluster names, and the Iranian practice of pinning
     hand-picked working cache IPs into a hosts file rides exactly
     this. The dial count stays flat. A substitute win logs "tunnel
     first-byte substituted" and counts substWins; a death demotes
     the POOL origin (box + mark drop) and never the host's answer
     set. Scoped to googlevideo flows only — the API fronts keep
     their own answers (their certs are not interchangeable with
     video-edge IPs). Knob: GoodIPSubstitution (negative = the exact
     debug.19 shape).
  2. **The self-feeding pool.** Every first-byte SUCCESS — raced
     winner, salvage winner, serial-path origin — marks the speaking
     origin good (gvideo-flagged for googlevideo hosts, 15-minute
     TTL): stronger evidence than the probe's synthetic hello, and
     the loop the report's 45 s never closed (the session's own
     foreign-cache win now arms every later flow).
  3. **The demote.** markDeadIPTTL drops the good mark on the same
     IP (the ADR-0028 documented behavior, actually implemented): the
     box's verdict outranks the stale preference while it lives, and
     the preference no longer outlives it — the debug.19 latent leak,
     and a poisoned substitution pool the moment the pool started
     mattering.
  4. **Platform scope.** Substitution is pool-driven and
     platform-neutral by design: a desktop without probe passes
     starts with an empty pool and behaves byte-for-byte like
     debug.19 until a flow succeeds; after that only better. The
     android-only parts of the round stay android-only (ADR-0032's
     cadences, ADR-0033's persistence).
- **Evidence:** internal/proxy/substitution_test.go (six tests): the
  all-corpse answer set saved by the pool origin (echo byte-exact,
  the substituted line, no terminal), the neutered control
  (GoodIPSubstitution=-1 terminals with the report's exact
  signature), the self-feeding loop (flow 1's success saves flow 2's
  different-host all-corpse set), the demote (a once-echo origin:
  probe-answered then flow-dead — good count 0, box marked), the
  churn signal (gvideoTerminals), and the substituted line's R21
  privacy. Verified BOTH WAYS: the substitution neutered (the raced
  path passes nil, the winner marking removed), the two headline
  tests FAIL — the flow terminals with "read echo: EOF" and the pool
  never gains the flow's evidence; the demote neutered, the demote
  test FAILS on good count 1; the churn counter neutered, the churn
  test FAILS at 0.
- **What this ADR does NOT decide:** cross-SERVICE substitution
  (youtube.com SNI onto googlevideo IPs — the certs are not
  interchangeable, and the API fronts answer fine on their own); the
  probe's cadence and coverage (ADR-0032); persistence (ADR-0033);
  any user-facing setting (the knob is engine wiring, like the M8-y
  box before it).
- **How the next report verifies it:** the new
  `substitution: attempts=N wins=M pool=G (gvideo V)` line — wins>0
  with a shrinking auto-timeline means the escape worked in the
  field; attempts>0 with wins=0 means the cross-cache bet failed and
  the round needs a different shape (the one honest failure mode this
  ADR cannot rule out from the armchair); the "tunnel first-byte
  substituted" lines ride the ring beside the raced ones.

## ADR-0032 — The probe pass's coverage and cadence: the name list
walks the learned code store ROUND-ROBIN (one rr name per code, the
freshest-2 bias that kept the pool local-only gone), a fast-start
cadence (45 s passes while the pool is thin), and the churn kick (a
googlevideo terminal verifies new origins immediately, debounced)
(M8-ab)

- **Status:** Accepted 2026-10-20 (M8-ab; the debug.19 session ran
  passes=1 in its whole cold start, and that one pass's name list
  was the stable fronts plus rr1..rr5 of the 2 FRESHEST learned
  codes — early in a session the local cluster twice, so the
  verified pool could not have escaped the blackhole even in
  principle).
- **Context:** the pool's content is only as diverse as the names
  the pass probes. The store is oldest-first and the freshest-2
  window sits at its tail — exactly where a young session's local
  codes cluster. Foreign clusters (the ones that actually serve)
  live deeper in the store and were never probed early; the 5-minute
  cadence meant the one pass at engine start was often the whole
  cold start's verification budget.
- **Decision:**
  1. **Rotation.** probePassNames (pure, cursor-owned-by-the-loop):
     stable OriginProbeNames first, then one rr name per code from a
     round-robin window (ProbeCodesPerPass=12 codes a pass, the rr
     index rotating with the cursor so a cluster's variants take
     turns), the cursor wrapping at the store's end — every
     remembered code is covered within a few passes. PerPass raised
     32->48.
  2. **Fast-start cadence.** While the engine is On and young (5
     minutes) and the gvideo pool is thin (<8), a 45 s ticker runs
     passes; the 5-minute prewarm cadence stands otherwise.
  3. **Churn kick.** failFlow on a googlevideo host bumps
     gvideoTerminals (the stats counter); the gvideo loop's 1 s
     watch kicks an immediate prewarm+pass on a fresh increase,
     debounced 10 s — verified origins land inside the app's next
     retry wave, not the next cycle.
  4. **Android-only, exactly like ADR-0028 decision 5:** the fast
     and watch lanes never fire passes on non-android platforms; the
     desktop loop is the M8-z shape byte-for-byte (the 5-minute
     prewarm stands).
- **Evidence:** internal/engine/m8ab_test.go
  TestProbePassNamesRotatesTheCodeWindow (window advance, rr
  rotation, wrap, oversized window, empty store) pins the pure
  builder; TestReportRendersM8abSections renders the new sections
  with R21 no-leak guards; the churn SIGNAL is pinned at the proxy
  layer (substitution_test.go shape 5). The loop's own select logic
  is 10 lines of ticker arithmetic exercised by every android-named
  engine test since M8-aa (the tickers fire, the gates hold).
- **What this ADR does NOT decide:** the probe's wire shape and
  verdicts (ADR-0028's exactly); the pool's substitution use
  (ADR-0031); persistence (ADR-0033); no ECS steering or DNS-answer
  manipulation (the DoH lane stays ADR-0025's).
- **How the next report verifies it:** origin probe passes= grows
  several entries inside one cold start; answered climbs with
  non-local clusters covered (the substitution pool's gvideo count
  in the new line); gvideoTerminals pairs with a pass shortly after
  a terminal in the ring.

## ADR-0033 — The persisted verified-live pool:
googlevideo-good-ips.txt seeded back at start with a shorter TTL (the
first video of session N+1 rides the origins session N verified), the
seeds self-correcting through the demote (M8-ab)

- **Status:** Accepted 2026-10-20 (M8-ab; every session of the debug
  circle restarted with an empty pool and re-paid discovery — the
  tester's recurring complaint is the FIRST video of a session).
- **Context:** the codes store (ADR-0025 §6.4) already persists
  CLUSTER memory across sessions, but the verified-live IPs
  themselves were session-scoped: session N+1 knew which clusters to
  resolve and nothing about which origins actually spoke. The probe
  pass needs minutes to re-verify; the player's first video pays the
  whole window.
- **Decision:** the engine persists the gvideo good pool (one IP per
  line, cap 32, GoogleVideoGoodIPsFile beside the codes store in the
  app's private directory) — written on change from the gvideo loop's
  15 s ticker and flushed at its Stop; seeded at listener start with
  GoodPoolSeedTTL=5m (yesterday's verdict is stale evidence).
  Desktop-included: the pool is an optimization, never a dependency
  (unreadable file = empty pool, write failure = next change), and a
  stale seed self-corrects through ADR-0031's demote (one flow's
  ~650 ms reset drops and boxes it). Same privacy footing as the
  codes file (D-entries on user data; infrastructure addresses in
  the app's private data directory, never a log line — R21).
- **Evidence:** internal/engine/m8ab_test.go
  TestGoodPoolPersistenceRoundTrip: a pre-written pool file renders
  pool=2 (gvideo 2) in the started engine's report and survives the
  Stop-time flush on disk. Verified BOTH WAYS with the report
  section proof (ADR-0027's shape): the M8-ab additions stashed, the
  pool line is absent and the seed unrendered.
- **What this ADR does NOT decide:** expiry of seeds beyond the TTL
  (the demote owns correctness); the probe's cadence (ADR-0032); any
  user-facing setting or UI.
- **How the next report verifies it:** the first minutes of a
  session that follows a working one show substitution wins BEFORE
  the first probe pass could have verified anything (the seeded
  pool's fingerprint), and the pool's gvideo count starts non-zero.

## ADR-0034 — The evidence-weighted marking and the
anti-concentration rotation: reset deaths box briefly without
demoting, pending stall kills demote, the probe REHABILITATES boxed
IPs, and the dial lists rotate their verified fronts (M8-ac, the
debug.21 hostile-network round)

- **Status:** Accepted 2026-10-20 (M8-ac; the debug.20 report's own
  evidence named the round: penalized=12 while the one-shot
  dead-origin probe ANSWERED on both SNI variants — the box held IPs
  the network was happy to serve; the gvideo pool ended at 1 despite
  substitution wins=5 because every terminal's deaths demoted the
  marks its wins had written; and www.youtube.com ran 17 flows for 17
  failures with first bytes at 282 ms — connections opened, answered,
  then died, all on the same pinned verified origin).
- **Context:** the censor moved from IP-blackholing to killing
  connections in flight (probe reset verdicts 0→35, DoH dials 6→59
  thirteen hours apart). Under that regime a flow death is evidence
  about ONE connection at ONE moment, not about the origin IP — but
  the M8-y/M8-aa box treated every classified death as a 60 s-to-30 m
  IP verdict, and the M8-ab demote drained the pool on every
  terminal. The ADR-0023/0028 design's core assumption (deaths are
  per-IP stable) broke; the machinery needed to stop fighting the new
  evidence shape.
- **Decision:**
  1. **Class-weighted deaths.** markDeadIPClass(ip, reason): a RESET
     death boxes for ResetIPPenalty=20 s (a within-retry-wave skip
     that spreads candidates) and NEVER demotes the good mark; every
     other first-byte death keeps DeadIPPenalty=60 s + the demote.
     Probe verdicts follow the same weighting: reset → 60 s (was the
     10-minute OriginProbeDeadTTL), silent → 10 m unchanged (a fresh
     connection given a full 2.5 s budget and staying blackholed is
     still the strong per-IP evidence it always was). Strike
     escalation tiers drop to [60 s, 2 m, 5 m] — a 30-minute boxing
     of a live origin is indefensible when the report's own probe
     says it answers.
  2. **The pending/idle stall demote.** The stall kill computes
     pending = lastUp > lastDown (the client spoke after the origin's
     last byte — a request the origin never answered). A PENDING kill
     demotes the origin's good mark at BOTH tiers (dead with work
     outstanding is dead-origin evidence whatever the budget); an
     IDLE kill never demotes (the shape is ambiguous: the app's
     pooled connection between requests OR a mid-body death — the
     rotation handles any concentration, and a pooled-healthy origin
     must not lose its preference).
  3. **The rehabilitation.** ProbeOrigins samples up to
     ProbeRehabPerPass=4 currently-boxed IPs (soonest-expiry first)
     and re-verifies them with a neutral-SNI hello: "answered"
     clears the box entry entirely (strikes gone), marks the IP good
     (never gvideo-flagged — the neutral hello proves the path, not
     the wildcard service), and counts probeRehabilitated; anything
     else refreshes the box with the class-weighted TTL. The box now
     self-corrects on fresh evidence instead of sitting wrong for its
     whole escalated TTL.
  4. **The rotation.** liveIPs and goodSubstitutes rotate their
     verified-live fronts per call through one server-wide counter:
     consecutive flows to the same host lead with DIFFERENT verified
     origins, so the raced primaries and rotations spread across the
     answer set instead of funneling onto the single freshest mark
     (the debug.20 concentration: one pinned origin ate 17/17
     failures). A pair-rate-limiting killer — the one shape the
     dead-origin probe's "answered" verdicts are consistent with —
     gets spread traffic for free.
- **Evidence:** internal/proxy/flowkill_test.go (five shapes): the
  rotating front (three calls, three different leads, neutral tail
  intact), the reset-class death (mark kept, brief box; the neutered
  control FAILS on the dropped mark), the pending demote (a request
  written inside the wake gap so the M8-x pacing proof does not
  escalate the flow first; the demote lands, cause=pending on the
  line), the idle guard (cause=idle, mark kept), the rehabilitation
  (a boxed echo origin cleared, marked, counted, rehab=1 on the pass
  line; the neutered control leaves the box stale). Both-ways: the
  rotation neutered → the two rotation tests FAIL on the static
  front; the reset class neutered → the mark test FAILS; the pending
  demote neutered → the demote test FAILS; the rehabilitation
  neutered → the rehab test FAILS.
- **What this ADR does NOT decide:** the stall budgets themselves
  (ADR-0020/0022/0029 unchanged — the kills are doing their job; the
  debug.20 kills carried genuine dead connections), the probe's
  cadence (ADR-0035), cross-origin substitution (ADR-0031 unchanged —
  it WORKED in the field: wins=5).
- **How the next report verifies it:** rehabilitated > 0 says the box
  was wrong and self-corrected; goodDrops tracks the pool's churn
  (a draining pool with reset-heavy deaths says the weighting needs
  another turn); the substitution line's pool (gvideo V) should hold
  above single digits through a hostile window; and the Top-10's
  failure rates should stop being 100%-on-one-host.

## ADR-0035 — The cadence rebalance and the DoH brownout: the 45 s
fast-start probe lane REMOVED (the persisted seed and the flow-fed
marks own its job), the churn kick kept and debounced, and every
background lane pauses while the DoH resolver's connections churn
(M8-ac)

- **Status:** Accepted 2026-10-20 (M8-ac; the debug.20 report's doh
  line: dials 59, reuse 127 — the connection-killer was murdering the
  resolver's warm HTTP/2 connections faster than they could serve,
  and the engine's own background lanes kept piling queries and
  scan-like probe bursts onto the dying lane while the app's flows
  waited behind them).
- **Context:** ADR-0032's fast-start lane (45 s passes while young
  and thin-pooled) presupposed that more probing stocks the pool
  faster. The debug.20 session falsified that premise under the new
  regime: the gvideo pool never crossed the floor (reset verdicts
  boxed what the passes verified), so the lane ran its whole 5-minute
  window — six-plus probe bursts right at the session start, exactly
  when the network was hostile and the first video was starting.
  Meanwhile the pool-stocking job found better owners: the persisted
  seed (ADR-0033) arms session N+1 before any probe runs, and the
  flow-fed marks (ADR-0031) verify origins with real traffic the app
  was sending anyway.
- **Decision:**
  1. **The fast-start lane is removed** (the ProbeFastStart* knobs
     with it). The gvideo loop keeps exactly three lanes: the 5-minute
     prewarm cadence, the 1 s churn watch (a googlevideo terminal
     kicks an immediate prewarm+pass, debounced 10 s), and the 15 s
     good-pool save tick.
  2. **The DoH brownout.** A rolling window over the DoH transport's
     dial counter: ≥ BrownoutDialThreshold=3 dials within
     BrownoutWindow=60 s enters the brownout (counted, one WARN
     line); the prewarm, the probe passes, and the churn kicks pause
     while it is active, lifting after BrownoutQuiet=60 s without a
     new dial. The app's own flows never pause — only the engine's
     background traffic withdraws from a dying lane.
  3. **Android-gated as before** (ADR-0028 decision 5): the desktops
     keep the byte-for-byte M8-z loop; the brownout gates only lanes
     the desktops never run.
- **Evidence:** internal/engine/m8ac_test.go
  TestDoHBrownoutTracker (injectable clock: enter at threshold, hold
  under continued dials, lift after the quiet period, re-arm on the
  next burst; the neutered control never enters) and the report's
  brownouts= line.
- **What this ADR does NOT decide:** the DoH transport itself (its
  hedged walk, warm-ping health checks, and stale serving are
  ADR-0025's, unchanged — the brownout does not touch the resolver,
  it only stops the engine's background lanes from leaning on it
  while it is being killed), the probe's wire shape (ADR-0028's).
- **How the next report verifies it:** doh: … brownouts=N — N > 0
  with the background quiet during the hostile windows (the log's
  probe lines should cluster in the calm gaps between brownouts, not
  inside them), and dials should stop climbing once the killer
  relents.

## ADR-0036 — The cause-split diagnostics: every stall kill says
whether the flow died PENDING (the client's request was never
answered — a dead origin) or IDLE (both ends quiet — either the
app's pooled connection or a mid-body death), the counters split to
match, and the report carries rehabilitated/goodDrops/brownouts
(M8-ac)

- **Status:** Accepted 2026-10-20 (M8-ac; the debug.20 report's seven
  lean kills all carried 6-15 KB of partial responses and the one
  visible www.youtube.com kill carried up=10.7 KB — the flows SPOKE
  and then died, but the report could not say whether the client had
  a request outstanding when the origin went silent, which is the
  difference between "the engine is killing dead connections" and
  "the engine is killing the app's healthy pool").
- **Context:** the lean stall kill frees the app's retry by design,
  but the debug.20 experience ("way way worse, doesn't even play")
  made the kill itself a suspect. The discriminator already existed
  inside the watch — the lastUp/lastDown activity stamps: a client
  that wrote AFTER the origin's last response byte is waiting on a
  request the origin never answered (the origin is dead — the kill
  is right and the demote is earned); a client quiet since the last
  byte is the ambiguous shape. The report needs the split so the
  NEXT paste answers which world the engine is in instead of the
  armchair guessing.
- **Decision:** watchStall computes pending = lastUp > lastDown at
  kill time; the stall line gains cause=pending|idle; the counters
  split (stallLeanPending / stallLeanIdle under the lean total); the
  report's stall-kills line renders "lean=N (pending=P idle=I)";
  the origin-probe line gains rehabilitated=, the counters line
  gains goodDrops, and the doh line gains brownouts= (ADR-0035's).
  The split also weights the demote (ADR-0034 decision 2) — the
  diagnostic and the policy read the same evidence.
- **Evidence:** internal/proxy/flowkill_test.go shapes 3a/3b (the
  pending kill: a request written inside the wake gap so the pacing
  proof does not escalate the flow first — cause=pending on the
  line, the mark demoted; the idle kill: cause=idle, the mark kept);
  internal/engine/m8ac_test.go TestReportRendersM8acSections (every
  new section renders, no-leak guards).
- **What this ADR does NOT decide:** any budget change (the kills
  carried genuine dead connections in the debug.20 paste — the 8 s
  lean budget and the tiering stand; IF the next report shows
  idle-kills dominating with data resuming after 9-15 s, THAT is the
  evidence that revisits ADR-0029).
- **How the next report verifies it:** pending dominating = the
  killer is real and the engine's fast-fail+retry is the right shape
  (and the demotes are earning their keep); idle dominating with the
  app recovering = the pooled-connection world and the next round
  lengthens the idle-lean budget or exempts no-pending flows; idle
  dominating with no recovery = mid-body deaths and the data path
  itself is the censor's target — a different tool than dial lists.


## ADR-0037 — Rich-flow liveness: the pending fast budget, the tuned
keepalive, and the trickle kill — every mid-life death of a proven
connection recovers in seconds, not at the app's patience (M8-ad)

- **Status:** Accepted 2026-10-20 (M8-ad; the tester's post-debug.21
  report, verbatim: "after a while playing and seeing a video like
  minute 5 of a 10min video, it goes loading, and as soon as you
  refresh it fixes" — the refresh IS the recovery mechanism the engine
  was missing, delivered by hand).
- **Context:** the M8-x tiering (ADR-0022) protects PROVEN flows from
  the lean budget's false kills — and with it, a rich flow that died
  mid-life waits the full 90 s hygiene budget before the engine closes
  it. Three death shapes hide inside that window: the player's next
  range request rides the dead conn UNANSWERED (pending — the stall
  watch sees both-silent and waits 90 s); the conn is blackholed with
  no data either way and no tuned keepalive ever notices (Go's default
  probes 9× at 15 s ≈ 2 h); and the per-connection THROTTLE — bytes
  keep arriving under the floor, the down stamp keeps refreshing, and
  the both-silent clock NEVER accumulates (the debug.20 report's
  5×-less-down shape). In all three the player buffers, drains, spins
  — and the user's refresh closes everything and recovers in ~2 s
  through the race + substitution machinery that already works.
- **Decision:** (1) `StallRichPendingTimeout` (10 s) — a rich flow
  whose client spoke after the origin's last byte (a request
  outstanding) closes at the fast budget: the demote lands (the M8-ac
  dead-with-work ruling, at any tier), the noise stays quiet (no error
  count), tier=rich cause=pending. (2) Tuned TCP keepalive on EVERY
  origin dial (netx applies `net.KeepAliveConfig{Idle: 15 s,
  Interval: 5 s, Count: 3}` when the engine configures it — the DoH
  lane's own dialer included): a blackholed peer surfaces as a read
  error in ~15 s + 3 × 5 s and the relay closes the app's side. (3)
  The trickle kill — a ring of cumulative delivered-down snapshots,
  one per stall-watch tick, `TrickleWindow` (12 s) wide; a rich flow
  past `TrickleFloorBytes` (1 MB) whose window keeps landing above
  zero but under `TrickleMinBytes` (128 KB ≈ 11 KB/s — TCP delivery
  of a range response is line-rate bursty, the player paces REQUESTS,
  not bytes) is closed QUIETLY: no demote (policing follows the
  CONNECTION — a fresh conn to the same origin runs at full speed; the
  debug.20 report's dead-origin probe showed the IPs themselves
  answer), no error count, tier=rich cause=trickle. Considered and
  REJECTED: proactive idle-gvideo rotation at ~30 s — it multiplies
  handshaves under a censor that punishes scan-like shapes, and the
  keepalive + pending budget already bound the pooled-conn waste.
- **Evidence:** internal/proxy/liveness_test.go — the rich pending
  kill fires at the fast budget with the demote and the app-side
  close; the trickle kill closes the throttled flow with the mark
  KEPT; internal/netx/keepalive_linux_test.go reads TCP_KEEPIDLE /
  TCP_KEEPINTVL / TCP_KEEPCNT back off the socket the dialer
  produced. Both-ways: the pending budget neutered → the pending test
  FAILS waiting for the kill; the ring neutered → the trickle test
  FAILS on the surviving flow.
- **Consequences:** the player's own retry becomes the recovery path
  for mid-life deaths (a kill is only as good as the app noticing —
  the close is the deliverable); the report's stall line gains the
  rich cause split (`rich=N (pending=Q trickle=T)`) so the next paste
  says which death shape dominates; a false-positive trickle kill
  costs one re-handshake on a conn that was crawling anyway.
- **What this ADR does NOT decide:** the lean tier's budgets (M8-v/x
  stand); any change to the race or substitution width (M8-w/ab
  stand — the scan-signature question stays open for a report that
  shows fresh-conn deaths immediately after bursts).

## ADR-0038 — Stall kills never invalidate the answer set: the kill is
an origin-connection death, and the resolver's cache is not the
suspect (M8-ad)

- **Status:** Accepted 2026-10-20 (M8-ad; the debug.20 report measured
  the churn this fed: dns=579-846 ms late-session, DoH dials 6→59 —
  every killed flow sent the next request back through the struggling
  DoH lane for a fresh answer set whose old IPs were fine).
- **Context:** the M8-v stall kill closed with `invalidateDNS(host)`
  — the M8-o feedback loop's reflex, imported from the dial-failure
  path where a WHOLE answer set failed to connect. But a mid-stream
  stall is a per-CONNECTION death: the per-IP demote (which the kill
  already performs on pending evidence, ADR-0034) carries the origin
  evidence into the dial lists, and the rotation spreads the rest. The
  old Python backend cached DNS forever and never re-queried on a flow
  death; the tester pointed back at it ("read the backend script
  again").
- **Decision:** remove the `invalidateDNS` call from the stall-kill
  site (lean tier; the rich tiers never had it). The answer set's own
  refresh stays exclusively with the dial-failure path (M8-o), where
  every address of the set failed to CONNECT — the shape that
  actually implicates the resolver's answers.
- **Evidence:** internal/proxy/liveness_test.go
  TestStallKillKeepsTheAnswerSet (the lean idle kill fires, the line
  logs, the resolver's Invalidate is NEVER called); the three M8-v/x
  era pins updated to the new contract
  (TestStallKillClosesSilentRelayFlow, TestStallLeanKillStaysLoud,
  TestStallRichEscalationDisabledByConfig). Both-ways: the call
  restored at the kill site → every one of them FAILS on the recorded
  invalidation.
- **Consequences:** a killed flow's retry reuses the cached answer
  set immediately (the demoted IP is skipped by the dial-list
  rotation, not by a re-resolve); the DoH lane's load during
  kill-storms drops to the queries the app itself causes.
- **What this ADR does NOT decide:** the DNS cache's TTL or the
  negative-cache re-ask (M8-z stands).

## ADR-0039 — Auto starts at LEGACY: the strength ladder is walked
DOWN, not up (M8-ad)

- **Status:** Accepted 2026-10-20 (M8-ad; the tester's own words —
  "read the backend script again, maybe it gives you ideas, because
  the problem is not solved yet (video lags to begin)" — the legacy
  Python's 299-cut hello is the shape his network proved for years).
- **Context:** auto mapped to balanced (80-200-byte pieces, ~4-6
  packets for a real ClientHello) since the rewrite, and
  `HealSweepOrder` tried legacy LAST — the strongest shape we ship
  was effectively unreachable. Meanwhile the regime changed (the
  debug.20/21 reports): probe reset verdicts 0→35 — the censor now
  RSTs recognizably-shaped hellos instead of blackholing IPs. Weak
  splitting is exactly what a reassembling DPI wants to see.
- **Decision:** `presetFor(auto)` returns `fragment.Legacy` (the
  Python rule, byte-for-byte: 299 random cuts at 1 ms pause — the
  preset has existed since the rewrite, un-reachable from auto);
  `config.HealSweepOrder` becomes legacy → strong → balanced → light
  so a failed start check tries the STRONGEST shape before conceding.
  The ~300 ms serialization cost rides inside the 2.5 s first-byte
  budget, parallel across the race's candidates, and the DoH lane
  follows the method (its hellos fragment with the same preset,
  exactly as the Python routed DoH through its own fragmented lane).
- **Evidence:** internal/engine/m8ad_test.go TestAutoStartsAtLegacy
  (auto → legacy; the explicit methods keep their direct mapping) and
  TestHealSweepLeadsWithLegacy (the order); the M8-r sweep test
  updated to the new first method. Both-ways: presetFor reverted to
  balanced → TestAutoStartsAtLegacy FAILS.
- **Consequences:** every auto session leads with the most
  DPI-resistant hello we have; a heal sweep can still walk DOWN to
  weaker shapes when legacy genuinely fails; users who picked an
  explicit method are untouched.
- **What this ADR does NOT decide:** the legacy preset's piece rule
  itself (byte-for-byte the Python's, by design); uTLS-style hello
  mimicry (a different tool for a report that shows even 299-cut
  hellos being RST).

## ADR-0040 — Wire discipline I: a verified front dials ONE hello; a round demotes the set once (M8-ae)

- **Status:** Accepted 2026-10-21 (M8-ae; the tester's post-debug.22
  report — "it doesn't even open video after a while. But the feed
  refreshes. probably one of the worst versions" — plus his two
  reference repositories, gfw_resist_HTTPS_proxy and
  WebImmortalGuards, offered as "knowing how this system works").
- **Context:** the reference implementations put ONE fragmented
  ClientHello per CONNECT on the wire, ever (backend.py: one dial per
  flow, DNS cached forever, nothing in the background; the xray
  TLSFrag config: fragment the first packet, done; WebImmortalGuards
  wraps the same Python). Younblock's raced round put FOUR parallel
  hellos on every budgeted flow — primary + FirstByteRaceDials(2) +
  one substitute — and each of debug.22's hellos is now the legacy
  299-cut shape (ADR-0039): ~1200 tiny writes per flow, on exactly
  the healthy flows a per-source SNI flag accumulates on. Every dead
  candidate ALSO called invalidateDNS (a 4-way all-dead round demoted
  the answer set 4x through the DoH lane for one identical effect —
  the debug.20/21 decay's dns=579-846 ms tail).
- **Decision:** (1) the raced round is SKIPPED when the primary's
  remote IP holds a live verified mark (goodMarkActive): the engine
  has SEEN this origin speak, so the extras are pure emission — one
  hello, the serial budget, the salvage machinery if the verified
  origin dies anyway; the duress fan-out returns only when the front
  is unknown or stale. (2) A raced round demotes the answer set
  exactly ONCE (sync.OnceFunc around invalidateDNS at the round's
  death sites); the M8-s terminal keeps its own invalidation.
- **Evidence:** internal/proxy/discipline_test.go
  TestGoodFrontSendsOneHello (the echo origin accepts exactly ONE
  flow connection, the corpse ZERO — the race never launched; the
  gvideoHellos meter reads 2 probes + 1 flow) and
  TestRacedDeathInvalidatesOncePerRound (3 dead candidates →
  invalidateDNS called exactly 2x: the round's one + the terminal's
  one). Both-ways: the good-front condition stashed → the echo count
  FAILS at the fan-out's 3; the once-per-round hunk stashed → the
  invalidation count FAILS at 4.
- **Consequences:** steady-state googlevideo emission drops to the
  reference implementation's ~1 hello per flow; unknown-front flows
  (young sessions, stale pools) keep the full duress shape; the DoH
  lane's churn under all-dead windows halves-to-quarters.
- **What this ADR does NOT decide:** the raced round's fan-out SIZE
  for unknown fronts (FirstByteRaceDials=2 stands, now the duress
  shape only); the emission breaker (ADR-0041).

## ADR-0041 — Wire discipline II: the probe trickle and the emission breaker (M8-ae)

- **Status:** Accepted 2026-10-21 (M8-ae; same report — "the feed
  refreshes" while video never opens is the per-SNI-class flag
  signature: youtube.com keeps serving through the same proxy while
  *.googlevideo.com stays dead, and the one behavior that sustains
  such a flag is the engine's own emission — the app's retry storm
  x the 4-way fan-out x the churn kick's 48-handshake burst, forever).
- **Context:** the origin-probe pass verified up to 48 IPs per pass
  — each a FULL TLS handshake with a real googlevideo SNI under the
  live legacy preset — at engine START, every 5 minutes, AND
  churn-kicked within 10 s of every all-dead flow (M8-ab's answer to
  googlevideo failure was MORE googlevideo volume: exactly backwards
  under a censor that flags scan-like sources). The debug.22 report's
  "worst version ever" is that policy at field scale: early session
  clean (flag accrual), then every googlevideo hello RST'd, the
  engine never going quiet, video never opening again.
- **Decision:** (1) the pass is a TRICKLE: OriginProbePerPass 48→10,
  clamped INSIDE ProbeOrigins (a caller cannot re-create the burst),
  on its own GoogleVideoProbeEvery=15min cadence, decoupled from the
  5-minute DoH-only prewarm (resolver traffic; no googlevideo packet
  ever leaves the phone). (2) The START-of-session pass is DELETED —
  the persisted good pool (ADR-0033) and flow-fed marks (ADR-0031)
  own the cold start; a fresh engine is SILENT on googlevideo until
  the first flow speaks. (3) The churn-kick lane is DELETED
  (ProbeChurnKickDebounce leaves the config surface). (4) The
  EMISSION BREAKER replaces it: GvideoBreakerTerminals=3 googlevideo
  flow terminals inside GvideoBreakerWindow=30 s arm a
  GvideoBreakerCooldown=45 s (plus proportional jitter ≤ 10 s)
  during which googlevideo flows fail FAST at BOTH doors (HTTP
  CONNECT 502 and the SOCKS reply — zero wire, no resolve, no dial)
  and the probe lane stays paused; door-blocks create no terminals,
  so the breaker self-extinguishes and service resumes on the next
  flow. The app's player does in that quiet what the reference
  implementations rely on it doing: re-fetch and rotate to a NEW
  hostname. Non-gvideo hosts are untouched — the feed keeps
  refreshing through the cooldown.
- **Evidence:** internal/proxy/discipline_test.go
  TestEmissionBreakerQuietsTheWire (two terminals arm the breaker;
  the cooldown's googlevideo CONNECT gets its 502 while the corpses'
  accept counts stand PERFECTLY still — zero wire; the same window's
  non-gvideo flow echoes byte-exact; the lapsed cooldown lets the
  next flow dial again) and TestProbePassIsClampedToTheTrickle
  (sixteen reset origins, a caller asking for 999 — the pass probes
  exactly 10, every probe hello counted in the gvideoHellos meter) +
  internal/engine/m8ae_test.go TestWireDisciplineKnobs (the knob
  contract pinned: 10 / 15 min / 3-30 s-45 s, and the churn-kick
  knob gone from the surface). Both-ways: the door stashed → the
  502 becomes a 200 and the wire speaks (FAIL); the clamp stashed →
  16 probes (FAIL).
- **Consequences:** "video never opens again" becomes "video pauses
  ~45-60 s while the app rotates hostnames, then resumes" whenever
  the flag regime is the cause; ordinary per-origin dirt (the
  debug.19 world) never trips the threshold because terminals that
  sparse fall outside the 30 s window; the background emission
  collapses from ~48 handshakes per 5 minutes (plus bursts) to ≤10
  per 15 minutes, rehab-first.
- **What this ADR does NOT decide:** the raced fan-out (ADR-0040);
  the diagnostics' shape (ADR-0042); the legacy preset itself
  (ADR-0039 stands — the round disciplines the COUNT, not the SHAPE).

## ADR-0042 — The gvideo discipline line: the emission is measured (M8-ae)

- **Status:** Accepted 2026-10-21 (M8-ae; every prior round's
  machinery was validated by its own counters while the one number
  that kept growing — how much googlevideo volume the engine emits —
  was never on any report).
- **Context:** the censor's per-source flag accumulates on exactly
  the emission: every flow candidate, every substitute, every probe
  job. debug.22's steady state was ~4 hellos per flow plus bursts;
  the references run ~1 per flow plus zero background. No report
  line ever said so.
- **Decision:** stats.gvideoHellos counts every googlevideo-SNI
  ClientHello the engine puts on the wire (the raced launches —
  per candidate after the write lands, the serial attempts, the
  probe jobs with gvideo names); stats.gvideoCooldowns counts the
  breaker's arms. The diagnostic report gains one line —
  "gvideo discipline: hellos=N cooldowns=M" — read against
  routedFlows on the counters line. Counts only (R21).
- **Evidence:** internal/engine/m8ae_test.go
  TestReportRendersM8aeSections (the line renders in a
  release-shaped report, no hostnames, no IPs) + the proxy tests'
  meter assertions (TestGoodFrontSendsOneHello reads 3 = 2 probes +
  1 flow; TestProbePassIsClampedToTheTrickle bounds the pass's own
  hellos at the clamp). Both-ways: the line stashed → the render
  test FAILS on the missing section.
- **Consequences:** the next report's headline decision number is
  hellos-vs-routedFlows (volume parity with the references is the
  goal) and cooldowns>0 (the breaker catching flag states that
  debug.22 rode into "never opens again").
- **What this ADR does NOT decide:** what the censor's threshold
  actually is (the round's bet is that the references' volume is
  below it by construction — matching them removes the question).

## ADR-0043 — The per-flow evidence: kernel TCP snapshots, the TLS record timeline, the winner class, and the environment lines (M8-af)

- **Status:** Accepted 2026-10-21 (M8-af, debug.24, spec W1/W2/W3/W9 —
  the prove-before-change round; the external engineer's review of the
  ENGINEERING-CONSULTATION document ruled the cause still unproven and
  the sizes suggestive only, and this ADR is the measurement half of
  the answer).
- **Context:** six rounds of field reports fixed everything around the
  media-body starvation while the starvation itself never changed, and
  the debug.23 consultation's §2.4 named the reason precisely: no
  measurement in the engine could distinguish "handshake completed,
  zero application data" from "a small HTTP error" in the 2.8 KB
  corpses, no line ever said WHETHER the request was even ACKed, and
  no report ever named the phone model or the operator (the reference
  tuning tables are per-ISP: MCI 80-250 fragments, Irancell 10-40).
- **Decision:** every budgeted googlevideo flow now carries evidence
  onto its own lines. (W1) The kernel's TCP_INFO is read twice on the
  origin socket — snapshot A through a write-wrapper at the exact
  moment the client's first post-handshake bytes go out (the request;
  its ACK lands an RTT later, so A is the pre-request baseline), and
  snapshot B at every flow end (relay unwind, stall kill, terminal —
  always before the close takes the fd's counters), both rendered on
  the done and stall-killed lines; each stalled googlevideo flow is
  classified (netx.ClassifyTCPVerdict: acked frozen + retrans/unacked
  grew = upstream-lost — the request never reached the server, the
  censor drops outbound data after the handshake; acked moved + rcvd
  frozen = downstream-lost — the request arrived, the answer is being
  dropped or throttled; both moved = both-ok; anything else unknown,
  never a guess) and aggregated in the report's `tcp verdicts:` line.
  A failed read logs `tcpinfo=unavailable reason=…` once per device
  and the flow then counts unknown — non-Linux platforms keep the
  no-op (desktop behavior unchanged; hard constraint 2). (W2) A light
  5-byte TLS record header parser (no decryption) watches the RELAYED
  bytes of both directions — the first 12 server→client and 8
  client→server records as `type:length@ms` on the done line, plus
  `first-app-byte` (the first server record after the client's request
  record: the corpse question — handshake-only vs HTTP error vs first
  media slice — goes from inferred to measured); direct deliveries
  (the raced winner's first bytes, the serial first byte) feed the
  same tracker so the timeline is whole. (W3) `win=orig|subst|race`
  rides every googlevideo done line — the A2/A3 substitution
  false-win cycle becomes visible per flow. (W9) the fragmenter
  counts every TCP segment it writes (fragment.PiecesTotal — hello
  COUNT was measured since debug.23, hello PACKET count never was;
  one legacy hello is up to 300 segments) rendered as `fragPackets=`
  on the discipline line, and the network test (ADR-0044) prints
  per-method deltas. The Android facade adds `device:` (Build fields,
  no unique identifiers) and `net:` lines (permission-free
  TelephonyManager getters + a shell-owned wifi/cellular heuristic
  recorded here as a heuristic — ConnectivityManager would need
  ACCESS_NETWORK_STATE, which the manifest deliberately does not
  hold). R21 stays: addresses in the network test table and the
  report render redacted (`addrN`, `<redacted>`) unless the debug
  tester build's LogHostnames is on.
- **Evidence:** internal/proxy/tcpmon.go (the holders), internal/netx/
  tcpinfo{,_linux,_other}.go (the snapshot + classifier), the wiring
  in internal/proxy/tunnel.go (pumpClient's request hook, relayEntry's
  read/write wrappers, every winner/demote/terminal site), stats
  TCPUpstreamLost/DownstreamLost/BothOK/Unknown, and the pins:
  internal/proxy/tcpmon_test.go
  TestGvideoFlowEvidenceOnDoneAndStallLines (the loopback corpse:
  `win=orig tcp A[acked=…] B[acked=… rcvd=…] rec s2c=[22:96@0]
  c2s=[22:1024@25 …] first-app-byte=none`, verdict
  downstream-lost), TestRecordTrackerParsesAcrossChunks,
  TestFirstAppByteRule, TestWinnerClassRidesTheDoneLine,
  TestFragmentPiecesCounted; internal/netx/tcpinfo_linux_test.go
  (the live-socket snapshot exchange + the classifier's six cases);
  internal/engine/m8af_test.go TestReportRendersM8afSections (the
  `tcp verdicts:` and `fragPackets=` lines render release-shaped,
  no leaks); mobile/report_test.go (the device:/net: lines). All
  both-ways: the doneSuffix wiring stashed → the evidence test FAILS
  on the missing fields; the classifier's acked-comparison stashed →
  the verdict pins FAIL; the tracker's desync guard stashed → the
  parser test FAILS on garbage tails; the piece counter stashed →
  TestFragmentPiecesCounted FAILS at zero delta.
- **Consequences:** the next report's decision numbers are the tcp
  verdicts split (the §7 decision table's second and third rows),
  first-app-byte (none = the request never got any answer — the
  corpse confirmed at the record layer), and win=subst flows' byte
  totals (A2 measured). The kernel-counters path is loopback-proof
  and race-clean, and the stalled-fixture echo bug the round exposed
  in the test herd (a single-read echo racing the fragmented hello's
  pieces — a ~10% flake all along, near-deterministic under -race
  once the evidence wrappers shifted timing) is fixed at the fixture
  (echo the whole first flight, byte-budgeted) — the real corpse's
  handshake completes, and now so does the fixture's.
- **What this ADR does NOT decide:** what the verdicts will say (that
  is the tester's report's job), and no behavior changes ride on the
  measurements yet — the censor-facing behavior changes of this round
  are ADR-0045's, decided on the consultation's own evidence.

## ADR-0044 — The experiment switches, the media-liveness probe, and the one-button network test (M8-af)

- **Status:** Accepted 2026-10-21 (M8-af, debug.24, spec W4/W7/W8 —
  the same-minute preset comparison and the media-capability
  certification the consultation's §2.3 open matrix demanded as items
  1-3).
- **Context:** every method comparison so far rode a DIFFERENT build
  at a different hour (preset vs hour-of-day confounded since
  debug.16's last good media report — 14.6 MB on `strong`, 12:53
  Tehran), substitution has never been field-tested as an isolated
  variable, and every "verified good" verdict the engine ever wrote
  certified a TLS handshake only — nothing had ever asked a
  googlevideo address for actual video bytes.
- **Decision:** (W4) the diagnostics screen gains an Experiments
  section: the method selector (legacy/strong/balanced/light — the
  existing presets, applied live through the existing settings path),
  the substitution and breaker toggles (settings v6
  `advanced.experimentSubstitution`/`experimentBreaker`, both shipping
  ON, migrated on for pre-v6 files exactly the v5 pattern — absent
  fields are the shipped default, never a choice), all three printed
  at the top of every report as `experiment: method=… substitution=…
  breaker=…` so each paste says which world its numbers came from;
  the proxy's SetExperimentFlags applies them to the NEXT flow (same
  sitting, same network — the A/B the consultation demanded), and
  disabling the breaker clears an active cooldown (off = the exact
  debug.22 shape). (W7) internal/probe is the media-liveness probe:
  dial the candidate address through the engine's one netx door
  (protect applies), TLS handshake with a REAL googlevideo SNI
  through the same fragmenter a flow uses, ALPN http/1.1, then a
  deliberately bad `GET /videoplayback` — ANY HTTP status line (even
  a 400) proves the address carries HTTP, therefore media; the
  classes are answered / silent-after-handshake (the corpse shape,
  measured directly) / hello-silent (the blackholed-address class,
  including dial timeouts) / reset / error. "Run network test" — the
  diagnostics screen's button, operator-pressed, never a timer —
  probes up to 6 CURRENT cluster addresses (the freshest learned
  code's rr1..rr5 resolutions, the redirector fallback when no codes
  are learned) under ALL FOUR methods in the same minute, then the
  SNI discrimination (real name vs www.google.com vs none) under the
  current method, the i.ytimg.com control per method (the test's own
  sanity row — a failing control means the table is not
  trustworthy), the per-method fragment-packet deltas, and (W8) the
  foreign-cluster lane: config.ExperimentNodes() embeds
  internal/config/experiment_nodes.txt (Ali's yt-dlp -g hostnames,
  resolved on-device through the normal DoH path, probed under each
  line's own real SNI) — the file ships EMPTY with a comment header,
  and an empty file skips the lane silently. The lane runs OUTSIDE
  the emission breaker's door rules (a measurement must not change
  what it measures) but every googlevideo-SNI hello it emits IS
  counted in the discipline meter — the censor sees them, the report
  says so. Two spec readings recorded: the report build itself never
  probes (a synchronous test would hang the report button for
  minutes; the button is the only trigger — the honest not-run line
  says so), and the network test's lines SURVIVE Stop (unlike the
  per-run probe latch) because the tester's paste is the
  measurement's whole value.
- **Evidence:** internal/probe/media.go (+ media_test.go: the 400
  answering server = answered, the handshake-then-silent server =
  silent-after-handshake, RST = reset, dead address never answers —
  all both-ways via the classifier branches); internal/engine/
  networktest.go (the orchestration, the status/run JSON, the
  counts-only ring line — R21); internal/config/settings_v6_test.go
  (the switches' defaults, migration, and the owner's-word shapes);
  internal/engine/m8af_test.go TestNetworkTestGatingAndStatus (NOT_ON
  while off, RUNNING while in flight, the done flip); the webui
  /api/networktest route (GET+POST, one registration — the mux
  panics on doubles); the DiagnosticsView Experiments section; and
  the ADR-0045 tests' primePool (the media-verdict seeding path the
  pool now depends on).
- **Consequences:** the next report carries the §7 decision table's
  primary inputs: answered-vs-silent per method (row 1 — the preset
  or its packet count triggers the censor), the tcp verdicts (rows
  2-3), the alt-SNI split (row 4 — the block keyed on the media
  SNI), probe-answered-but-flows-still-starve (row 5 — the real app
  traffic differs), and the foreign rows (row 6 — cross-cluster
  dialing on media-verified addresses only). The table is redacted in
  release shape (addrN labels) and full in the debug tester build.
- **What this ADR does NOT decide:** which row of the decision table
  the tester's network lands on (that is the report's job), and no
  relay/server option exists (§9: that decision is Ali's after the
  report).

## ADR-0045 — The breaker's sensor sees pending kills; the substitution pool eats media evidence only (M8-af)

- **Status:** Accepted 2026-10-21 (M8-af, debug.24, spec W5/W6 — the
  round's two behavior changes, both decided on the debug.23
  consultation's own measured evidence, not on the new
  instrumentation).
- **Context:** A4 (the consultation's provable-from-the-log defect):
  the emission breaker armed on first-byte terminals only, and the
  debug.23 session died 13 media flows by STALL KILL with
  gvideoTerminals=0 and cooldowns=0 — the breaker watched the wrong
  death class the whole evening while the flows it should have
  silenced kept feeding any per-source flag. A2/A3: the substitution
  pool's certification criterion is invalid in this regime — 18
  substitution "wins" (handshake-only evidence), every flow ≤12.1 KB,
  goodDrops=17, the gvideo pool 4→0: a self-consuming buffer of false
  positives that displaced raced slots and bound flows to
  handshake-alive but body-dead origins.
- **Decision:** (W5) googlevideo flows killed by the stall watchdog
  with cause=pending — the client's request outstanding, the exact
  death shape the debug.23 report measured — now feed the emission
  breaker's terminal ring exactly like first-byte terminals
  (Server.feedGvideoBreaker, shared by failFlow and every pending
  branch of watchStall; lean and rich tiers alike, never idle, never
  trickle — an idle kill is ambiguous by ADR-0036's own reasoning).
  The gvideoTerminals counter's meaning widens with it: it counts
  every ring feed (the breaker's own view of googlevideo deaths).
  (W6) the good pool's gvideo flag becomes MEDIA-VERIFIED evidence:
  only the network test's answered verdicts (ADR-0044's probe, an
  HTTP status line from a real-SNI GET /videoplayback) may write it
  (Server.MarkMediaVerifiedIP); first-byte wins and handshake-only
  probe verdicts still PREFER (front of the dial lists, liveIPs
  unchanged) but never feed the substitution pool; the persisted
  pool file keeps its name and gains the going-forward content rule
  (media-verified only), while SeedGoodIPs seeds NON-gvideo marks —
  every file written by an earlier round was certified by first-byte
  wins (exactly the false-positive class W6 removes) and the loader
  cannot distinguish an old file's lines from this build's writes,
  so the seed stays a preference and the pool starts every session
  empty until the network test re-verifies. The flip condition is
  recorded here: a versioned file format that can say
  "media-verified" per line may restore gvideo-seeding.
- **Evidence:** internal/proxy/tcpmon_test.go
  TestStallPendingFeedsBreaker (three pending kills inside the window
  ARM the breaker, the door's 502 with zero wire on the fourth flow —
  both-ways: the feedGvideoBreaker calls stashed, the test FAILS on
  cooldowns=0, the kills themselves unchanged);
  TestExperimentSwitches (the W4 toggles gating both behaviors live);
  the rewired internal/proxy/substitution_test.go shape 3
  (TestFlowSuccessFeedsTheSubstitutionPool: a successful flow
  prefers but the pool stays EMPTY, the later all-corpse flow
  TERMINALS with zero substitution attempts, and only
  MarkMediaVerifiedIP arms the substitute that saves the flow after
  it — both-ways: the raced-winner markGoodIP(ip, false) stashed, the
  test FAILS on the flow-fed gvideo pool); primePool in the same file
  (the probe verdict prefers, the media verdict feeds); the rewired
  M8-ab persistence round-trip (seeds render `pool=2 (gvideo 0)`, a
  media-verified mark round-trips through Stop — with the pool flush
  made synchronous in stopGvideoLoop, the mark no longer depends on
  the loop goroutine's ctx.Done flush racing the teardown).
- **Consequences:** with W6, `substitution: attempts=0` is the
  expected healthy reading until the network test certifies addresses
  (the pool measures the probe now, not the flow machinery); with W5,
  `cooldowns>0` finally becomes reachable in the corpse regime — the
  field question the ADR-0041 design wanted answered (do quiet
  windows + the player's own hostname rotation recover playback?)
  becomes askable for real. Both changes are scoped to googlevideo
  flows exactly like their M8-ae/M8-ab ancestors; non-gvideo hosts
  and every desktop keep byte-for-byte behavior (pinned by the
  existing herd).
- **What this ADR does NOT decide:** whether the breaker's quiet
  windows actually recover playback (the next report's cooldowns>0
  says whether it fires; the tester's play observations say whether
  it helps), and whether any address CAN be media-verified on the
  tester's network (the network test's answered column says so — the
  two ADRs share the field answer).

## ADR-0046 — The breaker stops starving the player: the pool escape, the scout, the lift, and a 15 s cooldown (M8-ag)

- **Status:** Accepted 2026-10-21 (M8-ag, debug.25 — decided entirely on
  the debug.24 field report, the first report with W5's fixed sensor
  and W6's media-verified pool running live).
- **Context:** the report inverted ADR-0041's own contract. The player
  pressed play; the app's burst raced googlevideo terminals from a
  dead cache cluster (the whole `sn-qxau5` set: flows=3 failures=3
  worstFirstByte=0 ms each); the W5-fixed sensor finally saw those
  stall kills, three landed inside the 30 s window, and the breaker
  armed — correct, as designed. Then the cost: ONE 45 s+jitter
  cooldown covered the rest of the session (`cooldowns=1`), the two
  doors refused every googlevideo dial (58 `door dial failed` pairs,
  zero wire), the player spent its whole retry budget against the door
  (last googlevideo attempt +46 s, then the app stopped asking), and
  the video never opened while the feed kept refreshing — the exact
  "video never opens again while the feed keeps working" outcome the
  breaker was built to PREVENT. Meanwhile the wire was proven healthy
  twice in the same report: the network test answered 5/5 addresses
  under the real SNI (media-verifying the pool to `pool=6 (gvideo 5)`),
  and the one media flow that slipped through before the arm delivered
  406 KB (`win=orig`, first-app-byte=77 ms). The breaker starved a
  healthy wire behind its own door — and it starved the substitution
  pool with it: `attempts=0`, because the door-block short-circuits
  BEFORE the raced round where the substitute lane engages.
- **Decision:** four changes, all inside the breaker's existing
  scope (googlevideo hosts at the doors + the cooldown's budget; every
  other SNI class and every desktop shape untouched). (1) THE POOL
  ESCAPE: `gvideoDoorBlocked` passes a flow when
  `goodSubstitutes(nil, 1)` finds a fresh, unboxed, media-verified
  origin — an address that ANSWERED under the real SNI is the
  strongest wire-alive evidence the engine owns, and blocking the flow
  it can serve is pure loss; the flow then rides the existing
  substitute lane (which the pass actually reaches, because the door
  no longer short-circuits it). Respects the W4 switches and knob
  exactly: breaker-on with substitution off keeps the pure M8-ae
  shape. (2) THE SCOUT: each cooldown hands its FIRST googlevideo
  flow to the wire (`claimGvideoScout`, once per arm, reset on every
  arm) — the app's own request, zero synthetic emission — so an armed
  breaker still learns the flag state it guessed; against a dead
  cluster the scout terminal normally (feeding the ring, never
  re-arming: in-cooldown feeds extend nothing), and against a live
  one it delivers video. (3) THE LIFT: any googlevideo first byte —
  every path funnels through `logTunnelFirstByte` (serial, raced
  winner, salvaged, substituted, plain relay) — ends an active
  cooldown early (`liftGvideoBreaker("first byte")`): a speaking wire
  falsifies the arm's premise for right now. The terminal ring stays
  untouched; if stalls resume, fresh terminals re-arm exactly as
  before. The breaker becomes self-correcting in both directions
  (silence keeps it armed, speech lifts it). (4) THE BUDGET:
  `GvideoBreakerCooldown` 45 s → 15 s — the debug.24 report measured
  the player's retry budget dying inside ONE 45 s cooldown, so the
  block must never outlive it; 15 s still spans the app's measured
  2-7 s retry cadence (2-6 blocked hostname rotations per cooldown)
  and the jitter shape (a quarter, capped at 10 s) is unchanged. Two
  counters ride the discipline line so the next report can tell the
  worlds apart: `lifts=` (a first byte ended a cooldown early) and
  `scouts=` (flows the door let probe); the line reads
  `gvideo discipline: hellos=… cooldowns=… lifts=… scouts=…
  fragPackets=…`.
- **Evidence:** internal/proxy/discipline_test.go
  TestEmissionBreakerQuietsTheWire, rewired to the new contract and
  extended: the corpse fixture arms the breaker, the FIRST post-arm
  flow is the scout (dials once, terminals, feeds the ring —
  cooldowns stays 1), the SECOND gets the door's 502 with the corpse
  counts standing perfectly still, a media-verified
  `MarkMediaVerifiedIP` mid-cooldown lets the NEXT flow through the
  SAME active cooldown (the pool escape), a non-gvideo host sails
  throughout, and the lapse serves again; internal/proxy/
  tcpmon_test.go TestStallPendingFeedsBreaker, rewired: three pending
  kills arm, the scout dials the live echo, its first byte LIFTS
  (the log line, `lifts=1`), and the very next flow passes on the
  lift, not the lapse. Both-ways proofs EXECUTED (stash → FAIL →
  restore): the scout claim stashed → the rewired shape-3 test FAILS
  on the scout's CONNECT getting the door's 502; the pool check
  stashed → FAILS on the pool-primed flow's 502; the lift hook
  stashed → the shape-2 test FAILS on the missing lift line. The knob
  pin: internal/engine/m8ae_test.go TestWireDisciplineKnobs wants
  15 s (it FAILED on the unchanged test suite at 45 s, the honest
  both-ways for the default). Full 19-package suite + mobile green;
  -race green on the house gate (proxy + probe + netx);
  `gofmt -l` identical to HEAD's list. The one -race flake
  investigated and dismissed as pre-existing:
  engine TestNetworkTestGatingAndStatus rides a REAL-network DoH
  failure (~3 s) against its 5 s poll — HEAD flakes at the same rate
  (27 pass / 3 fail over 30 HEAD runs vs 10/12 on this tree), which
  is why the house -race gate never included engine.
- **Consequences:** the debug.24 failure mode is unreachable by
  construction: an armed breaker can no longer sit blind through the
  player's whole retry budget (15 s cap), can no longer starve a
  media-verified pool (the escape), and can no longer stay armed
  against a wire that just spoke (the lift). The A/B levers survive
  intact: breaker-off stays the exact debug.22 shape, and
  breaker-on-with-substitution-off keeps the pure M8-ae door (plus
  the scout — one flow per cooldown is the new floor, the price of
  never being blind). The censor-facing emission budget per cooldown
  rises from zero to at most one scout round plus whatever the pool
  serves — bounded by the app's own demand, never the M8-ae spiral
  the churn kick rode. The next report reads the new discipline
  line: `lifts>0` after a play attempt means the wire spoke through
  an armed breaker (the flag hypothesis falsified in-session);
  `scouts>0` with `cooldowns>0` and playback recovered means the
  scout carried it; `attempts>0`/`wins>0` beside `pool=6 (gvideo 5)`
  means the substitution lane finally engaged through the pass.
- **What this ADR does NOT decide:** whether quiet windows recover
  playback on a genuinely flagged wire (the 15 s cooldown + scout
  makes the question askable per-cycle instead of per-session);
  whether the player's own backoff tolerates even 15 s (the next
  report's play timeline says so); and whether the scout's single
  raced round is emission the censor counts — the discipline line's
  hellos= read against routedFlows answers it the same way it always
  has.


## ADR-0047 — The dead-cluster memory: cluster-keyed blackhole detection, the payload-fed pool, and the multi-cluster arm gate (M8-ah)

- **Status:** Accepted 2026-10-21 (M8-ah, debug.26 — decided entirely on
  the debug.25 field report, the first report carrying M8-ag's
  lifts=/scouts= telemetry).
- **Context:** "every video takes initially 1-2 minutes to open, after
  opening it's smooth." The report decomposed the 1-2 minutes exactly:
  the whole sn-qxau5 cluster is BLACKHOLED — rr3..rr7 across
  btqd/btqy/btql/btqs, 21 flows, every one worstFirstByte=0 ms (the
  TCP dial accepts, no TLS byte ever returns), each costing its full
  ~3 s first-byte budget — while sn-aj4g55, sn-u14g55 and sn-4g5ednkd
  answered instantly (first bytes 453-570 ms) and carried 1.8 MB /
  559 KB / 406 KB. The one-shot dead-origin probe confirmed the
  class: the penalized qxau5 IPs stayed silent under EVERY SNI — an
  IP-range blackhole, not an SNI flag. And the session's own machinery
  stretched the recovery: cooldowns=7 (× ~15 s ≈ the whole open
  latency — the breaker re-armed cycle after cycle on the SAME dead
  cluster), one arm even blocked a live-cluster dial (r5-aj4g55 at
  00:10:33, connected at 00:10:43). The M8-ag escapes worked as
  designed (scouts=6 — the scout flows literally carried two videos,
  lifts=2) but the pool escape never engaged: pool=13 (gvideo 0) —
  W6 left the network test's operator-pressed button as the pool's
  only food, and it was never pressed, so substitution ran at
  attempts=0 while the app's own scout flows moved megabytes past the
  empty table.
- **Decision:** four changes, all keyed on the measured blackhole
  signature. (1) THE DEAD-CLUSTER MEMORY: the cluster is the sn-XXXX
  token of the hostname (rr6---sn-qxau5-btqs and rr5---sn-qxau5-btqs
  and rr3---sn-qxau5-btqd all key to sn-qxau5); three DISTINCT hosts
  of one cluster silent-terminaling (the failFlow reason=timeout
  class only — resets and errors are different censor classes) inside
  a 10-minute window mark the CLUSTER dead for 10 minutes
  (noteGvideoDeadHost / gvideoClusterTable). At the door, a
  dead-cluster host fails FAST (the instant 502, zero wire, no 3 s
  burn — the app's hostname rotation cycles in ~0 s) unless the
  media-verified pool serves the flow or the cluster's periodic probe
  is due (one door pass per cluster per 2 minutes, probedAt
  initialized at mark time so a fresh mark is never immediately
  probed). An answered OWN dial (serial first byte, raced own-set
  winner, salvage winner — never a substitute, whose answer says
  nothing about the dialed cluster) rehabilitates the cluster at
  once. (2) THE PAYLOAD-FED POOL: a googlevideo flow that DELIVERED
  PayloadVerifiedMinimum (64 KB — 5x above the debug.23
  handshake-only false-positive class at ≤12.1 KB, 6x below the
  smallest measured real flow at 406 KB) certifies its origin
  media-verified (logTunnelDone → MarkMediaVerifiedIP, the origin
  read from the evidence holder — substitute or primary alike): the
  pool feeds itself from the app's own traffic, no button, no
  synthetic probes; a wrong mark self-corrects through the same
  demote-on-death guard every pool entry carries. (3) THE
  MULTI-CLUSTER ARM GATE: the breaker's ring stamps carry their
  cluster and the arm needs terminals from TWO distinct clusters —
  a single-cluster death burst is the blackhole signature cluster
  memory owns, while the per-source flag the breaker exists for
  bleeds across clusters; a dead-marked cluster's terminals still
  COUNT (gvideoTerminals stays honest) but never feed the ring. (4)
  THE POOL FILE'S VERSIONED FORMAT (ADR-0045's recorded flip
  condition, implemented): GoogleVideoGoodIPsFile lines are "<ip>
  media" (this build's media-verified writes) or bare "<ip>"
  (pre-debug.24 files, first-byte-certified); the loader seeds media
  lines into the POOL (SeedMediaVerifiedIPs) and bare lines as
  preferences (SeedGoodIPs, unchanged) — so after one session in
  which any video played, every later session's FIRST video rides the
  verified origin instead of re-paying the dead-cluster discovery.
- **Evidence:** internal/proxy/discipline_test.go
  TestClusterMemoryFastFailsTheDeadCluster (the round's contract pin:
  three distinct corpse hosts of sn-zzz mark the cluster dead; the
  fourth host gets an INSTANT 502 (<500 ms measured) with the corpse
  counts standing still; MarkMediaVerifiedIP passes the next flow
  through the SAME dead mark; the fifth host's answered own-dial
  REHABILITATES while its 8 KB echo delivery media-verifies the
  origin by payload, pool gvideo ≥ 2); TestEmissionBreakerQuietsTheWire
  and tcpmon_test.go TestStallPendingFeedsBreaker rewired to arm
  across TWO clusters (their single-cluster shapes FAILED on the
  unchanged suite the moment the gate landed — the arm gate's
  natural both-ways); the M8-ab persistence round-trip extended a
  session-2 leg (the flushed "192.0.2.55 media" line seeds
  pool=1 (gvideo 1), the legacy bare lines still pool=2 (gvideo 0)
  in session 1). Both-ways proofs EXECUTED (stash → FAIL → restore):
  the door's cluster branch stashed → the fast-fail flow got a 200
  dial; the payload feed stashed → the media-verified-by-payload line
  missing; noteGvideoDeadHost stashed → the cluster never marked;
  SeedMediaVerifiedIPs' gvideo flag stashed → session 2 rendered
  gvideo 0. Full suite green; -race green on the house gate (proxy +
  probe + netx); gofmt -l identical to HEAD's list (the edit-tool
  space-ification caught again by the list diff and restored). The
  dead-cluster ring-feed exclusion (an explained death is not flag
  evidence) has no isolated pin — the arm gate subsumes it in every
  observable path; recorded here as the round's honest gap.
- **Consequences:** the 1-2-minute cold start decomposes. The
  discovery wave (3 × 3 s) is followed by INSTANT door refusals
  instead of 15-second cooldown cycles (the arm gate keeps the
  breaker out of single-cluster blackholes entirely); the app's own
  rotation reaches live clusters in seconds; the first live flow
  fills the pool by payload, and every later dead-cluster dial rides
  the substitute; after a restart the seeded pool serves the first
  video of the session too. The censor-facing cost: fast-fail 502s
  raise the app's short-term retry rate toward the dead cluster
  (bounded by the app's own retry budget; the wire cost per attempt
  drops from one fragmented hello + 3 s to zero), and the probe adds
  one dial budget per dead cluster per 2 minutes. The report gains
  `gvideo clusters: dead=… fastFails=… probes=… rehabilitated=…
  payloadVerified=…` — dead≥1 with fastFails>0 says the memory
  engaged; payloadVerified>0 beside pool (gvideo N>0) says the pool
  fed itself; attempts/wins>0 says the substitute lane finally
  carried flows; cooldowns≈0 in a single-cluster regime says the arm
  gate holds. The breaker survives intact for the regime it exists
  for (multi-cluster bleed), and the A/B switches keep their
  meanings.
- **What this ADR does NOT decide:** whether 64 KB is the right
  payload threshold in the field (a wrong mark self-corrects, but the
  next report's payloadVerified= read against attempts/wins says
  whether the class split cleanly); whether the app's faster retry
  cadence against a fast-failed cluster has any censor-facing cost
  the report can see (hellos= against routedFlows answers as always);
  and whether the qxau5 blackhole is stable across days (the probe's
  rehabilitated= count and the next session's discovery cost say so).

## ADR-0048 — The pool keeps its food: media certification becomes non-downgradable, and the verdict lane stays fed (M8-ai)

- **Status:** Accepted 2026-10-21 (M8-ai, debug.27 — decided entirely
  on the debug.26 field report, the first report carrying M8-ah's
  dead-cluster/payload-fed telemetry).
- **Context:** "I think we're closer than ever, this is probably our
  best so far… first video still buffers (less than before), later
  videos mostly open nearly quickly, some take a little longer." The
  M8-ah machinery measured WORKING: dead=1 (the sn-qxau5 memory
  engaged), payloadVerified=6 (the pool fed itself from real
  traffic), cooldowns=1 lifts=1 scouts=1 (the breaker and its
  escapes nearly silent — the arm gate holds), attempts=14 wins=12
  (the substitute lane carried flows). What remained was the app's
  dead-cluster LADDER: fastFails=150, doorFailures=150 — the player
  ground its hostname rotation against instant resets for ~40 s per
  unlucky open, because the verdict lane (the door's pool pass that
  answers a dead-cluster flow on live wire) STARVED mid-storm: pool=13
  → the two payload-fed gvideo marks were gone by the storm's middle,
  with goodDrops=16 and penalized=4 strikes=4 beside them. The
  drain had three mouths, all found in the code this round and one
  of them by the round's own test before the field ever saw it:
  (a) a substitute candidate's natural death demoted its pool origin
  (markDeadIPTTL's dropGoodIP — the M8-ab rule, written when pool
  entries were handshake-certified guesses); (b) a pending stall
  kill on a substituted flow demoted the origin it rode
  (tunnel.go's dropGoodIP); and (c) — the biggest, found when
  TestSubstitutedFlowsClassifyBySize's second flow lost its
  substitute — markGoodIP(ip, false) on every first-byte WIN
  REPLACED the winner's good entry, so a media-certified origin was
  UN-FLAGGED by the very next flow that used it: its own cluster's
  next dial, or a substituted verdict flow riding it. Six payload
  certifications in, the session ended holding two — the pool was an
  outflow machine. Two more readings retired old hypotheses: the
  substituted flows' 12 wins all delivered ~5 KB answers (the
  debug.23 report measured the identical shape — 18 wins, every
  flow ≤12.1 KB — on a pool of false positives; now the same shape
  on TRUE positives says the cross-cluster substitute is the
  fast-verdict lane, not a media carrier: every payloadVerified
  event rode an own-dial win=orig flow), and the two
  "udp443 fallback elapsed" samples (8.3 s / 23.9 s) are heuristic
  pairings (a drop beside the NEXT TCP flow to the same host,
  whenever the app's ladder got there) — NOT the app's QUIC timeout
  (gVisor's ICMPv4 type 3/code 3 for udp443 is already on the wire,
  pinned by the M8-j test; no QUIC work is warranted).
- **Decision:** four changes, all in service of one property — a
  media certification is the strongest evidence the engine owns
  (a ≥64 KB delivered payload or an answered real-SNI probe
  verdict, the only two writers), so it is revocable ONLY by its
  15-minute TTL. (1) THE NO-DOWNGRADE RULE (markGoodIP): a
  handshake mark on a media-certified origin refreshes the
  preference and KEEPS the flag — first-byte wins and probe
  handshake verdicts stop un-flagging the origins that carry them.
  (2) THE DEMOTE GUARD (dropGoodIP): death demotes (substitute
  race losses via markDeadIPTTL, pending stall kills) skip
  gvideo-flagged marks entirely — the penalty BOX with its strike
  escalation (60 s → 2 m → 5 m) is the rotation guard both
  goodSubstitutes and liveIPs already read, so a genuinely dead
  media origin still leaves every dial list; only the TTL revokes
  the certification. (3) THE FEED CLEARS THE BOX
  (MarkMediaVerifiedIP now also calls clearDeadIP): a fresh media
  delivery is direct liveness evidence that outranks the stale
  death that boxed it — the rehabilitation write's own contract —
  so a re-certified origin re-enters the lane at once instead of
  sitting invisible through its whole escalated box.
  (4) THE VERDICT LANE MADE VISIBLE: the door's dead-cluster pool
  pass counts (GvideoClusterVerdict → `verdicts=` on the clusters
  line) and logs ("gvideo cluster verdict (pool serves the dead
  cluster's flow)"); every substituted flow classifies at flow end
  by size — down ≥ PayloadVerifiedMinimum is big (real media
  cross-cluster), below is small (the fast 403/redirect verdict)
  — on the substitution line as `big=… small=…`; and both doors
  log the REASON they refused a googlevideo flow
  ("cluster-dead" vs "breaker-cooldown") instead of the M8-ae-era
  message that blamed 150 refusals on a breaker that armed once.
- **Evidence:** internal/proxy/m8ai_test.go, all shapes with
  EXECUTED both-ways proofs (stash → FAIL → restore): shape 1
  TestSubstituteLossKeepsTheMediaMark (a substitute's natural death
  on a media-certified origin: the mark survives, goodDrops stays
  0, the box holds the lane dry, and the next media feed re-opens
  it — with the dropGoodIP guard stashed the test FAILS on the
  demoted mark); shape 2 TestMediaFeedClearsThePenaltyBox (with
  clearDeadIP stashed, FAILS on the still-boxed origin); shape 3
  TestSubstitutedFlowsClassifyBySize (small echo answer → small,
  8 KB carried payload → big + payload feed — with the
  classification stashed, FAILS at small=0; with the no-downgrade
  guard stashed, FAILS on the un-flagged pool: flow B loses its
  substitute and terminals, the exact field starvation in
  miniature); shape 4 TestDoorVerdictLaneCountsAndReasons
  (cluster-dead with an empty pool, the counted verdict pass with
  a fed one, non-gvideo never gated — with the counter stashed,
  FAILS at verdicts=0; with the reason swapped, FAILS on the wrong
  word). discipline_test.go's door pin re-pointed at the honest
  reason line; m8af_test.go's clusters-line pin extended with
  verdicts=0. Full 19-package suite green; -race green on the
  house gate (proxy 82 s + probe + netx); gofmt -l identical to
  HEAD's list (the edit-tool space-ification caught and restored
  again, tunnel.go's HEAD indentation preserved through a
  python-inserted hunk).
- **Consequences:** the storm's shape inverts. With the pool fed
  (marks survive wins, deaths and re-certifications), every
  dead-cluster flow the app sends rides the verdict lane — one
  substitute dial, a fast HTTP answer on live wire — instead of an
  instant reset: verdicts= rises against fastFails=, and IF the
  player retires a URL set faster on a 403/redirect than on
  connection resets (the field question big=/small= plus the
  timeline's "first-byte substituted" lines beside the app's next
  move will settle), the unlucky opens lose their whole reset
  ladder. If verdicts change nothing for the app, the lane costs
  one dial per attempt (bounded by the app's own retry cadence)
  and the report says so honestly — the counters exist precisely
  so the next round decides with numbers. The report reads:
  `substitution: attempts=… wins=… big=… small=… pool=… (gvideo …)`
  and `gvideo clusters: dead=… fastFails=… verdicts=… probes=…
  rehabilitated=… payloadVerified=…`; the doors' WARN lines name
  their gate. The A/B switches keep their meanings (substitution
  off empties the lane exactly as before, the guards are no-ops on
  preference marks).
- **What this ADR does NOT decide:** whether an HTTP verdict
  actually shortens the player's URL-set ladder (the next report's
  verdicts= vs fastFails= timeline answers; if not, the door goes
  back to fast-fail and the lane stays a diagnostic); whether big=
  is ever nonzero in the field (a cross-cluster media carry would
  falsify the mn-binding reading of videoplayback URLs — zero
  across two debug builds so far); and whether 15 minutes is the
  right certification TTL (a session shorter than the censor's
  cluster churn keeps it; the pool file's media lines re-seed the
  next session regardless).

## ADR-0049 — The lane retires: the cross-origin substitution experiment concludes OFF, and the off-switch it never had (M8-aj)

- **Status:** Accepted 2026-10-21 (M8-aj, debug.28 — decided entirely
  on the debug.27 field report, the round ADR-0048 shipped).
- **Context:** "This one did not even open a single video, only feed
  refreshed." The debug.27 report is the cleanest experiment result
  the project has ever had — ADR-0048's open field question ("does an
  HTTP verdict shorten the player's URL-set ladder?") answered with
  numbers: `substitution: attempts=40 wins=37 big=0 small=37` — the
  fed pool kept the verdict lane engaged on EVERY dead-cluster
  attempt, every answer was the ~5 KB verdict shape, and the player
  NEVER left the dead URL set: `rr8---sn-qxau5-btql` alone took 20
  flows, retried every ~4 s for the whole session, and NO live
  cluster was ever dialed (no aj4g55/4g5ednkd/u14g55 rows anywhere).
  The session moved 1.2 MB total (debug.26: 23.1 MB) and
  `payloadVerified=0`. Worse, the lane starved the recovery machinery
  itself: `dead=0` — the cluster was never marked dead, because a
  substituted flow never TERMINALS (the round's winner is the
  substitute; the qxau5 primary's silent death is canceled as a
  teardown artifact) and noteGvideoDeadHost only ever ate silent
  first-byte terminals. In debug.26 the same regime recovered in ~40
  s precisely because the (accidentally drained) lane let the flows
  terminal, the cluster got marked, and the door's fast-fail resets
  let the player's ladder escalate to a fresh player-API fetch. The
  RST is the ladder's escalation signal; the 403/redirect verdict is
  its sedative. Two smaller readings: some substituted flows hung
  after TLS (9 s, first-app-byte=none — a third answer shape: the
  GFE holds cross-cluster requests it will not route), and the
  breaker armed once on those hung flows' pending kills and was
  lifted 1.5 s later by a substituted flow's first byte — the lane
  fed noise into the breaker from both sides.
- **Decision:** the four-round experiment concludes — the cross-origin
  googlevideo substitute NEVER carried media (debug.19: the app
  escaped on its own; debug.23: 18 wins ≤12.1 KB on false positives;
  debug.26: 12 wins ~5 KB on true positives; debug.27: 37 wins ~5 KB
  and a total regression) and is retired at the SETTINGS layer:
  (1) schema v7 — `experimentSubstitution` defaults OFF
  (DefaultSettings) and the v7 migration flips it off on every
  existing file (a v6 file's true is the shipped default recorded
  verbatim, never a choice; a v7 file's explicit true is the owner's
  word and rides); the report's experiment line reads
  `substitution=off`. (2) The off position is REAL now: the M8-af
  engine knob resolved the switch's off to 0 — which the proxy's
  Config defaulting reads as "use the default" (server.go's
  `if cfg.GoodIPSubstitution == 0`), so the switch's off position
  has been a NO-OP at engine start since debug.24 (the live toggle
  worked; a settings-off start never did — the breaker's knob got
  -1 right, substitution's got 0 wrong). goodSubstKnob now returns
  -1, the proxy knob's actual disabled shape. (3) Everything else
  from ADR-0048 stays exactly as shipped: the no-downgrade rule, the
  demote guard, the feed-clears-the-box, the verdict counters, the
  honest door reasons — with the lane retired they keep the pool's
  marks healthy for what the pool still does (liveIPs' dial-list
  preference, the report's pool line) and the counters become the
  tombstone (attempts=0, verdicts=0). The door's dead-cluster shape
  returns to debug.26's: probe → fast-fail, the reset the player's
  ladder escalates on; discovery terminals again feed the cluster
  memory; the breaker keeps its M8-ag escapes.
- **Evidence:** internal/config/settings_v7_test.go (renamed from the
  v6 file, the shapes rewritten): TestExperimentSwitchesRetiredOff
  (default/fresh/pre-v6 all land off/on, version 7) and
  TestExperimentSwitchesV7MigrationAndOwnersWord (a v6 file's
  shipped-on true retires; a v7 file's explicit true rides).
  internal/engine/m8af_test.go's experiment-line pin re-pointed at
  `substitution=off` — the pin that EXPOSED the knob bug: it failed
  against the unchanged engine because the report rendered on from a
  settings-off start. Both-ways proofs EXECUTED (stash → FAIL →
  restore): the v7 migration stashed → the v6-file leg FAILS (loads
  true); the default flip stashed → the default shape FAILS (on/on);
  the knob's -1 stashed back to 0 → TestReportRendersM8afSections
  FAILS (the report renders substitution=on). Full 19-package suite
  green; -race green on the house gate (proxy + probe + netx);
  gofmt -l identical to HEAD's list (engine.go's HEAD space
  indentation preserved through a python-inserted hunk). The
  sandbox was rebuilt from scratch this round (the environment was
  wiped between rounds): Go 1.27.0, JDK 21, Android SDK
  build-tools;36.1.0 + platforms 26/36 + NDK 29.0.14206865, gomobile
  at the go.mod pseudo-version — the build reproduced byte-honestly
  from the repo alone.
- **Consequences:** the app returns to the debug.26 recovery shape
  with everything that round measured working: qxau5 discovery
  terminals mark the cluster dead within ~3 flows, the door
  fast-fails its hosts with zero wire, the player's ladder escalates
  to a fresh player-API fetch on resets, and live clusters carry the
  video (the first video of a dead-set open still pays the
  discovery + ladder cost — the tester's debug.26 experience: first
  video slow-ish, later videos fast — that is the shape this round
  restores, minus the lane that broke it). The A/B lever stays: a
  v7 settings file that records an explicit true re-enables the lane
  at the next engine start (the live toggle's on position needs the
  restart too — the knob resolves at start; recorded here honestly).
  The report reads `substitution: attempts=0 … big=0 small=0` and
  `gvideo clusters: … verdicts=0 fastFails=…` — the tombstone beside
  the fast-fail counts that own the recovery again.
- **What this ADR does NOT decide:** whether the player-API refresh
  path itself can be accelerated (the debug.27 report's youtubei
  flows delivered 361 KB/144 KB responses and still the app stayed
  on the old URL set — whether that was the verdict sedative or the
  app's own URL caching is a question for a round with the lane
  retired and clean resets to compare); whether the dead-cluster
  discovery cost (3 flows × ~2.5 s before the mark) can be shortened
  without reintroducing HTTP answers (a sub-budget terminal would
  starve genuine slow origins); and whether the qxau5 blackhole's
  stability across days changes the calculus (the pool file's media
  lines still seed dial-list preference either way).

## ADR-0050 — The timeline measures video: the auto-timeline re-scopes to googlevideo (M8-ak)

- **Status:** Accepted 2026-10-21 (M8-ak, debug.29 — decided on the
  debug.28 field report, the round the tester called the best yet and
  asked for the method to be documented: doc 11).
- **Context:** "90% of the initial delay of each video is gone…
  still worth investigating the remaining 10-5% delay in mobile,
  which almost doesn't happen in pc and linux." The instrument for
  that investigation — the auto-timeline's play-suspected →
  first-data pairs — cannot currently measure the tester's quantity:
  the play stamp fires on ANY routed flow burst (a feed refresh looks
  exactly like a play wave) and the first-data stamp closes on ANY
  down burst (the debug.28 report's own log shows a 1.4 MB i.ytimg
  feed burst — no video at all — closing a pair). The 1000 ms/pairs=5
  average it reported is a mixture of feed loads and plays.
- **Decision:** both stamps re-scope to googlevideo at the source. A
  play-suspected is a GOOGLEVIDEO flow start after at least 5 s of
  googlevideo quiet (the app only dials googlevideo for
  videoplayback/redirector machinery — feed images and API calls
  never touch it); a first-data is a GOOGLEVIDEO first byte beyond
  the baseline the play stamped at (with the substitution lane
  retired there are no verdict answers left to fool it — a googlevideo
  first byte is a live video flow answering). Two new counters
  (GvideoFlows at logTunnelUp's routed branch, GvideoFirstByte beside
  the breaker lift at logTunnelFirstByte's gvideo branch); the
  burst-bytes knob retires with the old close; the report line says
  what it is now ("gvideo-gated" — R21 keeps the domain word out of
  the release-shaped report, caught by the leak guards on the first
  attempt this round). The manual marks stay authoritative.
- **Evidence:** internal/proxy/m8ak_test.go
  TestGvideoTimelineSignalsAreVideoScoped (a googlevideo flow's start
  and first byte count; a routed www.youtube.com flow — the feed's
  own class — moves neither while RoutedFlows moves for both);
  m8aa_test.go TestAutoTimelinePairsPlayWithData re-driven on the new
  counters. Both-ways proofs EXECUTED (stash → FAIL → restore): each
  tunnel hook, the play-stamp gate, and the first-data gate — all
  FAIL-as-expected / PASS-restored (the play-gate proof needed its
  second attempt: the first stash broke the baseline, not the gate —
  redone against the condition itself). Full suite green; -race green
  on the house gate; gofmt -l identical to HEAD's list.
- **Consequences:** the next report's auto-timeline pairs measure
  play → video data — the tester's "initial delay" — and the residual
  investigation (doc 11 §5) gets its decisive instrument: pair
  elapsed against the fast-fail log timestamps decomposes the
  residual into the app's ladder time vs the engine's setup cost.
  Feed activity no longer stamps or closes anything. The heuristic's
  known limits stay: poll granularity is 500 ms (same-tick answers
  read as ~0), autoplay previews produce their own fast pairs, and
  the buttons remain the authoritative marks.
- **What this ADR does NOT decide:** the residual's owner (the next
  report's gvideo-gated pairs against the fast-fail timestamps say
  whether it is the app's ladder, the per-flow setup cost, or
  buffering); whether the dead-cluster memory should persist across
  sessions (doc 11 §5.3's candidate — a future round with its own
  ADR if the first-video discovery cost proves to own the residual);
  and whether the stall-killer budgets for API hosts should loosen
  (untouched while the counts stay low).

## Decision-change procedure

When changing an accepted decision: new evidence, alternatives
reconsidered, affected files/requirements/tests, user-visible impact;
update this file and the pack README before implementing dependent
work. Gated selections (ADR-0008) switch on their listed criteria with
a measurement note, without a full re-litigation.

## ADR-0051 — The release-candidate polish: the owner's icon and the connect-time hand-off (M8-am)

- **Status:** Accepted 2026-10-21 (M8-am, debug.30, versionCode 29 —
  decided on the owner's two pre-release asks, D104/D105; the round
  after M8-al's verdict closed the residual question and left
  release-readiness standing).
- **Context:** "Before we release, add this: 1. use this for app icon
  [icon.ico]. 2. add this option in setting, checked by default: when
  app connects automatically open android's youtube app and refresh
  it." Two shell-surface asks, no playback change requested — and a
  32×32 source icon too small to upscale, plus a behavior that must
  fire at exactly the moment the tunnel is genuinely routing (never
  before, never twice).
- **Decision:**
  1. **The icon (D104):** `scripts/gen_android_icons.py` traces the
     owner's mark to vector geometry — the 32 px source upscaled 16×,
     snapped to its five flat colors (blend pixels resolve to the
     nearer endpoint of the pair that explains them; the bolt's light
     outline is its own layer; the impact dashes survive at speckle
     20), each color layer binary-traced (vtracer, its translate
     transform BAKED into the coordinates — a naive d-only
     reassembly drops it and lands every shape near the origin), the
     layers assembled in the source's z-order, and every layer gated
     at IoU ≥ 0.90 against the quantized source before anything is
     written. Output: the adaptive mipmap set (dark `#1E1F21`
     background, the mark at the 52/108 safe zone, monochrome layer
     included) plus the legacy raster tiles; the pill glyph becomes
     `drawable/ic_notification.xml` (the status bar tints small
     icons monochrome, where the simple mark reads).
  2. **The hand-off (D105):** settings schema v8 adds
     `autoOpenYouTube` — true on Android (fresh installs and the
     pre-v8 migration; the v5 platform-default pattern), inert on
     desktops. A new `autoOpenTarget` capability (Android only) gates
     the menu sheet's Settings row. The Kotlin service fires the
     hand-off ONCE per connect run, only when a phase event reports
     serving AND `startTun` actually bound (an "on" the shell cannot
     route must not launch YouTube into a dead tunnel):
     `killBackgroundProcesses` for the YouTube package (the refresh —
     a normal install-time permission), then the launch intent with
     NEW_TASK (the open). A degraded→on recovery flap cannot re-fire
     (the latch resets only when the run ends); a restart cycle
     without a fresh serving event is the same run and never re-fires
     (the establishTun already-serving path deliberately does not
     call the hand-off).
- **Evidence:** internal/config/settings_v8_test.go (the four shapes:
  the Android default, the migration, the desktop-inert false, the
  v8 owner's word, the round-trip — with both-ways stash notes);
  mobile/autoopen_test.go (the facade reports the ruling's default,
  then tracks the saved switch both ways);
  internal/platform/android/android_test.go pins the capability
  (both-ways note in-test). The icon: per-layer IoU 0.966–0.994
  printed by the generator, and the committed geometry VLM-verified
  FAITHFUL against the original mark on the adaptive circle, the
  adaptive square and the legacy tile. Full Go suite green (the
  engine's known real-network flake on its documented rerun); -race
  green on the house gate (proxy+probe+netx); `gofmt -l` identical
  to HEAD's list (the space-indented legacy files edited through
  byte-exact scripted replacements, not the editor); the frontend
  `svelte-check` delta zero (5 pre-existing errors in an untouched
  file, verified by stash) and `vite build` clean.
- **Consequences:** the tester's next report should show the hand-off
  working (YouTube opens by itself the moment the state reaches On;
  the logcat carries "auto-open:" lines for the off-setting,
  not-installed and refusal paths). Playback behavior is untouched —
  no proxy, dialer, pool, tier or instrumentation change rides in
  this build, so the blessed debug.28/29 behavior carries into the
  release candidate unchanged. The kill is best-effort: OEMs age
  `killBackgroundProcesses` differently, and a surviving warm
  process means a warm open, never a failure (the CLEAR_TASK
  escalation is recorded in D105 for the round that needs it). The
  icon's trace pipeline is pinned (vtracer's translate bake is the
  load-bearing detail; the version note in the script documents the
  mid-session 0.6.12→0.6.15 drift that motivated the guard).

## ADR-0052 — The warm gate: the hand-off waits for a proven video lane (M8-an)

- **Status:** Accepted 2026-10-21 (M8-an, debug.31, versionCode 30 —
  decided on the tester's own diagnosis and prescription, D106).
- **Context:** the debug.30 field report: "The app reverted to our old
  problem, waiting a lot for initial video to load, which took like
  1-2 minutes to load. other videos, (after initial video) opened
  fine. I think the app connection time is too short, so when it
  automatically switches to youtube app it's not ready yet. the better
  job is to make the user wait and see connecting, only you're sure
  that even initial video is smooth and fast." The code agrees with
  the tester: ADR-0051's hand-off fired at the first serving event
  with the TUN bound — second zero of the tunnel's life, the earliest
  instant the stack exists — and the refresh half killed the YouTube
  app into a cold start that then burst into a chain still paying its
  own per-flow establishment (~500-600 ms per flow, doc 11 §5
  candidate 2) with the dead-cluster memory empty (candidate 3: the
  marks are per-session) and the blackholed cluster's terminal burns
  unmarked — exactly the multi-cluster terminal shape the emission
  breaker's arm gate sums, the 1-2 minute mechanism of the
  debug.18-.25 era. The manual flow the hand-off replaced had carried
  seconds of human navigation the tunnel warmed through; second zero
  removed it.
- **Decision:**
  1. **The engine owns a readiness signal.** State gains `warm`
     (JSON, always present; meaningful while on/degraded): false
     while the serving run is settling, true once the run HELD
     through a settle window (`WarmGateSettle`, 3 s) and one light
     re-check — a plain `QuickCheck` through the door, the same call
     the watchdog makes every 120 s, the same blessed emission
     classes the connect sequence already paid (listener, DoH, one
     youtube flow; ADR-0041's googlevideo silence is untouched) —
     passed. A failing re-check retries on `WarmGateRetry` (2 s); the
     never-trap `WarmGateCap` (15 s) opens the gate regardless — a
     late hand-off is recoverable (the user taps YouTube), a trapped
     one is not. The flip is EMITTED as a state event.
  2. **The shell's hand-off waits for it.** The Kotlin service fires
     kill+launch only when a state event reports serving AND warm AND
     the TUN bound — the conjunction checked on every state event, so
     whichever half arrives first (the bind or the flag), the hand-off
     fires when both hold, still once per run (ADR-0051's latch
     unchanged).
  3. **The user sees the wait.** The connect screen holds its
     connecting presentation while on-but-warming (the word stays
     "Connecting…", the detail reads "Getting the first video
     ready…", the timer and the on-tone wait) on `autoOpenTarget`
     hosts only — the desktops, which have no hand-off to wait for,
     connect exactly as before. The Snapshot gains an optional
     `warming` the HTTP/SSE backend derives from the state's `warm`;
     absent fields never hold (older hosts, previews).
- **Evidence:** internal/engine/m8an_test.go — the hold (warm false
  through the settle window, sampled at half the window with wide
  margins), the re-check (the gate opened only after a QuickCheck
  beyond the connect's own), the emission (a state event carried
  warm=true), the cap (a network whose every check fails still opens
  the gate), the re-arm (off resets warm; the second connect holds
  again). Both-ways proofs, executed and recorded: stashing the
  `startWarmGate` hooks fails all three gate tests (warm never
  flips); stashing the `setPhaseLocked` reset fails the re-arm test
  ("warm=true after off"). mobile/warm_test.go pins the JSON field
  (stash the struct field and the key vanishes). Full suite green;
  -race green on the house gate (proxy+probe+netx); gofmt set
  identical to the baseline 22 (engine.go and the other space-indent
  legacy files edited through byte-exact scripted replacements).
- **Consequences:** the debug.31 report reads the wait (the connect
  screen holds ~4-6 s: the settle window plus one re-check) and then
  the hand-off. The gate adds ONE youtube-step emission per connect —
  the class every QuickCheck already pays. The knob set (settle,
  retry, cap) is config constants with opts overrides, the
  watchdog's pattern. What this ADR does NOT decide: the warm gate
  says nothing about the CLUSTER memory's state — that is ADR-0053's,
  and the two together are what "even initial video is smooth"
  actually means: the chain re-proved AND the blackholed clusters
  already marked.

## ADR-0053 — The dead-cluster marks persist: the pre-registered §5.3 fix, shipped on the debug.30 indictment (M8-an)

- **Status:** Accepted 2026-10-21 (M8-an, debug.31, versionCode 30 —
  doc 11 §5.3's pre-registered candidate fix, convicted into shipping
  by the debug.30 field report, D107).
- **Context:** doc 11 §5 pre-registered this exact fix ("persist
  cluster marks beside the pool file, with a short seeded TTL and the
  existing probe/rehab as the safety net") as the answer to candidate
  3, the first-video discovery cost — and M8-al's verdict shelved it
  as not indicted (the debug.29 session's six 500 ms pairs left no
  room for a multi-second outlier, and the dead mark held from early
  in that session). The debug.30 report re-indicted it: the session's
  first video is precisely the flow the hand-off moved to second
  zero, and at second zero the per-session cluster memory is EMPTY —
  the app's ladder pays ~3 flows × ~2.5 s of dial-accept-TLS-silent
  terminal burn before three distinct hosts mark the cluster, the
  burn is what the breaker's arm gate sums across clusters, and the
  felt result was the old 1-2 minutes. The pool file precedent
  (ADR-0033/0047) already carries the persistence shape.
- **Decision:**
  1. **The marks persist.** `GoogleVideoDeadClustersFile`
     ("googlevideo-dead-clusters.txt", the config directory beside
     the pool file): one sn-cluster key per line, written on the
     gvideo loop's pool tick and Stop-time flush (the same cadence
     and the same goodMu serialization as the pool's saves), capped
     at 16, carrying only marks LIVE at save time.
  2. **The next session seeds from it.** `SeedGvideoDeadClusters`
     plants each key as a dead mark with the shorter
     `GvideoClusterSeedTTL` (5 min, half the in-session
     GvideoClusterDeadTTL — the GoodPoolSeedTTL precedent). From
     second zero the door fast-fails the blackholed cluster's hosts
     at zero wire — the reset the app's own ladder escalates on (doc
     11 §4.2: the fast-fail is the protocol) — instead of burning
     terminal budgets.
  3. **The safety nets the pre-registration named.** A seeded mark
     never SHORTENS a live in-session mark (the fresh measurement
     outranks the stale seed — the evidence-rank rule); the own-dial
     rehabilitation (`noteGvideoClusterAlive`) clears a wrong seed
     the moment one of its origins answers; the per-cluster re-probe
     (`GvideoClusterReprobeEvery`) serves seeded marks on its cadence
     (probedAt stamps at seed time, so a lifted blackhole is learned
     inside one window); invalid keys never enter the table.
  4. **The report says so.** The cluster line gains its seeded half:
     `dead=N (seeded=M)` — dead counts marks THIS session measured,
     seeded counts the file's marks live from second zero. The
     tester's fresh-connect report reads exactly which world it is
     in; the seeding logs one line.
- **Evidence:** internal/proxy/deadseed_test.go — the door mark
  (without a seed `gvideoClusterDead` is false for the blackholed
  cluster's hosts — the debug.30 state; with it, true immediately),
  the rank rule (a seed over a live measured mark plants nothing:
  the until is unchanged, the counter unmoved), the invalid-key skip,
  the rehab clearing a seed, the probe handout waiting out its first
  window, the list carrying only live marks and sorted.
  internal/engine/m8an_test.go — the file seeds the next session
  (the report reads `dead=0 (seeded=1)` and the seeding log line).
  Both-ways proofs, executed and recorded: stashing the seeding hook
  fails the engine pin; stashing the seed body fails the door-mark
  pin. The m8af report pin re-pinned to the new line (the M8-aj
  precedent). Full suite green; -race green on the house gate;
  gofmt set identical to the baseline 22.
- **Consequences:** the debug.31 report, read right after connect
  and before any video, should show `dead=0 (seeded=1)` (or more) —
  the persistence working; then the first video's ladder meets
  instant resets. The stale-seed risk is bounded three ways (the
  half-TTL, the re-probe, the rehab) and costs at most one dial
  budget per cluster per 2-minute window. What this ADR does NOT
  decide: DNS-layer failure for dead-cluster hostnames (§5.1's
  post-release experiment candidate, unchanged) and the good pool's
  own seeding (ADR-0033/0047, already shipped) — the marks and the
  pool together are the "already marked dead and already fed" state
  doc 11 §5.2 claimed, now actually true at second zero.

## ADR-0054 — The owner's logo becomes the icon: the raster recomposition (M8-ao)

- **Status:** Accepted 2026-10-21 (M8-ao, debug.32, versionCode 31 —
  decided on the owner's ruling, D108; the round after M8-an shipped
  the wait-until-ready gate and the persisted marks).
- **Context:** the owner uploaded `younblock-logo-v1.png` (commit
  35dfcc1 — a 449×449 RGBA finished icon design, parked untouched
  for his word in M8-an) and then ruled: "use this picture for app's
  icon." The M8-am icon was a mark TRACED from a 32px icon.ico
  because that source was too small to upscale; this source is the
  opposite problem — a complete full-bleed design: his own squircle
  tile (a ~10% corner radius, a 1px light rim, a top glow band, a
  dark blue-grey gradient) carrying the 3D padlock with the play
  button, the silver open shackle, three red impact sparks, and the
  "younblock" wordmark ("youn" white, "block" red) in the bottom
  quarter.
- **Decision:**
  1. **Recomposed, not redrawn — his pixels everywhere.** His tile
     cannot go on the adaptive canvas full-bleed: the wordmark
     spans 79% of the tile width in the bottom quarter, which every
     launcher mask (the 72dp visible window) would crop away
     entirely, and the content's reach (224.6px of the 224.5px
     half-tile) exceeds what any full-bleed scale keeps whole. The
     art is split the way the adaptive spec intends: the
     **background** layer is his own dark gradient — reconstructed
     as a smooth 2D field (a masked 32px average-pool of the tile's
     107k background samples, nearest-cell fill, bicubic upsample;
     mean fidelity 3.24 RGB units) rendered FULL-BLEED on the
     108dp canvas, so the launcher's mask supplies the shape his
     squircle owns on other platforms; the **foreground** is his
     raster soft-matted against that field — alpha ramps on the
     local difference from the reconstructed background (the art
     solid, his drop shadows semi-transparent, plain background
     fully transparent, so no tile boundary exists to show), the
     2px rim band and the top glow band (tile-edge features, not
     art) killed, the matte scaled z=0.647 with its extent rigidly
     centered (a measured −20px y shift buys 5.6% larger art; his
     arrangement untouched) so every element sits inside the 66dp
     safe circle — the strictest mask guarantee, no
     corner-rounding asterisks; the **monochrome** layer is the
     same matte sharpened to a silhouette at the identical geometry
     (the letter counters survive because the matte follows his
     pixels); the **legacy** tiles are his squircle exactly as
     drawn, one per density.
  2. **The red-detection lesson, recorded:** the padlock body's 3D
     shadow side drops to brightness ~73 — BELOW the background's
     own ~270 — so no brightness threshold can split his red from
     the tile's dark (the first implementation's field averaged
     red into itself and its fidelity gate caught it, mean 19.03).
     The content detector is chroma-based: r−g>40 (the tile's
     background is neutral-blue, r−g≈−1; his red reads 65–174 at
     every shading).
  3. **The M8-am trace retires:** `drawable/ic_launcher_foreground.xml`
     (the icon.ico vector) is deleted, the `launcher_background`
     color entry leaves colors.xml (the background is his gradient
     raster now), and `scripts/gen_android_icons.py` is rewritten
     for the raster pipeline (vtracer/cairosvg leave with the
     trace). The pill glyph stays the notification mark
     (ADR-0051's split: the status bar tints small icons
     monochrome, where the simple mark reads).
- **Evidence:** the generator's gates — every one evaluated on
  in-memory renders BEFORE any file is written: the source shape
  gate (449×449 RGBA, the tile bbox pinned ±2px, sha256
  a93c9c9f… printed), the field fidelity gate (mean 3.24 ≤ 6.0),
  the per-density safe-circle reach gates (32≤33.8, 49≤50.3,
  65≤66.8, 98≤99.8, 131≤132.8 px), the geometry proof (the
  rendered foreground vs the source matte resampled to the same
  geometry: IoU 0.970–0.982 ≥ 0.92 on every density), the
  background opacity + dark-family gates, the foreground/monochrome
  alignment gate, the composed-preview content gates (11,475 red /
  3,119 white px at 432), the legacy tile gates. The visual proof:
  the composed 432px icon VLM-verified on the circle and squircle
  masks — every element (padlock, shackle, sparks, all the
  wordmark's letters) fully inside both masks, the gradient
  seamless, no fringes/halos/ghosts — and VLM-verified FAITHFUL
  against the owner's original (same design, colors, wordmark,
  gradient feel; only the expected mask crop and safe-zone scale).
  The both-ways proof: a tampered source (bright art planted in
  the glow band, where the band-kill rule would silently eat it)
  fails the content-top gate at exit 1 with the res tree
  byte-unchanged and the source restored byte-exact.
- **Consequences:** the tester sees the owner's logo as the
  launcher icon on every mask shape (the wordmark whole on circles
  too), the Android 13+ themed icon carries the same silhouette,
  and engine/playback behavior is untouched — no proxy, dialer,
  pool, tier or setting change rides in this build; the warm gate
  and the persisted dead-cluster marks from debug.31 carry
  unchanged. The pipeline's pinned knobs (EXPECTED_TILE,
  GLOW_BAND, CONTENT_RED) belong to THIS source; a future logo
  upload re-measures them or the gates refuse rather than ship a
  silent corruption.
