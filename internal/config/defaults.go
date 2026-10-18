// Package config holds Younblock's defaults and the user settings file.
//
// Everything an owner may want to tune lives in the CONFIGURATION block below,
// each constant with a one-line plain-English comment (file 01, section 4).
package config

import "time"

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
