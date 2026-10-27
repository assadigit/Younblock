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

// AppVersion — shown in the UI and reported by --version. 1.6.12 rides
// the lite desktop edition's beauty round (D115): the same one-exe,
// zero-dependency shell, its screen lifted to the v3 main screen itself
// — the state gradients, the key-cap lever, the live timer/stats and
// the menu sheet, redrawn natively (GDI+ shapes, GDI text, nothing to
// install). The engine itself is unchanged — the bump keeps the About
// line in lockstep with the APK's versionName (the build.gradle.kts
// comment block records why). NOTE: the lite exe's version-info
// resource (cmd/younblock-lite/rsrc_windows_*.syso) reads this
// constant — scripts/gen_lite_resources.py regenerates it.
const AppVersion = "1.6.12"

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
// verifies at most (M8-aa, ADR-0028; M8-ae, ADR-0041 cuts 48→10):
// the pass's every job is a FULL TLS handshake with a real
// googlevideo SNI under the live (legacy) preset — 48 of them is a
// scan burst, and the tester's two reference implementations send
// ZERO background handshakes ever (backend.py, xray TLSFrag,
// WebImmortalGuards: fragment + DoH, nothing else). 10 keeps the
// rehabilitation sample (4 boxed IPs) plus ~6 fresh verdicts as a
// slow trickle on the GoogleVideoProbeEvery cadence; the flows'
// own first bytes are the pool's real food (flow-fed marks,
// ADR-0031). The value is ALSO a hard clamp inside ProbeOrigins —
// a caller passing a larger budget cannot re-create the burst.
const OriginProbePerPass = 10

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
// marks on the same origin IP (M8-aa, ADR-0028; M8-ac rebalances the
// tiers for the flow-killing regime, ADR-0034). The debug.18 report
// measured the re-gamble the escalation ended; the debug.20 report
// measured the opposite failure: with the censor killing connections
// temporally, the one-shot dead-origin probe ANSWERED on penalized
// IPs — a 30-minute boxing of a live origin (the third-strike tier)
// is indefensible when the death evidence says nothing about the IP.
// The tiers now cap how long even a repeat offender is skipped
// within one hostile window — 60 s, 2 min, 5 min — and the
// rehabilitation sampling (ProbeRehabPerPass) re-verifies boxed IPs
// so a wrong box clears on evidence, not on expiry. Never a refusal:
// an all-penalized list still dials in full.
var DeadIPStrikeEscalation = [3]time.Duration{60 * time.Second, 2 * time.Minute, 5 * time.Minute}

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
// pool (M8-ab, ADR-0033): one entry per line, seeded back at engine
// start with the shorter GoodPoolSeedTTL so the FIRST video of
// session N+1 rides the origins session N verified. M8-ah (ADR-0047)
// implements ADR-0045's flip condition — the per-line format is now
// versioned: a line is "<ip> media" (this build's media-verified
// writes: the network test's answered verdicts AND payload-delivered
// flows) or a bare "<ip>" (a file written by a pre-debug.24 round,
// first-byte-certified — the false-positive class W6 removed). Only
// the media lines seed the substitution POOL; bare lines stay
// preferences exactly as before. Stale seeds self-correct — a flow
// death on a seeded origin drops its mark and boxes it like any
// other. Same privacy footing as the codes file: infrastructure
// addresses in the app's private data directory, never a log line
// (R21).
const GoogleVideoGoodIPsFile = "googlevideo-good-ips.txt"

