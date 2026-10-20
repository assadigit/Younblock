// Package config holds Younblock's defaults and the user settings file.
//
// Everything an owner may want to tune lives in the CONFIGURATION block below,
// each constant with a one-line plain-English comment (file 01, section 4).
package config

import (
	"strings"
	"time"
)

// ====================== CONFIGURATION (edit here) ======================

// AppName — the app's display name; the one place to rename it.
const AppName = "Younblock"

// AppVersion — shown in the UI and reported by --version.
const AppVersion = "1.6.6"

// ListenHost — the proxy listens only on this machine, never on the network.
const ListenHost = "127.0.0.1"

// ListenPort — the default local proxy port (the old Python app's port).
const ListenPort = 4525

// WebUIPort — the default port of the browser control page in the web
// edition (the --web flag). If it is busy at launch the core takes a
// free port instead and prints the real address.
const WebUIPort = 17500

// AutoPort — if the port is busy, try 4526-4545, then any free port.
const AutoPort = true

// DefaultMode — "youtube" sends only YouTube hosts through the proxy,
// "all" sends everything, "manual" leaves Windows untouched.
const DefaultMode = ModeYouTube

// DefaultMethod — "auto" starts with "balanced" and lets auto-heal choose;
// the other values are fixed fragmentation presets.
const DefaultMethod = MethodAuto

// DoHTimeout — how long to wait for each secure-DNS server before trying the next.
const DoHTimeout = 3 * time.Second

// DoHHedgeDelay — the stagger between servers in one hedged DoH walk
// (M8-z, ADR-0025): the sticky/first server is asked at t=0, the next
// at +DoHHedgeDelay, the third at +2×DoHHedgeDelay, and so on; the
// first VALID answer wins and cancels the rest, and a server that
// fails fast triggers the next one immediately. The serial walk this
// replaces charged a full DoHTimeout (3 s) per silent server — the
// debug.16 report's cold lookups at 1055–1806 ms were exactly that
// serial front, and a burst of fresh per-video names paid it at the
// worst moment (the first video's start). Zero in a doh Config → this
// default; negative → no hedging (the exact serial M8-y shape).
const DoHHedgeDelay = 300 * time.Millisecond

// DoHReadIdleTimeout — how long the shared HTTP/2 DoH connection may
// stay silent before the transport health-check pings it (M8-z,
// ADR-0025; golang.org/x/net/http2.Transport ReadIdleTimeout). A dead
// connection is noticed at 15 s + PingTimeout instead of surfacing as
// a slow failure on the next burst of lookups.
const DoHReadIdleTimeout = 15 * time.Second

// DoHPingTimeout — how long the health-check ping may take before the
// DoH connection is replaced (M8-z, ADR-0025; http2.Transport
// PingTimeout).
const DoHPingTimeout = 5 * time.Second

// GoogleVideoCodeCap — how many sn-<code> cache-node identifiers the
// resolver remembers for pre-resolution (M8-z, ADR-0025 §6.4): bounded
// memory, FIFO eviction past the cap.
const GoogleVideoCodeCap = 64

// GoogleVideoCodesFile — the file the learned googlevideo sn-<code>
// list persists to inside the engine's config directory (empty →
// memory-only). The codes are infrastructure node identifiers, not
// browsing history (the same code serves every rr variant of a
// location), and the file lives in the app's private data directory
// beside settings.json and the usage store — DECISIONS.md's privacy
// rules (R21 log redaction, D-entries on user data) are about logs and
// disclosures, not the app's own private state; the ADR records the
// check.
const GoogleVideoCodesFile = "googlevideo-codes.txt"

// GoogleVideoPrewarmEvery — how often the learned googlevideo codes
// are re-resolved in the background while the engine is On (M8-z,
// ADR-0025 §6.4): rr1…rr5 of each remembered code, at most
// GoogleVideoPrewarmInflight lookups in flight, so the next fresh
// per-video hostname often lands on a warm cache entry.
const GoogleVideoPrewarmEvery = 5 * time.Minute

// GoogleVideoPrewarmInflight — the concurrency cap of the background
// googlevideo pre-resolution loop (M8-z, ADR-0025 §6.4).
const GoogleVideoPrewarmInflight = 3

