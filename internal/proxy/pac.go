package proxy

import (
	"fmt"
	"strings"

	"github.com/assadigit/Younblock/internal/config"
)

// PAC returns the proxy auto-config script served at /proxy.pac
// (file 01, section 8). In youtube mode it sends the YouTube domain list
// through the local proxy and everything else direct; in all mode it sends
// everything except localhost and private ranges. DIRECT is always the
// fallback, so if the app is not running the browser goes direct.
func PAC(port int, domains []string, mode config.Mode) string {
	var b strings.Builder
	b.WriteString("// Younblock proxy auto-config script (generated; do not edit).\n")
	b.WriteString("function FindProxyForURL(url, host) {\n")
	b.WriteString("  host = String(host).toLowerCase();\n")
	switch mode {
	case config.ModeAll:
		b.WriteString("  if (isPlainHostName(host)) { return \"DIRECT\"; }\n")
		b.WriteString("  if (shExpMatch(host, \"*.local\")) { return \"DIRECT\"; }\n")
		b.WriteString("  var resolved = dnsResolve(host);\n")
		b.WriteString("  if (resolved) {\n")
		b.WriteString(`    if (isInNet(resolved, "127.0.0.0", "255.0.0.0")) { return "DIRECT"; }` + "\n")
		b.WriteString(`    if (isInNet(resolved, "10.0.0.0", "255.0.0.0")) { return "DIRECT"; }` + "\n")
		b.WriteString(`    if (isInNet(resolved, "172.16.0.0", "255.240.0.0")) { return "DIRECT"; }` + "\n")
		b.WriteString(`    if (isInNet(resolved, "192.168.0.0", "255.255.0.0")) { return "DIRECT"; }` + "\n")
		b.WriteString(`    if (isInNet(resolved, "169.254.0.0", "255.255.0.0")) { return "DIRECT"; }` + "\n")
		b.WriteString("  }\n")
		fmt.Fprintf(&b, "  return \"PROXY %s:%d; DIRECT\";\n", config.ListenHost, port)
	case config.ModeManual:
		// Manual mode never installs a PAC; if this script is ever fetched
		// it must not send anything through the proxy.
		b.WriteString("  return \"DIRECT\";\n")
	default: // youtube mode
		for _, d := range domains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d == "" {
				continue
			}
			fmt.Fprintf(&b, "  if (host === %q || shExpMatch(host, %q)) { return \"PROXY %s:%d; DIRECT\"; }\n",
				d, "*."+d, config.ListenHost, port)
		}
		b.WriteString("  return \"DIRECT\";\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// MatchDomain reports whether host is domain or a subdomain of it
// (exact match, or ends with "." + domain). Used by the Go-side PAC tests
// and available for diagnostics.
func MatchDomain(host, domain string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

// MatchAnyDomain reports whether host matches any domain in the list.
func MatchAnyDomain(host string, domains []string) bool {
	for _, d := range domains {
		if MatchDomain(host, d) {
			return true
		}
	}
	return false
}