// GoogleVideoDeadClustersFile — the persisted dead-cluster marks
// (M8-an, ADR-0053, doc 11 §5.3's pre-registered fix): one sn-cluster
// key per line, the clusters whose dead marks were LIVE when the last
// session wrote the file, seeded back at engine start with the shorter
// GvideoClusterSeedTTL. The debug.30 field report convicted the
// per-session amnesia: the connect-time hand-off opened a cold-started
// YouTube app into a second-zero tunnel whose cluster memory was
// EMPTY — the app's ladder burned first-byte terminals on the
// blackholed cluster (the ~2.5 s dial-accept-TLS-silent budget) before
// three distinct hosts marked it dead, exactly the terminal burn the
// breaker's arm gate sums across clusters. A seeded mark makes the
// door fail those hosts INSTANTLY from second zero (the reset the
// app's own ladder escalates on, §4.2), and the seed's cost is bounded
// three ways: the shorter TTL, the per-cluster re-probe
// (GvideoClusterReprobeEvery — a lifted blackhole is learned inside
// one window), and the own-dial rehabilitation (noteGvideoClusterAlive
// clears a wrong seed the moment one of its origins answers). Same
// privacy footing as the pool file: cache-cluster tokens in the app's
// private data directory, never a log line (R21).
const GoogleVideoDeadClustersFile = "googlevideo-dead-clusters.txt"

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

// GoogleVideoProbeEvery — the origin-probe pass cadence (M8-ae,
// ADR-0041): decoupled from the 5-minute DoH prewarm because a
// probe pass is googlevideo-SNI WIRE VOLUME and the prewarm is
// resolver-only (no googlevideo packet ever leaves the phone).
// The reference implementations send zero background handshakes;
// the pass here is a rehab-first trickle (ProbeRehabPerPass boxed
// IPs + a few fresh verdicts, capped by OriginProbePerPass) at a
// cadence slow enough that its emission is noise next to the
// app's own flows. The START-of-session pass is gone entirely —
// the persisted good pool (ADR-0033) and flow-fed marks (ADR-0031)
// own the cold start; a fresh engine must be SILENT on googlevideo
// until the first flow speaks.
const GoogleVideoProbeEvery = 15 * time.Minute

// ProbeRehabPerPass — how many currently-BOXED origin IPs one
// origin-probe pass re-verifies (M8-ac, ADR-0034): the debug.20
// report's smoking gun was penalized=12 while the one-shot
// dead-origin probe said those same IPs ANSWER — under the
// flow-killing regime a death says nothing stable about the IP, so
// the box fills with stale verdicts and dial lists skip live
// origins. Each pass samples this many boxed IPs (soonest-expiry
// first): an "answered" verdict clears the box entry and marks the
// IP good (counted as a rehabilitation); anything else refreshes it.
const ProbeRehabPerPass = 4

// GvideoBreakerTerminals — how many googlevideo flow terminals
// within GvideoBreakerWindow arm the emission breaker (M8-ae,
// ADR-0041): a terminal is a flow whose raced round AND salvage
// all died — under the flag regime that is 3+ failed
// hello-fanouts in half a minute, each one refreshing the very
// flag that killed it. The breaker's answer is silence, not more
// evidence. (This knob REPLACES the M8-ab churn kick, whose probe
// pass responded to googlevideo failure by sending MORE
// googlevideo volume — exactly backwards under a censor that
// flags scan-like sources.)
const GvideoBreakerTerminals = 3

// GvideoBreakerWindow — the window the arming terminals must fall
// inside (M8-ae, ADR-0041).
const GvideoBreakerWindow = 30 * time.Second

// GvideoBreakerCooldown — how long the breaker keeps the engine
// SILENT on googlevideo after arming (M8-ae, ADR-0041; M8-ag,
// ADR-0046 re-tuned): flows fail fast at the tunnel door (zero
// wire), probes stay paused, and the app's player does the one thing
// the reference implementations rely on — re-fetch and rotate to a
// NEW hostname. The cooldown's quiet lets a per-source SNI flag
// decay; door-blocks create no terminals, so the breaker
// self-extinguishes and service resumes on the next flow. Jitter of
// up to a quarter of the cooldown (capped at 10 s) staggers
// re-arming. Non-gvideo hosts are untouched — the feed keeps
// refreshing through the cooldown. The debug.24 field report set the
// budget: 45 s outlived the player's own retry patience — the last
// googlevideo attempt landed at +46 s, ONE cooldown covered the
// whole window (58 door failures), and the video never opened while
// the feed kept working. 15 s still spans the app's measured 2-7 s
// retry cadence (2-6 blocked rotations per cooldown) but can never
// outlive the player; the pool escape and the scout (M8-ag) bound
// the blindness further.
const GvideoBreakerCooldown = 15 * time.Second

