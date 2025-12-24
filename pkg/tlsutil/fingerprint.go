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
	"net/url"
	"strings"
	"time"
)

// FetchFingerprint connects to a host and returns the SHA256 fingerprint of its TLS certificate.
// This is used for TOFU (Trust On First Use) when discovering cluster peers.
// The host should be in the format "hostname:port" or "https://hostname:port".
func FetchFingerprint(host string) (string, error) {
	// Normalize the host to just host:port format
	targetHost := host
	if strings.HasPrefix(host, "https://") || strings.HasPrefix(host, "http://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return "", fmt.Errorf("failed to parse host URL: %w", err)
		}
		targetHost = parsed.Host
	}

	// Ensure port is present (default to 8006 for Proxmox)
	if _, _, err := net.SplitHostPort(targetHost); err != nil {
		targetHost = targetHost + ":8006"
	}

	// Create a TLS connection with InsecureSkipVerify to fetch the cert
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", targetHost, &tls.Config{
		InsecureSkipVerify: true,
	})
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

// FingerprintVerifier creates a custom TLS config that verifies server certificate fingerprint
func FingerprintVerifier(fingerprint string) *tls.Config {
	// Normalize fingerprint (remove colons, convert to lowercase)
	expectedFingerprint := strings.ToLower(strings.ReplaceAll(fingerprint, ":", ""))

	return &tls.Config{
		InsecureSkipVerify: true, // We'll do our own verification
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by server")
			}

			// Calculate SHA256 fingerprint of the leaf certificate
			fingerprint := sha256.Sum256(rawCerts[0])
			actualFingerprint := hex.EncodeToString(fingerprint[:])

			if actualFingerprint != expectedFingerprint {
				return fmt.Errorf("certificate fingerprint mismatch: expected %s, got %s",
					expectedFingerprint, actualFingerprint)
			}

			return nil
		},
	}
}

// CreateHTTPClient creates an HTTP client with appropriate TLS configuration
func CreateHTTPClient(verifySSL bool, fingerprint string) *http.Client {
	return CreateHTTPClientWithTimeout(verifySSL, fingerprint, 60*time.Second)
}

// CreateHTTPClientWithTimeout creates an HTTP client with appropriate TLS configuration and custom timeout
func CreateHTTPClientWithTimeout(verifySSL bool, fingerprint string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
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
	}

	if !verifySSL && fingerprint == "" {
		// Insecure mode - skip all verification
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if fingerprint != "" {
		// Fingerprint verification mode
		transport.TLSClientConfig = FingerprintVerifier(fingerprint)
	}
	// else: default secure mode with system CA verification

	// Use provided timeout, or default to 60 seconds if not specified
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
