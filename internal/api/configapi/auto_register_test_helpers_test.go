package configapi

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func stubAutoRegisterNetworkDeps(t *testing.T) {
	t.Helper()

	originalDetect := detectPVEClusterContext
	originalFingerprint := fetchTLSFingerprint
	originalCredential := verifyAutoRegisterCredential

	detectPVEClusterContext = func(context.Context, proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	fetchTLSFingerprint = func(string) (string, error) {
		return "", nil
	}
	verifyAutoRegisterCredential = func(context.Context, string, proxmox.ClientConfig) error {
		return nil
	}

	t.Cleanup(func() {
		detectPVEClusterContext = originalDetect
		fetchTLSFingerprint = originalFingerprint
		verifyAutoRegisterCredential = originalCredential
	})
}

// stubAutoRegisterClusterView stands in for the cluster a registering host
// reports for itself.
func stubAutoRegisterClusterView(t *testing.T, view func(proxmox.ClientConfig) (bool, []config.ClusterEndpoint)) {
	t.Helper()

	original := readAutoRegisterClusterView
	readAutoRegisterClusterView = func(_ context.Context, clientConfig proxmox.ClientConfig, _ string) (bool, []config.ClusterEndpoint) {
		return view(clientConfig)
	}
	t.Cleanup(func() { readAutoRegisterClusterView = original })
}