// OriginProbeNames — the STABLE routed names the origin-probe pass
// verifies first, before the googlevideo rr names, every cycle (M8-aa,
// ADR-0028): the player request itself rides youtubei.googleapis.com,
// and the app's first-play wave leans on the same handful of Google
// API fronts every time. Their answer sets get the fragmented-hello
// verification treatment at engine start and on every prewarm cycle,
// so the first flow to each of them dials an origin the engine has
// already SEEN speak. Infrastructure names, never per-user: the probe
// verdicts ride counters only (R21 — no names, no IPs in any log or
// report).
var OriginProbeNames = []string{
	"youtubei.googleapis.com",
	"www.googleapis.com",
	"www.youtube.com",
	"youtube.com",
	"i.ytimg.com",
	"yt3.ggpht.com",
}

// OriginProbeBudget — the per-probe wall budget of the background
// origin-probe pass (M8-aa, ADR-0028): a TCP dial plus one fragmented
// ClientHello plus the wait for the origin's first byte, exactly the
// FirstByteTimeout shape a real flow spends. A probe verdict past this
// budget is "silent" — the same evidence a flow's dead-origin
// classification would produce, bought in the background where no app
// flow is waiting on it.
const OriginProbeBudget = 2500 * time.Millisecond

// OriginProbePerPass — how many answer IPs one origin-probe pass
// verifies at most (M8-aa, ADR-0028): the stable OriginProbeNames
// first, then the learned googlevideo codes' rr names (round-robin,
// M8-ab, ADR-0032) until the budget fills. The pass runs while the
// engine is On, off the app's critical path, on the fast-start
// cadence first and the prewarm cycle after. 48 leaves ~30 slots
// after the stable fronts — a dozen distinct clusters per pass.
const OriginProbePerPass = 48

// OriginProbeInflight — the concurrency cap of the origin-probe pass
// (M8-aa, ADR-0028): at most this many probe connections in flight,
// keeping the background verification polite on a phone's radio.
const OriginProbeInflight = 3

// OriginProbeGoodTTL — how long an origin IP the probe watched ANSWER
// stays preferred in dial lists (M8-aa, ADR-0028): live-first ordering
// for every dial list (both doors' primaries, the raced extras, the
// salvage candidates), refreshed by every pass that re-verifies the
// IP. Three prewarm cycles of stickiness for a verdict that costs a
// handshake to buy.
const OriginProbeGoodTTL = 15 * time.Minute

// OriginProbeDeadTTL — the baseline penalty an origin IP the probe
// watched stay SILENT (or reset) carries (M8-aa, ADR-0028): longer
// than a flow-death's DeadIPPenalty because the probe's evidence is
// direct — the exact wire shape a real flow sends, sent, and never
// answered — and the blackhole is per cache-node IP, stable for far
// longer than a minute. The pass re-runs every cycle; an IP that stays
// silent keeps its mark refreshed, one that recovers is simply
// re-probed clean after it lapses. Still only ever a preference: a
// dial list the filter would empty dials in full.
const OriginProbeDeadTTL = 10 * time.Minute

// DeadIPStrikeEscalation — the penalty TTL tiers for REPEATED dead
// marks on the same origin IP (M8-aa, ADR-0028). The debug.18 report
// measured the re-gamble this ends: the top failing host ran 13 flows
// with 8 failures — the app's retry waves kept re-rolling the same
// answer set faster than the 60 s penalty lapsed, and the fully-dead
// hosts (failures=flows, worstFirstByte=0ms) re-gambled their whole
// boxed set every wave. A first mark keeps the requested TTL (60 s for
// a flow death, OriginProbeDeadTTL for a probe verdict); the second
// strike escalates to 5 minutes, the third and beyond to 30 minutes —
// persistent toxicity stops being re-learned. Never a refusal: an
// all-penalized list still dials in full, and a recovered IP is simply
// re-learned clean once its TTL lapses.
var DeadIPStrikeEscalation = [3]time.Duration{60 * time.Second, 5 * time.Minute, 30 * time.Minute}

// StallLeanTimeoutAndroid — the lean-tier stall budget on Android
// (M8-aa, ADR-0029): 8 seconds, not the desktop's 15. The debug.18
// report's cold start carried THREE lean-tier kills at exactly the 15 s
// budget — flows that had spoken (first byte plus 5.0–7.4 KB of a
// response) and then gone totally silent, the mid-response corpse
// shape — each one holding the app's retry hostage for the whole
// budget before the watch handed the failure back. A lean flow (never
// proved itself: under StallRichBytes, no survived gap) that has
// already spoken and then goes silent for 8 full seconds is dead on
// this network — the same number the plain-relay safety net already
// trusts for a never-spoken origin (PlainFirstByteTimeout) — and the
// app's retry lands on a fresh origin 7 s sooner, 21 s across the
// report's three stalls. Desktop keeps StallTimeout byte-for-byte
// (the platform gate lives in the engine's listener build, the proxy's
// semantics are untouched).
const StallLeanTimeoutAndroid = 8 * time.Second

