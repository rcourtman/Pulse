package api

import "sync"

// ResetTrustedProxyConfigForTests resets the trusted proxy configuration.
// This must be called after setting PULSE_TRUSTED_PROXY_CIDRS env var.
func ResetTrustedProxyConfigForTests() {
	trustedProxyCIDRs = nil
	trustedProxyOnce = sync.Once{}
}
