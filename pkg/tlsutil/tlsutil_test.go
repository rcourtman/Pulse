package tlsutil

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFingerprintVerifier_NormalizesFingerprint(t *testing.T) {
	// Fingerprint with colons
	fp1 := "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99"
	// Same fingerprint without colons, lowercase
	fp2 := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	config1 := FingerprintVerifier(fp1)
	config2 := FingerprintVerifier(fp2)

	// Both should have InsecureSkipVerify set (we do our own verification)
	if !config1.InsecureSkipVerify {
		t.Error("FingerprintVerifier should set InsecureSkipVerify to true")
	}
	if !config2.InsecureSkipVerify {
		t.Error("FingerprintVerifier should set InsecureSkipVerify to true")
	}

	// Both should have VerifyPeerCertificate function set
	if config1.VerifyPeerCertificate == nil {
		t.Error("FingerprintVerifier should set VerifyPeerCertificate")
	}
	if config2.VerifyPeerCertificate == nil {
		t.Error("FingerprintVerifier should set VerifyPeerCertificate")
	}
}

func TestFingerprintVerifier_NoCertificates(t *testing.T) {
	config := FingerprintVerifier("aabbccdd")

	// Should fail when no certificates presented
	err := config.VerifyPeerCertificate([][]byte{}, nil)
	if err == nil {
		t.Error("Should fail when no certificates presented")
	}
	if !strings.Contains(err.Error(), "no certificates") {
		t.Errorf("Error message should mention no certificates, got: %v", err)
	}
}

func TestFingerprintVerifier_MatchingFingerprint(t *testing.T) {
	// Create a mock certificate (just random bytes for testing)
	mockCert := []byte("mock certificate data for testing purposes")

	// Calculate its fingerprint
	fingerprint := sha256.Sum256(mockCert)
	expectedFP := hex.EncodeToString(fingerprint[:])

	config := FingerprintVerifier(expectedFP)

	// Should succeed with matching fingerprint
	err := config.VerifyPeerCertificate([][]byte{mockCert}, nil)
	if err != nil {
		t.Errorf("Should succeed with matching fingerprint, got: %v", err)
	}
}

func TestFingerprintVerifier_MismatchedFingerprint(t *testing.T) {
	mockCert := []byte("mock certificate data")

	// Use a different fingerprint
	wrongFP := "0000000000000000000000000000000000000000000000000000000000000000"

	config := FingerprintVerifier(wrongFP)

	// Should fail with mismatched fingerprint
	err := config.VerifyPeerCertificate([][]byte{mockCert}, nil)
	if err == nil {
		t.Error("Should fail with mismatched fingerprint")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("Error message should mention mismatch, got: %v", err)
	}
}

func TestCreateHTTPClient_InsecureMode(t *testing.T) {
	client := CreateHTTPClient(false, "")

	if client == nil {
		t.Fatal("CreateHTTPClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig should be set")
	}

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true in insecure mode")
	}
}

func TestCreateHTTPClient_FingerprintMode(t *testing.T) {
	fingerprint := "aabbccdd"
	client := CreateHTTPClient(false, fingerprint)

	if client == nil {
		t.Fatal("CreateHTTPClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig should be set")
	}

	// Should have custom verification function
	if transport.TLSClientConfig.VerifyPeerCertificate == nil {
		t.Error("VerifyPeerCertificate should be set in fingerprint mode")
	}
}

func TestCreateHTTPClient_SecureMode(t *testing.T) {
	client := CreateHTTPClient(true, "")

	if client == nil {
		t.Fatal("CreateHTTPClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	// In secure mode, TLSClientConfig should either be nil (use defaults)
	// or not have InsecureSkipVerify set
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should not be true in secure mode")
	}
}

func TestCreateHTTPClientWithTimeout_DefaultTimeout(t *testing.T) {
	client := CreateHTTPClientWithTimeout(true, "", 0)

	if client == nil {
		t.Fatal("CreateHTTPClientWithTimeout returned nil")
	}

	// Should use default timeout when 0 is passed
	if client.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", client.Timeout, 60*time.Second)
	}
}

