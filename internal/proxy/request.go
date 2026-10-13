package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// parseRequest reads one request from br (headers capped by the caller's
// limited reader). It uses the real http.ReadRequest parser (file 03, R11)
// instead of splitting raw bytes.
func parseRequest(br *bufio.Reader) (*http.Request, error) {
	return http.ReadRequest(br)
}

// routeKind is what the proxy decided to do with a request.
type routeKind int

const (
	routeConnect routeKind = iota
	routeRedirect
	routePAC
	routeHealth
	routeNotFound
	routeMethodNotAllowed
)

// route names the decision for a parsed request.
type route struct {
	kind   routeKind
	target string // host:port for CONNECT (port defaulted)
	https  string // the https URL for redirects
}

// decide routes one request (file 01, section 7 step 2). port is the local
// listening port, used to check the Host of local-service requests.
func decide(req *http.Request, port int) route {
	if req.Method == http.MethodConnect {
		target := req.URL.Host
		if target == "" {
			target = req.Host
		}
		// Default the port to 443 when missing (net.SplitHostPort errors then).
		if _, _, err := net.SplitHostPort(target); err != nil {
			target = target + ":443"
		}
		return route{kind: routeConnect, target: target}
	}
	// Absolute-form http:// URL: redirect to the https:// version.
	if req.URL.IsAbs() && req.URL.Scheme == "http" &&
		(req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodPost) {
		https := *req.URL
		https.Scheme = "https"
		return route{kind: routeRedirect, https: https.String()}
	}
	if req.Method == http.MethodGet {
		switch req.URL.Path {
		case "/proxy.pac", "/health":
			if isLocalHost(req.Host, port) {
				if req.URL.Path == "/proxy.pac" {
					return route{kind: routePAC}
				}
				return route{kind: routeHealth}
			}
			return route{kind: routeNotFound}
		}
	}
	return route{kind: routeMethodNotAllowed}
}

// isLocalHost reports whether an authority (Host header) is the local
// listener: 127.0.0.1:<port>, localhost:<port> or [::1]:<port>.
func isLocalHost(authority string, port int) bool {
	host, portStr, err := net.SplitHostPort(authority)
	if err != nil {
		return false
	}
	if portStr != fmt.Sprintf("%d", port) {
		return false
	}
	host = strings.ToLower(host)
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// drainBuffered returns the bytes bufio has already buffered past the request
// (the client may have sent early data) and empties br.
func drainBuffered(br *bufio.Reader) []byte {
	n := br.Buffered()
	if n == 0 {
		return nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(br, buf); err != nil {
		return nil
	}
	return buf
}
