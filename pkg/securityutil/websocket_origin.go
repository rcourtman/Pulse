package securityutil

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// NormalizeWebSocketOriginHost normalizes Origin/Host values for same-origin comparison.
func NormalizeWebSocketOriginHost(host string) string {
	normalized := strings.TrimSpace(strings.ToLower(host))
	if normalized == "" {
		return normalized
	}

	parsedHost, parsedPort, err := net.SplitHostPort(normalized)
	if err != nil {
		return normalized
	}
	if parsedPort == "80" || parsedPort == "443" {
		return parsedHost
	}
	return net.JoinHostPort(parsedHost, parsedPort)
}

// SameHostWebSocketOrigin validates that an Origin header is http(s) and matches the request host.
func SameHostWebSocketOrigin(origin string, requestHost string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	return NormalizeWebSocketOriginHost(parsed.Host) == NormalizeWebSocketOriginHost(requestHost)
}

// HTTPOriginForWebSocketBaseURL returns the http(s) Origin header for a Pulse websocket base URL.
func HTTPOriginForWebSocketBaseURL(raw string) (string, error) {
	parsed, err := NormalizePulseWebSocketBaseURL(raw)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	default:
		return "", fmt.Errorf("unsupported websocket origin scheme %q", parsed.Scheme)
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}
