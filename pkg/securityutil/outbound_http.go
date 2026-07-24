package securityutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultRestrictedRedirectLimit = 10

// RestrictedOutboundHTTPOptions controls outbound HTTP validation and transport policy.
type RestrictedOutboundHTTPOptions struct {
	AllowedSchemes  []string
	AllowPrivateIPs bool
	AllowLoopback   bool
	// ResponseHeaderTimeout bounds the wait for response headers without
	// imposing an overall deadline on a streaming response body.
	ResponseHeaderTimeout time.Duration
	TLSConfig             *tls.Config
	ResolveIPAddrs        func(ctx context.Context, host string) ([]net.IPAddr, error)
}

var resolveOutboundFetchIPs = net.DefaultResolver.LookupIPAddr

func allowedOutboundSchemes(opts RestrictedOutboundHTTPOptions) []string {
	if len(opts.AllowedSchemes) == 0 {
		return []string{"http", "https"}
	}
	return opts.AllowedSchemes
}

func isAllowedOutboundScheme(scheme string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), scheme) {
			return true
		}
	}
	return false
}

func validateOutboundIP(ip net.IP, opts RestrictedOutboundHTTPOptions) error {
	if ip == nil {
		return fmt.Errorf("invalid IP address")
	}
	if ip.IsLoopback() && !opts.AllowLoopback {
		return fmt.Errorf("loopback addresses are not allowed")
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return fmt.Errorf("metadata service address is not allowed")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local addresses are not allowed")
	}
	if ip.IsMulticast() {
		return fmt.Errorf("multicast addresses are not allowed")
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified addresses are not allowed")
	}
	if !opts.AllowPrivateIPs && ip.IsPrivate() {
		return fmt.Errorf("private addresses are not allowed")
	}
	return nil
}

func resolveOutboundIPAddrs(ctx context.Context, host string, opts RestrictedOutboundHTTPOptions) ([]net.IPAddr, error) {
	if resolver := opts.ResolveIPAddrs; resolver != nil {
		return resolver(ctx, host)
	}
	return resolveOutboundFetchIPs(ctx, host)
}

// resolvePermittedOutboundIPs resolves host and returns every permitted IP in
// resolution order. Callers that dial must try each returned IP: a host like
// "localhost" can resolve to ::1 first while the service listens only on
// 127.0.0.1, so pinning the first permitted IP alone turns an address-family
// mismatch into a hard connection failure.
func resolvePermittedOutboundIPs(ctx context.Context, host string, opts RestrictedOutboundHTTPOptions) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}

	switch strings.ToLower(host) {
	case "metadata.google.internal", "metadata.goog":
		return nil, fmt.Errorf("metadata service host is not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if err := validateOutboundIP(ip, opts); err != nil {
			return nil, err
		}
		return []net.IP{ip}, nil
	}

	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	resolveCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()

	addrs, err := resolveOutboundIPAddrs(resolveCtx, host, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("hostname %s did not resolve", host)
	}

	var permitted []net.IP
	var blockedErr error
	for _, addr := range addrs {
		if err := validateOutboundIP(addr.IP, opts); err != nil {
			blockedErr = err
			continue
		}
		permitted = append(permitted, addr.IP)
	}
	if len(permitted) > 0 {
		return permitted, nil
	}

	if blockedErr != nil {
		return nil, fmt.Errorf("hostname %s resolves only to blocked addresses: %w", host, blockedErr)
	}
	return nil, fmt.Errorf("hostname %s did not resolve", host)
}

func resolvePermittedOutboundIP(ctx context.Context, host string, opts RestrictedOutboundHTTPOptions) (net.IP, error) {
	ips, err := resolvePermittedOutboundIPs(ctx, host, opts)
	if err != nil {
		return nil, err
	}
	return ips[0], nil
}

