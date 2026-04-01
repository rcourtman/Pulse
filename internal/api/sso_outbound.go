package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

var allowLoopbackSSOFetch bool

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
	cleaned := filepath.Clean(strings.TrimSpace(rawPath))
	if cleaned == "" {
		return nil, errors.New("SSO file path is empty")
	}

	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return nil, fmt.Errorf("resolve SSO file path: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SSO file must be a regular file")
	}

	return os.ReadFile(absolute)
}
