package agenttls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// NewClientTLSConfig returns a client TLS config with optional insecure mode
// and optional custom CA bundle support.
func NewClientTLSConfig(caBundlePath string, insecureSkipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if insecureSkipVerify {
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
