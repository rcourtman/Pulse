package tlsutil

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

const minimumTLSVersion = tls.VersionTLS12

// FingerprintMismatchError identifies a failed local certificate-pin check.
// Provider response text mentioning a mismatch is not TLS trust evidence.
type FingerprintMismatchError struct {
	Expected string
	Actual   string
}

func (e *FingerprintMismatchError) Error() string {
	return fmt.Sprintf("certificate fingerprint mismatch: expected %s, got %s", e.Expected, e.Actual)
}

func verifyPresentedPeerCertificates(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no certificates presented by server")
	}

	for i, rawCert := range rawCerts {
		if _, err := x509.ParseCertificate(rawCert); err != nil {
			return fmt.Errorf("failed to parse peer certificate %d: %w", i, err)
		}
	}
	return nil
}

// UnverifiedPeerCertificateCaptureTLSConfig accepts any parseable peer
// certificate so discovery and TOFU fingerprint workflows can inspect identity
// material before an operator establishes trust.
func UnverifiedPeerCertificateCaptureTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:            minimumTLSVersion,
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyPresentedPeerCertificates,
	}
}

// FetchFingerprint connects to a host and returns the SHA256 fingerprint of its TLS certificate.
// This is used for TOFU (Trust On First Use) when discovering cluster peers.
// Explicit URLs retain their HTTP(S) authority, including the scheme's default
// port. A bare hostname or IP retains the legacy Proxmox default port, 8006.
// Capture is always a TLS handshake, not an HTTP request or a grant of trust.
func FetchFingerprint(host string) (string, error) {
	targetHost, err := fingerprintTarget(host)
	if err != nil {
		return "", fmt.Errorf("failed to parse host URL: %w", err)
	}

	// Create a TLS connection that explicitly accepts parseable peer certificates
	// for fingerprint capture without relying on blanket skip-verify alone.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", targetHost, UnverifiedPeerCertificateCaptureTLSConfig())
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", targetHost, err)
	}
	defer conn.Close()

	// Check context wasn't cancelled
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Get the peer certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates presented by %s", targetHost)
	}

	// Calculate SHA256 fingerprint of the leaf certificate
	fingerprint := sha256.Sum256(certs[0].Raw)
	return hex.EncodeToString(fingerprint[:]), nil
}

func fingerprintTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	explicitURL := strings.Contains(raw, "://")
	if !explicitURL {
		// A port on an IPv6 literal must be bracketed. An entire bare IPv6
		// address (including a zone) is a host, never a guessed host:port pair.
		host, port, splitErr := net.SplitHostPort(raw)
		if splitErr != nil {
			host = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
		}
		if address, err := netip.ParseAddr(host); err == nil && address.Is6() {
			authority := "[" + address.String() + "]"
			if splitErr == nil {
				authority = net.JoinHostPort(address.String(), port)
			}
			// URL.String escapes a zone's percent separator exactly once.
			raw = (&url.URL{Scheme: "https", Host: authority}).String()
		}
	}
	u, err := securityutil.NormalizeHTTPBaseURL(raw, "https")
	if err != nil {
		return "", err
	}
	if !explicitURL && u.Path != "" {
		return "", fmt.Errorf("a base path requires an explicit HTTP(S) URL")
	}
	if strings.HasPrefix(u.Host, "[") {
		if address, err := netip.ParseAddr(u.Hostname()); err != nil || !address.Is6() {
			return "", fmt.Errorf("brackets require a valid IPv6 address")
		}
	} else if strings.Count(u.Host, ":") > 1 {
		return "", fmt.Errorf("an IPv6 URL authority must be bracketed")
	}
	port := u.Port()
	if port == "" {
		if strings.HasSuffix(u.Host, ":") {
			return "", fmt.Errorf("port is empty")
		}
		port = "8006"
		if explicitURL {
			port = "443"
			if u.Scheme == "http" {
				port = "80"
			}
		}
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	return net.JoinHostPort(u.Hostname(), strconv.FormatUint(number, 10)), nil
}

