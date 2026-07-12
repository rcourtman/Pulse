package monitoring

import (
	"strings"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestConvertHostPackageUpdateStatusPreservesTypedInventoryWithoutAliasing(t *testing.T) {
	now := time.Now().UTC()
	input := &agentshost.PackageUpdateStatus{
		Supported: true, Manager: " apt ", InventoryHash: " sha256:" + strings.Repeat("a", 64) + " ", PendingCount: 2, CheckedAt: now.Add(24 * time.Hour), RebootRequired: true,
		Packages: []agentshost.PackageUpdate{{Name: " openssl ", InstalledVersion: " 1.0 ", AvailableVersion: " 1.1 "}},
	}
	got := convertHostPackageUpdateStatus(input, now)
	if got == nil || !got.Supported || got.Manager != "apt" || got.InventoryHash != "sha256:"+strings.Repeat("a", 64) || got.PendingCount != 2 || !got.RebootRequired || !got.CheckedAt.Equal(input.CheckedAt) || !got.ObservedAt.Equal(now) {
		t.Fatalf("status = %#v", got)
	}
	if len(got.Packages) != 1 || got.Packages[0].Name != "openssl" || got.Packages[0].InstalledVersion != "1.0" || got.Packages[0].AvailableVersion != "1.1" {
		t.Fatalf("packages = %#v", got.Packages)
	}
	input.Packages[0].Name = "mutated"
	if got.Packages[0].Name != "openssl" {
		t.Fatalf("converted package state aliases report input: %#v", got.Packages)
	}
}

func TestConvertHostStorageCleanupStatusPreservesAgentCheckedAndServerObservedTimes(t *testing.T) {
	now := time.Now().UTC()
	input := &agentshost.StorageCleanupStatus{
		Supported: true, Provider: " apt-package-cache ", Fingerprint: " sha256:" + strings.Repeat("b", 64) + " ", ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now.Add(48 * time.Hour), Error: " ",
	}
	got := convertHostStorageCleanupStatus(input, now)
	if got == nil || !got.Supported || got.Provider != "apt-package-cache" || got.Fingerprint != "sha256:"+strings.Repeat("b", 64) || got.ReclaimableBytes != 512*1024*1024 || !got.CheckedAt.Equal(input.CheckedAt) || !got.ObservedAt.Equal(now) || got.Error != "" {
		t.Fatalf("status = %#v", got)
	}
}
