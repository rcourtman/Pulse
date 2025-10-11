package tlsutil

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

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
		// Add specific timeouts for DNS, TLS handshake, and response headers
		// These prevent hanging on DNS resolution or TLS negotiation
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
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