// PlainFirstByteTimeout — how long a NOT-budgeted (class=plain) TLS
// flow's origin may stay silent after the tunnel wrote the client's
// first record upstream (M8-z, ADR-0024 work item B): a TLS server
// always answers a ClientHello quickly, so silence past this budget
// means the origin is dead or the path is blocked — close both sides
// and count it (plainFirstByteTimeouts) instead of hanging until the
// app's ~60 s patience. Flows that already delivered a byte keep the
// plain idle semantics (IdleTimeout); non-TLS flows never see this
// budget. Budgeted (YouTube-list) hosts keep the FirstByteTimeout
// machinery instead — the safety net is exactly for the plain cohort
// the debug.16 report could not see (a blocked ad/attestation host
// hung silently with no timeout at all). Zero in a proxy Config →
// this default; negative → disabled (the exact pre-M8-z plain shape:
// only the idle timeout).
const PlainFirstByteTimeout = 8 * time.Second

// DebugTesterBuild is the DEBUG TESTER lane marker (M8-z, ADR-0027
// §8.1): build-android.sh sets it to "true" via -ldflags -X (go's -X
// only sets STRING variables — hence the string shape and the
// IsDebugTesterBuild accessor). Only such a build turns LogHostnames
// on by default on Android (with the visible "DEBUG BUILD: hostnames
// included for diagnosis" line in the report); every release build
// keeps hostnames off (R21) — the empty default is the release value,
// so an unset flag can never leak names.
var DebugTesterBuild = ""

// IsDebugTesterBuild reports whether this is the debug-tester lane.
func IsDebugTesterBuild() bool {
	return strings.EqualFold(strings.TrimSpace(DebugTesterBuild), "true")
}

// DNSCacheMinTTL — shortest lifetime an answer is remembered for.
const DNSCacheMinTTL = 120 * time.Second

// DNSCacheMaxTTL — longest lifetime an answer is remembered for.
const DNSCacheMaxTTL = 3600 * time.Second

// DNSCacheServeStale — how long an expired answer may still serve when the lookup fails.
const DNSCacheServeStale = 6 * time.Hour

// DNSCacheNegativeTTL — how long a failed lookup is remembered before retrying.
const DNSCacheNegativeTTL = 15 * time.Second

// DNSCacheMaxEntries — the DNS cache never grows past this many names.
const DNSCacheMaxEntries = 2000

// DNSCacheNegativeReask — how often a host whose negative-cache entry
// says DNS_UNREACHABLE may RE-ASK the servers instead of failing
// instantly (M8-y, ADR-0023). The debug.16 report (the first from the
// right build, versionCode 15) measured the lockout this bounds: dns ok
// 142 / fail 50 with doorFailures=50 — every resolver failure a dead
// door dial — and the failure lines PAIRED at the same millisecond
// (four pairs inside 22 ms), the signature of instant negative-cache
// hits, not server walks (a walk is seconds). One DoH blip
// negative-caches a batch of fresh per-video hostnames (rr*.googlevideo
// names are per-video: no success entry to serve stale), and for the
// negative's whole 15 s TTL every retry wave dies at the door in
// milliseconds — the blip's tail became the 50 dead flows. A
// DNS_UNREACHABLE negative is evidence about the RESOLVER'S OWN PATH,
// not a verdict about the name, so a hit now re-asks the servers
// (through the singleflight — concurrent flows collapse into one walk);
// the next retry after the blip lands its answer in one walk (~0.5–1.5
// s) instead of dying for the rest of the TTL. This knob is the re-ask
// rate limit per host: at most one walk per window during a real
// outage. Zero → this default; negative → disabled (the exact M8-w
// shape: the negative answers until its TTL expires).
const DNSCacheNegativeReask = 1 * time.Second

// DialTimeoutPerIP — longest a single connection attempt to a site's address may take.
const DialTimeoutPerIP = 5 * time.Second

// DialTimeoutOverall — cap for trying all of a site's addresses.
const DialTimeoutOverall = 12 * time.Second