// FingerprintVerifier creates a custom TLS config that verifies server certificate fingerprint
func FingerprintVerifier(fingerprint string) *tls.Config {
	// Normalize fingerprint (remove colons, convert to lowercase)
	expectedFingerprint := strings.ToLower(strings.ReplaceAll(fingerprint, ":", ""))

	return &tls.Config{
		MinVersion:         minimumTLSVersion,
		InsecureSkipVerify: true, // We'll do our own verification
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by server")
			}

			// Calculate SHA256 fingerprint of the leaf certificate
			fingerprint := sha256.Sum256(rawCerts[0])
			actualFingerprint := hex.EncodeToString(fingerprint[:])

			if actualFingerprint != expectedFingerprint {
				return &FingerprintMismatchError{Expected: expectedFingerprint, Actual: actualFingerprint}
			}

			return nil
		},
	}
}

func shouldBypassProxyForTargetHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if normalized == "" {
		return false
	}

	if normalized == "localhost" ||
		strings.HasSuffix(normalized, ".localhost") ||
		strings.HasSuffix(normalized, ".local") {
		return true
	}

	if ip := net.ParseIP(normalized); ip != nil {
		return ip.IsLoopback() ||
			ip.IsPrivate() ||
			ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() ||
			ip.IsUnspecified() ||
			isCarrierGradeNAT(ip)
	}

	return !strings.Contains(normalized, ".")
}

func isCarrierGradeNAT(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

func proxyFromEnvironmentBypassingLocalInfrastructure(req *http.Request) (*url.URL, error) {
	if req != nil && req.URL != nil && shouldBypassProxyForTargetHost(req.URL.Hostname()) {
		return nil, nil
	}
	return http.ProxyFromEnvironment(req)
}

// Infrastructure requests can carry tokens, tickets, CSRF headers or replayable
// login bodies. Go's default redirect policy does not bind those to a port or
// scheme, and may forward some credentials to subdomains. Allow proxy path
// redirects only within the first request's origin, before any redirected dial.
func infrastructureRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if req == nil || len(via) == 0 || via[0] == nil {
		return fmt.Errorf("infrastructure redirects require an original request")
	}
	origin, originErr := infrastructureRedirectOrigin(via[0].URL)
	destination, destinationErr := infrastructureRedirectOrigin(req.URL)
	if originErr != nil || destinationErr != nil || origin != destination {
		return fmt.Errorf("infrastructure redirects must stay on the same origin")
	}
	return nil
}

func infrastructureRedirectOrigin(u *url.URL) (string, error) {
	if u == nil || u.User != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("invalid HTTP origin")
	}
	host := u.Hostname()
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.String() // Canonical IPv6, preserving zone case.
	} else {
		host = strings.ToLower(host)
	}
	port := u.Port()
	if port == "" {
		if strings.HasSuffix(u.Host, ":") {
			return "", fmt.Errorf("port is empty")
		}
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	return u.Scheme + "://" + net.JoinHostPort(host, strconv.FormatUint(number, 10)), nil
}

// CreateHTTPClient creates an HTTP client with appropriate TLS configuration
func CreateHTTPClient(verifySSL bool, fingerprint string) *http.Client {
	return CreateHTTPClientWithTimeout(verifySSL, fingerprint, 60*time.Second)
}

// CreateHTTPClientWithTimeout creates an HTTP client with appropriate TLS configuration and custom timeout
func CreateHTTPClientWithTimeout(verifySSL bool, fingerprint string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: proxyFromEnvironmentBypassingLocalInfrastructure,
		// Performance optimizations for concurrent requests
		MaxIdleConns:        100, // Increase from default 2
		MaxIdleConnsPerHost: 20,  // Increase from default 2
		MaxConnsPerHost:     20,  // Limit concurrent connections per host
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Disable compression for lower latency
		// Use DNS caching to reduce DNS queries
		// This prevents excessive DNS lookups for frequently accessed Proxmox hosts
		DialContext:           DialContextWithCache,
		TLSHandshakeTimeout:   10 * time.Second, // TLS handshake timeout
		ResponseHeaderTimeout: 10 * time.Second, // Time to wait for response headers
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: minimumTLSVersion},
	}

	if !verifySSL && fingerprint == "" {
		// Explicit opt-out mode still requires the peer to present parseable certificates.
		transport.TLSClientConfig = UnverifiedPeerCertificateCaptureTLSConfig()
	} else if fingerprint != "" {
		// Fingerprint verification mode
		transport.TLSClientConfig = FingerprintVerifier(fingerprint)
	}
	// else: default secure mode with system CA verification and explicit TLS floor

	// Use provided timeout, or default to 60 seconds if not specified
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: infrastructureRedirectPolicy,
	}
}