// ValidateOutboundFetchURL validates a fully-qualified HTTP(S) URL against the restricted outbound policy.
func ValidateOutboundFetchURL(ctx context.Context, raw string, opts RestrictedOutboundHTTPOptions) (*url.URL, error) {
	parsed, err := NormalizeAbsoluteHTTPURL(raw)
	if err != nil {
		return nil, err
	}

	allowedSchemes := allowedOutboundSchemes(opts)
	if !isAllowedOutboundScheme(parsed.Scheme, allowedSchemes) {
		return nil, fmt.Errorf("URL scheme must be one of: %s", strings.Join(allowedSchemes, ", "))
	}
	if parsed.Fragment != "" {
		return nil, fmt.Errorf("URL fragments are not allowed")
	}

	if _, err := resolvePermittedOutboundIP(ctx, parsed.Hostname(), opts); err != nil {
		return nil, err
	}

	return parsed, nil
}

func canonicalOriginHost(u *url.URL) string {
	if u == nil {
		return ""
	}

	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if host == "" || port == "" {
		return strings.ToLower(u.Host)
	}
	return net.JoinHostPort(host, port)
}

func sameOriginRedirectPolicy(opts RestrictedOutboundHTTPOptions) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if len(via) >= defaultRestrictedRedirectLimit {
			return fmt.Errorf("stopped after %d redirects", defaultRestrictedRedirectLimit)
		}

		validated, err := ValidateOutboundFetchURL(req.Context(), req.URL.String(), opts)
		if err != nil {
			return err
		}

		origin := via[0].URL
		if !strings.EqualFold(validated.Scheme, origin.Scheme) || canonicalOriginHost(validated) != canonicalOriginHost(origin) {
			return fmt.Errorf("redirects must stay on the same origin")
		}
		return nil
	}
}

func cloneRestrictedTransport(opts RestrictedOutboundHTTPOptions) *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	var clone *http.Transport
	if ok && transport != nil {
		clone = transport.Clone()
	} else {
		clone = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}

	switch {
	case opts.TLSConfig != nil:
		clone.TLSClientConfig = opts.TLSConfig.Clone()
	case clone.TLSClientConfig != nil:
		clone.TLSClientConfig = clone.TLSClientConfig.Clone()
	default:
		clone.TLSClientConfig = &tls.Config{}
	}

	if clone.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		clone.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
	if opts.ResponseHeaderTimeout > 0 {
		clone.ResponseHeaderTimeout = opts.ResponseHeaderTimeout
	}

	return clone
}

// restrictedRoundTripper wraps an underlying transport and validates the
// request URL hostname against the restricted outbound policy before
// forwarding. This provides defense-in-depth that works even when an HTTP
// proxy is configured: the proxy handles the actual connection, but the
// target host is still validated here so that SSRF targets (e.g. cloud
// metadata service addresses) cannot be reached through the proxy.
type restrictedRoundTripper struct {
	base http.RoundTripper
	opts RestrictedOutboundHTTPOptions
}

func (r *restrictedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}
	if _, err := resolvePermittedOutboundIP(req.Context(), host, r.opts); err != nil {
		return nil, err
	}
	return r.base.RoundTrip(req)
}

// NewRestrictedOutboundHTTPClient returns an HTTP client that validates redirects and pins direct outbound dials
// to the permitted resolved IPs for the requested host, trying each in resolution order until one connects.
func NewRestrictedOutboundHTTPClient(timeout time.Duration, opts RestrictedOutboundHTTPOptions) *http.Client {
	transport := cloneRestrictedTransport(opts)
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("parse outbound address %q: %w", addr, err)
		}

		permittedIPs, err := resolvePermittedOutboundIPs(ctx, host, opts)
		if err != nil {
			return nil, err
		}

		// Try every permitted IP from the validating resolution, not just the
		// first: "localhost" commonly resolves to ::1 ahead of 127.0.0.1, and a
		// service bound only to one loopback family (e.g. Ollama on 127.0.0.1)
		// would otherwise be unreachable even though curl and browsers connect
		// fine. Each candidate was validated above, so rebinding protection is
		// unchanged.
		dialer := net.Dialer{Timeout: 10 * time.Second}
		var dialErr error
		for _, permittedIP := range permittedIPs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(permittedIP.String(), port))
			if err == nil {
				return conn, nil
			}
			dialErr = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, dialErr
	}

	client := &http.Client{
		Transport:     &restrictedRoundTripper{base: transport, opts: opts},
		CheckRedirect: sameOriginRedirectPolicy(opts),
	}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}
