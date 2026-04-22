package securityutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const requestPlaceholderURL = "http://pulse.invalid"

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	cloned := *u
	return &cloned
}

func joinURLPath(basePath, relativePath string) string {
	parts := []string{basePath}
	if trimmed := strings.Trim(relativePath, "/"); trimmed != "" {
		parts = append(parts, trimmed)
	}

	joined := path.Join(parts...)
	switch joined {
	case ".", "/":
		return ""
	default:
		if strings.HasPrefix(joined, "/") {
			return joined
		}
		return "/" + joined
	}
}

// NormalizeAbsoluteHTTPURL validates a fully-qualified HTTP(S) URL.
func NormalizeAbsoluteHTTPURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URL userinfo is not allowed")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}

	return parsed, nil
}

// NormalizeHTTPBaseURL validates a base HTTP(S) URL and optionally adds a default scheme.
func NormalizeHTTPBaseURL(raw string, defaultScheme string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if defaultScheme != "" && !strings.Contains(trimmed, "://") {
		trimmed = defaultScheme + "://" + trimmed
	}

	parsed, err := NormalizeAbsoluteHTTPURL(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("base URL must not include query or fragment")
	}

	cleanedPath := path.Clean(parsed.Path)
	switch cleanedPath {
	case ".", "/":
		parsed.Path = ""
	default:
		if !strings.HasPrefix(cleanedPath, "/") {
			cleanedPath = "/" + cleanedPath
		}
		parsed.Path = cleanedPath
	}
	parsed.RawPath = ""

	return parsed, nil
}

// IsLoopbackHost reports whether host resolves to localhost or a loopback IP literal.
func IsLoopbackHost(host string) bool {
	normalized := strings.ToLower(strings.Trim(host, "[]"))
	if normalized == "" {
		return false
	}
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}

	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

// NormalizePulseHTTPBaseURL validates a Pulse control-plane base URL.
// HTTPS is required for non-loopback hosts; loopback localhost may use HTTP.
func NormalizePulseHTTPBaseURL(raw string) (*url.URL, error) {
	parsed, err := normalizePulseBaseURL(raw, false)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// NormalizePulseWebSocketBaseURL validates a Pulse command-channel base URL.
// Non-loopback hosts are normalized to WSS; loopback localhost may use WS.
func NormalizePulseWebSocketBaseURL(raw string) (*url.URL, error) {
	parsed, err := normalizePulseBaseURL(raw, true)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func normalizePulseBaseURL(raw string, websocket bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("Pulse URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("Pulse URL %q is invalid: %w", raw, err)
	}
	if parsed.Scheme == "" {
		if websocket {
			return nil, fmt.Errorf("Pulse URL %q must include scheme (https://, wss://, or loopback http:// / ws://)", raw)
		}
		return nil, fmt.Errorf("Pulse URL %q must include scheme (https:// or loopback http://)", raw)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("Pulse URL %q must include host", raw)
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("Pulse URL %q must include host", raw)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("Pulse URL %q must not include user credentials", raw)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("Pulse URL %q must not include query or fragment", raw)
	}

	if port := parsed.Port(); port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("invalid port %q: must be between 1 and 65535", port)
		}
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https":
		if websocket {
			parsed.Scheme = "wss"
		} else {
			parsed.Scheme = "https"
		}
	case "http":
		if !IsLoopbackHost(parsed.Hostname()) {
			if websocket {
				return nil, fmt.Errorf("Pulse URL %q must use https/wss unless host is loopback", raw)
			}
			return nil, fmt.Errorf("Pulse URL %q must use https unless host is loopback", raw)
		}
		if websocket {
			parsed.Scheme = "ws"
		} else {
			parsed.Scheme = "http"
		}
	case "wss":
		if !websocket {
			return nil, fmt.Errorf("Pulse URL %q has unsupported scheme %q", raw, parsed.Scheme)
		}
		parsed.Scheme = "wss"
	case "ws":
		if !websocket {
			return nil, fmt.Errorf("Pulse URL %q has unsupported scheme %q", raw, parsed.Scheme)
		}
		if !IsLoopbackHost(parsed.Hostname()) {
			return nil, fmt.Errorf("Pulse URL %q must use https/wss unless host is loopback", raw)
		}
		parsed.Scheme = "ws"
	default:
		return nil, fmt.Errorf("Pulse URL %q has unsupported scheme %q", raw, parsed.Scheme)
	}

	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed, nil
}

// AppendURLPath appends path segments onto a validated base URL.
func AppendURLPath(base *url.URL, segments ...string) *url.URL {
	cloned := cloneURL(base)
	if cloned == nil {
		return nil
	}

	parts := []string{cloned.Path}
	for _, segment := range segments {
		trimmed := strings.Trim(segment, "/")
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}

	joined := path.Join(parts...)
	if joined == "." || joined == "/" {
		cloned.Path = ""
	} else if strings.HasPrefix(joined, "/") {
		cloned.Path = joined
	} else {
		cloned.Path = "/" + joined
	}
	cloned.RawPath = ""
	cloned.Fragment = ""

	return cloned
}

// ResolveRelativeURL validates a rooted relative path and resolves it against base.
func ResolveRelativeURL(base *url.URL, relativePath string) (*url.URL, error) {
	if base == nil {
		return nil, fmt.Errorf("base URL is required")
	}

	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return nil, fmt.Errorf("relative path is required")
	}
	if strings.Contains(trimmed, `\`) {
		return nil, fmt.Errorf("relative path must not contain backslashes")
	}

	ref, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid relative path: %w", err)
	}
	if ref.IsAbs() || ref.Host != "" || ref.User != nil {
		return nil, fmt.Errorf("relative path must not include scheme or host")
	}
	if !strings.HasPrefix(ref.Path, "/") {
		return nil, fmt.Errorf("relative path must start with '/'")
	}

	cleanedPath := path.Clean(ref.Path)
	if !strings.HasPrefix(cleanedPath, "/") {
		cleanedPath = "/" + cleanedPath
	}
	target := cloneURL(base)
	if target == nil {
		return nil, fmt.Errorf("base URL is required")
	}
	target.Path = joinURLPath(base.Path, cleanedPath)
	escapedPath := path.Clean(ref.EscapedPath())
	if !strings.HasPrefix(escapedPath, "/") {
		escapedPath = "/" + escapedPath
	}
	target.RawPath = joinURLPath(base.EscapedPath(), escapedPath)
	if target.RawPath == target.Path {
		target.RawPath = ""
	}
	target.RawQuery = ref.RawQuery
	target.Fragment = ""
	return target, nil
}

// NewValidatedRequestWithContext builds an HTTP request from a pre-validated URL.
func NewValidatedRequestWithContext(ctx context.Context, method string, target *url.URL, body io.Reader) (*http.Request, error) {
	if target == nil {
		return nil, fmt.Errorf("target URL is required")
	}

	req, err := http.NewRequestWithContext(ctx, method, requestPlaceholderURL, body)
	if err != nil {
		return nil, err
	}
	req.URL = cloneURL(target)
	req.Host = req.URL.Host
	req.RequestURI = ""
	return req, nil
}

// NewRelativeRequestWithContext validates a rooted relative path and builds a request from it.
func NewRelativeRequestWithContext(ctx context.Context, method string, base *url.URL, relativePath string, body io.Reader) (*http.Request, error) {
	target, err := ResolveRelativeURL(base, relativePath)
	if err != nil {
		return nil, err
	}
	return NewValidatedRequestWithContext(ctx, method, target, body)
}