// TunHandshakeTimeout — the Android TUN stack's budget for one flow's
// door dial (the SOCKS5 exchange), which waits out the door's own honest
// setup before replying: the DoH resolve (up to DoHTimeout per server,
// three servers by default) plus the upstream dial (DialTimeoutOverall).
// The value is that full budget plus slack: 3×3 s + 12 s ≈ 21 s → 30 s.
// It must NEVER be shorter than the door's budget — a flow aborted here
// mid-setup is reset on the phone while the same flow succeeds on the
// desktop path (which has no such abort), the exact defect M8-n fixed
// (the old 10 s default killed every dial that needed the per-IP walk,
// and the app's retries piled up into the reported minutes-long
// first-video wait). The facade passes this to tunstack; the stack's own
// default carries the same value as the backstop.
const TunHandshakeTimeout = 30 * time.Second

// RequestReadTimeout — how long the browser may take to send its request headers.
const RequestReadTimeout = 10 * time.Second

// FirstRecordReadTimeout — how long the browser may take to start the secure handshake.
const FirstRecordReadTimeout = 10 * time.Second

// FirstByteTimeout — how long a processed host's origin may stay silent
// after the tunnel wrote its first record upstream (M8-s, ADR-0018;
// retuned at M8-t from the debug.12 tuning evidence that report was
// asked to carry). It proved the budget fires exactly as designed —
// "tunnel first-byte failed reason=timeout" dones at ~8.3 s — and that
// every flow it killed was completely silent (down=0 B, and the same
// flows sat silent through the app's full 60 s patience in the previous
// report: dead nodes, not slow ones), while every healthy routed origin
// answered in 420–508 ms (one 2399 ms outlier that also carries the
// ~0.1–0.2 s fragment-pause write before the budget clock starts).
// 2.5 s is ~5x the worst healthy routed wait and still covers that
// outlier, while a silent attempt costs 2.5 s instead of 8 s. Unrouted
// relays keep the plain idle semantics — the budget is part of the
// processing of the hosts this app processes (the D46 scoping), not of
// the relay.
const FirstByteTimeout = 2500 * time.Millisecond

// FirstByteSalvageDials — how many times a processed host's dead
// first-byte origin is re-dialed INSIDE the same flow (M8-t, ADR-0019).
// The debug.12 report gave the per-video start lag its shape: waves
// where roughly half the flows die at first byte (origin resets at
// ~0.5 s, the rest silent to the budget) and the app re-opens wave
// after wave until one lands. A salvage closes only the dead origin,
// re-dials the SAME host with the SAME preset (never a method change —
// per-flow preset variation is D102, Ali's ruling, not ours) and
// replays the client's buffered bytes, so the app never sees the
// failure and its serial retry chain collapses into bounded in-engine
// attempts. Zero in a proxy Config → this default; negative → disabled
// (exactly the M8-s fail-fast behavior).
const FirstByteSalvageDials = 2

// FirstByteRaceDials — how many EXTRA origin candidates a processed
// host's flow races in parallel with its primary dial from the very
// start (M8-w, ADR-0021). The debug.14 report (the first from the
// salvage-race build) proved M8-v works — the retry lines batch at the
// same millisecond (the re-dials race), the stall watch kills at 15 s —
// but named the two costs the serial front still charges: every routed
// flow still walks attempt 0 ALONE through its full budget before the
// race even launches, so a dead primary costs the budget + the race
// (six timeout terminals at 5320–5576 ms = the two-budget chain) and a
// dirty-but-mixed window only recovers at the salvage's 738–848 ms —
// while the SAME report's concurrent flows to the same hosts answered
// at 72–162 ms in the same seconds: the dirt is per-origin, not
// per-moment. Racing the primary against 2 extra candidates from t=0
// (distinct origins via the rotated answer set, the same preset per
// flow — D102 — the client's first record replayed byte-exact through
// each, first origin to speak wins, losers closed and reaped) spends
// the same dial budget the salvage would have spent AFTER a death at
// the moment it can still save the whole chain: a mixed-window flow
// lands at its fastest live origin (~72–500 ms), and an all-dead flow
// terminals at ~1 budget (~2.7 s) instead of 2 (~5.3 s) — the app's
// retry wave, which the report shows recovering at 60–261 ms, starts
// that much sooner. With racing enabled the salvage rounds never run
// (the budget is spent at the start); zero in a proxy Config → this
// default; negative → disabled (the exact M8-v shape: serial attempt 0
// + the raced salvage re-dials).
const FirstByteRaceDials = 2