// GvideoClusterDeadHosts — how many DISTINCT googlevideo hosts of one
// cache cluster silent-terminaling inside GvideoClusterDeadWindow
// mark the CLUSTER dead (M8-ah, ADR-0047). The cluster is the sn-XXXX
// token of the hostname: the debug.25 report's whole sn-qxau5 set
// (btqs/btqy/btqd/btql across rr3..rr7) accepted every TCP dial and
// never answered a TLS byte — 21 flows, 3 s each, all session long —
// while sn-aj4g55, sn-u14g55 and sn-4g5ednkd answered instantly and
// carried 1.8 MB / 559 KB. A censor that blackholes cache nodes by IP
// range kills the cluster, not the source: the dead-origin probe
// confirmed it (the penalized qxau5 IPs stayed silent under every
// SNI). Cluster memory turns that measurement into a door decision.
const GvideoClusterDeadHosts = 3

// GvideoClusterDeadWindow — the window the dead-marking hosts must
// fall inside (M8-ah, ADR-0047).
const GvideoClusterDeadWindow = 10 * time.Minute

// GvideoClusterDeadTTL — how long a dead cluster stays dead without
// rehabilitation (M8-ah, ADR-0047): an origin of the cluster
// answering on its OWN dial clears the mark at once, and the door
// lets one probe flow through every GvideoClusterReprobeEvery, so the
// TTL is only the worst-case recovery bound.
const GvideoClusterDeadTTL = 10 * time.Minute

// GvideoClusterReprobeEvery — how often the door lets ONE flow
// through to a dead-marked cluster (M8-ah, ADR-0047): a blackhole can
// lift, and without a probe the fast-fail would never learn — the
// probe bounds the stale mark's cost at one dial budget per cluster
// per window.
const GvideoClusterReprobeEvery = 2 * time.Minute

// GvideoClusterSeedTTL — how long a PERSISTED dead-cluster mark lives
// when seeded back at start (M8-an, ADR-0053): half
// GvideoClusterDeadTTL — the last session's blackhole measurement is
// stale evidence, and the re-probe (GvideoClusterReprobeEvery) plus
// the own-dial rehab bound a seed that went stale long before this
// TTL runs out.
const GvideoClusterSeedTTL = 5 * time.Minute

// WarmGateSettle — how long the engine must HOLD a serving phase
// before its readiness re-check runs (M8-an, ADR-0052): the
// connect-time hand-off's gate. The debug.30 field report showed the
// hand-off firing at second zero — the earliest instant the stack
// exists — so a cold-started YouTube app burst into a chain still
// paying its own per-flow establishment; the manual flow it replaced
// carried seconds of human navigation the tunnel warmed through. The
// settle window restores that delay under the engine's control; the
// re-check that follows it is the "sure" half.
const WarmGateSettle = 3 * time.Second

// WarmGateRetry — the re-check cadence while the readiness check
// keeps failing (M8-an, ADR-0052): one light check (listener, dns,
// youtube — the watchdog's own blessed emission classes, never
// googlevideo) every window until it passes or the cap fires.
const WarmGateRetry = 2 * time.Second

// WarmGateCap — the never-trap bound on the readiness gate (M8-an,
// ADR-0052): whatever the checks say, the gate opens by this age —
// a late hand-off is recoverable (the user taps YouTube), a trapped
// one is not (the M8-am refusal-path rule).
const WarmGateCap = 15 * time.Second

