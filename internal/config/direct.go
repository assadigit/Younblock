// direct.go — the never-processed top-level domains (Ali's ruling,
// DECISIONS.md D83, 2026-10-17): "keep every domain with .ir blocked —
// when opening any domain with .ir, this app must not be used and its
// routes should not work."
//
// Younblock never applies its unblocking machinery to these domains:
// no first-record fragmentation, no http→https redirect, no coverage
// claim — whatever the mode or the user's domain list says. They are
// resolved and dialed direct, byte-transparent, exactly as if the app
// were not running. Desktop youtube mode already behaved this way (the
// PAC sends only the routed list to the app); the ruling extends it to
// every mode and both editions, at all three doors:
//
//   - the PAC script answers DIRECT for them, so a desktop browser
//     never sends them to the app at all (proxy/pac.go);
//   - the HTTP and SOCKS5 doors relay them untouched even when they
//     arrive anyway (a manual system proxy, the Android TUN path) —
//     proxy/server.go's routedHost is the one decision all doors share.
//
// One honest limit, stated in doc 07 §5: on Android the routed apps'
// DNS — .ir lookups included — still resolves through the configured
// DoH providers, because the process has no other resolver (no
// /etc/resolv.conf on Android; ConnectivityManager would need the
// ACCESS_NETWORK_STATE permission the app deliberately does not
// request, D74). The answers are the public DNS tree's; the connection
// itself is direct and untouched.
package config

import "strings"

// directTLDs — the never-processed top-level domains. "ir" is Iran's
// country-code domain: domestic hosts must never depend on, or be
// shaped by, this app.
var directTLDs = []string{"ir"}

// DirectTLDs returns the never-processed TLD list (a copy) — the PAC
// generator and the relay decision share this one truth.
func DirectTLDs() []string {
	return append([]string(nil), directTLDs...)
}

// UnderDirectTLD reports whether host — a domain name, not an IP
// literal — is one of the never-processed TLDs itself or a subdomain
// under one. Case-insensitive; surrounding spaces and the trailing
// root dot are tolerated.
func UnderDirectTLD(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	for _, tld := range directTLDs {
		if host == tld || strings.HasSuffix(host, "."+tld) {
			return true
		}
	}
	return false
}
