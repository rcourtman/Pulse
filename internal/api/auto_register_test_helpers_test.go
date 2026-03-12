package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func stubAutoRegisterNetworkDeps(t *testing.T) {
	t.Helper()

	originalDetect := detectPVECluster
	originalFingerprint := fetchTLSFingerprint

	detectPVECluster = func(proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	fetchTLSFingerprint = func(string) (string, error) {
		return "", nil
	}

	t.Cleanup(func() {
		detectPVECluster = originalDetect
		fetchTLSFingerprint = originalFingerprint
	})
}
