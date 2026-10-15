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
const AppVersion = "1.6.2"

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

// RequestReadTimeout — how long the browser may take to send its request headers.
const RequestReadTimeout = 10 * time.Second

// FirstRecordReadTimeout — how long the browser may take to start the secure handshake.
const FirstRecordReadTimeout = 10 * time.Second

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