func TestCreateHTTPClientWithTimeout_CustomTimeout(t *testing.T) {
	customTimeout := 30 * time.Second
	client := CreateHTTPClientWithTimeout(true, "", customTimeout)

	if client == nil {
		t.Fatal("CreateHTTPClientWithTimeout returned nil")
	}

	if client.Timeout != customTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, customTimeout)
	}
}

func TestCreateHTTPClientWithTimeout_NegativeTimeout(t *testing.T) {
	client := CreateHTTPClientWithTimeout(true, "", -10*time.Second)

	if client == nil {
		t.Fatal("CreateHTTPClientWithTimeout returned nil")
	}

	// Should use default timeout when negative is passed
	if client.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v (default)", client.Timeout, 60*time.Second)
	}
}

func TestCreateHTTPClient_TransportSettings(t *testing.T) {
	client := CreateHTTPClient(true, "")

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	// Verify transport settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %v, want 100", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 20 {
		t.Errorf("MaxIdleConnsPerHost = %v, want 20", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxConnsPerHost != 20 {
		t.Errorf("MaxConnsPerHost = %v, want 20", transport.MaxConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
	if !transport.DisableCompression {
		t.Error("DisableCompression should be true")
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
}

func TestFingerprintVerifier_IgnoresVerifiedChains(t *testing.T) {
	mockCert := []byte("test certificate")
	fingerprint := sha256.Sum256(mockCert)
	expectedFP := hex.EncodeToString(fingerprint[:])

	config := FingerprintVerifier(expectedFP)

	// verifiedChains parameter should be ignored
	mockChains := [][]*x509.Certificate{{&x509.Certificate{}}}
	err := config.VerifyPeerCertificate([][]byte{mockCert}, mockChains)
	if err != nil {
		t.Errorf("Should ignore verifiedChains, got error: %v", err)
	}
}

func TestGetDNSResolver(t *testing.T) {
	resolver := GetDNSResolver()

	if resolver == nil {
		t.Fatal("GetDNSResolver returned nil")
	}

	// Call again - should return same instance (singleton)
	resolver2 := GetDNSResolver()
	if resolver != resolver2 {
		t.Error("GetDNSResolver should return same instance")
	}
}

func TestFingerprintVerifier_ColonSeparatedFingerprint(t *testing.T) {
	mockCert := []byte("test cert with colons")
	fingerprint := sha256.Sum256(mockCert)

	// Format with colons (common format from openssl)
	fpBytes := fingerprint[:]
	var parts []string
	for _, b := range fpBytes {
		parts = append(parts, hex.EncodeToString([]byte{b}))
	}
	colonSeparated := strings.ToUpper(strings.Join(parts, ":"))

	config := FingerprintVerifier(colonSeparated)

	err := config.VerifyPeerCertificate([][]byte{mockCert}, nil)
	if err != nil {
		t.Errorf("Should handle colon-separated fingerprint, got: %v", err)
	}
}

func TestCreateHTTPClient_FingerprintTakesPrecedence(t *testing.T) {
	// Even if verifySSL is true, fingerprint mode should be used if fingerprint is provided
	fingerprint := "aabbccdd"
	client := CreateHTTPClient(true, fingerprint)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	// Should use fingerprint verification
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig should be set")
	}
	if transport.TLSClientConfig.VerifyPeerCertificate == nil {
		t.Error("Should use fingerprint verification when fingerprint is provided")
	}
}

func TestFingerprintVerifier_TLSVersion(t *testing.T) {
	config := FingerprintVerifier("aabbccdd")

	// Check that config is a valid TLS config
	if config == nil {
		t.Fatal("FingerprintVerifier returned nil config")
	}

	// MinVersion should not be set to an insecure version
	// (default is fine, or TLS 1.2+)
	if config.MinVersion != 0 && config.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion should be 0 (default) or >= TLS 1.2, got %v", config.MinVersion)
	}
}