// DeadIPPenalty — how long an origin IP that connected but never spoke
// (the classified first-byte death: a reset at the hello write, or
// silence to the budget) is SKIPPED when building dial lists (M8-y,
// ADR-0023). The debug.16 report measured the re-gamble this ends: ~17
// reason=timeout terminals at 2892–4304 ms plus ~8 reason=reset at
// ~0.6 s, all down=0 B through the raced round — and the second video's
// start (a NEW rr*.googlevideo hostname) paid the full discovery cost
// AGAIN five terminals deep, because the dead-IP knowledge died with
// each flow while the authoritative DNS answer kept returning the same
// blackholed cache-node IPs. The censor's blackhole is per ORIGIN IP
// (cache nodes shared across rr hostnames), so the penalty outlives the
// flow and crosses hostnames: every classified first-byte death marks
// its origin IP for this long, and every dial list (both doors, the
// raced round's extras, the salvage candidates) skips the marked IPs.
// The box is a preference, never a verdict: a list the filter would
// empty dials in FULL (an all-penalized set gambles exactly as before,
// never an instant fail). NOT marked: stall kills (mid-stream deaths
// are not first-byte evidence — ADR-0022's tiering owns those),
// client-side deaths, teardown-canceled deaths, and dial failures
// (no single-IP evidence). Zero in a proxy Config → this default;
// negative → disabled (the exact M8-x shape: every flow re-gambles the
// whole answer set).
const DeadIPPenalty = 60 * time.Second

// GoodIPSubstitution — the cross-origin escape hatch (M8-ab,
// ADR-0031). The debug.19 report proved the dead-origin pool works but
// only ever inside ONE hostname's rotating answer set: five retry
// waves (auto-timeline elapsed=44999ms pairs=5) re-gambled fresh
// unknown qxau5 IPs while the good table held 24 verified origins no
// dial list ever saw, and the video started only when the app itself
// fell back to a foreign cache hostname. Google's video edge serves
// any *.googlevideo.com origin (one wildcard service behind the
// per-cluster names — the Iranian practice of pinning hand-picked
// working cache IPs into a hosts file rides exactly this), so a flow
// whose own answer set keeps dying may dial a VERIFIED-LIVE origin
// from the good pool instead: the raced round's substitute candidate
// replays the identical fragmented ClientHello — SNI unchanged, the
// client's own hello — onto a pool origin that is not in the current
// answer set and not penalized, freshest verdict first. Substitution
// is scoped to *.googlevideo.com flows only (same-service wildcard;
// the API fronts keep their own answers), the dial count stays flat
// (the substitute replaces one own-set rotation), and a flow whose
// pool bet dies keeps the untried-set salvage exactly as before.
// Flow first-byte SUCCESSES feed the pool too (stronger evidence than
// the probe's synthetic hello — the session's one foreign-cache win
// arms every later flow), and a first-byte DEATH on a pool origin
// drops its good mark and boxes it (the ADR-0028 demote, actually
// implemented — the debug.19 code left the stale mark live). Positive
// in a proxy Config → the per-flow substitute candidate cap; zero →
// this default; negative → disabled (the exact debug.19 shape).
const GoodIPSubstitution = 1

// SubstitutionPoolDials — how many pool origins one substitute
// candidate's dial list may walk (M8-ab, ADR-0031): DialHost staggers
// them Happy-Eyeballs style, so a dead pool origin costs ~750 ms
// before the next starts — inside the raced round's budget, never
// blocking a live winner (the first speaker breaks the collect).
const SubstitutionPoolDials = 3

// GoogleVideoGoodIPsFile — the persisted verified-live googlevideo
// pool (M8-ab, ADR-0033): one IP per line, gvideo-flagged, seeded back
// at engine start with the shorter GoodPoolSeedTTL so the FIRST video
// of session N+1 rides the origins session N verified. Stale seeds
// self-correct — a flow death on a seeded origin drops its mark and
// boxes it like any other. Same privacy footing as the codes file:
// infrastructure addresses in the app's private data directory, never
// a log line (R21).
const GoogleVideoGoodIPsFile = "googlevideo-good-ips.txt"

// GoodPoolSeedTTL — how long a PERSISTED good mark lives when seeded
// back at start (M8-ab, ADR-0033): shorter than OriginProbeGoodTTL —
// yesterday's verdict is stale evidence, and the demote-on-death
// guard bounds the cost of a seed that went bad overnight.
const GoodPoolSeedTTL = 5 * time.Minute

// GoodPoolSaveEvery — how often the engine re-checks whether the
// gvideo good pool changed and needs writing to
// GoogleVideoGoodIPsFile (M8-ab, ADR-0033). The write itself only
// happens on change.
const GoodPoolSaveEvery = 15 * time.Second

