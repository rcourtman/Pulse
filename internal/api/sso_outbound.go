package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

var allowLoopbackSSOFetch bool

const maxSSOFileSize = 4 << 20

func ssoOutboundHTTPOptions(tlsConfig *tls.Config) securityutil.RestrictedOutboundHTTPOptions {
	return securityutil.RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   allowLoopbackSSOFetch,
		TLSConfig:       tlsConfig,
	}
}

func validateSSOFetchURL(ctx context.Context, rawURL string) (*url.URL, error) {
	return securityutil.ValidateOutboundFetchURL(ctx, rawURL, ssoOutboundHTTPOptions(nil))
}

func newSSOHTTPClient(timeout time.Duration, tlsConfig *tls.Config) *http.Client {
	return securityutil.NewRestrictedOutboundHTTPClient(timeout, ssoOutboundHTTPOptions(tlsConfig))
}

func readSSORegularFile(rawPath string) ([]byte, error) {
	absolute, err := normalizeConfiguredSSOFilePath(rawPath)
	if err != nil {
		return nil, err
	}

	return securityutil.ReadSecureStorageFile(absolute, maxSSOFileSize)
}

// normalizeConfiguredSSOFilePath establishes the privileged operator-config
// boundary for SSO certificate and key files. Request data must never reach it.
func normalizeConfiguredSSOFilePath(rawPath string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(rawPath))
	if cleaned == "" || cleaned == "." {
		return "", errors.New("SSO file path is empty")
	}

	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve SSO file path: %w", err)
	}
	return filepath.Clean(absolute), nil
}
