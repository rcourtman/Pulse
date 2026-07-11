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
	if got == nil || !got.Supported || got.Manager != "apt" || got.InventoryHash != "sha256:"+strings.Repeat("a", 64) || got.PendingCount != 2 || !got.RebootRequired || !got.CheckedAt.Equal(now) {
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
