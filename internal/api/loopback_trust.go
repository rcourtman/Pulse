package api

import (
	"net"
	"net/http"
	"strings"
)

// loopbackProxyHeaders are request headers that a reverse proxy adds and a
// local client talking straight to Pulse does not send. Any of them means the
// loopback peer may be forwarding someone else's request.
var loopbackProxyHeaders = []string{
	"X-Forwarded-For",
	"Forwarded",
	"X-Real-IP",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Forwarded-Server",
	"X-Forwarded-Port",
	"X-Forwarded-Scheme",
	"Via",
	"CF-Connecting-IP",
	"True-Client-IP",
}

// isDirectLoopbackRequest is the trust rule for a client on the Pulse host
// talking straight to Pulse. It gates no-auth anonymous access, recovery
// tokens and sessions, unauthenticated config transfer and quick setup, and
// the rate-limit exemption.
//
// A loopback peer is not enough on its own. The request must also:
//   - carry no reverse-proxy forwarding headers;
//   - name a local Host (localhost, *.localhost or a loopback address), so a
//     same-host proxy passing on a public Host, or a page whose name was
//     re-resolved to 127.0.0.1, is not mistaken for a local client;
//   - not come from another origin's page in a browser on the Pulse host.
//
// A same-host proxy that rewrites Host to a loopback address and adds no
// forwarding headers still cannot be told apart from a local client, so such
// a proxy must add X-Forwarded-For (see docs/REVERSE_PROXY.md).
func isDirectLoopbackRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	ip := net.ParseIP(extractRemoteIP(req.RemoteAddr))
	if ip == nil || !ip.IsLoopback() {
		return false
	}

	for _, header := range loopbackProxyHeaders {
		if _, present := req.Header[http.CanonicalHeaderKey(header)]; present {
			return false
		}
	}

	if !isLocalRequestHost(req.Host) {
		return false
	}

	return isSameOriginOrNonBrowserRequest(req)
}

// isSameOriginOrNonBrowserRequest is the browser half of loopback trust. A
// browser's Sec-Fetch-Site cannot be set by page script, so "same-origin" and
// "none" (typed or bookmarked) are accepted as they are. Any other request that
// names a browser origin must name exactly this one, reads and WebSocket
// upgrades included: a page on another local port is same-site but must not
// borrow loopback trust, and a WebSocket is not held back by CORS. A client
// that sends no browser metadata at all, such as curl, is accepted.
func isSameOriginOrNonBrowserRequest(req *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(req.Header.Get("Sec-Fetch-Site"))) {
	case "same-origin", "none":
		return true
	case "":
		return browserOriginMetadataMatches(req, requestOrigin(req), true)
	default:
		return browserOriginMetadataMatches(req, requestOrigin(req), false)
	}
}

// isLocalRequestHost reports whether a Host header names this machine without
// relying on DNS: localhost, a *.localhost name (browsers resolve these to
// loopback themselves), or a loopback IP literal.
func isLocalRequestHost(raw string) bool {
	host, ok := validateRequestHost(raw)
	if !ok {
		return false
	}
	name := strings.TrimSuffix(strings.ToLower(host.hostname), ".")
	if name == "localhost" || strings.HasSuffix(name, ".localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}