// PayloadVerifiedMinimum — the downstream bytes a googlevideo flow
// must have DELIVERED before its origin is certified media-verified
// (M8-ah, ADR-0047): the pool's automatic food. The debug.23
// false-positive pool was fed by handshake-only wins whose flows
// carried ≤12.1 KB; real media flows measured 406 KB, 559 KB and
// 1.8 MB (the debug.25 report's scout-carried videos). 64 KB sits 5x
// above the false class and 6x below the smallest real flow — an
// origin that moved that much answer payload carries media, no
// synthetic probe needed. A wrong mark self-corrects through the
// same demote-on-death guard every pool entry carries.
const PayloadVerifiedMinimum = 64 * 1024

// ResetIPPenalty — how long an origin IP that RESET a flow's
// first-byte attempt is skipped in dial lists (M8-ac, ADR-0034): a
// WITHIN-WAVE skip, not an IP verdict. The debug.20 report measured
// the regime change this answers: the censor moved from IP
// blackholing (the dead-origin probe used to say silent) to killing
// connections in flight — 35 of 77 probe verdicts came back reset,
// the one-shot dead-origin probe ANSWERED on both SNI variants from
// penalized IPs, and every flow-death mark therefore became stale the
// moment it landed. A reset is now evidence about ONE connection,
// not the origin: the next wave may well get a full response from
// the same IP, so the box skips it only long enough to spread one
// retry wave's candidates elsewhere.
const ResetIPPenalty = 20 * time.Second

// OriginProbeResetTTL — the box TTL a probe verdict of "reset" writes
// (M8-ac, ADR-0034): the probe's own fresh reset is the same temporal
// signal a flow's reset death is, so it boxes briefly — unlike the
// probe's "silent" verdict (a fresh connection given a full 2.5 s
// budget and staying blackholed — still the strong per-IP evidence
// it always was, OriginProbeDeadTTL).
const OriginProbeResetTTL = 60 * time.Second

// BrownoutDialThreshold — how many DoH connection replacements within
// BrownoutWindow trip the DoH brownout (M8-ac, ADR-0035): the
// debug.20 report's doh line read dials 59 (was 6 thirteen hours
// earlier) — the connection-killer was murdering the resolver's warm
// HTTP/2 conns faster than they could serve. While the brownout is
// live the engine's BACKGROUND lanes (the googlevideo prewarm, the
// origin-probe passes, the churn kicks) pause — the app's own flows
// keep the resolver busy enough — until BrownoutQuiet passes without
// a new dial.
const BrownoutDialThreshold = 3

// BrownoutWindow — the rolling window the DoH dial count is read
// over (M8-ac, ADR-0035).
const BrownoutWindow = 60 * time.Second

// BrownoutQuiet — how long the DoH lane must stay dial-quiet before
// the brownout lifts and the background lanes resume (M8-ac,
// ADR-0035).
const BrownoutQuiet = 60 * time.Second

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

// StallRichPendingTimeout — M8-ad (ADR-0037): a PROVEN (rich) flow whose
// client has a request OUTSTANDING (it spoke after the origin's last
// byte) is closed at this budget, not StallRichTimeout. The tester's
// post-debug.21 report named the wedge exactly: "minute 5 of a 10-minute
// video goes loading, and as soon as you refresh it fixes" — the
// player's pooled video connection dies mid-life, the player's next
// range request rides the dead conn unanswered, and the 90 s rich
// hygiene budget held the app hostage far past its own patience; the
// REFRESH closed everything and a fresh connection (through the race +
// substitution machinery that works) recovered in ~2 s. A pending rich
// kill is definite dead-with-work evidence — it demotes the good mark
// like the lean pending kill (M8-ac) but stays QUIET (no error count,
// tier=rich cause=pending on the line). 10 s bounds the wedge at
// playback: far past any live origin's response latency on a proven
// conn, far under the 90 s hygiene. Zero in a proxy Config → this
// default; negative (or ≥ StallRichTimeout) → disabled, pending rich
// kills wait the full rich budget (the exact M8-x/M8-ac semantics).
const StallRichPendingTimeout = 10 * time.Second