// ProbeCodesPerPass — how many learned googlevideo codes ONE
// origin-probe pass covers (M8-ab, ADR-0032): the debug.19 pass
// probed rr1..rr5 of the 2 FRESHEST codes — early in a session that
// is the local cluster twice, so the verified pool could not escape
// the blackhole even in principle. The pass now walks the code store
// round-robin, one rr name per code (rr1..rr5 share the cluster's
// rotating answers), so every remembered code — foreign clusters
// included — is verified within a few passes.
const ProbeCodesPerPass = 12

// ProbeFastStartEvery — the origin-probe pass cadence while the
// engine is young (M8-ab, ADR-0032): passes every 45 s until the
// gvideo good pool carries ProbeFastStartPoolFloor origins or
// ProbeFastStartWindow passes, then the 5-minute prewarm cadence
// takes over. The debug.19 session ran ONE pass in its whole cold
// start; the 45 s window puts fresh verdicts inside the first
// video's retry waves. Android lane only (ADR-0028 decision 5: the
// desktops keep the byte-for-byte M8-z loop).
const ProbeFastStartEvery = 45 * time.Second

// ProbeFastStartWindow — how long the fast-start probe cadence may
// run before settling to the prewarm cadence (M8-ab, ADR-0032).
const ProbeFastStartWindow = 5 * time.Minute

// ProbeFastStartPoolFloor — the gvideo good-pool size that ends the
// fast-start cadence early (M8-ab, ADR-0032).
const ProbeFastStartPoolFloor = 8

// ProbeChurnKickDebounce — the minimum spacing between churn-kicked
// origin-probe passes (M8-ab, ADR-0032): a googlevideo flow that
// terminaled (its raced round AND its untried salvage all dead) bumps
// the gvideoTerminals counter, and the gvideo loop's 1 s watch kicks
// an immediate pass — verified origins land inside the app's next
// retry wave instead of the next 5-minute cycle. The debounce keeps
// a hopeless minute from spending the radio on back-to-back passes.
const ProbeChurnKickDebounce = 10 * time.Second

// StallTimeout — how long a processed host's relay may stay silent in
// BOTH directions before the flow is closed as a mid-stream stall
// (M8-v, ADR-0020). The debug.13 report gave the per-video start lag
// its remaining shape: the salvage absorbed what a re-dial could fix
// (retry lines + salvaged dials=1/2 lines on-device), but five
// video-start flows lived 57–64 s — first byte plus ~7 KB down, then
// total silence — until the APP's own ~60 s patience killed them all
// in the same instant, and the retry wave right after answered in
// 60–261 ms. Nothing before M8-v looked past the first byte: the
// budget owns the handshake, the salvage stops at the first byte, and
// the idle timeout is 300 s, so the engine held the dead flows open
// through the whole minute. A both-directions stall is dead — no relay
// shape the app needs is silent both ways for long (a pure download
// streams down, a pure upload streams up) — so the watch closes both
// ends at the stall budget and the app retries onto a fresh origin
// (with its own budget + salvage) in seconds instead of a minute. One
// direction flowing always refreshes the watch. Zero in a proxy
// Config → this default; negative → disabled (the M8-u behavior
// exactly). Unrouted relays keep the plain idle semantics — the watch,
// like the budget and the salvage before it, is part of the processing
// of the hosts this app processes (the D46 scoping).
const StallTimeout = 15 * time.Second

// StallRichTimeout — the escalated stall budget for a flow that has
// PROVEN itself (M8-x, ADR-0022). The debug.14 build's second report
// (versionCode 13, 2026-10-19T11:43Z) named the M8-v watch's own
// cost: thirteen stall kills in ~70 s of log, every one a connection
// that had ALREADY carried real data — 2.1 MB, 1.6 MB, 723.6 KB,
// 633.7 KB, 162.6 KB down — closed at exactly the 15 s budget while
// the app idled it between segment fetches. The player paces its
// pooled video connections: buffer, quiet for tens of seconds, next
// range ON THE SAME CONNECTION — the exact reuse that made "next"
// instant in the pre-watch builds ("videos used to lag only at
// start; now even going next and back in the same video lags" —
// Ali, that report). A flow that has delivered StallRichBytes down,
// or survived one idle gap and woken (the pacing proof), is closed
// only at this budget and QUIETLY: no answer-set demotion (the
// origin just carried the goods — the opposite of suspect), no error
// count (hygiene, not a dead-origin signal), tier=rich on the stall
// line. The lean tier keeps the exact M8-v semantics for the dead
// cohort's shape: first byte plus headers, never woke, silent to the
// 15 s budget. 90 s bounds the zombie — well past every real pacing
// gap at playback, well under the 300 s idle timeout. Zero in a
// proxy Config → this default; negative (or ≤ StallTimeout) → no
// escalation, every flow keeps the exact M8-v budget and semantics.
const StallRichTimeout = 90 * time.Second

