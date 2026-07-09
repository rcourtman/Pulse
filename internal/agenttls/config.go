package agenttls

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

// NewClientTLSConfig returns a client TLS config with optional insecure mode
// and optional custom CA bundle support.
func NewClientTLSConfig(caBundlePath string, insecureSkipVerify bool, expectedServerFingerprint string) (*tls.Config, error) {
	expectedServerFingerprint = strings.TrimSpace(expectedServerFingerprint)
	if expectedServerFingerprint != "" {
		normalized := strings.ReplaceAll(expectedServerFingerprint, ":", "")
		decoded, err := hex.DecodeString(normalized)
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("server fingerprint must be a SHA-256 value (64 hexadecimal characters)")
		}
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if expectedServerFingerprint != "" {
		tlsConfig = tlsutil.FingerprintVerifier(expectedServerFingerprint)
		tlsConfig.MinVersion = tls.VersionTLS12
	} else if insecureSkipVerify {
		//nolint:gosec // Insecure mode is explicitly user-controlled.
		tlsConfig.InsecureSkipVerify = true
	}

	caBundlePath = strings.TrimSpace(caBundlePath)
	if caBundlePath == "" {
		return tlsConfig, nil
	}

	caData, err := os.ReadFile(caBundlePath)
	if err != nil {
		return nil, fmt.Errorf("read CA bundle %s: %w", caBundlePath, err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(caData); !ok {
		return nil, fmt.Errorf("CA bundle %s does not contain any certificates", caBundlePath)
	}

	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}