// OriginKeepAliveIdle — M8-ad (ADR-0037): TCP keepalive idle period on
// every ORIGIN socket the engine dials (netx applies it post-dial). The
// connection-killing regime the debug.20/21 reports measured — resets
// and silent blackholes mid-life — leaves a dead pooled conn
// undetectable to the stall watch (both directions quiet looks EXACTLY
// like a healthy pooled conn) until the 90 s rich budget or the 300 s
// idle timeout; Go's default keepalive probes 9 times at 15 s (~2 h).
// Tuned probes surface a blackholed peer as a read error in ~15 s +
// 3 × 5 s, and the relay then closes the app's side — the app retries
// on a fresh origin in seconds. Zero (with Interval zero) → the Go
// default (since 1.23, 15 s interval, OS-default count) — deliberately
// conservative, not off.
const OriginKeepAliveIdle = 15 * time.Second

// OriginKeepAliveInterval — the gap between keepalive probes once the
// idle period lapses (M8-ad, ADR-0037). See OriginKeepAliveIdle.
const OriginKeepAliveInterval = 5 * time.Second

// OriginKeepAliveCount — probes without an ACK before the kernel marks
// the conn dead (M8-ad, ADR-0037): 3 × 5 s after a 15 s idle ≈ a
// blackholed origin surfacing within ~30 s. Zero → the OS default (9
// on Linux).
const OriginKeepAliveCount = 3

// TrickleWindow — M8-ad (ADR-0037): the rolling window the trickle
// detector watches on a rich flow's delivered-down bytes. The regime's
// other mid-life weapon is the per-connection THROTTLE: the conn stays
// open, bytes keep arriving, and the stall watch never fires because
// the down stamp keeps refreshing — the player's buffer drains at
// minute ~5 while the connection crawls, and only a REFRESH (a fresh
// conn at full speed) recovers it. TCP delivery of a range response is
// line-rate bursty (the player paces REQUESTS, not bytes), so a rich
// flow whose window delivered > 0 but < TrickleMinBytes sustained is
// being policed, not slow: the kill closes both ends quietly (no
// demote — policing follows the CONNECTION, and a fresh conn to the
// same origin runs at full speed; no error count) and the player's
// retry lands on a fresh conn through the race machinery. Zero in a
// proxy Config → this default; negative → the detector is off.
const TrickleWindow = 12 * time.Second

// TrickleMinBytes — the byte floor a rich flow must deliver inside
// TrickleWindow to count as alive (M8-ad, ADR-0037). 128 KB over 12 s
// ≈ 11 KB/s — under the bitrate of even 144p playback as DELIVERY
// (responses stream at line rate), and an order under the 1080p+ the
// tester actually watches. Zero → this default; negative → off.
const TrickleMinBytes = 128 * 1024

// TrickleFloorBytes — the delivered-down total that arms the trickle
// detector (M8-ad, ADR-0037): below it a flow is still proving itself
// and slow bytes are the lean tier's own subject. 1 MB sits past the
// first media segments into steady-state playback. Zero → this
// default; negative → off.
const TrickleFloorBytes = 1024 * 1024

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
// M8-ad (ADR-0039): legacy leads. The regime the debug.20/21 reports
// measured — resets on recognizably-shaped hellos — punishes weak
// splitting, and legacy (the old Python app's 299-cut rule, the shape
// the tester's own network proved for years) is the strongest shape we
// ship; auto STARTS there now, so the sweep walks down the strength
// ladder (legacy → strong → balanced → light) instead of saving its
// strongest card for last.
var HealSweepOrder = []Method{MethodLegacy, MethodStrong, MethodBalanced, MethodLight}

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