// StallRichBytes — the flow's delivered-down byte count that
// escalates its stall budget to StallRichTimeout (M8-x, ADR-0022):
// past a TLS handshake plus response headers and into real payload.
// The evidence brackets it — the debug.14 build's murdered cohort
// started at 162.6 KB down, the debug.13 report's dead cohort topped
// out at ~7 KB (one 121 KB outlier) — and 32 KB sits between them.
// Zero in a proxy Config → this default; negative → bytes never
// escalate (only a survived idle gap escalates).
const StallRichBytes = 32 * 1024

// IdleTimeout — quiet connections are closed after this long (refreshed by activity).
const IdleTimeout = 300 * time.Second

// MaxConcurrentConnections — above this the proxy answers 503 and closes.
const MaxConcurrentConnections = 1024

// MaxHeaderBytes — request headers are rejected beyond this size.
const MaxHeaderBytes = 16 * 1024

// MaxTLSRecord — largest first message treated as one fragmentation unit (header included).
const MaxTLSRecord = 16*1024 + 5

// BlockPrivateTargets — reject DNS answers in private, loopback and reserved ranges.
const BlockPrivateTargets = true

// StartWithWindows — open the app when Windows starts (the UI asks once; default off).
const StartWithWindows = false

// ConnectOnLaunch — turn on automatically when the app opens (default off).
const ConnectOnLaunch = false

// OpenBrowserOnConnect — open a dedicated Chrome window (its own session,
// pointed at this app's proxy) when the app turns on. This is the old Python
// app's behavior; the new default is off because the spec's approach sets
// the system proxy instead (user-requested option; DECISIONS.md D40).
const OpenBrowserOnConnect = false

// SelfTestURL — what the built-in test fetches through the proxy (expects 204).
const SelfTestURL = "https://www.youtube.com/generate_204"

// SelfTestFallbackURL — used instead when the first URL is unavailable (expects 200).
const SelfTestFallbackURL = "https://www.youtube.com/robots.txt"

// SelfTestVideoHost — a TLS handshake to this host proves the video servers answer.
const SelfTestVideoHost = "redirector.googlevideo.com:443"

// SelfTestDNSName — the name the "secure lookup" step resolves.
const SelfTestDNSName = "www.youtube.com"

// HealStepTimeout — budget per method during an automatic repair sweep.
const HealStepTimeout = 6 * time.Second

// HealSweepCooldown — minimum time between automatic repair sweeps.
const HealSweepCooldown = 10 * time.Minute

// StartTotalTimeout — maximum time from pressing the button to a final answer.
const StartTotalTimeout = 35 * time.Second

// WatchdogInterval — how often the light health check runs while On.
const WatchdogInterval = 120 * time.Second

// WatchdogFailures — consecutive failed checks before a repair sweep starts.
const WatchdogFailures = 3

// NotificationRateLimit — minimum time between failure/recovery notifications.
const NotificationRateLimit = 10 * time.Minute

// NoticeDuration — how long a notice line stays in the state (file 02, section 4.1).
const NoticeDuration = 10 * time.Second

// LogRingLines — how many recent log lines are kept in memory for the diagnostic report.
const LogRingLines = 500

// LogFileSizeLimit — each log file is rotated when it grows past this size.
const LogFileSizeLimit = 2 * 1024 * 1024

// LogFileCount — total log files kept (the live one plus rotated backups).
const LogFileCount = 3

// PrewarmNames — names resolved ahead of time right after Start (optional warm-up).
var PrewarmNames = []string{"www.youtube.com", "youtube.com", "i.ytimg.com", "yt3.ggpht.com"}

// HealSweepOrder — methods tried, in order, by the automatic repair sweep.
var HealSweepOrder = []Method{MethodBalanced, MethodStrong, MethodLight, MethodLegacy}

// defaultDoHServers — secure-DNS services tried in order until one answers.
// M8-z (ADR-0025 §6.3) appends two IP-literal endpoints at the END:
// dialed by IP with no DNS name in the way; Go's TLS client omits SNI
// for IP literals, so the ClientHello carries no name for SNI
// inspection to match, and their certificates carry IP SANs. They are
// the walk's last resorts — the sticky/first servers keep their
// position, and the hedged walk reaches these only when the earlier
// ones stall or fail.
var defaultDoHServers = []DoHServer{
	{
		Name:      "RethinkDNS",
		URL:       "https://sky.rethinkdns.com/?dns={dns}",
		Bootstrap: []string{"172.67.162.27"},
	},
	{
		Name:      "Cloudflare",
		URL:       "https://cloudflare-dns.com/dns-query?dns={dns}",
		Bootstrap: []string{"1.1.1.1", "1.0.0.1"},
	},
	{
		Name:      "Google",
		URL:       "https://dns.google/dns-query?dns={dns}",
		Bootstrap: []string{"8.8.8.8", "8.8.4.4"},
	},
	{
		Name:      "Cloudflare IP",
		URL:       "https://1.1.1.1/dns-query?dns={dns}",
		Bootstrap: []string{"1.1.1.1"},
	},
	{
		Name:      "Google IP",
		URL:       "https://8.8.8.8/dns-query?dns={dns}",
		Bootstrap: []string{"8.8.8.8"},
	},
}

// defaultYouTubeDomains — hosts the PAC script sends through the proxy in "youtube" mode.
var defaultYouTubeDomains = []string{
	"youtube.com",
	"youtu.be",
	"googlevideo.com",
	"ytimg.com",
	"ggpht.com",
	"youtube-nocookie.com",
	"youtubei.googleapis.com",
}

// =======================================================================

// Mode selects which hosts Windows sends through the proxy.
type Mode string

const (
	// ModeYouTube proxies only the YouTube domain list (default).
	ModeYouTube Mode = "youtube"
	// ModeAll proxies every host except localhost and private ranges.
	ModeAll Mode = "all"
	// ModeManual touches nothing; the UI shows the address to enter by hand.
	ModeManual Mode = "manual"
)

// Method selects the fragmentation preset.
type Method string

const (
	// MethodAuto starts with balanced and lets auto-heal choose.
	MethodAuto Method = "auto"
	// MethodLight sends few, large pieces.
	MethodLight Method = "light"
	// MethodBalanced is the default first try.
	MethodBalanced Method = "balanced"
	// MethodStrong sends smaller pieces.
	MethodStrong Method = "strong"
	// MethodLegacy reproduces the old Python rule exactly.
	MethodLegacy Method = "legacy"
)

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool {
	return m == ModeYouTube || m == ModeAll || m == ModeManual
}

// Valid reports whether m is a known method.
func (m Method) Valid() bool {
	return m == MethodAuto || m == MethodLight || m == MethodBalanced ||
		m == MethodStrong || m == MethodLegacy
}

// DoHServer is one secure-DNS service: a display name, a URL template that
// contains {dns} where the encoded query goes, and bootstrap IP addresses used
// to reach the service before DNS itself works.
type DoHServer struct {
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Bootstrap []string `json:"bootstrap"`
}

// Defaults returns the built-in DoH server list (a copy).
func Defaults() []DoHServer {
	out := make([]DoHServer, len(defaultDoHServers))
	for i, s := range defaultDoHServers {
		out[i] = DoHServer{Name: s.Name, URL: s.URL, Bootstrap: append([]string(nil), s.Bootstrap...)}
	}
	return out
}

// DefaultDomains returns the built-in YouTube domain list (a copy).
func DefaultDomains() []string {
	return append([]string(nil), defaultYouTubeDomains...)
}

// DefaultApps returns the built-in per-app allow-list: the YouTube app
// plus the four mainstream browsers (Ali's ruling, DECISIONS.md D87 —
// "Add chrome to the default addlist as well, also firefox, opera,
// brave browser", received in the same feedback round that D86 opened:
// the first ruling put the YouTube app on the list, and this second
// ruling extends it to the browsers, closing D86's open question). The
// list starts with these five and the user adds or removes from there;
// SAVING an empty list still means empty (user agency — the M7
// NO_TARGET_APPS refusal door, D81, owns what happens at connect, never
// a restore-to-default). Entries whose apps are not installed are
// skipped at TUN establish (NameNotFoundException is caught per entry),
// so the five-app default is safe on devices with only some of them.
// A fresh copy every call; never share the slice.
func DefaultApps() []string {
	return []string{
		"com.google.android.youtube",
		"com.android.chrome",
		"org.mozilla.firefox",
		"com.opera.browser",
		"com.brave.browser",
	}
}
